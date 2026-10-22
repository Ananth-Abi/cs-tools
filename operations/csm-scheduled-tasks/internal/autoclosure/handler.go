// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package autoclosure

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// Stage names, as entity-service spells them.
const (
	StageReleaseHold    = "release_hold"
	StageClose          = "close"
	StageSecondReminder = "second_reminder"
	StageFirstReminder  = "first_reminder"
)

// stageOrder is the order one run works through the stages. Holds are released
// first so a ticket whose hold just ended is judged this run. The later stages
// then go furthest-along first: the daily budget is shared, and a ticket
// waiting to be closed matters more than one waiting for its first reminder. No
// ticket moves two stages in one run (each advance changes its step and time,
// so it is no candidate for the next stage until a full interval has passed).
var stageOrder = []string{StageReleaseHold, StageClose, StageSecondReminder, StageFirstReminder}

// RunConfig is what the workflow is run with. None of it is business logic, only
// the knobs: the intervals, the daily limit, which states are switched on and
// the comment wording.
type RunConfig struct {
	// IntervalDays is how long each stage waits. 0 lets entity-service use its
	// default (7).
	IntervalDays int
	// DailyLimit caps reminders and closures per UTC day across all stages. 0
	// means no cap. entity-service enforces it from its audit table, so a run
	// that is retried the same day cannot go past it.
	DailyLimit int
	// States restricts the workflow to awaiting_info and/or solution_proposed.
	// Empty means both. Solution Proposed is meant to run alone for a while
	// before Awaiting Info is switched on.
	States []string
	// Templates overrides the comment wording, keyed "<state>.<kind>".
	Templates map[string]string
}

type workClient interface {
	Candidates(ctx context.Context, req CandidatesRequest) ([]Candidate, error)
	Advance(ctx context.Context, id string, req AdvanceRequest) (Outcome, error)
}

// maxErrorsReported bounds the failure alert email: a run that fails for
// thousands of tickets must not produce thousands of lines.
const maxErrorsReported = 10

// Run returns a registry.Task.Handler that works through every stage once.
//
// One ticket's failure never stops the others. Each stage's list is asked for
// again after the previous stage's tickets moved, and entity-service takes what
// today has already done off the daily budget, so the handler is safe to run
// again after a failure (the engine retries it): tickets already advanced are no
// longer due, and the limit holds across the retries.
//
// It returns an error when any listing or any ticket failed, so the engine's
// alert names the problem and the next tick retries only what is still due.
func Run(client workClient, cfg RunConfig) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var (
			errs                                           []error
			evaluated, reminded, closed, released, skipped int
			failed                                         int
		)
		for _, stage := range stageOrder {
			if err := ctx.Err(); err != nil {
				errs = append(errs, err)
				break
			}
			candidates, err := client.Candidates(ctx, CandidatesRequest{
				Stage: stage, IntervalDays: cfg.IntervalDays, DailyLimit: cfg.DailyLimit, States: cfg.States,
			})
			if err != nil {
				failed++
				errs = append(errs, fmt.Errorf("list tickets due for %s: %w", stage, err))
				continue
			}
			for _, c := range candidates {
				evaluated++
				outcome, err := client.Advance(ctx, c.ID, AdvanceRequest{
					Stage: stage, IntervalDays: cfg.IntervalDays, States: cfg.States, Templates: cfg.Templates,
				})
				switch {
				case err != nil:
					failed++
					slog.WarnContext(ctx, "autoclosure: stage failed for a ticket", "stage", stage, "ticket", c.Number, "id", c.ID, "err", err)
					errs = append(errs, fmt.Errorf("%s %s: %w", stage, c.Number, err))
				case outcome != OutcomeAdvanced:
					skipped++
				case stage == StageClose:
					closed++
				case stage == StageReleaseHold:
					released++
				default:
					reminded++
				}
			}
		}

		slog.InfoContext(ctx, "autoclosure: run complete",
			"evaluated", evaluated, "reminded", reminded, "closed", closed,
			"holdsReleased", released, "skipped", skipped, "failed", failed)

		if len(errs) == 0 {
			return nil
		}
		shown := errs
		if len(shown) > maxErrorsReported {
			shown = append(append([]error{}, errs[:maxErrorsReported]...),
				fmt.Errorf("... and %d more failures", len(errs)-maxErrorsReported))
		}
		return errors.Join(shown...)
	}
}

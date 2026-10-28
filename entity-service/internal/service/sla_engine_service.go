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

package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacondition"
)

// SLAEngineService is this service's own in-process SLA clock engine: it
// registers, completes, pauses, resumes and cancels source='CSM' "sla"
// rows (migration 0134's own original enum intent -- see
// repository.SLAEngineRepository's own doc comment for why this engine's
// own clocks must never carry the 'SERVICENOW' tag, which belongs to a
// real, independent, continuously-running external sync job) in reaction
// to case-lifecycle events, sourcing real durations AND
// real applicability from the actual, ServiceNow-synced, source='SERVICENOW'
// sla_policy rows (decoded by migrations 0250-0258) -- evaluated via
// internal/slacondition against the case's own current field values
// (slaPolicyResolver, repository.CaseFacts), not a hardcoded severity map
// and not a name-keyed lookup against CSM-only rows (that mechanism, and the
// 16 rows backing it, are gone -- migration 0259; see git history before
// that migration, and sla_policy_resolver.go's own doc comment, for what
// this replaces).
//
// Every method is called directly, in-process, from snCaseService's own
// case-lifecycle hooks (sn_case_service.go) -- never over HTTP, and never
// gated on Event Hub/publisher being configured, same reasoning the old
// design gave for being independent of s.publisher: a deployment with no
// Event Hub configured must not lose SLA tracking as a side effect.
//
// Every method is best-effort: a failure here is logged and never returned
// to the caller as an error, because none of this may ever fail case
// creation, comment creation, or a state-changing PATCH -- the case/comment
// mutation itself has already succeeded (in ServiceNow) by the time any of
// these run, and treating an SLA-tracking hiccup as a failed case mutation
// would be strictly worse than tracking nothing.
type SLAEngineService interface {
	// RegisterCaseClocks resolves and registers a new source='CSM'
	// "sla" row for every clock target (RESPONSE/WORKAROUND/RESOLUTION) whose
	// current real sla_policy start_condition matches the case -- called
	// once, from CreateCase, right after a case is created. projectID is
	// accepted for call-site compatibility but not used directly: it's
	// folded into the case's own CaseFacts (project type) by a fresh
	// database read, same as every other fact. severity may be nil (a
	// real, if rare, production state -- see domain.CaseView.Severity's
	// own doc comment on confirmed null rates); when given, it overrides
	// whatever a fresh read of the case's own severity would show (see
	// loadCaseFacts' own doc comment for why an override is needed at
	// all) -- several real policies (ServiceRequest SLA, the Query/
	// SecReport Analysis family) have no severity condition at all, so a
	// nil severity no longer means "register nothing" the way it used to.
	RegisterCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, projectID string)
	// ReviseCaseClocks cancels every existing active clock for the case and
	// registers an entirely fresh set matching its NEW severity, atomically
	// (see repository.SLAEngineRepository.ReviseClocks) -- called from
	// UpdateCase when an EXISTING case's severity changes (see
	// RegisterCaseClocks's own doc comment: registration only ever runs
	// once, from CreateCase, so a case re-severitized after creation would
	// otherwise keep running its already-registered clocks against the
	// original severity's resolved policies forever).
	//
	// Per explicit product direction, the new clocks have NO relation to the
	// old ones: no revised-in-place policy/duration, no carried-over
	// start_on or elapsed time. The old clocks run into a terminal CANCELLED
	// state (per-target, see repository.SLAEngineRepository.ReviseClocks'
	// own doc comment) and the new ones start from zero, exactly as if the
	// case had just been created at the new severity/facts.
	ReviseCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, projectID string)
	// CompleteResponseClock marks the case's CSM-authored "response" clock
	// ACHIEVED -- called from CreateCaseComment when the new comment
	// qualifies as the case's first substantive support-engineer reply (see
	// sn_case_service.go's applyResponseSLAOnComment-equivalent hook for the
	// exact qualification check, ported from the old design). This is also
	// the one existing signal repository.CaseFacts.CSEngineerResponseComment
	// is derived from -- see that field's own doc comment.
	CompleteResponseClock(ctx context.Context, caseID string)
	// CompleteWorkaroundClock marks the case's CSM-authored "workaround"
	// clock ACHIEVED -- called when a case's WorkaroundProvided is set to
	// true (the "Provide Workaround" action), the one genuine "workaround
	// was provided" signal that exists anywhere in the domain model.
	CompleteWorkaroundClock(ctx context.Context, caseID string)
	// CompleteFixEtaSharedClocks marks the case's CSM-authored "workaround"
	// AND "resolution" clocks ACHIEVED -- called when a PATCH shares a fix
	// ETA with the customer (req.AddPublicComment true alongside a fix-ETA
	// date, the webapp's "Share fix ETA with customer" action; ServiceNow
	// data source only -- see case_service.go's own UpdateCase rejection
	// list, AddPublicComment has no Postgres equivalent). Once WSO2 has
	// committed a fix timeline to the customer, neither clock has anything
	// further to track.
	CompleteFixEtaSharedClocks(ctx context.Context, caseID string)
	// ApplyCaseStateEffects evaluates the case's CURRENTLY ACTIVE clocks
	// (RESPONSE/WORKAROUND/RESOLUTION) against their OWN bound sla_policy's
	// stop_condition/cancel_condition/pause_condition/resume_condition --
	// the real, decoded conditions (migrations 0250-0258), not a hardcoded
	// switch on state. Replaces the old design's applyCaseStateSLAEffects
	// port (a fixed per-state pause/resume/complete-on-CLOSED switch): most
	// real policies' own stop_condition already includes "stateINCLOSED" (or
	// similar), so closing a case naturally completes its clocks through
	// this same generic evaluation, with no special-cased CLOSED branch
	// needed here any more.
	//
	// For each clock target with a currently open (non-final-stage) clock:
	//  1. stop_condition true -> CompleteClock (a genuine completion, e.g.
	//     a reply went out, or the case closed).
	//  2. else cancel_condition true -> CancelClock (the policy that was
	//     backing this clock no longer applies at all, e.g. the case's type
	//     changed away from what it required).
	//  3. else pause_condition true -> SetPaused(true).
	//  4. else resume_condition true -> SetPaused(false).
	//  5. else: left exactly as it is. A policy with no pause_condition at
	//     all (every real RESPONSE policy) is simply never paused by this
	//     method, matching the old design's "RESPONSE is never paused"
	//     behavior for free, as a consequence of the real data rather than
	//     a special case written here.
	// A clock already in a genuine completion/cancellation stage is not
	// matched by ActiveClockPolicy's own open-stage filter at all, so
	// calling this unconditionally on every state-changing PATCH is safe.
	ApplyCaseStateEffects(ctx context.Context, caseID string, state domain.CaseState)
}

type slaEngineService struct {
	resolver *slaPolicyResolver
	repo     repository.SLAEngineRepository
}

// NewSLAEngineService constructs an SLAEngineService backed by the given
// repository.
func NewSLAEngineService(repo repository.SLAEngineRepository) SLAEngineService {
	return &slaEngineService{resolver: newSLAPolicyResolver(repo), repo: repo}
}

// loadCaseFacts reads caseID's current CaseFacts fresh from Postgres and
// applies the two optional overrides callers may already know more
// authoritatively than a fresh read would: severityOverride (an in-flight
// severity a ServiceNow dual-write case's own Postgres mirror may not have
// caught up to yet) and stateOverride (ApplyCaseStateEffects is always
// handed the case's brand new state directly from the PATCH that triggered
// it, for the exact same reason). Every other fact -- case type, project
// type, product, workaround/fix-ETA/account presence, creation date, and
// whether a CS engineer has already replied -- comes from the fresh read:
// none of those are ever passed in by a caller today.
func (s *slaEngineService) loadCaseFacts(ctx context.Context, caseID string, severityOverride *domain.CaseSeverity, stateOverride *domain.CaseState) (repository.CaseFacts, error) {
	facts, err := s.repo.GetCaseFacts(ctx, caseID)
	if err != nil {
		return repository.CaseFacts{}, err
	}
	if severityOverride != nil {
		if label, ok := slaSeverityLabel[*severityOverride]; ok {
			facts.Severity = label
		}
	}
	if stateOverride != nil {
		if label, ok := slaStateLabel[*stateOverride]; ok {
			facts.State = label
		}
	}
	return facts, nil
}

// resolveApplicablePolicies resolves, for every clock target (slaTargetOrder:
// RESPONSE, WORKAROUND, RESOLUTION), the one real sla_policy whose
// start_condition currently matches facts -- shared by RegisterCaseClocks
// and ReviseCaseClocks so both derive the same "what should this case's
// clocks look like" answer the same way.
//
// The second return, lookupFailed, distinguishes two very different reasons
// a clock target can be missing from the returned slice: slaPolicyResolver.
// resolve's own doc comment explains why a genuinely-unmatched target (no
// error) and a failed lookup (an error, e.g. a database blip) both drop
// that target from the slice the same way, but callers must NOT treat them
// the same. RegisterCaseClocks (case creation) safely ignores lookupFailed
// -- nothing existing is at risk, and a missing clock target there is
// retried the next time this case is touched. ReviseCaseClocks must check
// it: proceeding to ReviseClocks with a policy list that's incomplete
// because of a lookup failure (not because no policy genuinely applies)
// would cancel the case's existing clocks and commit no replacement for the
// target that failed to resolve.
func (s *slaEngineService) resolveApplicablePolicies(ctx context.Context, facts repository.CaseFacts) (policies []repository.SLAPolicyRef, lookupFailed bool) {
	policies = make([]repository.SLAPolicyRef, 0, len(slaTargetOrder))
	for _, clockType := range slaTargetOrder {
		target := slaClockTypeTarget[clockType]
		policy, ok, err := s.resolver.resolve(ctx, target, facts)
		if err != nil {
			// resolve already logged why -- this target's policy couldn't
			// be determined right now, not that none applies.
			lookupFailed = true
			continue
		}
		if !ok {
			// resolve already logged why -- skipping this target is the
			// same "no applicable policy" behavior the old slaDurations
			// map's absent map entries had.
			continue
		}
		policies = append(policies, policy)
	}
	return policies, lookupFailed
}

// RegisterCaseClocks implements SLAEngineService.
func (s *slaEngineService) RegisterCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, _ string) {
	facts, err := s.loadCaseFacts(ctx, caseID, severity, nil)
	if err != nil {
		slog.ErrorContext(ctx, "sla engine: register clocks: load case facts failed", "caseId", caseID, "err", err)
		return
	}
	// lookupFailed deliberately ignored here -- see resolveApplicablePolicies'
	// own doc comment: nothing existing is at risk at case-creation time, a
	// missing clock target just means one target doesn't get registered
	// this time.
	policies, _ := s.resolveApplicablePolicies(ctx, facts)
	if len(policies) == 0 {
		slog.InfoContext(ctx, "sla engine: not registering clocks, no applicable policy matched", "caseId", caseID)
		return
	}
	for _, policy := range policies {
		registered, err := s.repo.RegisterClock(ctx, caseID, policy)
		if err != nil {
			slog.ErrorContext(ctx, "sla engine: register clock failed", "caseId", caseID, "clockType", policy.Target, "policy", policy.Name, "err", err)
			continue
		}
		if !registered {
			slog.InfoContext(ctx, "sla engine: clock already registered, skipped", "caseId", caseID, "clockType", policy.Target)
		}
	}
}

// ReviseCaseClocks implements SLAEngineService.
func (s *slaEngineService) ReviseCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, _ string) {
	facts, err := s.loadCaseFacts(ctx, caseID, severity, nil)
	if err != nil {
		slog.ErrorContext(ctx, "sla engine: revise clocks: load case facts failed", "caseId", caseID, "err", err)
		return
	}
	policies, lookupFailed := s.resolveApplicablePolicies(ctx, facts)
	if lookupFailed {
		// Do NOT proceed to ReviseClocks with an incomplete policy list --
		// see resolveApplicablePolicies' own doc comment. Leaving the case's
		// existing clocks completely untouched (stale, but intact) is safer
		// than cancelling them and committing no replacement for the target
		// whose policy lookup failed.
		slog.ErrorContext(ctx, "sla engine: revise clocks skipped, a policy lookup failed", "caseId", caseID)
		return
	}
	// Bounded retry (2 attempts, short pause between): ReviseClocks is one
	// atomic transaction (see its own doc comment), so retrying it is safe
	// -- a second attempt after a failed first one just repeats the same
	// cancel-then-register-fresh transaction, it never duplicates rows.
	var cancelled int
	for attempt := 1; attempt <= 2; attempt++ {
		cancelled, err = s.repo.ReviseClocks(ctx, caseID, policies)
		if err == nil || attempt == 2 {
			break
		}
		slog.WarnContext(ctx, "sla engine: revise clocks failed, retrying once", "caseId", caseID, "err", err)
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		// Atomic: a failure here means NOTHING changed -- the case's prior
		// clocks are exactly as they were.
		slog.ErrorContext(ctx, "sla engine: revise clocks for severity change failed after retry", "caseId", caseID, "err", err)
		return
	}
	if cancelled > 0 {
		slog.InfoContext(ctx, "sla engine: cancelled active clocks for severity change", "caseId", caseID, "count", cancelled)
	}
}

// CompleteResponseClock implements SLAEngineService.
func (s *slaEngineService) CompleteResponseClock(ctx context.Context, caseID string) {
	if _, err := s.repo.CompleteClock(ctx, caseID, slaClockTypeTarget[slaClockTypeResponse]); err != nil {
		slog.ErrorContext(ctx, "sla engine: complete response clock failed", "caseId", caseID, "err", err)
	}
}

// CompleteWorkaroundClock implements SLAEngineService.
func (s *slaEngineService) CompleteWorkaroundClock(ctx context.Context, caseID string) {
	if _, err := s.repo.CompleteClock(ctx, caseID, slaClockTypeTarget[slaClockTypeWorkaround]); err != nil {
		slog.ErrorContext(ctx, "sla engine: complete workaround clock failed", "caseId", caseID, "err", err)
	}
}

// CompleteFixEtaSharedClocks implements SLAEngineService.
func (s *slaEngineService) CompleteFixEtaSharedClocks(ctx context.Context, caseID string) {
	if _, err := s.repo.CompleteClock(ctx, caseID, slaClockTypeTarget[slaClockTypeWorkaround]); err != nil {
		slog.ErrorContext(ctx, "sla engine: complete workaround clock on fix eta shared failed", "caseId", caseID, "err", err)
	}
	if _, err := s.repo.CompleteClock(ctx, caseID, slaClockTypeTarget[slaClockTypeResolution]); err != nil {
		slog.ErrorContext(ctx, "sla engine: complete resolution clock on fix eta shared failed", "caseId", caseID, "err", err)
	}
}

// ApplyCaseStateEffects implements SLAEngineService.
//
// A case transitioning to CLOSED always force-completes every clock still
// open on it, regardless of what that clock's own policy's stop_condition
// says -- a safety net on TOP of (never instead of) the condition-based
// completion below, not a replacement for it. Before the condition-driven
// engine, this method unconditionally completed every clock on a CLOSED
// transition; that blanket behavior is gone now that completion is
// per-policy-condition, and while every real case policy's stop_condition
// this codebase has been able to confirm does reach a CLOSED-matching clause
// (migration 0254/0256's own stop_condition values all do), a future synced
// policy whose stop_condition simply doesn't name CLOSED would otherwise
// leave its clock running forever on an already-closed case with nobody
// able to notice -- a stuck clock is a worse failure mode than a redundant
// CompleteClock call, so this net applies unconditionally rather than only
// after confirming every policy's text by hand. CompleteClock/CancelClock
// are themselves idempotent against a clock that isn't open (their own
// slaEngineOpenStageFilter scope), so calling CompleteClock here for a
// target this same pass already completed or cancelled is a harmless no-op
// -- still skipped via `settled` purely to avoid a pointless second query.
func (s *slaEngineService) ApplyCaseStateEffects(ctx context.Context, caseID string, state domain.CaseState) {
	facts, err := s.loadCaseFacts(ctx, caseID, nil, &state)
	if err != nil {
		slog.ErrorContext(ctx, "sla engine: apply case state effects: load case facts failed", "caseId", caseID, "state", state, "err", err)
		return
	}
	f := caseConditionFacts{c: facts}

	for _, clockType := range slaTargetOrder {
		target := slaClockTypeTarget[clockType]
		policy, ok, err := s.repo.ActiveClockPolicy(ctx, caseID, target)
		if err != nil {
			slog.ErrorContext(ctx, "sla engine: apply case state effects: active clock policy lookup failed", "caseId", caseID, "clockType", target, "err", err)
			continue
		}
		if !ok {
			continue // no open clock of this target -- nothing to evaluate
		}

		settled := false
		switch {
		case policy.StopCondition != "" && slacondition.Parse(policy.StopCondition).Evaluate(f):
			if _, err := s.repo.CompleteClock(ctx, caseID, target); err != nil {
				slog.ErrorContext(ctx, "sla engine: apply case state effects: complete clock failed", "caseId", caseID, "clockType", target, "err", err)
			}
			settled = true
		case policy.CancelCondition != "" && slacondition.Parse(policy.CancelCondition).Evaluate(f):
			if _, err := s.repo.CancelClock(ctx, caseID, target); err != nil {
				slog.ErrorContext(ctx, "sla engine: apply case state effects: cancel clock failed", "caseId", caseID, "clockType", target, "err", err)
			}
			settled = true
		case policy.PauseCondition != "" && slacondition.Parse(policy.PauseCondition).Evaluate(f):
			if _, err := s.repo.SetPaused(ctx, caseID, target, true); err != nil {
				slog.ErrorContext(ctx, "sla engine: apply case state effects: pause clock failed", "caseId", caseID, "clockType", target, "err", err)
			}
		case policy.ResumeCondition != "" && slacondition.Parse(policy.ResumeCondition).Evaluate(f):
			if _, err := s.repo.SetPaused(ctx, caseID, target, false); err != nil {
				slog.ErrorContext(ctx, "sla engine: apply case state effects: resume clock failed", "caseId", caseID, "clockType", target, "err", err)
			}
		default:
			// Neither pause nor resume condition is configured/matches: left
			// exactly as it is -- see this type's own ApplyCaseStateEffects doc
			// comment, case 5.
		}

		if !settled && state == domain.CaseStateClosed {
			if _, err := s.repo.CompleteClock(ctx, caseID, target); err != nil {
				slog.ErrorContext(ctx, "sla engine: apply case state effects: safety-net complete clock on CLOSED failed", "caseId", caseID, "clockType", target, "err", err)
			}
		}
	}
}

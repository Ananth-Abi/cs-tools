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
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	autoclosureDefaultIntervalDays = 7
	autoclosureDefaultLimit        = 500
	autoclosureMaxLimit            = 5000
	autoclosureMaxIntervalDays     = 365
)

// AutoclosureService runs the reminder-and-close workflow for tickets that sit
// in Awaiting Info or Solution Proposed with no activity. It is driven from
// outside (operations/csm-scheduled-tasks asks for the tickets due, then
// advances them one at a time) and decides nothing about scheduling itself.
type AutoclosureService interface {
	// FindCandidates lists the tickets due for a stage, oldest first.
	FindCandidates(ctx context.Context, req domain.AutoclosureCandidatesRequest) (domain.AutoclosureCandidatesResponse, error)
	// Advance performs a stage on one ticket if, and only if, it is still due.
	// It is safe to call twice: the second call finds the ticket already moved
	// on and reports it skipped.
	Advance(ctx context.Context, caseID string, req domain.AutoclosureAdvanceRequest) (domain.AutoclosureAdvanceResponse, error)
}

// autoclosureComments is the slice of CaseService the workflow needs. A
// narrow interface keeps this service testable without the whole case service.
type autoclosureComments interface {
	CreateCaseCommentAs(ctx context.Context, req domain.CreateCaseCommentRequest, actorEmail string) (domain.CreateCaseCommentResponse, error)
	GetCaseByID(ctx context.Context, id string) (domain.CaseView, error)
	UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error)
}

type autoclosureService struct {
	repo        repository.AutoclosureRepository
	cases       autoclosureComments
	access      AccessService
	systemEmail string
	now         func() time.Time
}

// NewAutoclosureService constructs the service. systemEmail is the system
// user the reminder and closure comments are written as and the close is
// attributed to.
func NewAutoclosureService(repo repository.AutoclosureRepository, cases CaseService, access AccessService, systemEmail string) AutoclosureService {
	return &autoclosureService{repo: repo, cases: cases, access: access, systemEmail: systemEmail, now: time.Now}
}

// autoclosureStageSpec is what a stage does to the workflow position.
type autoclosureStageSpec struct {
	// toStep is the step the ticket holds after the stage.
	toStep string
	// template is the comment kind posted, "" for none.
	template string
}

var autoclosureStages = map[domain.AutoclosureStage]autoclosureStageSpec{
	domain.AutoclosureStageReleaseHold:    {toStep: domain.AutoclosureStepDefault},
	domain.AutoclosureStageFirstReminder:  {toStep: domain.AutoclosureStepFirstComment, template: "first_reminder"},
	domain.AutoclosureStageSecondReminder: {toStep: domain.AutoclosureStepSecondComment, template: "second_reminder"},
	// The ticket keeps SECOND_COMMENT: it is closed, and a closed ticket is
	// never a candidate again whatever its step says.
	domain.AutoclosureStageClose: {toStep: domain.AutoclosureStepSecondComment, template: "closure"},
}

// requireInternal refuses anyone but an internal (WSO2 or machine) caller: the
// workflow comments on and closes tickets, and a customer must never be able
// to start it.
func (s *autoclosureService) requireInternal(ctx context.Context) error {
	return RequireInternalCaller(ctx, s.access, "this resource is only available to internal callers")
}

func (s *autoclosureService) validate(stage domain.AutoclosureStage, intervalDays, limit int, states []domain.CaseState) (days, lim int, stateLabels []string, err error) {
	if _, ok := autoclosureStages[stage]; !ok {
		return 0, 0, nil, &apierror.ValidationError{Msg: "stage must be one of release_hold, first_reminder, second_reminder, close"}
	}
	days = intervalDays
	if days == 0 {
		days = autoclosureDefaultIntervalDays
	}
	if days < 0 || days > autoclosureMaxIntervalDays {
		return 0, 0, nil, &apierror.ValidationError{Msg: fmt.Sprintf("intervalDays must be between 1 and %d", autoclosureMaxIntervalDays)}
	}
	lim = limit
	if lim == 0 {
		lim = autoclosureDefaultLimit
	}
	if lim < 0 || lim > autoclosureMaxLimit {
		return 0, 0, nil, &apierror.ValidationError{Msg: fmt.Sprintf("limit must be between 1 and %d", autoclosureMaxLimit)}
	}
	if len(states) == 0 {
		return days, lim, []string{"AWAITING_INFO", "SOLUTION_PROPOSED"}, nil
	}
	for _, st := range states {
		switch st {
		case domain.CaseStateAwaitingInfo, domain.CaseStateSolutionProposed:
			stateLabels = append(stateLabels, strings.ToUpper(string(st)))
		default:
			return 0, 0, nil, &apierror.ValidationError{Msg: "states may only contain awaiting_info and solution_proposed"}
		}
	}
	return days, lim, stateLabels, nil
}

// FindCandidates implements AutoclosureService.
func (s *autoclosureService) FindCandidates(ctx context.Context, req domain.AutoclosureCandidatesRequest) (domain.AutoclosureCandidatesResponse, error) {
	if err := s.requireInternal(ctx); err != nil {
		return domain.AutoclosureCandidatesResponse{}, err
	}
	days, limit, states, err := s.validate(req.Stage, req.IntervalDays, req.Limit, req.States)
	if err != nil {
		return domain.AutoclosureCandidatesResponse{}, err
	}
	if req.DailyLimit < 0 || req.DailyLimit > autoclosureMaxLimit*10 {
		return domain.AutoclosureCandidatesResponse{}, &apierror.ValidationError{Msg: "dailyLimit must not be negative or absurdly large"}
	}
	// With a daily limit and no explicit page size, one call may return up to
	// the whole day's budget instead of the 500 default, so a limit above 500
	// is not silently halved.
	if req.Limit == 0 && req.DailyLimit > 0 {
		limit = min(req.DailyLimit, autoclosureMaxLimit)
	}
	// Reminders and closures share one budget per UTC day, and a hold release
	// notifies nobody so it is exempt. What this day has already done counts
	// against it, which is what keeps a retried run inside the limit.
	if req.DailyLimit > 0 && req.Stage != domain.AutoclosureStageReleaseHold {
		now := s.now().UTC()
		done, err := s.repo.CountAdvancedSince(ctx, time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC))
		if err != nil {
			return domain.AutoclosureCandidatesResponse{}, err
		}
		remaining := req.DailyLimit - done
		if remaining <= 0 {
			return domain.AutoclosureCandidatesResponse{Candidates: []domain.AutoclosureCandidate{}}, nil
		}
		if remaining < limit {
			limit = remaining
		}
	}
	rows, err := s.repo.FindCandidates(ctx, repository.AutoclosureCandidateQuery{
		Stage: req.Stage, Now: s.now().UTC(), IntervalDays: days, Limit: limit,
		States: states, SystemEmail: s.systemEmail,
	})
	if err != nil {
		return domain.AutoclosureCandidatesResponse{}, err
	}
	out := domain.AutoclosureCandidatesResponse{Candidates: make([]domain.AutoclosureCandidate, 0, len(rows))}
	for _, r := range rows {
		out.Candidates = append(out.Candidates, r.AutoclosureCandidate)
	}
	return out, nil
}

// Advance implements AutoclosureService.
func (s *autoclosureService) Advance(ctx context.Context, caseID string, req domain.AutoclosureAdvanceRequest) (domain.AutoclosureAdvanceResponse, error) {
	if err := s.requireInternal(ctx); err != nil {
		return domain.AutoclosureAdvanceResponse{}, err
	}
	if err := validateUUIDs("id", []string{caseID}); err != nil {
		return domain.AutoclosureAdvanceResponse{}, err
	}
	days, _, states, err := s.validate(req.Stage, req.IntervalDays, 0, req.States)
	if err != nil {
		return domain.AutoclosureAdvanceResponse{}, err
	}
	spec := autoclosureStages[req.Stage]
	now := s.now().UTC()
	resp := domain.AutoclosureAdvanceResponse{CaseID: caseID, Stage: req.Stage}
	skip := func(reason string) (domain.AutoclosureAdvanceResponse, error) {
		resp.Outcome = domain.AutoclosureOutcomeSkipped
		resp.Reason = reason
		return resp, nil
	}

	// Ask again, with the predicate that listed the ticket, whether it is still
	// due: it may since have been answered, put on hold, moved, closed or given
	// an open child. Nothing is trusted from whatever listed it.
	rows, err := s.repo.FindCandidates(ctx, repository.AutoclosureCandidateQuery{
		Stage: req.Stage, Now: now, IntervalDays: days, Limit: 1,
		States: states, OnlyID: caseID, SystemEmail: s.systemEmail,
	})
	if err != nil {
		return resp, err
	}
	if len(rows) == 0 {
		return skip("the ticket is not due for this stage any more")
	}
	c := rows[0]
	resp.FromStep = c.Step
	toStep := spec.toStep
	resp.ToStep = &toStep

	// Claim before acting. A second run reaching this point finds the step and
	// time already changed and loses, so a ticket is never reminded twice.
	claimed, err := s.repo.ClaimStep(ctx, c, spec.toStep, now)
	if err != nil {
		return resp, err
	}
	if !claimed {
		return skip("another run handled the ticket first")
	}

	var actErr error
	switch req.Stage {
	case domain.AutoclosureStageReleaseHold:
		// Nothing to write beyond the claim.
	case domain.AutoclosureStageClose:
		actErr = s.closeCase(ctx, c, s.commentText(c.State, spec.template, req.Templates))
	default:
		_, actErr = s.cases.CreateCaseCommentAs(ctx, domain.CreateCaseCommentRequest{
			CaseID:  caseID,
			Type:    domain.CommentTypeComment,
			Content: s.commentText(c.State, spec.template, req.Templates),
		}, s.systemEmail)
	}
	if actErr != nil {
		// Put the position back so the next run retries it, rather than leaving
		// a ticket marked as reminded that never was.
		if rErr := s.repo.RestoreStep(ctx, c, now); rErr != nil {
			slog.ErrorContext(ctx, "autoclosure: restore after failed action", "caseId", caseID, "stage", req.Stage, "error", rErr)
		}
		s.record(ctx, caseID, req.Stage, c.Step, &toStep, "FAILED", actErr.Error())
		return resp, actErr
	}

	s.record(ctx, caseID, req.Stage, c.Step, &toStep, "ADVANCED", "")
	resp.Outcome = domain.AutoclosureOutcomeAdvanced
	return resp, nil
}

// closeCase closes the ticket as the system user and then writes the closure
// comment. The comment follows the close, not the other way round: a comment
// first and a refused close (an open child appeared, a concurrent edit) would
// leave a "this case is closed" comment on a case that is not.
//
// Awaiting Info tickets get the non-responsiveness resolution code (which
// keeps them out of the KB process); Solution Proposed tickets keep whatever
// resolution they were proposed with.
func (s *autoclosureService) closeCase(ctx context.Context, c repository.AutoclosureCandidateRow, comment string) error {
	view, err := s.cases.GetCaseByID(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("read ticket before closing: %w", err)
	}
	closed := domain.CaseStateClosed
	update := domain.UpdateCaseRequest{ID: c.ID, State: &closed, CloseNotes: &comment}
	if view.Cause != nil {
		update.Cause = view.Cause
	} else {
		unknown := domain.CaseCauseUnknown
		update.Cause = &unknown
	}
	if c.State == domain.CaseStateAwaitingInfo {
		code := domain.CaseResolutionCodeAbruptlyClosedDueToNonResponsiveness
		update.ResolutionCode = &code
	} else if view.ResolutionCode != nil {
		update.ResolutionCode = view.ResolutionCode
	}

	// Closing is done as the system user, which is also what allows the one
	// move no staff table offers (Awaiting Info -> Closed).
	if _, err := s.cases.UpdateCase(withAutoclosureActor(ctx, s.systemEmail), update); err != nil {
		return fmt.Errorf("close ticket: %w", err)
	}
	if _, err := s.cases.CreateCaseCommentAs(ctx, domain.CreateCaseCommentRequest{
		CaseID: c.ID, Type: domain.CommentTypeComment, Content: comment,
	}, s.systemEmail); err != nil {
		// The ticket is closed; a missing closure comment is a lesser failure
		// than undoing the close. Logged, and the audit row still says ADVANCED.
		slog.ErrorContext(ctx, "autoclosure: ticket closed but its closure comment failed", "caseId", c.ID, "error", err)
	}
	return nil
}

func (s *autoclosureService) record(ctx context.Context, caseID string, stage domain.AutoclosureStage, from, to *string, outcome, detail string) {
	if err := s.repo.RecordEvent(ctx, repository.AutoclosureEvent{
		WorkItemID: caseID, Stage: stage, FromStep: from, ToStep: to, Outcome: outcome, Detail: detail,
	}); err != nil {
		slog.ErrorContext(ctx, "autoclosure: audit row failed", "caseId", caseID, "stage", stage, "error", err)
	}
}

// commentText returns the comment for a state and stage, preferring a caller's
// override.
func (s *autoclosureService) commentText(state domain.CaseState, kind string, overrides map[string]string) string {
	key := string(state) + "." + kind
	if text, ok := overrides[key]; ok && strings.TrimSpace(text) != "" {
		return text
	}
	return autoclosureDefaultTemplates[key]
}

// autoclosureDefaultTemplates holds the comments sent to the customer, keyed
// "<state>.<kind>". They are defaults: a caller may send its own for any key
// (the scheduled task reads them from its configuration), so wording can
// change without a deploy of this service.
var autoclosureDefaultTemplates = map[string]string{
	"solution_proposed.first_reminder": "Hello,\n\n" +
		"This case will be automatically closed as a solution has been proposed and no further updates have been received. " +
		"Please let us know whether you were able to try out the proposed solution by either selecting Accept or Reject.\n\n" +
		"Thanks,\nSupport Team",
	"solution_proposed.second_reminder": "Hello,\n\n" +
		"Please let us know whether you need further support for this case. " +
		"Please note that if you are unable to respond, the case will be closed off after one week.\n\n" +
		"Thanks,\nSupport Team",
	"solution_proposed.closure": "Hello,\n\n" +
		"Please note that this case has been closed due to inactivity after the solution was suggested. " +
		"Thank you for contacting the Support Team.\n\n" +
		"Thanks,\nSupport Team",
	"awaiting_info.first_reminder": "Hello,\n\n" +
		"This case will be automatically closed as no further updates have been received. " +
		"Please share the requested details to continue the investigation of the issue.\n\n" +
		"Thanks,\nSupport Team",
	"awaiting_info.second_reminder": "Hello,\n\n" +
		"Gentle reminder for the requested details. If you are able to provide the information or wish to keep this case open, please reply to this message.\n\n" +
		"Please note that if you are unable to respond, the case will be closed off after one week.\n\n" +
		"Thanks,\nSupport Team",
	"awaiting_info.closure": "Hello,\n\n" +
		"Please note that this case is closed as we did not receive an update. " +
		"Thank you for contacting the Support Team.\n\n" +
		"Thanks,\nSupport Team",
}

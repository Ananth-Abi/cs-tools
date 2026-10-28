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

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/businessdays"
)

// slaScheduleWSO2BusinessDays is the one non-24x7 schedule this engine
// actually implements -- see internal/businessdays' own package doc comment.
// Every other schedule value (including "8-5 weekdays"/"8-5 weekdays
// excluding holidays", seen on a handful of real rows this engine never
// resolves a case policy to -- see sla_policy_resolver.go's own case-policy
// classification) is treated as plain 24x7 flat wall-clock time, same as a
// NULL/empty schedule always has been, rather than guessed at.
const slaScheduleWSO2BusinessDays = "WSO2 Business days"

// businessDayElapsedSnapshot computes a 'WSO2 Business days' clock's
// elapsed/remaining/percentage/breached figures as of instant `at`, given
// its stored start_on and the business-day pause time already accumulated
// from every COMPLETED pause/resume cycle (businessPauseSoFar never
// includes an open, still-running pause segment -- a caller finalizing one
// of those into the total before calling, e.g. completeClockBusinessDays
// folding in the segment up to its own `at`, is what keeps this one
// function correct for completion, pause and a live read alike). Shared by
// completeClockBusinessDays, setPausedBusinessDays, recomputeActiveBusinessDays
// and SLAStatusRepository.GetClockState's own live business-day branch, so
// the one real elapsed-time formula is written once.
func businessDayElapsedSnapshot(startOn, at time.Time, businessPauseSoFar, duration time.Duration) (elapsed, remaining time.Duration, percentage float64, breached bool) {
	elapsed = businessdays.Elapsed(startOn, at) - businessPauseSoFar
	if elapsed < 0 {
		elapsed = 0
	}
	remaining = duration - elapsed
	if remaining < 0 {
		remaining = 0
	}
	if duration > 0 {
		percentage = elapsed.Seconds() / duration.Seconds() * 100
	}
	breached = elapsed >= duration
	return elapsed, remaining, percentage, breached
}

// slaEngineActiveStageFilter names the terminal sla.stage values a "sla" row
// can never leave -- shared by RegisterClock/ReviseClocks' own insert guard
// so a row this engine has already finished registering against is never
// resurrected by a later, out-of-order call (e.g. a retried case-create hook
// after a slow first attempt already registered the clock). Used only for
// that "is there already a clock for this (work_item, target) pair" check --
// NOT by CompleteClock/SetPaused, which use the narrower
// slaEngineOpenStageFilter below, since BREACHED is very much not finished
// from their point of view.
const slaEngineActiveStageFilter = `NOT IN ('ACHIEVED', 'BREACHED', 'CANCELLED', 'COMPLETED')`

// slaEngineOpenStageFilter names the only truly final sla.stage values --
// the ones a clock can never leave because its own disposition already
// happened (a reply came in, a workaround was provided, a case closed, or
// the clock was deliberately cancelled on a severity change). Deliberately
// excludes BREACHED, unlike slaEngineActiveStageFilter above: a clock whose
// wall-clock duration merely ran out without yet being satisfied is NOT
// finished -- it must keep being recomputed (RecomputeActive) and must
// still be completable or pausable by its own real event, whenever that
// event finally happens, with the TRUE elapsed time at that moment however
// far past 100% it has climbed. Treating BREACHED as terminal here was a
// real, reported bug: once a response SLA breached, a later qualifying
// comment's CompleteResponseClock call silently matched zero rows, leaving
// the clock -- and its business_elapsed_percentage/business_duration --
// frozen forever at whatever RecomputeActive last wrote before excluding it
// too (see RecomputeActive's own doc comment for its matching half of this
// fix).
const slaEngineOpenStageFilter = `NOT IN ('ACHIEVED', 'CANCELLED', 'COMPLETED')`

// slaEngineRevisionBlockStages names the stages that must block a fresh
// registration for the given clock TARGET on a severity revision
// (ReviseClocks) -- deliberately per-target, per explicit product
// direction: RESPONSE and WORKAROUND/RESOLUTION disagree on whether a mere
// BREACHED (the wall clock ran out without the clock ever being satisfied)
// counts as done.
//
//   - RESPONSE: "did a support engineer reply at all" is a fact about the
//     past that a later severity change cannot un-happen or un-miss --
//     ACHIEVED (a reply came in) and BREACHED (a reply never came, and the
//     window for a first reply has already closed) both permanently retire
//     this clock type for the case. A severity change must not re-open "are
//     we still waiting for a first reply" once that window has already
//     closed one way or the other.
//   - WORKAROUND/RESOLUTION: these track ongoing remediation work, which
//     genuinely restarts under a new severity's own duration regardless of
//     whether the OLD severity's clock ran out first -- only a REAL
//     completion (a workaround actually provided, a case actually closed,
//     via CompleteClock) means there is nothing left to track. Only
//     ACHIEVED/COMPLETED block a fresh registration for these two; BREACHED
//     does not -- a real, reported bug had this treated identically to
//     RESPONSE, leaving a case's workaround/resolution tracking permanently
//     stuck on a stale, timed-out clock instead of starting over under the
//     new severity.
//
// CANCELLED never blocks either way -- it means "this clock was
// deliberately retired and its slot is free for a fresh one" (see
// ReviseClocks).
func slaEngineRevisionBlockStages(target string) []string {
	if target == "RESPONSE" {
		return []string{"ACHIEVED", "BREACHED", "COMPLETED"}
	}
	return []string{"ACHIEVED", "COMPLETED"}
}

// ReviseClocks' own cancellation query encodes this same per-target split
// directly in SQL (RESPONSE never cancels a BREACHED row; WORKAROUND/
// RESOLUTION do) rather than calling back into this function -- see that
// method's own doc comment. Kept as one written-out rule rather than two
// (this function plus a SQL mirror) since ReviseClocks' cancellation scans
// every CSM row on the work item by its own target column, not a per-policy
// Go-side loop, so there is no natural call site for a Go-side helper here.

// SLAPolicyRef is the subset of an sla_policy row the engine's resolver
// needs: enough to register a new "sla" row against it, nothing this
// service would otherwise have to re-derive (target, duration) or display
// (name, for logging).
type SLAPolicyRef struct {
	ID       string
	Name     string
	Target   string
	Duration time.Duration
	// Schedule is sla_policy.schedule verbatim ("24 x 7", "WSO2 Business
	// days", or one of the unsupported values -- see
	// slaScheduleWSO2BusinessDays's own doc comment). Empty for a policy
	// with no schedule recorded, treated the same as "24 x 7".
	Schedule string
	// StartCondition/StopCondition/PauseCondition/ResumeCondition/
	// CancelCondition are the raw sla_policy condition columns, exactly as
	// migrations 0250-0258 left them -- parsed by internal/slacondition,
	// never interpreted here. Empty when the column is NULL.
	StartCondition  string
	StopCondition   string
	PauseCondition  string
	ResumeCondition string
	CancelCondition string
}

// CaseFacts is the subset of a case's current field values the condition
// engine (internal/slacondition, via service.caseConditionFacts) needs to
// evaluate an sla_policy condition against. State/Severity are the raw
// case_state_enum/case_severity_enum text Postgres stores (e.g. "OPEN",
// "S1") -- these already match sla_policy's own decoded condition
// vocabulary verbatim (migrations 0250/0252), so no further translation
// happens in this package; domain.CaseState/domain.CaseSeverity only enter
// the picture at the service layer, when a caller's own in-flight value
// (not yet necessarily visible to a fresh read -- see
// GetCaseFacts' own doc comment) needs to override what's stored.
type CaseFacts struct {
	// State is "" when the case has no state recorded at all (a real,
	// non-trivial share of rows -- see domain.CaseView's own doc comment).
	State string
	// Severity is "" when the case has no severity recorded (case-type
	// work items only ever carry one; every other case-like type is
	// always "").
	Severity string
	// CaseType is "case"/"engagement"/"service_request"/
	// "security_report_analysis"/"announcement" (lowercased
	// caseLikeRowTypeColumn) -- "hosting"/"hosting_query"/"hosting_task"
	// are part of sla_policy's own decoded vocabulary (ServiceNow-only
	// case types, migration 0252) but can never appear here: this
	// Postgres schema's work_item_type_enum has no matching value at all,
	// so no work_item row can ever carry one. A policy conditioned on one
	// of those three case types simply never matches on this data source
	// -- not a bug, a real, structural gap in what this schema can
	// represent today (see sla_policy_resolver.go's own doc comment).
	CaseType string
	// ProjectType is the case's project's resolved project_type.name, ""
	// when the case has no project or the project has no type.
	ProjectType string
	// Product is the case's deployed product's resolved product.name, ""
	// when the case has no deployed product.
	Product string
	// CSEngineerResponseComment answers sla_policy's own
	// "cs_engineer_response_comment" field: whether a CS engineer has
	// already posted a qualifying reply. No dedicated column for this
	// exists anywhere in this schema (confirmed, see migration 0250's own
	// doc comment) -- this reuses the one existing signal that already
	// means exactly that: the case's own CSM-native RESPONSE clock (the
	// "sla" row this same engine registers) having reached ACHIEVED,
	// which CompleteResponseClock only ever sets when a qualifying reply
	// was actually posted (sla_engine_service.go's own
	// completeResponseSLAOnComment/applyResponseSLAOnComment). See
	// GetCaseFacts' own doc comment for the one known approximation this
	// carries.
	CSEngineerResponseComment bool
	// WorkaroundProvided answers "u_workaround_providedISNOTEMPTY" --
	// backed by the existing work_item.workaround_provided_on column.
	WorkaroundProvided bool
	// FixEtaShared answers "fix_eta_sharedISNOTEMPTY" -- backed by the
	// existing work_item.eta_shared_on column.
	FixEtaShared bool
	// HasAccount answers "accountISNOTEMPTY".
	HasAccount bool
	// CreatedOn answers "opened_at"/"sys_created_on" (both ServiceNow
	// names for the same fact) -- work_item.created_on.
	CreatedOn time.Time
}

// SLAEngineRepository defines the read/write operations backing this
// service's own in-process SLA engine (internal/service/sla_policy_resolver.go,
// sla_engine_service.go). Every clock it writes is tagged source='CSM'
// (migration 0134's enum) -- this is migration 0134's own original intent,
// unchanged: 'SERVICENOW' means a row the ServiceNow sync job itself wrote,
// 'CSM' means a row this in-process engine wrote, and the two are NEVER
// supposed to share ownership of the same (work_item, target) pair.
//
// **This was briefly, wrongly changed to 'SERVICENOW' and then reverted.**
// The earlier change was triggered by a report that a stale-looking
// `s.source = 'CSM'` filter looked like leftover wiring from the old,
// deleted, name-keyed CSM-policy engine -- a misreading: "CSM" here was
// never about that old engine's design, it is this engine's own OWNERSHIP
// TAG, same meaning before and after the old engine was replaced by the
// condition-driven one. Confirmed, concretely, not just by re-reading the
// migration comment: `digiops-cs/csm-sync-service`'s own
// `configs/mappings/task_sla.yaml` maps ServiceNow's `task_sla` table
// directly into this database's `sla` table (`sync.tier: fast`, i.e. runs
// continuously) and tags every row it writes `source='SERVICENOW'` -- a
// real, independent, continuously-running external writer, not a
// hypothetical one. Tagging this engine's own clocks `'SERVICENOW'` too
// would have let this engine's writes collide with that sync job's rows for
// the same case (a dual-write-mode case has both), each silently
// overwriting state the other owns. **`'SERVICENOW'` must never be reused
// for an engine-managed clock again.**
//
// This repository still only ever writes/mutates rows it itself registered
// via RegisterClock (every method here scopes its WHERE clause to a
// specific work_item_id/target pair it just resolved a policy for, AND to
// source='CSM'), so it can never touch a row the ServiceNow sync job owns.
// Read-side consumers of "sla" (GET /sla-status, POST /task-slas/search)
// need no changes at all -- every row already looks the same to them
// regardless of who wrote it, and both GetClockState (single-clock) and
// SearchActiveSLAStatuses (bulk) already accept the identical source query
// parameter, so a caller that wants only this engine's own clocks (e.g.
// csm-notification-service's pre-alert check, see that repo's own
// CLAUDE.md) scopes to source=csm on either call, not just the bulk one.
//
// The `sla_policy` table's own `source` column is a SEPARATE, unrelated
// concept -- it distinguishes a real ServiceNow-synced POLICY definition
// from the 16 `source='CSM'` policy rows this engine used to seed itself
// (migration 0203, deleted by migration 0259 once this engine started
// evaluating the real synced policies directly). That policy-table cleanup
// was correct and stays; only the `sla` (clock) table's own tagging was
// ever wrongly touched.
type SLAEngineRepository interface {
	// FindPolicyByName resolves the single active sla_policy row matching
	// name/target, preferring a source='SERVICENOW' row over a source='CSM'
	// one should both ever exist under the same name (they shouldn't in
	// practice: this engine's own resolver only ever looks up the
	// deterministic "<severity> - <target> (CSM)" names migration 0203
	// seeds, e.g. "S4 - RESPONSE (CSM)", a name ServiceNow's own sync would
	// never produce). Returns apierror.NotFoundError if no row matches.
	FindPolicyByName(ctx context.Context, name, target string) (SLAPolicyRef, error)

	// ListCandidatePolicies returns every active, source='SERVICENOW'
	// sla_policy row for target (RESPONSE/WORKAROUND/RESOLUTION), condition
	// columns included -- the raw material service.slaPolicyResolver
	// evaluates against a case's CaseFacts to pick the one real policy that
	// actually applies. Deliberately returns every row (case policies,
	// incident/escalation policies, inactive rows already excluded) rather
	// than filtering to "case policies" here -- that classification is
	// business logic (see sla_policy_resolver.go's own isCasePolicy), not a
	// SQL concern. Ordered by id for a deterministic tie-break when more
	// than one policy's start_condition matches the same case (see
	// slaPolicyResolver.resolve's own doc comment).
	ListCandidatePolicies(ctx context.Context, target string) ([]SLAPolicyRef, error)

	// GetCaseFacts reads the current field values of workItemID needed to
	// evaluate an sla_policy condition against it (internal/slacondition).
	// Returns apierror.NotFoundError if no such work item exists.
	//
	// Every field is read fresh from Postgres at call time -- there is no
	// in-flight override here. A caller that already knows a more
	// authoritative value than what a fresh read would see right now (e.g.
	// ApplyCaseStateEffects is handed the case's brand new state as a
	// parameter, from a PATCH that may not have committed its own mirror
	// write yet on the ServiceNow dual-write path) is expected to override
	// that one field on the returned CaseFacts itself -- see
	// slaEngineService's own call sites.
	GetCaseFacts(ctx context.Context, workItemID string) (CaseFacts, error)

	// CancelClock marks the source='CSM' clock for (workItemID, target)
	// CANCELLED -- the condition-driven engine's own per-policy
	// cancel_condition outcome (ApplyCaseStateEffects), as opposed to
	// ReviseClocks' own unconditional bulk cancellation on a severity
	// change. Matches slaEngineOpenStageFilter, same as SetPaused/
	// CompleteClock -- an already-BREACHED clock can still be explicitly
	// cancelled (e.g. a case whose case_type changed away from what the
	// breached clock's own policy required). Returns whether a row was
	// found and updated.
	CancelClock(ctx context.Context, workItemID, target string) (bool, error)

	// ActiveClockPolicy returns the sla_policy row currently backing
	// workItemID's open (slaEngineOpenStageFilter) clock for target, so the
	// condition-driven engine (ApplyCaseStateEffects) can evaluate THAT
	// specific policy's own stop/pause/resume/cancel conditions -- never a
	// fresh re-resolution of "which policy should apply now" (facts can
	// shift in ways that would otherwise flip a running clock onto a
	// different policy's numbers mid-flight). ok=false (no error) when the
	// case has no open clock for target at all.
	ActiveClockPolicy(ctx context.Context, workItemID, target string) (SLAPolicyRef, bool, error)

	// RegisterClock inserts a new source='CSM' "sla" row for
	// (workItemID, policy.Target) and starts it running now, UNLESS an
	// active (see slaEngineActiveStageFilter) source='CSM' row already
	// exists for that (work_item, target) pair -- idempotent, so a retried
	// case-create hook never double-registers -- OR a row already exists in
	// a stage slaEngineRevisionBlockStages(policy.Target) names as blocking
	// for that target (per-target -- see that function's own doc comment for
	// why RESPONSE differs from WORKAROUND/RESOLUTION). A CANCELLED row does
	// NOT block a fresh insert -- cancellation deliberately frees that clock
	// type up for a genuinely new one (see ReviseClocks). Returns whether a
	// row was actually inserted.
	RegisterClock(ctx context.Context, workItemID string, policy SLAPolicyRef) (bool, error)

	// CompleteClock marks the source='CSM' clock for (workItemID, target)
	// ACHIEVED (end_on=now, business_elapsed_percentage set to the clock's
	// real, uncapped elapsed percentage at completion time, not
	// unconditionally 100 -- see this method's own implementation comment),
	// and sets is_active = FALSE -- a real, reported bug this fixes: is_active
	// was previously only ever set at RegisterClock's own INSERT and never
	// cleared again, so GET /sla-status (and csm-notification-service's own
	// Redis-recovery Reconcile, which reads it) kept reporting a clock as
	// "currently active" forever after it had genuinely, cleanly completed.
	// Matches a clock in ANY not-yet-final stage, including BREACHED -- a
	// clock whose window already ran out is still completable by its own
	// real finishing event, with whatever real overrun percentage that
	// event happened at (see slaEngineOpenStageFilter's own doc comment).
	// Returns whether a row was found and updated -- false (not an error) when no
	// such clock was ever registered, e.g. a LOW/Query-severity case, which
	// never gets a "response" clock's workaround/resolution siblings, or a
	// case whose policy lookup found nothing at create time.
	CompleteClock(ctx context.Context, workItemID, target string) (bool, error)

	// SetPaused pauses (true) or resumes (false) the source='CSM' clock for
	// (workItemID, target), matching any not-yet-final stage including
	// BREACHED (see slaEngineOpenStageFilter) -- so an already-breached
	// clock can still be paused while its case waits on the customer,
	// rather than continuing to silently accumulate elapsed time. Idempotent
	// and a no-op (not an error) when no such clock exists, same reasoning
	// as CompleteClock.
	SetPaused(ctx context.Context, workItemID, target string, paused bool) (bool, error)

	// RecomputeActive recomputes business_elapsed_percentage (uncapped --
	// it keeps climbing past 100 for as long as a clock remains BREACHED)
	// for every source='CSM' row currently IN_PROGRESS or BREACHED
	// (deliberately narrower than slaEngineOpenStageFilter -- see this
	// method's own doc comment on its implementation for why PAUSED is
	// excluded too), flips has_breached and stage to BREACHED once elapsed
	// time first reaches the policy duration, and returns how many rows
	// were touched. Including BREACHED here (not just IN_PROGRESS) is what
	// keeps a breached clock's elapsed time/percentage moving until its own
	// real completing event finalizes it, instead of freezing forever at
	// whatever value the tick that first crossed 100% happened to compute.
	RecomputeActive(ctx context.Context) (int, error)

	// ReviseClocks marks every source='CSM' clock for workItemID CANCELLED,
	// per-target (RESPONSE excludes BREACHED from cancellation; WORKAROUND/
	// RESOLUTION include it -- see the implementation's own doc comment),
	// then registers a fresh row for each given policy (same insert shape
	// and per-target blocking guard as RegisterClock) -- both in ONE
	// database transaction. Used when a case's severity changes: per
	// explicit product direction, the old severity's clocks must not be
	// revised or carried forward in any way, they run into a terminal
	// CANCELLED state, and the new severity's clocks start completely fresh
	// with no relation to the old numbers (see SLAEngineService.
	// ReviseCaseClocks).
	//
	// The whole operation is one transaction, not two independent
	// statements, specifically so a failure partway through registering the
	// new clocks rolls back the cancellation too -- the case is left with
	// its OLD clocks exactly as they were, never with the old ones
	// cancelled and no replacement in their place. A clock already in a
	// stage slaEngineRevisionBlockStages names as blocking for its own
	// target (e.g. a response clock CompleteResponseClock already marked
	// ACHIEVED, or -- for RESPONSE only -- already BREACHED) is left
	// untouched by the cancellation step and never resurrected by the
	// registration step either (RegisterClock's own guard applies here
	// too). policies may be empty (e.g. a nil/unresolvable severity) -- the
	// cancellation still runs, nothing gets registered. Returns how many
	// rows were cancelled.
	ReviseClocks(ctx context.Context, workItemID string, policies []SLAPolicyRef) (int, error)
}

type slaEngineRepo struct {
	db *Scoped
}

// NewSLAEngineRepository constructs an SLAEngineRepository backed by the
// given Scoped connection. This engine runs on a process-startup background
// worker (cmd/api/main.go's slaEngineCtx), never an HTTP request, so that
// context must carry WithSystemIdentity(ctx) rather than inheriting nothing
// -- Scoped requires an identity on ctx even though sla itself has no RLS
// since migration 0153.
func NewSLAEngineRepository(db *Scoped) SLAEngineRepository {
	return &slaEngineRepo{db: db}
}

// FindPolicyByName implements SLAEngineRepository.
//
// ORDER BY source: 'SERVICENOW' sorts before 'CSM' lexically, so a plain
// ORDER BY source, then LIMIT 1, prefers the real synced policy whenever
// both a SERVICENOW and a CSM row happen to share a name -- which should
// never actually happen (this engine only ever seeds names ServiceNow's
// own real policy set is confirmed NOT to define, e.g. the P0 rows from
// migration 0136), but preferring the synced row costs nothing and
// removes any doubt about which one wins if it ever did.
func (r *slaEngineRepo) FindPolicyByName(ctx context.Context, name, target string) (SLAPolicyRef, error) {
	// EXTRACT(EPOCH FROM duration) rather than scanning the INTERVAL column
	// directly into time.Duration -- pgx v5 has no default scan plan from
	// PostgreSQL INTERVAL to time.Duration (it scans into pgtype.Interval,
	// whose Months/Days fields have no fixed conversion), so a direct scan
	// errors at query time. Same pattern task_sla_repo.go's
	// scanTaskSlaView already uses for its own INTERVAL columns.
	const query = `
		SELECT id, name, target::TEXT, EXTRACT(EPOCH FROM duration)
		FROM sla_policy
		WHERE name = $1 AND target = $2::sla_policy_target_enum
		  AND source IN ('SERVICENOW', 'CSM')
		  AND (is_active IS NULL OR is_active)
		  AND duration IS NOT NULL
		ORDER BY source
		LIMIT 1`

	var ref SLAPolicyRef
	var durationSeconds float64
	err := r.db.QueryRow(ctx, query, name, target).Scan(&ref.ID, &ref.Name, &ref.Target, &durationSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return SLAPolicyRef{}, &apierror.NotFoundError{Msg: "no sla_policy found named " + name}
	}
	if err != nil {
		return SLAPolicyRef{}, fmt.Errorf("find sla policy by name: %w", err)
	}
	ref.Duration = time.Duration(durationSeconds * float64(time.Second))
	return ref, nil
}

// ListCandidatePolicies implements SLAEngineRepository.
func (r *slaEngineRepo) ListCandidatePolicies(ctx context.Context, target string) ([]SLAPolicyRef, error) {
	const query = `
		SELECT id, name, target::TEXT, EXTRACT(EPOCH FROM duration), COALESCE(schedule, ''),
		       COALESCE(start_condition, ''), COALESCE(stop_condition, ''),
		       COALESCE(pause_condition, ''), COALESCE(resume_condition, ''), COALESCE(cancel_condition, '')
		FROM sla_policy
		WHERE target = $1::sla_policy_target_enum
		  AND source = 'SERVICENOW'
		  AND (is_active IS NULL OR is_active)
		  AND duration IS NOT NULL
		ORDER BY id`

	rows, err := r.db.Query(ctx, query, target)
	if err != nil {
		return nil, fmt.Errorf("list candidate sla policies: %w", err)
	}
	defer rows.Close()

	var refs []SLAPolicyRef
	for rows.Next() {
		var ref SLAPolicyRef
		var durationSeconds float64
		if err := rows.Scan(&ref.ID, &ref.Name, &ref.Target, &durationSeconds, &ref.Schedule,
			&ref.StartCondition, &ref.StopCondition, &ref.PauseCondition, &ref.ResumeCondition, &ref.CancelCondition); err != nil {
			return nil, fmt.Errorf("list candidate sla policies: scan: %w", err)
		}
		ref.Duration = time.Duration(durationSeconds * float64(time.Second))
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list candidate sla policies: rows: %w", err)
	}
	return refs, nil
}

// GetCaseFacts implements SLAEngineRepository.
//
// Reuses case_repo.go's own caseLikeRowTypeColumn/caseLikeStateColumn/
// caseLikeJoins (same package) rather than a second, independent
// case-like/state resolution -- see those constants' own doc comments for
// why a case-like work item's state/type can't be read off a single column.
// severity is read from "case" directly (c.severity): only a CASE-typed
// work item ever has one (domain.CaseView's own doc comment).
func (r *slaEngineRepo) GetCaseFacts(ctx context.Context, workItemID string) (CaseFacts, error) {
	const query = `
		SELECT LOWER(` + caseLikeRowTypeColumn + `), ` + caseLikeStateColumn + `, c.severity::TEXT,
		       wi.created_on, COALESCE(pt.name, ''), COALESCE(prod.name, ''),
		       (wi.account_id IS NOT NULL),
		       (wi.workaround_provided_on IS NOT NULL),
		       (wi.eta_shared_on IS NOT NULL),
		       EXISTS (
		           SELECT 1 FROM sla rs
		           JOIN sla_policy rsp ON rsp.id = rs.sla_policy_id
		           WHERE rs.work_item_id = wi.id AND rs.source = 'CSM'
		             AND rsp.target::TEXT = 'RESPONSE' AND rs.stage::TEXT = 'ACHIEVED'
		       )
		FROM work_item wi
		LEFT JOIN "case" c ON c.id = wi.id
		` + caseLikeJoins + `
		LEFT JOIN project proj ON proj.id = wi.project_id
		LEFT JOIN project_type pt ON pt.id = proj.project_type_id
		LEFT JOIN deployed_product dp ON dp.id = wi.deployed_product_id
		LEFT JOIN product prod ON prod.id = dp.product_id
		WHERE wi.id = $1`

	var facts CaseFacts
	var caseType string
	var state, severity *string
	err := r.db.QueryRow(ctx, query, workItemID).Scan(
		&caseType, &state, &severity, &facts.CreatedOn, &facts.ProjectType, &facts.Product,
		&facts.HasAccount, &facts.WorkaroundProvided, &facts.FixEtaShared, &facts.CSEngineerResponseComment,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return CaseFacts{}, &apierror.NotFoundError{Msg: "work item not found: " + workItemID}
	}
	if err != nil {
		return CaseFacts{}, fmt.Errorf("get case facts: %w", err)
	}
	facts.CaseType = caseType
	if state != nil {
		facts.State = *state
	}
	if severity != nil {
		facts.Severity = *severity
	}
	return facts, nil
}

// CancelClock implements SLAEngineRepository.
func (r *slaEngineRepo) CancelClock(ctx context.Context, workItemID, target string) (bool, error) {
	const query = `
		UPDATE sla s
		SET stage = 'CANCELLED'::sla_stage_enum, is_active = FALSE, updated_on = NOW(), updated_by = $3
		FROM sla_policy sp
		WHERE s.sla_policy_id = sp.id
		  AND s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $2
		  AND s.stage::TEXT ` + slaEngineOpenStageFilter

	tag, err := r.db.Exec(ctx, query, workItemID, target, sqlActorLiteral)
	if err != nil {
		return false, fmt.Errorf("cancel csm sla clock: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ActiveClockPolicy implements SLAEngineRepository.
func (r *slaEngineRepo) ActiveClockPolicy(ctx context.Context, workItemID, target string) (SLAPolicyRef, bool, error) {
	const query = `
		SELECT sp.id, sp.name, sp.target::TEXT, EXTRACT(EPOCH FROM sp.duration), COALESCE(sp.schedule, ''),
		       COALESCE(sp.start_condition, ''), COALESCE(sp.stop_condition, ''),
		       COALESCE(sp.pause_condition, ''), COALESCE(sp.resume_condition, ''), COALESCE(sp.cancel_condition, '')
		FROM sla s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $2
		  AND s.stage::TEXT ` + slaEngineOpenStageFilter + `
		ORDER BY s.created_on DESC
		LIMIT 1`

	var ref SLAPolicyRef
	var durationSeconds float64
	err := r.db.QueryRow(ctx, query, workItemID, target).Scan(&ref.ID, &ref.Name, &ref.Target, &durationSeconds, &ref.Schedule,
		&ref.StartCondition, &ref.StopCondition, &ref.PauseCondition, &ref.ResumeCondition, &ref.CancelCondition)
	if errors.Is(err, pgx.ErrNoRows) {
		return SLAPolicyRef{}, false, nil
	}
	if err != nil {
		return SLAPolicyRef{}, false, fmt.Errorf("active clock policy: %w", err)
	}
	ref.Duration = time.Duration(durationSeconds * float64(time.Second))
	return ref, true, nil
}

// slaEngineRegisterClockQuery inserts a new source='CSM' "sla" row for
// ($1=workItemID, $5=policy.Target) unless an existing row for that pair is
// either still active (slaEngineActiveStageFilter) or already reached a
// stage $6 names as blocking (slaEngineRevisionBlockStages(policy.Target),
// bound by registerClockExec -- per-target, see that function's own doc
// comment) -- a CANCELLED row blocks neither, deliberately, since
// cancellation is what frees a clock type up for a fresh registration (see
// SLAEngineRepository.ReviseClocks). Shared, identical SQL text between
// RegisterClock (run against the pool directly) and ReviseClocks (run
// inside its own transaction) -- sqlExecutor is satisfied by both
// *pgxpool.Pool and pgx.Tx.
const slaEngineRegisterClockQuery = `
	INSERT INTO sla (
		id, created_on, updated_on, created_by, updated_by,
		work_item_id, sla_policy_id, schedule, is_active, stage, start_on,
		duration, business_elapsed_percentage, business_duration,
		remaining_business_duration, has_breached, source
	)
	SELECT gen_random_uuid(), NOW(), NOW(), $3, $3,
	       $1::uuid, $2::uuid, $7, TRUE, 'IN_PROGRESS'::sla_stage_enum, NOW(),
	       $4::interval, 0, INTERVAL '0', $4::interval, FALSE, 'CSM'::sla_source_enum
	WHERE NOT EXISTS (
		SELECT 1 FROM sla s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $5
		  AND s.stage::TEXT ` + slaEngineActiveStageFilter + `
	)
	AND NOT EXISTS (
		SELECT 1 FROM sla s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $5
		  AND s.stage::TEXT = ANY($6::text[])
	)`

// sqlExecutor is the subset of *pgxpool.Pool/pgx.Tx this file's queries
// need -- lets slaEngineRegisterClockQuery run identically against either.
type sqlExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// registerClockExec runs slaEngineRegisterClockQuery against any sqlExecutor
// -- the pool directly for RegisterClock's own standalone call, or a
// transaction for ReviseClocks, so both share one query and one insert
// decision instead of two copies that could drift apart. The blocking-stage
// list ($6) is resolved per policy.Target via slaEngineRevisionBlockStages
// -- see that function's own doc comment for why RESPONSE differs from
// WORKAROUND/RESOLUTION.
func registerClockExec(ctx context.Context, exec sqlExecutor, workItemID string, policy SLAPolicyRef) (bool, error) {
	tag, err := exec.Exec(ctx, slaEngineRegisterClockQuery, workItemID, policy.ID, sqlActorLiteral, formatIntervalLiteral(policy.Duration), policy.Target, slaEngineRevisionBlockStages(policy.Target), nullableSchedule(policy.Schedule))
	if err != nil {
		return false, fmt.Errorf("register csm sla clock: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// RegisterClock implements SLAEngineRepository.
func (r *slaEngineRepo) RegisterClock(ctx context.Context, workItemID string, policy SLAPolicyRef) (bool, error) {
	return registerClockExec(ctx, r.db, workItemID, policy)
}

// CompleteClock implements SLAEngineRepository.
//
// business_elapsed_percentage, business_duration and remaining_business_duration
// are all set from the clock's REAL elapsed time at completion (same flat
// wall-clock formula RecomputeActive uses), not a hardcoded 100%/zeroed-out
// pair -- an earlier version of this query always wrote
// business_elapsed_percentage=100 regardless of how much of the clock's
// duration had actually passed, which made a response answered within
// minutes look identical, on read, to one that ran the entire window and
// barely made it. That false 100% had a real, live-observed downstream
// effect beyond just a misleading UI number: csm-notification-service's
// SLA-breach poller (internal/slaengine.tierForStatus) treats
// businessElapsedPercent >= 100 as tier 100 on its own, with no regard for
// stage -- so a same-clock ACHIEVED reading at a fabricated 100% was
// indistinguishable from a genuine breach, and fired a spurious "Response
// SLA Violation" Chat alert for a case answered well within its window.
// business_duration/remaining_business_duration were never written by this
// query at all before -- both columns stayed NULL forever for every
// source='CSM' row, which task_sla_repo.go's read renders as an absent
// "Business elapsed time"/"Business time left" ("--" in the webapp) even
// for a clock that has since completed. Computing all three here means an
// early completion reads (and alerts) as what it actually was, on every
// column the UI shows.
//
// business_elapsed_percentage is no longer capped at 100 -- a clock
// completed well past its deadline (e.g. a support engineer replying long
// after a response SLA breached) now shows its real overrun (e.g. 134%)
// rather than an identical-looking 100%, matching RecomputeActive's own
// uncapped formula below so the two never disagree on a clock that was
// BREACHED right up until this call finalized it. Still floored at 0 --
// GREATEST(0, ...) -- for the same reason it always was: a clock processed
// a moment before its own start_on (clock skew, or a near-simultaneous
// register+complete) must not show a negative percentage.
//
// Matches on slaEngineOpenStageFilter, not slaEngineActiveStageFilter --
// see that constant's own doc comment: a BREACHED clock is exactly the
// case this needs to still match, so its real completing event (whenever
// it finally happens) finalizes it instead of silently matching zero rows.
// completedElapsedExpr is the wall-clock time between a clock's start and
// its completion MINUS every moment it spent paused along the way -- the
// same pause-subtraction sla_live and SetPaused's own pause-time freeze
// already apply, just evaluated at completion instead of at read time or
// pause time. Before this, CompleteClock used a bare "NOW() - s.start_on",
// completely ignoring s.pause_duration -- a real, reported bug: a clock
// paused for, say, two days and then resumed and completed showed a final
// business_duration/business_elapsed_percentage that counted those two
// paused days as elapsed working time. COALESCE(s.pause_duration, INTERVAL
// '0') covers every already-closed pause segment; the CASE covers the rare
// defensive case of a clock somehow still marked paused (paused_on IS NOT
// NULL) at the exact moment it's force-completed -- every real caller
// (ApplyStateEffects' CLOSED branch) already resumes a clock before
// completing it, but this makes the formula correct regardless of calling
// order, the same way reconcileClock's own pauseOffset does on the
// csm-notification-service side for the identical reason.
const completedElapsedExpr = `(NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0') -
		    CASE WHEN s.paused_on IS NOT NULL THEN NOW() - s.paused_on ELSE INTERVAL '0' END)`

// CompleteClock also finalizes any open pause segment (paused_on IS NOT
// NULL) into pause_duration and clears paused_on. ApplyStateEffects'
// CLOSED branch always resumes a clock before completing it, but that
// resume can itself fail and be logged-and-ignored -- this keeps a clock
// that reaches CompleteClock still paused from ending up ACHIEVED with
// paused_on left set, which would make GetClockState report a finished
// clock as paused with an incomplete pause total.
// CompleteClock dispatches to the flat wall-clock SQL path above for every
// 24x7 (or NULL/unsupported-schedule) clock, and to completeClockBusinessDays
// (internal/businessdays) for a clock backed by a 'WSO2 Business days'
// policy -- see that method's own doc comment for why this is done in Go
// rather than hand-rolled as a second SQL formula. completeClockFlat's own
// WHERE clause excludes 'WSO2 Business days' rows so the two paths can never
// both match the same row.
func (r *slaEngineRepo) CompleteClock(ctx context.Context, workItemID, target string) (bool, error) {
	ok, err := r.completeClockFlat(ctx, workItemID, target)
	if err != nil || ok {
		return ok, err
	}
	return r.completeClockBusinessDays(ctx, workItemID, target)
}

func (r *slaEngineRepo) completeClockFlat(ctx context.Context, workItemID, target string) (bool, error) {
	const query = `
		UPDATE sla s
		SET stage = 'ACHIEVED'::sla_stage_enum, end_on = NOW(), is_active = FALSE,
		    business_elapsed_percentage = GREATEST(0,
		        EXTRACT(EPOCH FROM ` + completedElapsedExpr + `) / NULLIF(EXTRACT(EPOCH FROM s.duration), 0) * 100
		    ),
		    business_duration = ` + completedElapsedExpr + `,
		    remaining_business_duration = GREATEST(s.duration - ` + completedElapsedExpr + `, INTERVAL '0'),
		    pause_duration = COALESCE(s.pause_duration, INTERVAL '0') +
		        CASE WHEN s.paused_on IS NOT NULL THEN NOW() - s.paused_on ELSE INTERVAL '0' END,
		    paused_on = NULL,
		    updated_on = NOW(), updated_by = $3
		FROM sla_policy sp
		WHERE s.sla_policy_id = sp.id
		  AND s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $2
		  AND COALESCE(sp.schedule, '') != '` + slaScheduleWSO2BusinessDays + `'
		  AND s.stage::TEXT ` + slaEngineOpenStageFilter

	tag, err := r.db.Exec(ctx, query, workItemID, target, sqlActorLiteral)
	if err != nil {
		return false, fmt.Errorf("complete csm sla clock: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// completeClockBusinessDays is CompleteClock's 'WSO2 Business days' branch:
// reads the one open row's raw instants under a row lock, computes its real
// elapsed/remaining business time in Go (internal/businessdays.Elapsed),
// and writes the result back -- the same final shape completeClockFlat
// writes via SQL, just computed with a calendar SQL has no notion of.
// Finalizes an open pause segment into both pause_duration (flat) and
// business_pause_duration (business-day), mirroring completedElapsedExpr's
// own CASE for the flat path.
func (r *slaEngineRepo) completeClockBusinessDays(ctx context.Context, workItemID, target string) (bool, error) {
	var found bool
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		var slaID string
		var startOn time.Time
		var pausedOn *time.Time
		var pauseSeconds, businessPauseSeconds, durationSeconds float64
		row := tx.QueryRow(ctx, `
			SELECT s.id, s.start_on, s.paused_on,
			       COALESCE(EXTRACT(EPOCH FROM s.pause_duration), 0),
			       COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0),
			       EXTRACT(EPOCH FROM s.duration)
			FROM sla s
			JOIN sla_policy sp ON sp.id = s.sla_policy_id
			WHERE s.work_item_id = $1::uuid
			  AND s.source = 'CSM'
			  AND sp.target::TEXT = $2
			  AND sp.schedule = '`+slaScheduleWSO2BusinessDays+`'
			  AND s.stage::TEXT `+slaEngineOpenStageFilter+`
			FOR UPDATE OF s`, workItemID, target)
		if err := row.Scan(&slaID, &startOn, &pausedOn, &pauseSeconds, &businessPauseSeconds, &durationSeconds); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("complete business-day sla clock: select: %w", err)
		}

		now := time.Now().UTC()
		flatPause := time.Duration(pauseSeconds * float64(time.Second))
		businessPause := time.Duration(businessPauseSeconds * float64(time.Second))
		if pausedOn != nil {
			// An open pause segment at completion time -- finalize it into
			// both totals, same as completedElapsedExpr's CASE does for the
			// flat path.
			flatPause += now.Sub(*pausedOn)
			businessPause += businessdays.Elapsed(*pausedOn, now)
		}

		elapsed := businessdays.Elapsed(startOn, now) - businessPause
		if elapsed < 0 {
			elapsed = 0
		}
		duration := time.Duration(durationSeconds * float64(time.Second))
		remaining := duration - elapsed
		if remaining < 0 {
			remaining = 0
		}
		pct := 0.0
		if durationSeconds > 0 {
			pct = elapsed.Seconds() / durationSeconds * 100
		}

		if _, err := tx.Exec(ctx, `
			UPDATE sla SET stage = 'ACHIEVED'::sla_stage_enum, end_on = NOW(), is_active = FALSE,
			    business_elapsed_percentage = $2,
			    business_duration = $3::interval,
			    remaining_business_duration = $4::interval,
			    pause_duration = $5::interval,
			    business_pause_duration = $6::interval,
			    paused_on = NULL,
			    updated_on = NOW(), updated_by = $7
			WHERE id = $1::uuid`,
			slaID, pct, formatIntervalLiteral(elapsed), formatIntervalLiteral(remaining),
			formatIntervalLiteral(flatPause), formatIntervalLiteral(businessPause), sqlActorLiteral,
		); err != nil {
			return fmt.Errorf("complete business-day sla clock: update: %w", err)
		}
		found = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

// SetPaused implements SLAEngineRepository.
//
// Matches on slaEngineOpenStageFilter, not slaEngineActiveStageFilter --
// a BREACHED clock must still be pausable (e.g. a workaround/resolution
// clock that breached while the case was still open, which then moves to
// AWAITING_INFO) so RecomputeActive actually stops touching it rather than
// continuing to climb its elapsed time while the case waits on the
// customer; see slaEngineOpenStageFilter's own doc comment.
//
// paused_on/pause_duration are now real, accumulated state -- not just a
// "currently paused" marker. Until this fix, pausing only ever reset
// paused_on to the current instant and resuming only ever cleared it back
// to NULL; pause_duration (the sla table's own column, since migration
// 0048 -- ServiceNow-synced rows already use it) was never written for a
// source='CSM' row at all, so there was no durable record of how long a
// clock had actually spent paused across one or more pause/resume cycles --
// only this process's own fleeting "is it paused right now," gone on
// restart, and nothing at all on csm-notification-service's side either
// (its own Redis clock hash has even less: a bare boolean, no timestamp --
// see that repo's own CLAUDE.md, "SLA breach-alerting engine"). One UPDATE
// now behaves correctly for every transition:
//   - pause -> pause (already PAUSED, paused again): paused_on is left as
//     it was (COALESCE keeps the existing value) rather than reset to NOW()
//     -- a duplicate/idempotent pause call (e.g. a redelivered
//     case.status_changed event) must not shrink the apparent pause
//     duration by silently moving the pause's own start time forward.
//   - resume (paused -> running): the elapsed pause interval
//     (NOW() - paused_on) is added to the running pause_duration total,
//     then paused_on is cleared -- both read the row's own pre-update
//     values, so there is no ordering hazard within the single statement.
//   - resume called on a row that was not actually paused (paused_on
//     already NULL): adds zero (NOW() - NULL is NULL, COALESCE falls back).
//
// business_pause_duration is deliberately left untouched -- this engine has
// no business-hours calendar (see RecomputeActive's own doc comment on its
// "flat wall-clock, no business hours" crudeness), so there is nothing
// correct to compute for it; it stays NULL for every source='CSM' row, same
// as before this fix.
//
// business_elapsed_percentage/business_duration/remaining_business_duration
// are now frozen at their REAL value the instant a clock pauses -- a real,
// reported gap with the identical shape CompleteClock's own doc comment
// already describes fixing for completion: this UPDATE used to leave all
// three completely untouched on pause, same as it always had, so a paused
// clock's displayed elapsed/remaining fell back to whatever was stored at
// registration (effectively 0%/the full duration) rather than the true
// elapsed time up to the moment it was paused -- confirmed live, a case
// moved to Awaiting Info minutes after its workaround/resolution clocks
// started showed "0 Minutes elapsed" / the full target duration still
// "remaining" for both, even though real working time had passed.
// sla_live only computes these three live for an IN_PROGRESS/BREACHED row
// (see that view's own doc comment); for PAUSED it just passes the stored
// columns through unchanged, so whatever this UPDATE writes here is exactly
// what the case detail's SLA tab, GET /sla-status and task_sla_repo.go's
// own reads will show for as long as the clock stays paused.
//
// The freeze point is COALESCE(s.paused_on, NOW()), not a bare NOW() --
// deliberately the SAME expression paused_on itself is computed from just
// above. A genuinely new pause (paused_on currently NULL) freezes at NOW(),
// same instant paused_on is set to. A duplicate/idempotent pause call on an
// already-PAUSED clock (e.g. a redelivered case.status_changed event,
// arbitrarily later in wall-clock time) must re-derive the IDENTICAL frozen
// values, not silently advance them as if the clock kept accumulating
// elapsed time while paused -- using the row's own already-set paused_on
// (now non-NULL, so COALESCE no longer falls back to NOW()) gives exactly
// that: the same frozen snapshot every time, however many times pause is
// redelivered. The elapsed math itself mirrors sla_live's own live_*
// formula, just evaluated at the pause instant instead of at read time:
// (freeze point) - start_on - the pause total ALREADY completed before this
// pause began (s.pause_duration, left unmodified by the pause branch above)
// -- never the segment this pause is itself only just starting.
func (r *slaEngineRepo) SetPaused(ctx context.Context, workItemID, target string, paused bool) (bool, error) {
	ok, err := r.setPausedFlat(ctx, workItemID, target, paused)
	if err != nil || ok {
		return ok, err
	}
	return r.setPausedBusinessDays(ctx, workItemID, target, paused)
}

func (r *slaEngineRepo) setPausedFlat(ctx context.Context, workItemID, target string, paused bool) (bool, error) {
	const query = `
		UPDATE sla s
		SET stage = CASE WHEN $3 THEN 'PAUSED'::sla_stage_enum ELSE 'IN_PROGRESS'::sla_stage_enum END,
		    pause_duration = CASE WHEN $3 THEN s.pause_duration
		                           ELSE COALESCE(s.pause_duration, INTERVAL '0') + COALESCE(NOW() - s.paused_on, INTERVAL '0')
		                      END,
		    paused_on = CASE WHEN $3 THEN COALESCE(s.paused_on, NOW()) ELSE NULL END,
		    business_elapsed_percentage = CASE WHEN $3 THEN GREATEST(0,
		            EXTRACT(EPOCH FROM (COALESCE(s.paused_on, NOW()) - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')))
		            / NULLIF(EXTRACT(EPOCH FROM s.duration), 0) * 100
		        )
		        ELSE s.business_elapsed_percentage
		    END,
		    business_duration = CASE WHEN $3 THEN GREATEST(
		            COALESCE(s.paused_on, NOW()) - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'), INTERVAL '0'
		        )
		        ELSE s.business_duration
		    END,
		    remaining_business_duration = CASE WHEN $3 THEN GREATEST(
		            s.duration - (COALESCE(s.paused_on, NOW()) - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')), INTERVAL '0'
		        )
		        ELSE s.remaining_business_duration
		    END,
		    updated_on = NOW(), updated_by = $4
		FROM sla_policy sp
		WHERE s.sla_policy_id = sp.id
		  AND s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND sp.target::TEXT = $2
		  AND COALESCE(sp.schedule, '') != '` + slaScheduleWSO2BusinessDays + `'
		  AND s.stage::TEXT ` + slaEngineOpenStageFilter

	tag, err := r.db.Exec(ctx, query, workItemID, target, paused, sqlActorLiteral)
	if err != nil {
		return false, fmt.Errorf("set csm sla clock paused: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// setPausedBusinessDays is SetPaused's 'WSO2 Business days' branch -- same
// transition shape as setPausedFlat (pause freezes the live figures at the
// pause instant and leaves them alone on every subsequent idempotent pause
// call; resume accumulates this segment's real elapsed business time into
// business_pause_duration/pause_duration and leaves the frozen figures
// alone), computed with internal/businessdays.Elapsed instead of flat SQL
// arithmetic.
func (r *slaEngineRepo) setPausedBusinessDays(ctx context.Context, workItemID, target string, paused bool) (bool, error) {
	var found bool
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		var slaID string
		var startOn time.Time
		var pausedOn *time.Time
		var pauseSeconds, businessPauseSeconds, durationSeconds float64
		row := tx.QueryRow(ctx, `
			SELECT s.id, s.start_on, s.paused_on,
			       COALESCE(EXTRACT(EPOCH FROM s.pause_duration), 0),
			       COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0),
			       EXTRACT(EPOCH FROM s.duration)
			FROM sla s
			JOIN sla_policy sp ON sp.id = s.sla_policy_id
			WHERE s.work_item_id = $1::uuid
			  AND s.source = 'CSM'
			  AND sp.target::TEXT = $2
			  AND sp.schedule = '`+slaScheduleWSO2BusinessDays+`'
			  AND s.stage::TEXT `+slaEngineOpenStageFilter+`
			FOR UPDATE OF s`, workItemID, target)
		if err := row.Scan(&slaID, &startOn, &pausedOn, &pauseSeconds, &businessPauseSeconds, &durationSeconds); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("set business-day sla clock paused: select: %w", err)
		}

		now := time.Now().UTC()
		flatPause := time.Duration(pauseSeconds * float64(time.Second))
		businessPause := time.Duration(businessPauseSeconds * float64(time.Second))
		duration := time.Duration(durationSeconds * float64(time.Second))

		var (
			newStage                            string
			newPausedOn                         *time.Time
			newFlatPause, newBusinessPause      time.Duration
			pct                                 *float64
			businessDuration, remainingDuration *time.Duration
		)
		if paused {
			newStage = "PAUSED"
			freeze := now
			if pausedOn != nil {
				freeze = *pausedOn // idempotent re-pause: re-derive the identical frozen snapshot
			}
			newPausedOn = &freeze
			newFlatPause = flatPause
			newBusinessPause = businessPause
			elapsed := businessdays.Elapsed(startOn, freeze) - businessPause
			if elapsed < 0 {
				elapsed = 0
			}
			remaining := duration - elapsed
			if remaining < 0 {
				remaining = 0
			}
			p := 0.0
			if durationSeconds > 0 {
				p = elapsed.Seconds() / durationSeconds * 100
			}
			pct = &p
			businessDuration = &elapsed
			remainingDuration = &remaining
		} else {
			newStage = "IN_PROGRESS"
			newPausedOn = nil
			newFlatPause = flatPause
			newBusinessPause = businessPause
			if pausedOn != nil {
				newFlatPause += now.Sub(*pausedOn)
				newBusinessPause += businessdays.Elapsed(*pausedOn, now)
			}
			// business_elapsed_percentage/business_duration/
			// remaining_business_duration are left untouched on resume --
			// same as setPausedFlat's own ELSE branches.
		}

		if _, err := tx.Exec(ctx, `
			UPDATE sla SET stage = $2::sla_stage_enum,
			    paused_on = $3,
			    pause_duration = $4::interval,
			    business_pause_duration = $5::interval,
			    business_elapsed_percentage = COALESCE($6, business_elapsed_percentage),
			    business_duration = COALESCE($7::interval, business_duration),
			    remaining_business_duration = COALESCE($8::interval, remaining_business_duration),
			    updated_on = NOW(), updated_by = $9
			WHERE id = $1::uuid`,
			slaID, newStage, newPausedOn, formatIntervalLiteral(newFlatPause), formatIntervalLiteral(newBusinessPause),
			pct, formatIntervalOrNil(businessDuration), formatIntervalOrNil(remainingDuration), sqlActorLiteral,
		); err != nil {
			return fmt.Errorf("set business-day sla clock paused: update: %w", err)
		}
		found = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

// formatIntervalOrNil is formatIntervalLiteral for an optional *time.Duration
// -- nil passes through as a real SQL NULL so the UPDATE's own
// COALESCE($n, column) keeps the column's existing value, used by
// setPausedBusinessDays' resume branch (which must not touch these three
// columns at all).
func formatIntervalOrNil(d *time.Duration) any {
	if d == nil {
		return nil
	}
	return formatIntervalLiteral(*d)
}

// RecomputeActive implements SLAEngineRepository.
//
// Scoped to stage IN ('IN_PROGRESS', 'BREACHED') -- deliberately still
// narrower than slaEngineOpenStageFilter, which would also match PAUSED: a
// paused clock's whole point is to stop accumulating elapsed time, so
// recomputing its percentage against wall-clock "now" would silently undo
// SetPaused's own effect the very next tick. Excluding PAUSED here is what
// makes pause actually pause.
//
// BREACHED is included here -- unlike an earlier version of this query,
// which stopped touching a row the moment it first flipped to BREACHED --
// specifically so a clock that has already breached keeps accumulating
// real elapsed time/percentage until its own genuine completing event
// (CompleteClock) finally finalizes it, however much later that is. That
// earlier version was a real, reported bug: a response SLA observed live
// froze at its business_duration/business_elapsed_percentage from the
// exact tick it crossed 100% (e.g. stuck at "59m" forever), never
// reflecting that real time kept passing while the case sat unanswered.
// business_elapsed_percentage is no longer capped at 100 for the same
// reason CompleteClock's own cap was dropped -- see that method's doc
// comment; a clock still climbs past 100% here for as long as it remains
// BREACHED, and CompleteClock later reads the exact same uncapped value at
// whatever moment it actually finishes. Still floored at 0 (GREATEST(0,
// ...)) for the same clock-skew reason CompleteClock keeps that floor.
//
// No business-hours calendar: elapsed is flat wall-clock time since
// start_on, exactly as crude as the deleted sla_clocks map this replaces
// (see internal/service/sla_policy_resolver.go's own doc comment) -- and,
// same as that old code, resuming a paused clock does NOT extend start_on
// or duration to account for the paused interval, so time spent paused is
// silently counted once the clock resumes. A known, accepted gap, not
// something this method works around.
func (r *slaEngineRepo) RecomputeActive(ctx context.Context) (int, error) {
	n, err := r.recomputeActiveFlat(ctx)
	if err != nil {
		return n, err
	}
	m, err := r.recomputeActiveBusinessDays(ctx)
	return n + m, err
}

// recomputeActiveFlat is RecomputeActive's original flat wall-clock path,
// now scoped away from 'WSO2 Business days' rows -- see
// recomputeActiveBusinessDays for those.
func (r *slaEngineRepo) recomputeActiveFlat(ctx context.Context) (int, error) {
	const query = `
		UPDATE sla s
		SET business_elapsed_percentage = GREATEST(0,
		        EXTRACT(EPOCH FROM (NOW() - s.start_on)) / NULLIF(EXTRACT(EPOCH FROM s.duration), 0) * 100
		    ),
		    business_duration = NOW() - s.start_on,
		    remaining_business_duration = GREATEST(s.duration - (NOW() - s.start_on), INTERVAL '0'),
		    has_breached = s.has_breached OR (EXTRACT(EPOCH FROM (NOW() - s.start_on)) >= EXTRACT(EPOCH FROM s.duration)),
		    stage = CASE
		        WHEN EXTRACT(EPOCH FROM (NOW() - s.start_on)) >= EXTRACT(EPOCH FROM s.duration)
		        THEN 'BREACHED'::sla_stage_enum
		        ELSE s.stage
		    END,
		    updated_on = NOW(), updated_by = $1
		FROM sla_policy sp
		WHERE s.sla_policy_id = sp.id
		  AND s.source = 'CSM'
		  AND s.stage IN ('IN_PROGRESS', 'BREACHED')
		  AND s.start_on IS NOT NULL
		  AND s.duration IS NOT NULL
		  AND COALESCE(sp.schedule, '') != '` + slaScheduleWSO2BusinessDays + `'`

	tag, err := r.db.Exec(ctx, query, sqlActorLiteral)
	if err != nil {
		return 0, fmt.Errorf("recompute csm sla clocks: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// recomputeActiveBusinessDays is RecomputeActive's 'WSO2 Business days'
// branch: PAUSED rows are excluded by the same IN_PROGRESS/BREACHED-only
// scope the flat path uses (see RecomputeActive's own doc comment on why --
// pause's whole point is to stop accumulating elapsed time), so every row
// here has no open pause segment to account for; only business_pause_duration
// (completed segments) is subtracted.
func (r *slaEngineRepo) recomputeActiveBusinessDays(ctx context.Context) (int, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id, s.start_on, COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0),
		       EXTRACT(EPOCH FROM s.duration), s.has_breached, s.stage::TEXT
		FROM sla s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE s.source = 'CSM'
		  AND s.stage IN ('IN_PROGRESS', 'BREACHED')
		  AND s.start_on IS NOT NULL
		  AND s.duration IS NOT NULL
		  AND sp.schedule = '`+slaScheduleWSO2BusinessDays+`'`)
	if err != nil {
		return 0, fmt.Errorf("recompute business-day sla clocks: select: %w", err)
	}
	type row struct {
		id                   string
		startOn              time.Time
		businessPauseSeconds float64
		durationSeconds      float64
		hasBreached          bool
		stage                string
	}
	var toUpdate []row
	for rows.Next() {
		var rr row
		if err := rows.Scan(&rr.id, &rr.startOn, &rr.businessPauseSeconds, &rr.durationSeconds, &rr.hasBreached, &rr.stage); err != nil {
			rows.Close()
			return 0, fmt.Errorf("recompute business-day sla clocks: scan: %w", err)
		}
		toUpdate = append(toUpdate, rr)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("recompute business-day sla clocks: rows: %w", err)
	}
	rows.Close()

	now := time.Now().UTC()
	n := 0
	for _, rr := range toUpdate {
		businessPause := time.Duration(rr.businessPauseSeconds * float64(time.Second))
		elapsed := businessdays.Elapsed(rr.startOn, now) - businessPause
		if elapsed < 0 {
			elapsed = 0
		}
		duration := time.Duration(rr.durationSeconds * float64(time.Second))
		remaining := duration - elapsed
		if remaining < 0 {
			remaining = 0
		}
		pct := 0.0
		if rr.durationSeconds > 0 {
			pct = elapsed.Seconds() / rr.durationSeconds * 100
		}
		breached := rr.hasBreached || elapsed >= duration
		stage := rr.stage
		if rr.stage == "IN_PROGRESS" && elapsed >= duration {
			stage = "BREACHED"
		}
		// Stage-guarded like the flat path's single UPDATE ... FROM above
		// (its WHERE re-checks s.stage IN (...) against the live row, not
		// just the snapshot the SELECT above already filtered on): the SELECT
		// and this UPDATE are two separate statements, so a CompleteClock,
		// SetPaused or CancelClock between them is entirely possible for a
		// clock that happens to tick over into that window -- without this
		// guard, that concurrent write would be silently reverted back to
		// IN_PROGRESS/BREACHED by a stale computation the moment this
		// transaction-less UPDATE lands. The stage condition here must stay
		// the exact two values rr.stage was read from in the SELECT above.
		tag, err := r.db.Exec(ctx, `
			UPDATE sla SET business_elapsed_percentage = $2, business_duration = $3::interval,
			    remaining_business_duration = $4::interval, has_breached = $5, stage = $6::sla_stage_enum,
			    updated_on = NOW(), updated_by = $7
			WHERE id = $1::uuid AND stage IN ('IN_PROGRESS', 'BREACHED')`,
			rr.id, pct, formatIntervalLiteral(elapsed), formatIntervalLiteral(remaining), breached, stage, sqlActorLiteral)
		if err != nil {
			return n, fmt.Errorf("recompute business-day sla clocks: update: %w", err)
		}
		n += int(tag.RowsAffected())
	}
	return n, nil
}

// ReviseClocks implements SLAEngineRepository.
//
// Cancels every clock whose CURRENT stage is cancel-eligible for its own
// target (the cancelQuery's own OR branches below) -- per-target, unlike
// slaEngineActiveStageFilter (which excludes BREACHED for every target
// uniformly): RESPONSE never cancels a BREACHED row (a first-reply window
// that already closed unanswered is permanent, exactly like ACHIEVED is
// permanent -- see slaEngineRevisionBlockStages' own doc comment), while
// WORKAROUND/RESOLUTION do cancel a BREACHED row, since ongoing remediation
// work genuinely restarts under the new severity's own duration. A
// severity change means every OTHER clock type starts over "as if the case
// had just been created at the new severity" (see SLAEngineService.
// ReviseCaseClocks's own doc comment) -- this scans every CSM row on the
// work item, not just the targets in `policies`, so a clock type no longer
// applicable after a severity DOWNGRADE (e.g. losing "workaround"/
// "resolution") is still cancelled even though `policies` won't re-register
// it. Also sets is_active = FALSE on every row it cancels -- see
// CompleteClock's own doc comment for the is_active bug this is the other
// half of the fix for.
// it.
func (r *slaEngineRepo) ReviseClocks(ctx context.Context, workItemID string, policies []SLAPolicyRef) (int, error) {
	const cancelQuery = `
		UPDATE sla s
		SET stage = 'CANCELLED'::sla_stage_enum, is_active = FALSE,
		    updated_on = NOW(), updated_by = $2
		FROM sla_policy sp
		WHERE s.sla_policy_id = sp.id
		  AND s.work_item_id = $1::uuid
		  AND s.source = 'CSM'
		  AND (
		    (sp.target::TEXT = 'RESPONSE' AND s.stage::TEXT IN ('IN_PROGRESS', 'PAUSED'))
		    OR (sp.target::TEXT != 'RESPONSE' AND s.stage::TEXT IN ('IN_PROGRESS', 'PAUSED', 'BREACHED'))
		  )`

	var cancelled int
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, cancelQuery, workItemID, sqlActorLiteral)
		if err != nil {
			return fmt.Errorf("revise csm sla clocks: cancel active: %w", err)
		}
		cancelled = int(tag.RowsAffected())

		for _, policy := range policies {
			if _, err := registerClockExec(ctx, tx, workItemID, policy); err != nil {
				return fmt.Errorf("revise csm sla clocks: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return cancelled, nil
}

// sqlActorLiteral is created_by/updated_by for every row this repository
// writes -- see domain.SLAEngineActor's own doc comment.
const sqlActorLiteral = "sla-engine"

// formatIntervalLiteral renders a time.Duration as a Postgres INTERVAL
// literal ("3600 seconds") -- binding a Go time.Duration directly as
// ::interval has no pgx encode plan registered in this codebase (same
// class of issue as comment_repo.go's ::text[]::comment_type_enum[] cast
// note), so this passes a plain numeric-seconds string instead, which
// Postgres's own ::interval cast parses natively.
func formatIntervalLiteral(d time.Duration) string {
	return fmt.Sprintf("%d seconds", int64(d.Seconds()))
}

// nullableSchedule returns schedule as a bindable value, or a real SQL NULL
// for an empty string -- sla.schedule is nullable and an empty string isn't
// a meaningful schedule value to store (every real sla_policy row's
// schedule is either a real value or NULL, never "").
func nullableSchedule(schedule string) any {
	if schedule == "" {
		return nil
	}
	return schedule
}

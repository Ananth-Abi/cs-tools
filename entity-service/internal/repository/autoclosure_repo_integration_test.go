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

// Auto-closure candidate selection, the claim, and the reset trigger
// (migration 0245) against a real Postgres, run as a non-superuser so
// row-level security is in force. Skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://csm_app:...@host/db go test -p 1 ./internal/repository/ ./internal/service/ -run Autoclosure
//
// Use -p 1 when running this package together with internal/service: the daily
// limit test counts audit rows database-wide, and the service package's tests
// write them too, so running both at once makes its count race.

package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const acPrefix = "92000000-0000-0000-0000-0000000000"

func acID(n int) string { return fmt.Sprintf("%s%02d", acPrefix, n) }

const (
	acStaffUserID  = "92000000-0000-0000-0000-0000000000a1"
	acSystemUserID = "92000000-0000-0000-0000-0000000000a2"
	acCustUserID   = "92000000-0000-0000-0000-0000000000a3"
	acStaffEmail   = "ac.staff@example.com"
	acSystemEmail  = "ac.system@example.com"
	acCustEmail    = "ac.customer@example.com"
)

type acFixture struct {
	t      *testing.T
	ctx    context.Context
	scoped *repository.Scoped
	now    time.Time
}

func (f *acFixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.scoped.Exec(f.ctx, sql, args...); err != nil {
		f.t.Fatalf("seed (%.70s): %v", sql, err)
	}
}

func newACFixture(t *testing.T, pool *pgxpool.Pool) *acFixture {
	t.Helper()
	f := &acFixture{t: t, ctx: repository.WithSystemIdentity(context.Background()), scoped: repository.NewScoped(pool), now: time.Now().UTC()}
	cleanup := func() {
		_, _ = f.scoped.Exec(f.ctx, `DELETE FROM work_item WHERE id::text LIKE $1`, acPrefix+"%")
		_, _ = f.scoped.Exec(f.ctx, `DELETE FROM "user" WHERE id::text LIKE $1`, acPrefix+"%")
	}
	cleanup()
	t.Cleanup(cleanup)
	for _, u := range []struct{ id, email, typ string }{
		{acStaffUserID, acStaffEmail, "INTERNAL"},
		{acSystemUserID, acSystemEmail, "SYSTEM"},
		{acCustUserID, acCustEmail, "EXTERNAL"},
	} {
		f.exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email, is_active)
			VALUES ($1, $2, $2, $3, $3, $3, TRUE)`, u.id, f.now, u.email)
		// user_type is derived from roles by a trigger that fires on insert, so
		// it is set afterwards; the update trigger watches is_system_user only.
		f.exec(`UPDATE "user" SET user_type = $2::user_type_enum WHERE id = $1`, u.id, u.typ)
	}
	return f
}

// ticket seeds one "case" row. daysAgo helpers keep the fixtures readable.
type acTicket struct {
	n          int
	state      string
	step       *string
	stateOn    *time.Time
	resolvedOn *time.Time
	parent     *string
	disabled   bool
}

func (f *acFixture) ago(days float64) *time.Time {
	v := f.now.Add(-time.Duration(days * 24 * float64(time.Hour)))
	return &v
}

func (f *acFixture) ticket(tk acTicket) string {
	f.t.Helper()
	id := acID(tk.n)
	f.exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, parent_id)
		VALUES ($1, $2, $2, 'test', 'test', $3, $3, 'autoclosure fixture', 'CASE'::work_item_type_enum, $4::uuid)`,
		id, f.now.Add(-60*24*time.Hour), fmt.Sprintf("AC-TEST-%04d", tk.n), tk.parent)
	f.exec(`INSERT INTO "case" (id, state, autoclosure_step, autoclosure_state_on, resolved_on, autoclose_enabled)
		VALUES ($1, $2::case_state_enum, $3, $4, $5, $6)`,
		id, tk.state, tk.step, tk.stateOn, tk.resolvedOn, !tk.disabled)
	return id
}

func (f *acFixture) comment(ticketID, by string, daysAgo float64) {
	f.t.Helper()
	f.exec(`INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
		VALUES (gen_random_uuid(), $1, $2, 'COMMENT', $3, 'fixture')`, f.ago(daysAgo), by, ticketID)
}

func acStr(s string) *string { return &s }

func acIDs(rows []repository.AutoclosureCandidateRow) map[string]bool {
	out := map[string]bool{}
	for _, r := range rows {
		out[r.ID] = true
	}
	return out
}

func (f *acFixture) find(repo repository.AutoclosureRepository, stage domain.AutoclosureStage, onlyID string) []repository.AutoclosureCandidateRow {
	f.t.Helper()
	rows, err := repo.FindCandidates(f.ctx, repository.AutoclosureCandidateQuery{
		Stage: stage, Now: f.now, IntervalDays: 7, Limit: 500,
		States: []string{"AWAITING_INFO", "SOLUTION_PROPOSED"}, SystemEmail: acSystemEmail, OnlyID: onlyID,
	})
	if err != nil {
		f.t.Fatalf("FindCandidates(%s): %v", stage, err)
	}
	return rows
}

func TestAutoclosureIntegration_FirstReminderSelection(t *testing.T) {
	pool := caseStatsPool(t)
	f := newACFixture(t, pool)
	repo := repository.NewAutoclosureRepository(f.scoped)

	due := map[string]string{
		"SP resolved 8 days ago":                        f.ticket(acTicket{n: 1, state: "SOLUTION_PROPOSED", resolvedOn: f.ago(8)}),
		"AI, WSO2 replied 9 days ago":                   f.ticket(acTicket{n: 3, state: "AWAITING_INFO"}),
		"AI, a customer comment must not count":         f.ticket(acTicket{n: 4, state: "AWAITING_INFO"}),
		"AI, the system user's reminder must not count": f.ticket(acTicket{n: 5, state: "AWAITING_INFO"}),
		"SP, step DEFAULT reset 8 days ago":             f.ticket(acTicket{n: 14, state: "SOLUTION_PROPOSED", step: acStr("DEFAULT"), stateOn: f.ago(8), resolvedOn: f.ago(20)}),
		"SP, parent whose only child is closed":         f.ticket(acTicket{n: 10, state: "SOLUTION_PROPOSED", resolvedOn: f.ago(20)}),
		"SP, empty-string step is no step":              f.ticket(acTicket{n: 15, state: "SOLUTION_PROPOSED", step: acStr(""), resolvedOn: f.ago(20)}),
	}
	f.comment(due["AI, WSO2 replied 9 days ago"], acStaffEmail, 9)
	f.comment(due["AI, a customer comment must not count"], acStaffEmail, 9)
	f.comment(due["AI, a customer comment must not count"], acCustEmail, 1)
	f.comment(due["AI, the system user's reminder must not count"], acStaffEmail, 9)
	f.comment(due["AI, the system user's reminder must not count"], acSystemEmail, 1)
	f.ticket(acTicket{n: 11, state: "CLOSED", parent: acStr(due["SP, parent whose only child is closed"])})

	notDue := map[string]string{
		"SP resolved 2 days ago":                    f.ticket(acTicket{n: 2, state: "SOLUTION_PROPOSED", resolvedOn: f.ago(2)}),
		"AI, WSO2 replied 2 days ago":               f.ticket(acTicket{n: 6, state: "AWAITING_INFO"}),
		"SP with an open child":                     f.ticket(acTicket{n: 7, state: "SOLUTION_PROPOSED", resolvedOn: f.ago(20)}),
		"SP, auto-closure switched off":             f.ticket(acTicket{n: 8, state: "SOLUTION_PROPOSED", resolvedOn: f.ago(20), disabled: true}),
		"a child ticket never has its own workflow": f.ticket(acTicket{n: 9, state: "SOLUTION_PROPOSED", resolvedOn: f.ago(20), parent: acStr(acID(7))}),
		"SP on hold":                                f.ticket(acTicket{n: 12, state: "SOLUTION_PROPOSED", resolvedOn: f.ago(20), step: acStr("ON_HOLD"), stateOn: f.ago(-5)}),
		"SP, step DEFAULT reset 1 day ago (fresh)":  f.ticket(acTicket{n: 13, state: "SOLUTION_PROPOSED", step: acStr("DEFAULT"), stateOn: f.ago(1), resolvedOn: f.ago(20)}),
		"SP, already at FIRST_COMMENT":              f.ticket(acTicket{n: 16, state: "SOLUTION_PROPOSED", resolvedOn: f.ago(20), step: acStr("FIRST_COMMENT"), stateOn: f.ago(20)}),
		"a ticket in a state outside the workflow":  f.ticket(acTicket{n: 17, state: "WORK_IN_PROGRESS", resolvedOn: f.ago(20)}),
	}
	f.comment(notDue["AI, WSO2 replied 2 days ago"], acStaffEmail, 2)
	f.ticket(acTicket{n: 18, state: "OPEN", parent: acStr(notDue["SP with an open child"])})

	got := acIDs(f.find(repo, domain.AutoclosureStageFirstReminder, ""))
	for name, id := range due {
		if !got[id] {
			t.Errorf("%s: should be due for the first reminder", name)
		}
	}
	for name, id := range notDue {
		if got[id] {
			t.Errorf("%s: must not be due for the first reminder", name)
		}
	}

	// Oldest reference first, so a capped run never starves the longest-waiting ticket.
	rows := f.find(repo, domain.AutoclosureStageFirstReminder, "")
	for i := 1; i < len(rows); i++ {
		if rows[i].ReferenceTime.Before(rows[i-1].ReferenceTime) {
			t.Fatalf("candidates are not ordered oldest first: %v before %v", rows[i-1].ReferenceTime, rows[i].ReferenceTime)
		}
	}
	// OnlyID narrows to exactly one ticket, using the same predicate.
	if one := f.find(repo, domain.AutoclosureStageFirstReminder, due["SP resolved 8 days ago"]); len(one) != 1 {
		t.Errorf("OnlyID of a due ticket returned %d rows, want 1", len(one))
	}
	if none := f.find(repo, domain.AutoclosureStageFirstReminder, notDue["SP resolved 2 days ago"]); len(none) != 0 {
		t.Errorf("OnlyID of a ticket that is not due returned %d rows, want 0", len(none))
	}
}

func TestAutoclosureIntegration_LaterStagesAndHoldRelease(t *testing.T) {
	pool := caseStatsPool(t)
	f := newACFixture(t, pool)
	repo := repository.NewAutoclosureRepository(f.scoped)

	second := f.ticket(acTicket{n: 1, state: "SOLUTION_PROPOSED", step: acStr("FIRST_COMMENT"), stateOn: f.ago(8)})
	secondNot := f.ticket(acTicket{n: 2, state: "SOLUTION_PROPOSED", step: acStr("FIRST_COMMENT"), stateOn: f.ago(3)})
	closeDue := f.ticket(acTicket{n: 3, state: "AWAITING_INFO", step: acStr("SECOND_COMMENT"), stateOn: f.ago(8)})
	closeNot := f.ticket(acTicket{n: 4, state: "AWAITING_INFO", step: acStr("SECOND_COMMENT"), stateOn: f.ago(6)})
	blocked := f.ticket(acTicket{n: 5, state: "AWAITING_INFO", step: acStr("SECOND_COMMENT"), stateOn: f.ago(9)})
	f.ticket(acTicket{n: 6, state: "OPEN", parent: acStr(blocked)})
	// A hold protects through the day it names: released the day after, not before.
	today := time.Date(f.now.Year(), f.now.Month(), f.now.Day(), 0, 0, 0, 0, time.UTC)
	heldOver := f.ticket(acTicket{n: 7, state: "SOLUTION_PROPOSED", step: acStr("ON_HOLD"), stateOn: acTime(today.AddDate(0, 0, -1))})
	heldToday := f.ticket(acTicket{n: 8, state: "SOLUTION_PROPOSED", step: acStr("ON_HOLD"), stateOn: &today})
	heldLater := f.ticket(acTicket{n: 9, state: "SOLUTION_PROPOSED", step: acStr("ON_HOLD"), stateOn: acTime(today.AddDate(0, 0, 14))})

	second2 := acIDs(f.find(repo, domain.AutoclosureStageSecondReminder, ""))
	if !second2[second] || second2[secondNot] || second2[closeDue] {
		t.Errorf("second reminder set wrong: %v", second2)
	}
	closing := acIDs(f.find(repo, domain.AutoclosureStageClose, ""))
	if !closing[closeDue] || closing[closeNot] || closing[blocked] || closing[second] {
		t.Errorf("close set wrong: %v", closing)
	}
	release := acIDs(f.find(repo, domain.AutoclosureStageReleaseHold, ""))
	if !release[heldOver] || release[heldToday] || release[heldLater] {
		t.Errorf("hold release set wrong: over=%v today=%v later=%v", release[heldOver], release[heldToday], release[heldLater])
	}
}

func acTime(t time.Time) *time.Time { return &t }

func TestAutoclosureIntegration_ClaimIsExclusiveAndRestorable(t *testing.T) {
	pool := caseStatsPool(t)
	f := newACFixture(t, pool)
	repo := repository.NewAutoclosureRepository(f.scoped)

	id := f.ticket(acTicket{n: 1, state: "SOLUTION_PROPOSED", resolvedOn: f.ago(10)})
	rows := f.find(repo, domain.AutoclosureStageFirstReminder, id)
	if len(rows) != 1 {
		t.Fatalf("candidate not found")
	}
	c := rows[0]
	now := f.now

	won, err := repo.ClaimStep(f.ctx, c, "FIRST_COMMENT", now)
	if err != nil || !won {
		t.Fatalf("first claim: won=%v err=%v", won, err)
	}
	// A second run holding the same stale snapshot must lose: this is what stops a duplicate reminder.
	won, err = repo.ClaimStep(f.ctx, c, "FIRST_COMMENT", now)
	if err != nil || won {
		t.Fatalf("second claim of the same snapshot: won=%v err=%v, want it refused", won, err)
	}
	// And it is no longer a first-reminder candidate.
	if len(f.find(repo, domain.AutoclosureStageFirstReminder, id)) != 0 {
		t.Fatalf("a claimed ticket is still due for the first reminder")
	}

	// A failed action puts the position back so the next run retries.
	if err := repo.RestoreStep(f.ctx, c, now); err != nil {
		t.Fatalf("RestoreStep: %v", err)
	}
	if len(f.find(repo, domain.AutoclosureStageFirstReminder, id)) != 1 {
		t.Fatalf("a restored ticket is not due again")
	}

	// A claim for a state the ticket has left is refused.
	f.exec(`UPDATE "case" SET state = 'WORK_IN_PROGRESS' WHERE id = $1`, id)
	won, err = repo.ClaimStep(f.ctx, c, "FIRST_COMMENT", now)
	if err != nil || won {
		t.Fatalf("claim after the ticket changed state: won=%v err=%v, want it refused", won, err)
	}
}

func TestAutoclosureIntegration_ResetTrigger(t *testing.T) {
	pool := caseStatsPool(t)
	f := newACFixture(t, pool)

	read := func(id string) (step *string, on *time.Time) {
		t.Helper()
		if err := f.scoped.QueryRow(f.ctx, `SELECT autoclosure_step, autoclosure_state_on FROM "case" WHERE id = $1`, id).Scan(&step, &on); err != nil {
			t.Fatalf("read: %v", err)
		}
		return
	}
	mid := func(n int, state string) string {
		return f.ticket(acTicket{n: n, state: state, step: acStr("SECOND_COMMENT"), stateOn: f.ago(30)})
	}

	next := 40
	for _, tc := range []struct {
		name        string
		from, to    string
		resets      bool
		asChildOf   bool
		stepSurvive string
	}{
		{"to Waiting on WSO2", "AWAITING_INFO", "WAITING_ON_WSO2", true, false, ""},
		{"to Reopened", "SOLUTION_PROPOSED", "REOPENED", true, false, ""},
		{"Awaiting Info to Solution Proposed", "AWAITING_INFO", "SOLUTION_PROPOSED", true, false, ""},
		{"Solution Proposed to Awaiting Info", "SOLUTION_PROPOSED", "AWAITING_INFO", true, false, ""},
		{"Awaiting Info to Closed does not reset", "AWAITING_INFO", "CLOSED", false, false, "SECOND_COMMENT"},
		{"Work in Progress to Awaiting Info does not reset", "WORK_IN_PROGRESS", "AWAITING_INFO", false, false, "SECOND_COMMENT"},
		{"a child is never reset", "AWAITING_INFO", "WAITING_ON_WSO2", false, true, "SECOND_COMMENT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := next
			next += 2
			id := mid(n, tc.from)
			if tc.asChildOf {
				parent := f.ticket(acTicket{n: n + 1, state: "WORK_IN_PROGRESS"})
				f.exec(`UPDATE work_item SET parent_id = $1 WHERE id = $2`, parent, id)
			}
			before := time.Now()
			f.exec(`UPDATE "case" SET state = $2::case_state_enum WHERE id = $1`, id, tc.to)
			step, on := read(id)
			if tc.resets {
				if step == nil || *step != "DEFAULT" || on == nil || on.Before(before.Add(-time.Minute)) {
					t.Errorf("not reset: step=%v on=%v", step, on)
				}
			} else if step == nil || *step != tc.stepSurvive {
				t.Errorf("step changed to %v, want %s kept", step, tc.stepSurvive)
			}
		})
	}

	// Rewriting the state to the value it already has is not a transition.
	id := mid(90, "AWAITING_INFO")
	f.exec(`UPDATE "case" SET state = 'AWAITING_INFO' WHERE id = $1`, id)
	if step, _ := read(id); step == nil || *step != "SECOND_COMMENT" {
		t.Errorf("a no-op state write reset the workflow: %v", step)
	}
}

func TestAutoclosureIntegration_AuditCountBacksTheDailyLimit(t *testing.T) {
	pool := caseStatsPool(t)
	f := newACFixture(t, pool)
	repo := repository.NewAutoclosureRepository(f.scoped)
	id := f.ticket(acTicket{n: 1, state: "SOLUTION_PROPOSED"})

	startOfDay := time.Date(f.now.Year(), f.now.Month(), f.now.Day(), 0, 0, 0, 0, time.UTC)
	count := func() int {
		n, err := repo.CountAdvancedSince(f.ctx, startOfDay)
		if err != nil {
			t.Fatalf("CountAdvancedSince: %v", err)
		}
		return n
	}
	base := count()

	record := func(stage domain.AutoclosureStage, outcome string) {
		t.Helper()
		if err := repo.RecordEvent(f.ctx, repository.AutoclosureEvent{WorkItemID: id, Stage: stage, Outcome: outcome}); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
	}
	record(domain.AutoclosureStageFirstReminder, "ADVANCED")
	record(domain.AutoclosureStageClose, "ADVANCED")
	record(domain.AutoclosureStageSecondReminder, "FAILED") // a failed attempt does not use the budget
	record(domain.AutoclosureStageReleaseHold, "ADVANCED")  // a release notifies nobody
	if got := count() - base; got != 2 {
		t.Errorf("counted %d, want 2 (two successes; a failure and a hold release are not counted)", got)
	}

	// Yesterday's work is not today's budget.
	f.exec(`UPDATE case_autoclosure_event SET created_on = created_on - INTERVAL '2 days' WHERE work_item_id = $1`, id)
	if got := count() - base; got != 0 {
		t.Errorf("counted %d after the events aged a day, want 0", got)
	}
}

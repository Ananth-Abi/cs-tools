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

package sweep

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/closure"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/notify"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/recipients"
)

func TestProcessProject_NoEndDateIsNoOp(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: nil}

	err := processProject(context.Background(), reader, updater, ntf, time.Now(), proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0", len(updater.calls))
	}
	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0", len(ntf.sent))
	}
}

// TestProcessProject_InternalOnlyWindowSkipsCustomerContactLookup covers a
// 90-day window: internal-only per the confirmed audience matrix. Only one
// notify.Send should occur, Recipients.Customer must stay nil, and no
// contact-search calls should happen at all, since the customer side isn't
// consulted for this window.
func TestProcessProject_InternalOnlyWindowSkipsCustomerContactLookup(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			t.Fatal("SearchProjectContacts should not be called for an internal-only window")
			return nil, nil
		},
		searchAccountContactsFn: func(ctx context.Context, accountID string, body []byte) ([]byte, error) {
			t.Fatal("SearchAccountContacts should not be called for an internal-only window")
			return nil, nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89) // fires the 90-day window
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 1 {
		t.Fatalf("ntf.sent = %d, want 1", len(ntf.sent))
	}
	if ntf.sent[0].Recipients.Customer != nil {
		t.Errorf("Recipients.Customer = %v, want nil for an internal-only window", ntf.sent[0].Recipients.Customer)
	}

	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1", len(updater.calls))
	}
	var body struct {
		SuspensionProcessState struct {
			BasedOnSubscriptionEndDate struct {
				EventType string `json:"event_type"`
			} `json:"based_on_subscription_end_date"`
		} `json:"suspensionProcessState"`
	}
	if err := json.Unmarshal(updater.calls[0].body, &body); err != nil {
		t.Fatalf("parse update body: %v", err)
	}
	if got := body.SuspensionProcessState.BasedOnSubscriptionEndDate.EventType; got != "90_days_notice" {
		t.Errorf("event_type = %q, want %q", got, "90_days_notice")
	}
}

// TestProcessProject_RecordsIgnoredWhenNotifierDoesNotDeliver verifies that
// recordNoticeSent writes "IGNORED" rather than "SUCCESSFUL" when the
// notifier in use doesn't actually deliver notices (as LoggingNotifier
// never does) — a real Send succeeding is not the same fact as a real email
// having been sent, and the recorded state must not claim otherwise.
func TestProcessProject_RecordsIgnoredWhenNotifierDoesNotDeliver(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{deliversFn: func() bool { return false }}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89) // fires the 90-day window
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1", len(updater.calls))
	}
	var body struct {
		SuspensionProcessState struct {
			BasedOnSubscriptionEndDate struct {
				ActionSendEmailNotification string `json:"actionSendEmailNotification"`
			} `json:"based_on_subscription_end_date"`
		} `json:"suspensionProcessState"`
	}
	if err := json.Unmarshal(updater.calls[0].body, &body); err != nil {
		t.Fatalf("parse update body: %v", err)
	}
	if got := body.SuspensionProcessState.BasedOnSubscriptionEndDate.ActionSendEmailNotification; got != "IGNORED" {
		t.Errorf("actionSendEmailNotification = %q, want %q", got, "IGNORED")
	}
}

// TestProcessProject_CustomerAudienceWindowNotifiesBusinessContact covers a
// 7-day window: both internal and customer per the confirmed audience
// matrix. A project contact with the business-contact role should populate
// Recipients.Customer on the single day-count reminder notice.
func TestProcessProject_CustomerAudienceWindowNotifiesBusinessContact(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			return []byte(`{"contacts":[{"name":"Bob","email":"bob@customer.example","roles":["business_contact"]}]}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 6) // fires the 7-day window
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 1 {
		t.Fatalf("ntf.sent = %d, want 1 (one consolidated reminder)", len(ntf.sent))
	}
	notice := ntf.sent[0]
	if notice.Recipients.Customer == nil {
		t.Fatal("Recipients.Customer = nil, want populated")
	}
	if notice.Recipients.Customer.Email != "bob@customer.example" {
		t.Errorf("Recipients.Customer.Email = %q, want %q", notice.Recipients.Customer.Email, "bob@customer.example")
	}
	if notice.ResolvedVia != recipients.ResolvedViaBusinessContact {
		t.Errorf("ResolvedVia = %q, want %q", notice.ResolvedVia, recipients.ResolvedViaBusinessContact)
	}

	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1", len(updater.calls))
	}
}

// TestProcessProject_CustomerAudienceWindowSendsNoBusinessContactNoticeWhenNoContactFound
// covers the three-tier fallback's last resort: no business contact, no
// primary contact. Both the day-count reminder (Customer nil — nothing
// resolved) and a separate no-business-contact urgent notice (to the
// Account Owner only) must be sent.
func TestProcessProject_CustomerAudienceWindowSendsNoBusinessContactNoticeWhenNoContactFound(t *testing.T) {
	reader := &mockEntityReader{
		getAccountFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"accountManager": {"id": "am-1", "name": "Jordan Perera", "email": "jordan.perera@wso2.example"}}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 6) // fires the 7-day window
	startDate := now.AddDate(-1, 0, 0)
	proj := project{
		ID:         "p1",
		Name:       "HFC Subscription - Subscription",
		ProjectKey: "HFCSUBS",
		Account:    &projectAccountRef{ID: "a1"},
		StartDate:  &startDate,
		EndDate:    &endDate,
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 2 {
		t.Fatalf("ntf.sent = %d, want 2 (reminder + no-business-contact)", len(ntf.sent))
	}

	reminder, nudge := ntf.sent[0], ntf.sent[1]

	if reminder.Recipients.Customer != nil {
		t.Errorf("reminder Recipients.Customer = %v, want nil", reminder.Recipients.Customer)
	}
	if reminder.Recipients.AccountOwner.Email != "jordan.perera@wso2.example" {
		t.Errorf("reminder Recipients.AccountOwner.Email = %q, want %q", reminder.Recipients.AccountOwner.Email, "jordan.perera@wso2.example")
	}

	const wantSubject = "[Urgent] [ACP] No Business Contacts Specified for Project HFC Subscription - Subscription"
	if nudge.Subject != wantSubject {
		t.Errorf("nudge Subject = %q, want %q", nudge.Subject, wantSubject)
	}
	if nudge.Recipients.AccountOwner.Email != "jordan.perera@wso2.example" {
		t.Errorf("nudge Recipients.AccountOwner.Email = %q, want %q", nudge.Recipients.AccountOwner.Email, "jordan.perera@wso2.example")
	}
	if nudge.Recipients.Customer != nil {
		t.Errorf("nudge Recipients.Customer = %v, want nil", nudge.Recipients.Customer)
	}
	if nudge.Body == "" {
		t.Error("nudge Body is empty, want the no-business-contact template populated")
	}
	wantBodyContains := []string{
		"Project Name: HFC Subscription - Subscription",
		"Project Key: HFCSUBS",
		"Account Owner: Jordan Perera",
	}
	for _, want := range wantBodyContains {
		if !strings.Contains(nudge.Body, want) {
			t.Errorf("nudge Body missing %q, got:\n%s", want, nudge.Body)
		}
	}
}

// TestProcessProject_CustomerAudienceWindowSkipsAccountContactLookupWhenNoAccount
// verifies that a project with no linked account never calls
// SearchAccountContacts. Calling it anyway with an empty account ID would hit
// SearchAccountContacts(ctx, "", ...), which fetchContacts must not do when
// there's no account to search — it should fall through to the
// no-business-contact notice exactly as it does when a real account search
// simply returns no contacts.
func TestProcessProject_CustomerAudienceWindowSkipsAccountContactLookupWhenNoAccount(t *testing.T) {
	reader := &mockEntityReader{
		searchAccountContactsFn: func(ctx context.Context, accountID string, body []byte) ([]byte, error) {
			t.Fatal("SearchAccountContacts should not be called when the project has no linked account")
			return nil, nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 6) // fires the 7-day window
	proj := project{ID: "p1", Account: nil, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 2 {
		t.Fatalf("ntf.sent = %d, want 2 (reminder + no-business-contact)", len(ntf.sent))
	}
	if ntf.sent[0].Recipients.Customer != nil {
		t.Errorf("reminder Recipients.Customer = %v, want nil (no account linked)", ntf.sent[0].Recipients.Customer)
	}
}

// TestProcessProject_NotifyFailureBlocksStateWrite verifies that when
// notify.Send fails, no suspensionProcessState write happens — leaving
// lastNoticeWindow unchanged so the same window is retried on the next run,
// with no separate FAILED-marker bookkeeping needed.
func TestProcessProject_NotifyFailureBlocksStateWrite(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{
		sendFn: func(ctx context.Context, n notify.Notice) error {
			return errors.New("smtp relay unreachable")
		},
	}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89) // fires the 90-day window (internal-only)
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err == nil {
		t.Fatal("processProject() error = nil, want non-nil")
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0", len(updater.calls))
	}
}

// TestProcessProject_Day0SuccessfulNotifyThenSuspend verifies the day-0
// ordering: when the final notice email succeeds, suspend is attempted
// afterward — two separate UpdateProject calls, notice-state first.
func TestProcessProject_Day0SuccessfulNotifyThenSuspend(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			return []byte(`{"contacts":[{"name":"Bob","email":"bob@customer.example","roles":["business_contact"]}]}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3) // 3 days past due
	open := "Open"
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate, ClosureState: &open}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 1 {
		t.Fatalf("ntf.sent = %d, want 1 (one consolidated reminder, with Customer resolved)", len(ntf.sent))
	}

	if len(updater.calls) != 2 {
		t.Fatalf("updater.calls = %d, want 2 (record notice, then suspend)", len(updater.calls))
	}

	var noticeBody struct {
		SuspensionProcessState struct {
			BasedOnSubscriptionEndDate struct {
				EventType string `json:"event_type"`
			} `json:"based_on_subscription_end_date"`
		} `json:"suspensionProcessState"`
	}
	if err := json.Unmarshal(updater.calls[0].body, &noticeBody); err != nil {
		t.Fatalf("parse first update body: %v", err)
	}
	if got := noticeBody.SuspensionProcessState.BasedOnSubscriptionEndDate.EventType; got != "suspend" {
		t.Errorf("first call event_type = %q, want %q", got, "suspend")
	}

	var suspendBody struct {
		EndDateClosureState string `json:"endDateClosureState"`
	}
	if err := json.Unmarshal(updater.calls[1].body, &suspendBody); err != nil {
		t.Fatalf("parse second update body: %v", err)
	}
	if suspendBody.EndDateClosureState != "Suspended" {
		t.Errorf("second call endDateClosureState = %q, want %q", suspendBody.EndDateClosureState, "Suspended")
	}
}

// TestProcessProject_Day0RetrySkipsNotifyWhenAlreadyRecorded covers the
// retry case: a prior run already recorded the terminal marker (email
// done), but suspend itself previously failed. This run should skip notify
// entirely and go straight to suspend.
func TestProcessProject_Day0RetrySkipsNotifyWhenAlreadyRecorded(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			t.Fatal("SearchProjectContacts should not be called when notify is already recorded")
			return nil, nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3)
	open := "Open"
	proj := project{
		ID:                     "p1",
		Account:                &projectAccountRef{ID: "a1"},
		EndDate:                &endDate,
		ClosureState:           &open,
		SuspensionProcessState: []byte(`{"based_on_subscription_end_date":{"event_type":"suspend"}}`),
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0 (notify already recorded)", len(ntf.sent))
	}
	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1 (suspend only)", len(updater.calls))
	}
	var suspendBody struct {
		EndDateClosureState string `json:"endDateClosureState"`
	}
	if err := json.Unmarshal(updater.calls[0].body, &suspendBody); err != nil {
		t.Fatalf("parse update body: %v", err)
	}
	if suspendBody.EndDateClosureState != "Suspended" {
		t.Errorf("endDateClosureState = %q, want %q", suspendBody.EndDateClosureState, "Suspended")
	}
}

// TestProcessProject_SuspendGuardSkipsAlreadySuspendedProject verifies the
// suspend guard: if endDateClosureState already reads "Suspended" (from this
// run's already-fetched data), no UpdateProject call happens at all for
// suspend. This is the field suspend() itself writes — closureState is a
// separate, derived roll-up field this code never sets, so the guard must
// not be keyed on it (confirmed bug: see EndDateClosureState's doc comment
// in types.go).
func TestProcessProject_SuspendGuardSkipsAlreadySuspendedProject(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3)
	suspended := "Suspended"
	proj := project{
		ID:                     "p1",
		Account:                &projectAccountRef{ID: "a1"},
		EndDate:                &endDate,
		EndDateClosureState:    &suspended,
		SuspensionProcessState: []byte(`{"based_on_subscription_end_date":{"event_type":"suspend"}}`),
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0 (already suspended, no-op)", len(updater.calls))
	}
}

// TestProcessProject_SuspendGuardSkipsWhenEndDateClosureStateIsClosed
// verifies the guard also treats "Closed" as already-handled, not just an
// exact "Suspended" match. Confirmed via a real suspended project fetched
// directly (Postman, project acac149b-eba1-4714-fcf5-f5dabad0cdb1,
// closureState="Suspended" but endDateClosureState="Closed") that this field
// can progress past "Suspended" via a process outside this component — an
// equality check against "Suspended" alone would miss this real case and
// re-suspend indefinitely.
func TestProcessProject_SuspendGuardSkipsWhenEndDateClosureStateIsClosed(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3)
	closed := "Closed"
	proj := project{
		ID:                     "p1",
		Account:                &projectAccountRef{ID: "a1"},
		EndDate:                &endDate,
		EndDateClosureState:    &closed,
		SuspensionProcessState: []byte(`{"based_on_subscription_end_date":{"event_type":"suspend"}}`),
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0 (already closed, no-op)", len(updater.calls))
	}
}

// TestProcessProject_SuspendProceedsWhenEndDateClosureStateIsExplicitlyOpen
// covers the explicit (not just nil) "Open" case — confirmed via real data
// that sibling closure-state dimensions come back as an explicit "Open"
// string, not a null, for an untouched project. This guards against a
// future casing/logic slip in the inequality check landing on the wrong
// side for this specific, real value.
func TestProcessProject_SuspendProceedsWhenEndDateClosureStateIsExplicitlyOpen(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			return []byte(`{"contacts":[{"name":"Bob","email":"bob@customer.example","roles":["business_contact"]}]}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3) // 3 days past due
	open := "Open"
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate, EndDateClosureState: &open}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 2 {
		t.Errorf("updater.calls = %d, want 2 (record notice, then suspend)", len(updater.calls))
	}
}

func TestProcessProject_NothingDueIsNoOp(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 200) // far beyond the 90-day window
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0", len(updater.calls))
	}
	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0", len(ntf.sent))
	}
}

// TestProcessProject_ReminderUsesRealAccountContacts is a regression test
// using the real GetAccount response shape confirmed via direct Postman
// testing against the dedicated test account: a populated accountManager,
// technicalOwner, and renewalAccountManager, all with real emails. The
// reminder notice's Recipients must carry all three, and GetAccount must be
// called with the project's account ID.
func TestProcessProject_ReminderUsesRealAccountContacts(t *testing.T) {
	const realGetAccountResponse = `{
		"id": "f213fdd1-1b4b-a650-a002-c9d3604bcbac",
		"name": "ACP Test Partner Account",
		"technicalOwner": {
			"id": "tech-1",
			"name": "Alex Fernando",
			"email": "alex.fernando@wso2.example"
		},
		"accountManager": {
			"id": "am-1",
			"name": "Jordan Perera",
			"email": "jordan.perera@wso2.example"
		},
		"renewalAccountManager": {
			"id": "ram-1",
			"name": "Sam Jayasuriya",
			"email": "sam.jayasuriya@wso2.example"
		}
	}`

	var gotAccountID string
	reader := &mockEntityReader{
		getAccountFn: func(ctx context.Context, id string) ([]byte, error) {
			gotAccountID = id
			return []byte(realGetAccountResponse), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89) // fires the 90-day (internal-only) window
	proj := project{
		ID:      "p1",
		Account: &projectAccountRef{ID: "f213fdd1-1b4b-a650-a002-c9d3604bcbac"},
		EndDate: &endDate,
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if gotAccountID != "f213fdd1-1b4b-a650-a002-c9d3604bcbac" {
		t.Errorf("GetAccount called with id = %q, want the project's account ID", gotAccountID)
	}
	if len(ntf.sent) != 1 {
		t.Fatalf("ntf.sent = %d, want 1", len(ntf.sent))
	}
	got := ntf.sent[0].Recipients
	if got.AccountOwner.Email != "jordan.perera@wso2.example" {
		t.Errorf("AccountOwner.Email = %q, want %q", got.AccountOwner.Email, "jordan.perera@wso2.example")
	}
	if got.RenewalManager.Email != "sam.jayasuriya@wso2.example" {
		t.Errorf("RenewalManager.Email = %q, want %q", got.RenewalManager.Email, "sam.jayasuriya@wso2.example")
	}
	if got.TechnicalOwner.Email != "alex.fernando@wso2.example" {
		t.Errorf("TechnicalOwner.Email = %q, want %q", got.TechnicalOwner.Email, "alex.fernando@wso2.example")
	}
}

// TestProcessProject_ReminderHasEmptyAccountOwnerEmailWhenNoAccountManager
// covers the legitimate-absence case: an account with no accountManager
// assigned at all (nested key entirely missing, not just empty). The
// reminder notice must still be sent, with an empty AccountOwner.Email —
// this is not an error, per recipients.AccountManagerEmail's contract.
func TestProcessProject_ReminderHasEmptyAccountOwnerEmailWhenNoAccountManager(t *testing.T) {
	reader := &mockEntityReader{
		getAccountFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"id": "a1", "name": "Some Account"}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89)
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(ntf.sent) != 1 {
		t.Fatalf("ntf.sent = %d, want 1", len(ntf.sent))
	}
	if got := ntf.sent[0].Recipients.AccountOwner.Email; got != "" {
		t.Errorf("AccountOwner.Email = %q, want \"\" (no account manager assigned)", got)
	}
}

// TestReminderSubject covers the subject-line template directly, confirmed
// against two real examples from Chamara: the [ACP] prefix applies only to
// the internal-only 90/60/30 windows, and ProjectName/AccountName sit in a
// fixed "for {ProjectName} of {AccountName}" shape.
func TestReminderSubject(t *testing.T) {
	tests := []struct {
		name        string
		window      closure.NoticeWindow
		projectName string
		accountName string
		want        string
	}{
		{
			name:        "90-day window is [ACP]-prefixed",
			window:      90,
			projectName: "TICKETNETWORK - Subscription",
			accountName: "TicketNetwork",
			want:        "[ACP] 90 Days Reminder of Project for TICKETNETWORK - Subscription of TicketNetwork",
		},
		{
			name:        "60-day window is [ACP]-prefixed",
			window:      60,
			projectName: "P",
			accountName: "A",
			want:        "[ACP] 60 Days Reminder of Project for P of A",
		},
		{
			name:        "30-day window is [ACP]-prefixed",
			window:      30,
			projectName: "P",
			accountName: "A",
			want:        "[ACP] 30 Days Reminder of Project for P of A",
		},
		{
			name:        "15-day window has no prefix",
			window:      15,
			projectName: "P",
			accountName: "A",
			want:        "15 Days Reminder of Project for P of A",
		},
		{
			name:        "7-day window has no prefix",
			window:      7,
			projectName: "P",
			accountName: "A",
			want:        "7 Days Reminder of Project for P of A",
		},
		{
			name:        "0-day window has no prefix",
			window:      0,
			projectName: "P",
			accountName: "A",
			want:        "0 Days Reminder of Project for P of A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reminderSubject(tt.window, tt.projectName, tt.accountName); got != tt.want {
				t.Errorf("reminderSubject(%v, %q, %q) = %q, want %q", tt.window, tt.projectName, tt.accountName, got, tt.want)
			}
		})
	}
}

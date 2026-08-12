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

// Package sweep orchestrates one full ACP evaluation pass: fetch open
// projects, decide what's due, resolve recipients, notify, and write back
// state. Dry-run is not a flag branched on inside this package — it is
// entirely a property of which projectUpdater/notifier implementation the
// caller injects (see types.go).
package sweep

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/closure"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/notify"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/recipients"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/suspensionstate"
)

// processProject evaluates and, if anything is due, acts on a single
// project. Notify happens before suspend is ever attempted, and an error
// from notify returns immediately — this ordering, not a separate flag, is
// what guarantees suspend never proceeds after a failed notify (the day-0
// "email first, stop on failure" contract).
func processProject(ctx context.Context, reader entityReader, updater projectUpdater, ntf notifier, now time.Time, proj project) error {
	if proj.EndDate == nil {
		return nil
	}

	lastWindow, err := suspensionstate.LastNoticeWindow(proj.SuspensionProcessState)
	if err != nil {
		return fmt.Errorf("sweep: parse suspensionProcessState for project %s: %w", proj.ID, err)
	}

	decision := closure.Decide(now, *proj.EndDate, lastWindow)
	if !decision.Fires {
		return nil
	}

	if decision.ShouldNotify {
		if err := notifyForWindow(ctx, reader, ntf, proj, decision.Window); err != nil {
			return fmt.Errorf("sweep: notify project %s: %w", proj.ID, err)
		}
		if err := recordNoticeSent(ctx, updater, proj, decision.Window, ntf.Delivers()); err != nil {
			return fmt.Errorf("sweep: record notice for project %s: %w", proj.ID, err)
		}
	}

	if decision.ShouldSuspend {
		if err := suspend(ctx, updater, proj); err != nil {
			return fmt.Errorf("sweep: suspend project %s: %w", proj.ID, err)
		}
	}

	return nil
}

// needsCustomerAudience reports whether window's confirmed audience matrix
// includes the customer, not just the Account Manager. 90/60/30 are
// internal-only; 15/7/0 are both.
func needsCustomerAudience(window closure.NoticeWindow) bool {
	switch window {
	case closure.NoticeWindow15, closure.NoticeWindow7, closure.NoticeWindow0:
		return true
	default:
		return false
	}
}

// internalNoticeSubject builds the always-sent internal (Account Owner/
// Renewal Manager/Technical Owner) notice's title line. Every internal copy
// is [ACP]-prefixed regardless of window — that prefix marks "this is the
// internal-audience copy," not "this is a 90/60/30 window" (confirmed
// against real examples from Chamara spanning 90 through 0; an earlier
// version of this function had the rule backwards). Day-0 gets distinct
// "suspension notice" wording instead of "N Days Reminder" — there's no
// "days remaining" left to report once a project is actually suspended.
func internalNoticeSubject(window closure.NoticeWindow, projectName, accountName string) string {
	if window == closure.NoticeWindow0 {
		return fmt.Sprintf("[ACP] Project Suspension Notice of %s of %s", projectName, accountName)
	}
	return fmt.Sprintf("[ACP] %d Days Reminder of Project for %s of %s", int(window), projectName, accountName)
}

// customerNoticeSubject builds the customer-facing notice's title line
// (15/7/0 only) — never [ACP]-prefixed, never names the account, and uses
// future tense ("Upcoming") before day-0 versus past tense once actually
// suspended.
func customerNoticeSubject(window closure.NoticeWindow, projectName string) string {
	if window == closure.NoticeWindow0 {
		return fmt.Sprintf("Project Suspension Notice - %s", projectName)
	}
	return fmt.Sprintf("Upcoming Project Suspension Notice - %s", projectName)
}

// internalReminderBodyTemplate is the day-count (90/60/30/15/7) internal
// notice body, confirmed verbatim from real examples Chamara shared. The
// greeting always names the Account Manager (%[1]s), regardless of which of
// the three internal recipients is actually reading their copy — not
// personalized per recipient. Dates are formatted 2006-01-02.
const internalReminderBodyTemplate = `Dear %[1]s

The following project has a non renewed contract. Please find the details below.

Project Name: %[2]s

Project Key: %[3]s

Account Owner: %[1]s

Start Date: %[4]s

End Date: %[5]s

Since projects needs contract renewal, kindly take the remedial actions to avoid any disruptions of subscription support. We appreciate your understanding and your prompt attention to this matter.

Best Regards,
WSO2 Team`

// internalSuspensionBodyTemplate is the day-0 internal notice body — past
// tense ("has been suspended"), asking for reinstatement rather than
// renewal.
const internalSuspensionBodyTemplate = `Dear %[1]s

The following project has been suspended due to non renewed contract. Kindly request that you take the appropriate action to reinitiate the suspended support account.

Project Name: %[2]s

Project Key: %[3]s

Account Owner: %[1]s

Start Date: %[4]s

End Date: %[5]s

Since the project is suspended, kindly take the remedial actions to reinstate the subscription support. We appreciate your prompt attention to this matter.

Best Regards,
WSO2 Team`

func internalNoticeBody(window closure.NoticeWindow, proj project, accountOwnerName string) string {
	template := internalReminderBodyTemplate
	if window == closure.NoticeWindow0 {
		template = internalSuspensionBodyTemplate
	}
	return fmt.Sprintf(template, accountOwnerName, proj.Name, proj.ProjectKey, formatDate(proj.StartDate), formatDate(proj.EndDate))
}

// customerUpcomingSuspensionBodyTemplate is the 15/7-day customer-facing
// body — future tense, no greeting (customer-facing notices never open with
// "Dear X", confirmed explicitly). Dates are formatted 01/02/2006 (US
// style), distinct from the internal bodies' 2006-01-02 — matches Chamara's
// real examples exactly.
const customerUpcomingSuspensionBodyTemplate = `We regret to inform you that the project %[1]s will be suspended on %[2]s due to non-renewal of the contracts upon the end of the previous subscription period.

Please ensure to take necessary actions on or before %[2]s to avoid service disruption. If you have any questions or need assistance, please contact your Account Manager or the WSO2 Customer Success Team.

Best Regards,
WSO2 Team`

// customerAlreadySuspendedBodyTemplate is the day-0 customer-facing body —
// past tense, once the project has actually been suspended.
const customerAlreadySuspendedBodyTemplate = `We trust this message finds you well. This project %[1]s was suspended %[2]s due to non-renewal of the contracts upon the end of the previous subscription period.

To avoid future suspensions, please ensure that all necessary actions are completed in a timely manner. If you have any questions or need assistance, please contact your Account Manager or the WSO2 Customer Success Team.

Best Regards,
WSO2 Team`

func customerNoticeBody(window closure.NoticeWindow, proj project) string {
	template := customerUpcomingSuspensionBodyTemplate
	if window == closure.NoticeWindow0 {
		template = customerAlreadySuspendedBodyTemplate
	}
	return fmt.Sprintf(template, proj.Name, formatDateUS(proj.EndDate))
}

// noBusinessContactBodyTemplate is the fixed body for the urgent
// no-business-contact notice, confirmed verbatim from a real existing
// notice Chamara shared. Dates are formatted 2006-01-02; Account Owner is
// the resolved Account Manager's *name*, not email.
const noBusinessContactBodyTemplate = `Internal - Customer Project without Business Contacts

Urgent reminder regarding the project %[1]s.

Please note that no Business Contacts was found for the project %[1]s. Immediate action is required to address this issue, as without a business contact, the customers will not receive essential notifications regarding their project suspension status. Additionally, this will lead to failures in further automated actions related to project suspension.

Please refer this document to Update Business Contact

Project Name: %[1]s
Project Key: %[2]s
Account Owner: %[3]s
Start Date: %[4]s
End Date: %[5]s`

func noBusinessContactBody(proj project, accountOwnerName string) string {
	return fmt.Sprintf(noBusinessContactBodyTemplate,
		proj.Name, proj.ProjectKey, accountOwnerName, formatDate(proj.StartDate), formatDate(proj.EndDate))
}

func formatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// formatDateUS formats a date 01/02/2006 (US style), for the customer-facing
// prose bodies specifically — distinct from formatDate's 2006-01-02, used
// everywhere else (internal bodies' "Start Date:"/"End Date:" fields).
func formatDateUS(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("01/02/2006")
}

func timeValue(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func accountName(proj project) string {
	if proj.Account == nil {
		return ""
	}
	return proj.Account.Name
}

// notifyForWindow sends the always-fired internal notice (Account Owner/
// Renewal Manager/Technical Owner) for every firing window, and, only for
// the customer-audience windows (15/7/0), a second, separate notice with
// entirely different subject/body: either the customer notice (Customer
// populated, Account Owner/Renewal Manager/Technical Owner copied alongside
// as context) when the three-tier contact fallback resolves, or the
// no-business-contact urgent notice (to all three internal recipients) when
// it doesn't. Internal and customer notices are always two separate Send
// calls, never merged into one — their content genuinely differs, not just
// their recipient list.
func notifyForWindow(ctx context.Context, reader entityReader, ntf notifier, proj project, window closure.NoticeWindow) error {
	contacts, err := resolveAccountContacts(ctx, reader, proj.accountID())
	if err != nil {
		return fmt.Errorf("resolve account contacts: %w", err)
	}

	internalRecipients := notify.Recipients{
		AccountOwner:   contacts.AccountOwner,
		RenewalManager: contacts.RenewalManager,
		TechnicalOwner: contacts.TechnicalOwner,
	}

	internalNotice := notify.Notice{
		ProjectID:   proj.ID,
		ProjectName: proj.Name,
		ProjectKey:  proj.ProjectKey,
		StartDate:   timeValue(proj.StartDate),
		EndDate:     timeValue(proj.EndDate),
		Window:      window,
		Subject:     internalNoticeSubject(window, proj.Name, accountName(proj)),
		Body:        internalNoticeBody(window, proj, contacts.AccountOwner.Name),
		Recipients:  internalRecipients,
	}

	if !needsCustomerAudience(window) {
		return ntf.Send(ctx, internalNotice)
	}

	if err := ntf.Send(ctx, internalNotice); err != nil {
		return fmt.Errorf("send internal notice: %w", err)
	}

	projectContacts, accountContactsList, err := fetchContacts(ctx, reader, proj)
	if err != nil {
		return err
	}
	resolution := recipients.ResolveCustomerContact(projectContacts, accountContactsList)

	if !resolution.NeedsAMNudge {
		return ntf.Send(ctx, notify.Notice{
			ProjectID:   proj.ID,
			ProjectName: proj.Name,
			ProjectKey:  proj.ProjectKey,
			StartDate:   timeValue(proj.StartDate),
			EndDate:     timeValue(proj.EndDate),
			Window:      window,
			Subject:     customerNoticeSubject(window, proj.Name),
			Body:        customerNoticeBody(window, proj),
			Recipients: notify.Recipients{
				AccountOwner:   contacts.AccountOwner,
				RenewalManager: contacts.RenewalManager,
				TechnicalOwner: contacts.TechnicalOwner,
				Customer:       resolution.CustomerContact,
			},
			ResolvedVia: resolution.ResolvedVia,
		})
	}

	return ntf.Send(ctx, notify.Notice{
		ProjectID:   proj.ID,
		ProjectName: proj.Name,
		ProjectKey:  proj.ProjectKey,
		StartDate:   timeValue(proj.StartDate),
		EndDate:     timeValue(proj.EndDate),
		Window:      window,
		Subject:     fmt.Sprintf("[Urgent] [ACP] No Business Contacts Specified for Project %s", proj.Name),
		Body:        noBusinessContactBody(proj, contacts.AccountOwner.Name),
		Recipients:  internalRecipients,
		ResolvedVia: resolution.ResolvedVia,
	})
}

// accountContacts is the three account-level people a day-count reminder's
// Recipients draws from.
type accountContacts struct {
	AccountOwner   recipients.Contact
	RenewalManager recipients.Contact
	TechnicalOwner recipients.Contact
}

// resolveAccountContacts fetches the account and extracts its Account
// Manager (Account Owner), Renewal Account Manager, and Technical Owner as
// Contacts. Each Contact's Email may legitimately be "" — no person
// assigned to that role, or one assigned with no email on file — mirroring
// recipients.AccountManagerEmail's existing treatment of that as a
// non-error state, now applied to all three roles. Returns the zero
// accountContacts (not an error) if the project has no linked account.
func resolveAccountContacts(ctx context.Context, reader entityReader, accountID string) (accountContacts, error) {
	if accountID == "" {
		return accountContacts{}, nil
	}

	raw, err := reader.GetAccount(ctx, accountID)
	if err != nil {
		return accountContacts{}, fmt.Errorf("get account: %w", err)
	}

	var acc accountDTO
	if err := json.Unmarshal(raw, &acc); err != nil {
		return accountContacts{}, fmt.Errorf("parse account: %w", err)
	}

	return accountContacts{
		AccountOwner:   contactFromPersonRef(acc.AccountManager),
		RenewalManager: contactFromPersonRef(acc.RenewalAccountManager),
		TechnicalOwner: contactFromPersonRef(acc.TechnicalOwner),
	}, nil
}

// contactFromPersonRef converts a possibly-nil personRefDTO into a
// recipients.Contact, reusing recipients.AccountManagerEmail's nil/no-email
// handling (role-agnostic despite the name) rather than duplicating it.
func contactFromPersonRef(p *personRefDTO) recipients.Contact {
	if p == nil {
		return recipients.Contact{}
	}
	ref := recipients.PersonRef{ID: p.ID, Name: p.Name, Email: p.Email}
	return recipients.Contact{Name: p.Name, Email: recipients.AccountManagerEmail(&ref)}
}

func fetchContacts(ctx context.Context, reader entityReader, proj project) ([]recipients.ProjectContact, []recipients.AccountContact, error) {
	pcRaw, err := reader.SearchProjectContacts(ctx, proj.ID, []byte(`{}`))
	if err != nil {
		return nil, nil, fmt.Errorf("search project contacts: %w", err)
	}
	var pcResp projectContactSearchResponse
	if err := json.Unmarshal(pcRaw, &pcResp); err != nil {
		return nil, nil, fmt.Errorf("parse project contacts: %w", err)
	}

	projectContacts := make([]recipients.ProjectContact, len(pcResp.Contacts))
	for i, c := range pcResp.Contacts {
		projectContacts[i] = recipients.ProjectContact{Name: c.Name, Email: c.Email, Roles: c.Roles}
	}

	if proj.accountID() == "" {
		return projectContacts, nil, nil
	}

	acRaw, err := reader.SearchAccountContacts(ctx, proj.accountID(), []byte(`{}`))
	if err != nil {
		return nil, nil, fmt.Errorf("search account contacts: %w", err)
	}
	var acResp accountContactSearchResponse
	if err := json.Unmarshal(acRaw, &acResp); err != nil {
		return nil, nil, fmt.Errorf("parse account contacts: %w", err)
	}

	accountContacts := make([]recipients.AccountContact, len(acResp.Contacts))
	for i, c := range acResp.Contacts {
		accountContacts[i] = recipients.AccountContact{Name: c.Name, Email: c.Email, IsPrimary: c.IsPrimary}
	}
	return projectContacts, accountContacts, nil
}

// recordNoticeSent writes the new window into suspensionProcessState's
// based_on_subscription_end_date key, preserving every other key untouched.
// actionSendEmailNotification records "SUCCESSFUL" only when delivered is
// true (the notifier in use actually sends real notices); otherwise it
// records "IGNORED" — the notice was logged, not sent, and the state must
// not claim a delivery that never happened.
func recordNoticeSent(ctx context.Context, updater projectUpdater, proj project, window closure.NoticeWindow, delivered bool) error {
	action := "IGNORED"
	if delivered {
		action = "SUCCESSFUL"
	}
	newState, err := suspensionstate.WithSubscriptionEndDateState(proj.SuspensionProcessState, window, map[string]string{
		"actionSendEmailNotification": action,
	})
	if err != nil {
		return fmt.Errorf("build suspensionProcessState: %w", err)
	}

	body, err := json.Marshal(map[string]json.RawMessage{"suspensionProcessState": newState})
	if err != nil {
		return fmt.Errorf("marshal update request: %w", err)
	}

	_, err = updater.UpdateProject(ctx, proj.ID, body)
	return err
}

// suspend writes endDateClosureState=Suspended, unless this dimension has
// already moved past its initial "Open" state (checked against the
// already-fetched endDateClosureState — no extra round-trip), mirroring
// legacy's checkForOpenProject guard. Guarding on "not Open" rather than "==
// Suspended" matters because endDateClosureState can progress further to
// "Closed" via a process outside this component (confirmed via a real
// suspended project, Postman, project
// acac149b-eba1-4714-fcf5-f5dabad0cdb1) — an equality check against
// "Suspended" alone would miss that real case and re-suspend indefinitely.
func suspend(ctx context.Context, updater projectUpdater, proj project) error {
	if proj.EndDateClosureState != nil && *proj.EndDateClosureState != "Open" {
		return nil
	}

	body, err := json.Marshal(map[string]string{"endDateClosureState": "Suspended"})
	if err != nil {
		return fmt.Errorf("marshal update request: %w", err)
	}

	_, err = updater.UpdateProject(ctx, proj.ID, body)
	return err
}

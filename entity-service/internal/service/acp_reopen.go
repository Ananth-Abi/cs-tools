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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// The ACP reopen check: when Salesforce reports a change to a project or an
// invoice, a project the Account Closure Process suspended goes back to Open
// once the reason it was suspended for is gone, as the ServiceNow process did.
//
//   - End-date track (end_date_closure_state CLOSED): reopens when the
//     project's end date is in the future again (a renewal moves it).
//   - Invoice track (invoice_due_date_closure_state SUSPENDED): reopens when
//     no invoice ACP counts is past its suspend date any more (paid).
//
// Only a track ACP suspended is touched: its suspensionProcessState section
// must carry event_type "suspend", which ACP writes when it suspends and a
// manual suspension never does. A reopened track gets event_type "open", so
// ACP's reminders start over for the new term. The Salesforce project's
// State__c goes back to Open only once the overall closure state is OPEN
// (every track, compliance included), and only then is project.reactivated
// published for the "Back to Open state" email.
//
// Everything is read live from Salesforce, as acp-closure-service reads it,
// so the answer is the one that service would give: the rules are its own,
// copied in acp_reopen_rules.go.

// ACPReopenSalesforce is the part of the REST sales-entity client the reopen
// check uses. Satisfied by *salesentity.Client.
type ACPReopenSalesforce interface {
	GetProject(ctx context.Context, id string) (salesentity.Project, error)
	GetCustomer(ctx context.Context, id string) (salesentity.Customer, error)
	GetInvoice(ctx context.Context, id string) (salesentity.Invoice, error)
	GetOpportunity(ctx context.Context, id string) (salesentity.Opportunity, error)
	LinkedOpportunitiesByProject(ctx context.Context, projectID string) ([]salesentity.LinkedOpportunity, error)
	LinkedOpportunitiesByOpportunity(ctx context.Context, opportunityID string) ([]salesentity.LinkedOpportunity, error)
	InvoicesByOpportunity(ctx context.Context, opportunityID string) ([]salesentity.Invoice, error)
	UpdateProjectStatus(ctx context.Context, projectID, status string) error
}

// ACPReopen bundles the dependencies of the reopen check. It is optional: a
// salesforceEventService without it never reopens anything, which is how
// CSM_ACP_REOPEN_ENABLED=false is realised in routes.go.
type ACPReopen struct {
	Projects    repository.ACPReopenRepository
	SalesEntity ACPReopenSalesforce
	// Publisher publishes project.reactivated (PROJECT_EVENT_HUB_TOPIC). nil:
	// the project is still reopened, no email is sent.
	Publisher EventPublisherService
	// DryRun decides and logs what it would do, and writes nothing
	// (CSM_ACP_REOPEN_DRY_RUN, default true).
	DryRun bool
	// TestProjectSfID, when set, limits the check to that one Salesforce
	// project; every other project is left alone
	// (CSM_ACP_REOPEN_TEST_PROJECT_SF_ID).
	TestProjectSfID string
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

func (a *ACPReopen) enabled() bool {
	return a != nil && a.Projects != nil && a.SalesEntity != nil
}

func (a *ACPReopen) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// WithACPReopen turns the reopen check on for a service built by one of the
// NewSalesforceEventService constructors and returns it.
func WithACPReopen(svc SalesforceEventService, reopen ACPReopen) SalesforceEventService {
	if s, ok := svc.(*salesforceEventService); ok {
		s.acpReopen = &reopen
	}
	return svc
}

// acpReopenTriggers are the event types after which a project may have
// changed in a way that clears a suspension. A DELETED record cannot.
func acpReopenTriggers(eventType string) bool {
	switch eventType {
	case domain.SalesforceEventCreated, domain.SalesforceEventUpdated, domain.SalesforceEventRestored:
		return true
	}
	return false
}

// reopenAfterProjectEvent runs the reopen check for the project a Project__c
// event names. It runs whether or not the Project ingest is on: the check
// reads Salesforce itself and needs nothing the ingest writes.
func (s *salesforceEventService) reopenAfterProjectEvent(ctx context.Context, req domain.SalesforceEventRequest) error {
	if !s.acpReopen.enabled() || !acpReopenTriggers(req.EventType) {
		return nil
	}
	return s.acpReopen.reopenProject(ctx, req.ReferenceID)
}

// reopenAfterInvoiceEvent runs the reopen check for every project linked to
// the opportunity of the invoice an Invoice__c event names.
func (s *salesforceEventService) reopenAfterInvoiceEvent(ctx context.Context, req domain.SalesforceEventRequest) error {
	if !s.acpReopen.enabled() || !acpReopenTriggers(req.EventType) {
		return nil
	}
	return s.acpReopen.reopenForInvoice(ctx, req.ReferenceID)
}

// reopenForInvoice finds the projects an invoice belongs to (invoice ->
// opportunity -> linked projects) and checks each. Every project is tried;
// the first error is returned so Service Bus redelivers the event, which is
// safe: a project already reopened has nothing left to do.
func (a *ACPReopen) reopenForInvoice(ctx context.Context, invoiceSfID string) error {
	inv, err := a.SalesEntity.GetInvoice(ctx, invoiceSfID)
	if err != nil {
		return err
	}
	oppID := strings.TrimSpace(derefString(inv.OpportunityID))
	if oppID == "" {
		return nil
	}
	links, err := a.SalesEntity.LinkedOpportunitiesByOpportunity(ctx, oppID)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var firstErr error
	for _, l := range links {
		projectSfID := salesforceID18(derefString(l.ProjectID))
		if projectSfID == "" || seen[projectSfID] {
			continue
		}
		seen[projectSfID] = true
		if err := a.reopenProject(ctx, projectSfID); err != nil {
			slog.ErrorContext(ctx, "acp reopen: check failed", "projectSfId", projectSfID, "invoiceSfId", invoiceSfID, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// acpTracks is which of a project's tracks ACP suspended.
type acpTracks struct {
	endDate bool
	invoice bool
}

// acpSuspendedTracks reads which tracks of p ACP suspended: the track's
// closure state is suspended and its suspensionProcessState section records
// ACP's suspension.
func acpSuspendedTracks(p domain.ACPReopenProject) (acpTracks, error) {
	endEvent, err := acpSectionEventType(p.SuspensionProcessState, acpEndDateSection)
	if err != nil {
		return acpTracks{}, err
	}
	invoiceEvent, err := acpSectionEventType(p.SuspensionProcessState, acpInvoiceSection)
	if err != nil {
		return acpTracks{}, err
	}
	end := p.EndDateClosureState != nil &&
		(*p.EndDateClosureState == "CLOSED" || *p.EndDateClosureState == "SUSPENDED")
	inv := p.InvoiceDueDateClosureState != nil && strings.HasPrefix(*p.InvoiceDueDateClosureState, "SUSPENDED")
	return acpTracks{
		endDate: end && endEvent == acpSuspendEventType,
		invoice: inv && invoiceEvent == acpSuspendEventType,
	}, nil
}

// reopenProject is the check for one project. It returns nil when there is
// nothing to do, and an error only when a retry can help (Salesforce or the
// database failed); bad data is logged and acknowledged.
func (a *ACPReopen) reopenProject(ctx context.Context, rawSfID string) error {
	sfID := salesforceID18(rawSfID)
	if sfID == "" {
		return nil
	}
	if a.TestProjectSfID != "" && !strings.EqualFold(salesforceID18(a.TestProjectSfID), sfID) {
		return nil
	}
	proj, err := a.Projects.GetProjectBySfID(ctx, sfID)
	if err != nil {
		return err
	}
	if proj == nil {
		return nil
	}
	suspended, err := acpSuspendedTracks(*proj)
	if err != nil {
		slog.WarnContext(ctx, "acp reopen: suspensionProcessState is not readable, leaving the project as it is",
			"projectId", proj.ID, "projectSfId", sfID, "err", err)
		return nil
	}
	if !suspended.endDate && !suspended.invoice {
		return nil
	}

	sp, err := a.SalesEntity.GetProject(ctx, sfID)
	if err != nil {
		return err
	}
	now := a.now()
	var customer *salesentity.Customer
	loadCustomer := func() (*salesentity.Customer, error) {
		if customer != nil {
			return customer, nil
		}
		customerID := strings.TrimSpace(derefString(sp.CustomerID))
		if customerID == "" {
			customer = &salesentity.Customer{}
			return customer, nil
		}
		c, err := a.SalesEntity.GetCustomer(ctx, customerID)
		if err != nil {
			return nil, err
		}
		customer = &c
		return customer, nil
	}

	var reopen acpTracks
	if suspended.endDate {
		reopen.endDate = a.endDateClear(ctx, proj.ID, sp, now)
	}
	if suspended.invoice {
		c, err := loadCustomer()
		if err != nil {
			return err
		}
		clear, err := a.invoiceTrackClear(ctx, proj.ID, sp, c, now)
		if err != nil {
			return err
		}
		reopen.invoice = clear
	}
	if !reopen.endDate && !reopen.invoice {
		slog.InfoContext(ctx, "acp reopen: still suspended, nothing to reopen",
			"projectId", proj.ID, "projectSfId", sfID,
			"endDateSuspended", suspended.endDate, "invoiceSuspended", suspended.invoice)
		return nil
	}
	if a.DryRun {
		slog.InfoContext(ctx, "acp reopen: dry run, would reopen",
			"projectId", proj.ID, "projectSfId", sfID,
			"endDate", reopen.endDate, "invoice", reopen.invoice)
		return nil
	}
	// The notice needs the account's people; read them before anything is
	// written, so a failure here is retried with nothing changed.
	c, err := loadCustomer()
	if err != nil {
		return err
	}

	var written domain.ACPReopenWrite
	plan := func(current domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error) {
		// Re-read under the row lock: ACP or a person may have changed the
		// project since it was read above.
		still, err := acpSuspendedTracks(current)
		if err != nil {
			return domain.ACPReopenWrite{}, false, err
		}
		w := domain.ACPReopenWrite{
			ReopenEndDate:          reopen.endDate && still.endDate,
			ReopenInvoice:          reopen.invoice && still.invoice,
			SuspensionProcessState: current.SuspensionProcessState,
		}
		if !w.ReopenEndDate && !w.ReopenInvoice {
			return w, false, nil
		}
		if w.ReopenEndDate {
			if w.SuspensionProcessState, err = acpWithSectionOpen(w.SuspensionProcessState, acpEndDateSection); err != nil {
				return domain.ACPReopenWrite{}, false, err
			}
		}
		if w.ReopenInvoice {
			if w.SuspensionProcessState, err = acpWithSectionOpen(w.SuspensionProcessState, acpInvoiceSection); err != nil {
				return domain.ACPReopenWrite{}, false, err
			}
		}
		written = w
		return w, true, nil
	}
	// Salesforce is set to Open inside the transaction, before the commit:
	// if it refuses, the database is rolled back and the event is retried.
	// If the commit fails after Salesforce accepted, the retry finds the
	// project still suspended in CSM and sets Salesforce to Open again, which
	// is harmless.
	beforeCommit := func(ctx context.Context, outcome domain.ACPReopenOutcome) error {
		if !outcome.Open {
			return nil
		}
		return a.SalesEntity.UpdateProjectStatus(ctx, sp.ID, salesentity.ProjectStatusOpen)
	}
	outcome, done, err := a.Projects.Reopen(ctx, proj.ID, plan, beforeCommit)
	if err != nil {
		return err
	}
	if !done {
		return nil
	}
	slog.InfoContext(ctx, "acp reopen: reopened",
		"projectId", proj.ID, "projectSfId", sfID,
		"endDate", written.ReopenEndDate, "invoice", written.ReopenInvoice, "projectOpen", outcome.Open)
	if outcome.Open {
		a.publishReactivated(ctx, *proj, sp, c, written, now)
	}
	return nil
}

// endDateClear reports whether the end-date track may reopen: the project's
// end date is set and in the future.
func (a *ACPReopen) endDateClear(ctx context.Context, projectID string, sp salesentity.Project, now time.Time) bool {
	end, err := acpParseDate(sp.EndDate)
	if err != nil {
		slog.WarnContext(ctx, "acp reopen: project end date is not a date, end-date track stays suspended",
			"projectId", projectID, "projectSfId", sp.ID, "err", err)
		return false
	}
	return end != nil && acpDaysBetween(now, *end) > 0
}

// invoiceTrackClear reads the project's invoices from Salesforce, as ACP
// reads them, and reports whether the invoice track may reopen. An invoice
// date that cannot be read keeps the track suspended (never reopen on data
// that cannot be checked).
func (a *ACPReopen) invoiceTrackClear(ctx context.Context, projectID string, sp salesentity.Project, c *salesentity.Customer, now time.Time) (bool, error) {
	start, err := acpParseDate(sp.StartDate)
	if err != nil {
		slog.WarnContext(ctx, "acp reopen: project start date is not a date, invoice track stays suspended",
			"projectId", projectID, "projectSfId", sp.ID, "err", err)
		return false, nil
	}
	isPartner := strings.EqualFold(strings.TrimSpace(derefString(c.AccountClassification)), "Partner")
	hasPrimaryPartner := strings.TrimSpace(derefString(c.PrimaryPartnerAccountID)) != ""

	links, err := a.SalesEntity.LinkedOpportunitiesByProject(ctx, sp.ID)
	if err != nil {
		return false, err
	}
	var invoices []acpInvoice
	seen := map[string]bool{}
	for _, l := range links {
		oppID := strings.TrimSpace(derefString(l.OpportunityID))
		if oppID == "" || seen[oppID] {
			continue
		}
		seen[oppID] = true
		opp, err := a.SalesEntity.GetOpportunity(ctx, oppID)
		if err != nil {
			return false, err
		}
		if !acpOpportunityCounts(opp.StageName, opp.EulaVersion.Value) {
			continue
		}
		eula := acpEULAVersionDecimal(opp.EulaVersion.Value)
		rows, err := a.SalesEntity.InvoicesByOpportunity(ctx, oppID)
		if err != nil {
			return false, err
		}
		for _, inv := range rows {
			parsed, ok := acpParseInvoice(inv, eula)
			if !ok {
				slog.WarnContext(ctx, "acp reopen: an invoice date is not a date, invoice track stays suspended",
					"projectId", projectID, "projectSfId", sp.ID, "invoiceSfId", derefString(inv.ID))
				return false, nil
			}
			invoices = append(invoices, parsed)
		}
	}
	return acpInvoiceTrackClear(now, invoices, start, isPartner, hasPrimaryPartner), nil
}

// acpParseInvoice reads the dates of one Salesforce invoice; ok is false when
// one of them is not a date.
func acpParseInvoice(inv salesentity.Invoice, eula float64) (acpInvoice, bool) {
	out := acpInvoice{Name: inv.Name, Classification: inv.Classification, EULAVersion: eula}
	var err error
	if out.InvoiceDate, err = acpParseDate(inv.InvoiceDate); err != nil {
		return acpInvoice{}, false
	}
	if out.DueDate, err = acpParseDate(inv.DueDate); err != nil {
		return acpInvoice{}, false
	}
	if out.PaidDate, err = acpParseDate(inv.PaidDate); err != nil {
		return acpInvoice{}, false
	}
	return out, true
}

// publishReactivated publishes project.reactivated for the "Back to Open
// state" email. Best effort: the project is already reopened, so a failure is
// logged (and recorded by the publisher), never returned.
func (a *ACPReopen) publishReactivated(ctx context.Context, p domain.ACPReopenProject, sp salesentity.Project, c *salesentity.Customer, w domain.ACPReopenWrite, now time.Time) {
	if a.Publisher == nil {
		slog.InfoContext(ctx, "acp reopen: no event publisher, no reactivation email", "projectId", p.ID)
		return
	}
	payload := buildProjectReactivatedPayload(p, sp, c, w, now)
	raw, err := json.Marshal(payload)
	if err != nil {
		slog.ErrorContext(ctx, "acp reopen: marshal project.reactivated", "projectId", p.ID, "err", err)
		return
	}
	if err := a.Publisher.Publish(ctx, events.TypeProjectReactivated, p.ID, raw); err != nil {
		slog.ErrorContext(ctx, "acp reopen: publish project.reactivated failed", "projectId", p.ID, "err", err)
	}
}

// buildProjectReactivatedPayload assembles the notice from the Salesforce
// project and account as they were read before the reopen; the name falls
// back to CSM's. The name is kept as it is, "[S]" included, as the
// ServiceNow email showed it: Salesforce removes the marker after the reopen.
func buildProjectReactivatedPayload(p domain.ACPReopenProject, sp salesentity.Project, c *salesentity.Customer, w domain.ACPReopenWrite, now time.Time) events.ProjectReactivatedPayload {
	name := strings.TrimSpace(derefString(sp.Name))
	if name == "" {
		name = p.Name
	}
	key := strings.TrimSpace(derefString(sp.Key))
	if key == "" {
		key = p.Key
	}
	payload := events.ProjectReactivatedPayload{
		ProjectID:     p.ID,
		ProjectSfID:   sp.ID,
		ProjectName:   name,
		ProjectKey:    key,
		StartDate:     acpDateOnly(sp.StartDate),
		EndDate:       acpDateOnly(sp.EndDate),
		Reasons:       []string{},
		Recipients:    []string{},
		ReactivatedOn: now.UTC().Format(time.RFC3339),
	}
	if payload.ProjectSfID == "" {
		payload.ProjectSfID = p.SfID
	}
	if w.ReopenEndDate {
		payload.Reasons = append(payload.Reasons, events.ProjectReactivatedReasonRenewal)
	}
	if w.ReopenInvoice {
		payload.Reasons = append(payload.Reasons, events.ProjectReactivatedReasonInvoicePaid)
	}
	if c == nil {
		return payload
	}
	if c.Owner != nil {
		payload.AccountOwnerName = strings.TrimSpace(derefString(c.Owner.Name))
		payload.AccountOwnerEmail = strings.TrimSpace(derefString(c.Owner.Email))
	}
	seen := map[string]bool{}
	add := func(email *string) {
		e := strings.TrimSpace(derefString(email))
		if e == "" || seen[strings.ToLower(e)] {
			return
		}
		seen[strings.ToLower(e)] = true
		payload.Recipients = append(payload.Recipients, e)
	}
	if c.Owner != nil {
		add(c.Owner.Email)
	}
	if c.RenewalManager != nil {
		add(c.RenewalManager.Email)
	}
	add(c.TechnicalOwner)
	return payload
}

// acpDateOnly is the date part of a Salesforce date or datetime, "" when blank.
func acpDateOnly(v *string) string {
	s := strings.TrimSpace(derefString(v))
	if len(s) > 10 {
		s = s[:10]
	}
	return s
}

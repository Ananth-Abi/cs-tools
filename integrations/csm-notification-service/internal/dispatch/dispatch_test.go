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

package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
)

type sentEmail struct {
	to       []string
	subject  string
	htmlBody string
}

type mockEmailSender struct {
	err   error
	calls []sentEmail
}

func (m *mockEmailSender) SendEmail(ctx context.Context, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error {
	m.calls = append(m.calls, sentEmail{to: to, subject: subject, htmlBody: htmlBody})
	return m.err
}

type sentChatAlert struct {
	product, title, shortDescription, portalURL string
}

type mockGoogleChatSender struct {
	err   error
	calls []sentChatAlert
}

func (m *mockGoogleChatSender) SendIncidentAlert(ctx context.Context, product, title, shortDescription, portalURL string) error {
	m.calls = append(m.calls, sentChatAlert{product, title, shortDescription, portalURL})
	return m.err
}

type sentCall struct {
	to, message string
}

type mockCallSender struct {
	err   error
	calls []sentCall
}

func (m *mockCallSender) MakeCall(ctx context.Context, to, message string) error {
	m.calls = append(m.calls, sentCall{to, message})
	return m.err
}

// mockLinkResolver defaults to resolving every recipient to the same fixed
// link (https://csm.example/cases/<caseID>) so tests that don't care about
// link resolution itself don't need their own resolver setup. linkFor, when
// set, lets a test give different recipients different links (e.g. to
// exercise groupByLink's grouping).
type mockLinkResolver struct {
	linkFor func(email string) string
	err     error

	gotEmails               []string
	gotProjectID, gotCaseID string
}

// CSMLink mirrors recipientlinks.Resolver.CSMLink's own shape closely enough
// for tests that check the Google Chat alert's portal link — a fixed base
// (no configuration plumbed through this mock) plus the caseID.
func (m *mockLinkResolver) CSMLink(caseID string) string {
	return "https://csm.example/cases/" + caseID
}

func (m *mockLinkResolver) ResolveLinks(ctx context.Context, emails []string, projectID, caseID string) ([]recipientlinks.RecipientLink, error) {
	m.gotEmails = emails
	m.gotProjectID = projectID
	m.gotCaseID = caseID
	if m.err != nil {
		return nil, m.err
	}
	links := make([]recipientlinks.RecipientLink, len(emails))
	for i, email := range emails {
		link := "https://csm.example/cases/" + caseID
		if m.linkFor != nil {
			link = m.linkFor(email)
		}
		links[i] = recipientlinks.RecipientLink{Email: email, CaseLink: link}
	}
	return links, nil
}

const testRecipient = "test-recipient@example.com"

func newTestDispatcher(email emailSender, chat googleChatSender, call callSender) *Dispatcher {
	return NewDispatcher(email, chat, call, &mockLinkResolver{}, false, nil, true, "", "")
}

func TestDispatcher_Handle_CaseCreated(t *testing.T) {
	mock := &mockEmailSender{}
	chat := &mockGoogleChatSender{}
	d := newTestDispatcher(mock, chat, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(mock.calls))
	}
	got := mock.calls[0]
	if len(got.to) != 1 || got.to[0] != testRecipient {
		t.Errorf("to = %v, want [%s]", got.to, testRecipient)
	}
	if !strings.Contains(got.subject, "Something broke") {
		t.Errorf("subject = %q, want it to contain the case title", got.subject)
	}
	if !strings.Contains(got.htmlBody, "Something broke") {
		t.Error("htmlBody does not contain the case title")
	}

	if len(chat.calls) != 1 {
		t.Fatalf("expected 1 Google Chat alert sent, got %d", len(chat.calls))
	}
	gotChat := chat.calls[0]
	if gotChat.title != "Something broke" || gotChat.shortDescription != "desc" || gotChat.portalURL != "https://csm.example/cases/CASE-1" {
		t.Errorf("unexpected SendIncidentAlert args: %+v", gotChat)
	}
}

// TestDispatcher_Handle_CaseCreated_ChatUsesDefaultProduct verifies
// case.created's Chat alert falls back to Dispatcher.defaultChatProduct when
// the payload omits product, the same fallback handleIncidentCreated uses.
func TestDispatcher_Handle_CaseCreated_ChatUsesDefaultProduct(t *testing.T) {
	chat := &mockGoogleChatSender{}
	d := NewDispatcher(&mockEmailSender{}, chat, &mockCallSender{}, &mockLinkResolver{}, false, nil, true, "api-manager", "")

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(chat.calls) != 1 || chat.calls[0].product != "api-manager" {
		t.Fatalf("expected the chat alert to use the default product, got %+v", chat.calls)
	}
}

// TestDispatcher_Handle_CaseCreated_ChatFailureStillSendsEmail verifies the
// two reactions are independent, the same as
// TestDispatcher_Handle_IncidentCreated_ChatFailureStillPlacesCall.
func TestDispatcher_Handle_CaseCreated_ChatFailureStillSendsEmail(t *testing.T) {
	mock := &mockEmailSender{}
	chat := &mockGoogleChatSender{err: errors.New("webhook unreachable")}
	d := newTestDispatcher(mock, chat, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected the chat error to propagate")
	}
	if len(mock.calls) != 1 {
		t.Fatal("expected the email to still be sent despite the chat failure")
	}
}

// TestDispatcher_Handle_EmailDebugMode_RedirectsToConfiguredRecipients
// verifies EMAIL_DEBUG_MODE: Handle still succeeds and still resolves
// recipient links against the event's real recipients (a broken link
// resolver should still surface as an error, debug mode or not), but
// SendEmail is called with emailDebugRecipients instead of the real
// resolved recipients.
func TestDispatcher_Handle_EmailDebugMode_RedirectsToConfiguredRecipients(t *testing.T) {
	mock := &mockEmailSender{}
	links := &mockLinkResolver{}
	debugRecipients := []string{"debug-1@example.com", "debug-2@example.com"}
	d := NewDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{}, links, true, debugRecipients, true, "", "")

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 email still sent in debug mode, got %d", len(mock.calls))
	}
	if got := mock.calls[0].to; len(got) != 2 || got[0] != "debug-1@example.com" || got[1] != "debug-2@example.com" {
		t.Errorf("to = %v, want the configured debug recipients %v", got, debugRecipients)
	}
	if len(links.gotEmails) != 1 || links.gotEmails[0] != testRecipient {
		t.Errorf("expected link resolution to still run against the real recipient in debug mode, gotEmails = %v", links.gotEmails)
	}
}

// TestDispatcher_Handle_EmailDebugMode_NoRecipientsConfigured_SkipsSend
// verifies that EMAIL_DEBUG_MODE=true with an empty EMAIL_DEBUG_RECIPIENTS
// (misconfigured) skips SendEmail entirely rather than calling it with zero
// recipients.
func TestDispatcher_Handle_EmailDebugMode_NoRecipientsConfigured_SkipsSend(t *testing.T) {
	mock := &mockEmailSender{}
	d := NewDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{}, &mockLinkResolver{}, true, nil, true, "", "")

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 0 {
		t.Fatalf("expected 0 emails sent with no debug recipients configured, got %d", len(mock.calls))
	}
}

func TestDispatcher_Handle_CommentAdded(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(mock.calls))
	}
	if !strings.Contains(mock.calls[0].htmlBody, "fixed it") {
		t.Error("htmlBody does not contain the comment text")
	}
}

// TestDispatcher_Handle_CommentAdded_LinksToCommentFragment verifies
// commentLinkFor's suffix actually reaches the rendered email: the "Add
// Comment" CTA must link to <resolved case link>#<commentId>, matching the
// CSM portal frontend's own comment-permalink format.
func TestDispatcher_Handle_CommentAdded_LinksToCommentFragment(t *testing.T) {
	mock := &mockEmailSender{}
	links := &mockLinkResolver{linkFor: func(string) string { return "https://csm.example.com/cases/CASE-1" }}
	d := NewDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{}, links, false, nil, true, "", "")

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	want := "https://csm.example.com/cases/CASE-1#C-1"
	if !strings.Contains(mock.calls[0].htmlBody, want) {
		t.Errorf("htmlBody does not contain the comment permalink %q", want)
	}
}

func TestDispatcher_Handle_StatusChanged(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","newStatus":"Work In Progress","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !strings.Contains(mock.calls[0].subject, "Work In Progress") {
		t.Errorf("subject = %q, want it to contain the new status", mock.calls[0].subject)
	}
}

func TestDispatcher_Handle_CaseAssigned(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.assigned","entityId":"CASE-1","payload":{"assignerName":"Assigner","assignerEmail":"assigner@example.com","projectId":"PROJ-1","caseId":"CASE-1","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !strings.Contains(mock.calls[0].htmlBody, "assigner@example.com") {
		t.Error("htmlBody does not contain the assigner's email")
	}
}

// TestDispatcher_Handle_TwoRecipientsTwoLinks_SendsTwoEmails is the core
// regression test for the recipientlinks migration: a comment-added event
// with two recipients that resolve to two different portal links must
// result in two separate SendEmail calls, each to only the recipient(s) who
// resolved to that link, each body carrying that link — not one shared
// email with one link for everyone.
func TestDispatcher_Handle_TwoRecipientsTwoLinks_SendsTwoEmails(t *testing.T) {
	mock := &mockEmailSender{}
	links := &mockLinkResolver{linkFor: func(email string) string {
		if email == "customer@acme.com" {
			return "https://customer.example.com/projects/PROJ-1/support/cases/CASE-1"
		}
		return "https://csm.example.com/cases/CASE-1"
	}}
	d := NewDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{}, links, false, nil, true, "", "")

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["customer@acme.com","agent@wso2.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 2 {
		t.Fatalf("expected 2 emails sent (one per resolved link), got %d", len(mock.calls))
	}
	byRecipient := make(map[string]sentEmail)
	for _, call := range mock.calls {
		if len(call.to) != 1 {
			t.Fatalf("each group should have exactly 1 recipient here, got %v", call.to)
		}
		byRecipient[call.to[0]] = call
	}
	customerEmail, ok := byRecipient["customer@acme.com"]
	if !ok {
		t.Fatal("no email sent to customer@acme.com")
	}
	if !strings.Contains(customerEmail.htmlBody, "https://customer.example.com/projects/PROJ-1/support/cases/CASE-1") {
		t.Errorf("customer email body = %q, want the customer portal link", customerEmail.htmlBody)
	}
	agentEmail, ok := byRecipient["agent@wso2.com"]
	if !ok {
		t.Fatal("no email sent to agent@wso2.com")
	}
	if !strings.Contains(agentEmail.htmlBody, "https://csm.example.com/cases/CASE-1") {
		t.Errorf("agent email body = %q, want the CSM portal link", agentEmail.htmlBody)
	}
}

// TestDispatcher_Handle_TwoRecipientsSameLink_SendsOneEmail proves
// groupByLink batches recipients sharing a resolved link into a single
// SendEmail call, rather than fanning out one call per recipient
// regardless of link.
func TestDispatcher_Handle_TwoRecipientsSameLink_SendsOneEmail(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["agent1@wso2.com","agent2@wso2.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 email sent (both recipients share a link), got %d", len(mock.calls))
	}
	if len(mock.calls[0].to) != 2 {
		t.Errorf("to = %v, want both recipients batched into the one call", mock.calls[0].to)
	}
}

// TestDispatcher_Handle_ResolveLinksFails_NoEmailSent verifies a
// recipientlinks failure (e.g. entity-service unreachable) fails the whole
// record rather than silently sending to a wrong/default link.
func TestDispatcher_Handle_ResolveLinksFails_NoEmailSent(t *testing.T) {
	mock := &mockEmailSender{}
	links := &mockLinkResolver{err: errors.New("entity-service unreachable")}
	d := NewDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{}, links, false, nil, true, "", "")

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","newStatus":"Open","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected the resolver error to propagate")
	}
	if len(mock.calls) != 0 {
		t.Error("SendEmail should not be called when link resolution fails")
	}
}

// TestDispatcher_Handle_EmptyRecipients exercises events.Validate, the only
// validation boundary this service has left (see Handle's doc comment) —
// Dispatcher.groupByLink has its own defensive backstop for the same case,
// but Validate should reject this before groupByLink is ever reached.
func TestDispatcher_Handle_EmptyRecipients(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","newStatus":"Open","recipients":[]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected an error when the payload's recipients list is empty")
	}
	if len(mock.calls) != 0 {
		t.Error("SendEmail should not be called when recipients is empty")
	}
}

func TestDispatcher_Handle_InvalidPayload_MissingRequiredField(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected an error for a payload missing newStatus")
	}
	if len(mock.calls) != 0 {
		t.Error("SendEmail should not be called for an invalid payload")
	}
}

func TestDispatcher_Handle_InvalidPayload_EntityIDMismatch(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-2","newStatus":"Open","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected an error when the payload's caseId disagrees with the envelope's entityId")
	}
	if len(mock.calls) != 0 {
		t.Error("SendEmail should not be called for a mismatched entityId/caseId")
	}
}

func TestDispatcher_Handle_UnknownType(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.deleted","entityId":"CASE-1","payload":{}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected an error for an unknown event type")
	}
}

func TestDispatcher_Handle_MalformedEnvelope(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`not json`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected an error for a malformed envelope")
	}
}

func TestDispatcher_Handle_SendFailurePropagates(t *testing.T) {
	mock := &mockEmailSender{err: context.DeadlineExceeded}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","newStatus":"Open","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected the underlying SendEmail error to propagate")
	}
}

const validIncidentRecord = `{"type":"incident.created","entityId":"INC-1","payload":{"product":"api-manager","title":"P1 outage","shortDescription":"Everything is down","incidentLink":"https://x/INC-1","callTo":"+15551234567"}}`

func TestDispatcher_Handle_IncidentCreated(t *testing.T) {
	chat := &mockGoogleChatSender{}
	call := &mockCallSender{}
	d := NewDispatcher(&mockEmailSender{}, chat, call, &mockLinkResolver{}, false, nil, true, "", "")

	record := eventbus.Record{Value: []byte(validIncidentRecord)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(chat.calls) != 1 {
		t.Fatalf("expected 1 Google Chat alert sent, got %d", len(chat.calls))
	}
	gotChat := chat.calls[0]
	if gotChat.product != "api-manager" || gotChat.title != "P1 outage" ||
		gotChat.shortDescription != "Everything is down" || gotChat.portalURL != "https://x/INC-1" {
		t.Errorf("unexpected SendIncidentAlert args: %+v", gotChat)
	}
	if len(call.calls) != 1 {
		t.Fatalf("expected 1 call placed, got %d", len(call.calls))
	}
	gotCall := call.calls[0]
	if gotCall.to != "+15551234567" {
		t.Errorf("call to = %q, want %q", gotCall.to, "+15551234567")
	}
	if !strings.Contains(gotCall.message, "P1 outage") || !strings.Contains(gotCall.message, "Everything is down") {
		t.Errorf("call message = %q, want it to mention the title and description", gotCall.message)
	}
}

func TestDispatcher_Handle_IncidentCreated_ChatFailureStillPlacesCall(t *testing.T) {
	chat := &mockGoogleChatSender{err: errors.New("webhook unreachable")}
	call := &mockCallSender{}
	d := NewDispatcher(&mockEmailSender{}, chat, call, &mockLinkResolver{}, false, nil, true, "", "")

	record := eventbus.Record{Value: []byte(validIncidentRecord)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected the chat error to propagate")
	}
	if len(call.calls) != 1 {
		t.Fatal("expected the call to still be placed despite the chat failure")
	}
}

func TestDispatcher_Handle_IncidentCreated_CallFailureStillSendsChat(t *testing.T) {
	chat := &mockGoogleChatSender{}
	call := &mockCallSender{err: errors.New("twilio unreachable")}
	d := NewDispatcher(&mockEmailSender{}, chat, call, &mockLinkResolver{}, false, nil, true, "", "")

	record := eventbus.Record{Value: []byte(validIncidentRecord)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected the call error to propagate")
	}
	if len(chat.calls) != 1 {
		t.Fatal("expected the chat alert to still be sent despite the call failure")
	}
}

// TestDispatcher_Handle_IncidentCreated_RetryDoesNotResendSucceededChannel is
// a regression test: eventbus.Consumer retries the whole Handle call on any
// error. Before the per-channel done-tracking existed, a persistently
// failing call would cause the chat alert to be resent on every retry too.
// It also covers the done-map cleanup on the final attempt: the call channel
// here never succeeds, so IsFinalAttempt (not "both succeeded") is what
// releases its and chat's tracking entries once eventbus.Consumer is done
// retrying — without that, they'd stay in d.done forever.
func TestDispatcher_Handle_IncidentCreated_RetryDoesNotResendSucceededChannel(t *testing.T) {
	chat := &mockGoogleChatSender{}
	call := &mockCallSender{err: errors.New("twilio unreachable")}
	d := NewDispatcher(&mockEmailSender{}, chat, call, &mockLinkResolver{}, false, nil, true, "", "")

	record := eventbus.Record{Topic: "case-events", Partition: 1, Offset: 42, Value: []byte(validIncidentRecord)}

	for attempt := 1; attempt <= 3; attempt++ {
		record.IsFinalAttempt = attempt == 3
		if err := d.Handle(context.Background(), record); err == nil {
			t.Fatalf("attempt %d: expected the call error to still propagate", attempt)
		}
	}

	if len(chat.calls) != 1 {
		t.Errorf("chat sent %d times across 3 retries, want exactly 1 (call kept failing, chat should not be resent)", len(chat.calls))
	}
	if len(call.calls) != 3 {
		t.Errorf("call attempted %d times across 3 retries, want 3 (the genuinely failing channel should keep retrying)", len(call.calls))
	}
	if len(d.done) != 0 {
		t.Errorf("done map should be empty after the final attempt, has %d entries (leaked tracking for a channel that never succeeded)", len(d.done))
	}
}

// TestDispatcher_Handle_IncidentCreated_ForgetsAfterFullSuccess is a
// regression test for the other direction: once both channels succeed
// (possibly across separate Handle calls), a later, unrelated record must
// not be affected by stale tracking, and re-processing the *same* record key
// again (e.g. after a restart-triggered redelivery) starts fresh.
func TestDispatcher_Handle_IncidentCreated_ForgetsAfterFullSuccess(t *testing.T) {
	chat := &mockGoogleChatSender{}
	call := &mockCallSender{}
	d := NewDispatcher(&mockEmailSender{}, chat, call, &mockLinkResolver{}, false, nil, true, "", "")

	record := eventbus.Record{Topic: "case-events", Partition: 1, Offset: 42, Value: []byte(validIncidentRecord)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(d.done) != 0 {
		t.Errorf("done map should be empty after full success, has %d entries", len(d.done))
	}
}

func TestDispatcher_Handle_IncidentCreated_BothFail(t *testing.T) {
	chat := &mockGoogleChatSender{err: errors.New("webhook unreachable")}
	call := &mockCallSender{err: errors.New("twilio unreachable")}
	d := NewDispatcher(&mockEmailSender{}, chat, call, &mockLinkResolver{}, false, nil, true, "", "")

	record := eventbus.Record{Value: []byte(validIncidentRecord)}

	err := d.Handle(context.Background(), record)
	if err == nil {
		t.Fatal("expected a combined error")
	}
	if !strings.Contains(err.Error(), "webhook unreachable") || !strings.Contains(err.Error(), "twilio unreachable") {
		t.Errorf("error = %q, want it to mention both underlying failures", err.Error())
	}
}

// TestDispatcher_Handle_IncidentCreated_UsesDefaultsWhenOmitted verifies a
// publisher that can't determine which Chat space or on-call number applies
// (e.g. entity-service) can omit product/callTo, and the Dispatcher's own
// configured defaults are used instead.
func TestDispatcher_Handle_IncidentCreated_UsesDefaultsWhenOmitted(t *testing.T) {
	chat := &mockGoogleChatSender{}
	call := &mockCallSender{}
	d := NewDispatcher(&mockEmailSender{}, chat, call, &mockLinkResolver{}, false, nil, true, "api-manager", "+15559998888")

	record := eventbus.Record{Value: []byte(`{"type":"incident.created","entityId":"INC-1","payload":{"title":"P1 outage","shortDescription":"Everything is down","incidentLink":"https://x/INC-1"}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(chat.calls) != 1 || chat.calls[0].product != "api-manager" {
		t.Fatalf("expected the chat alert to use the default product, got %+v", chat.calls)
	}
	if len(call.calls) != 1 || call.calls[0].to != "+15559998888" {
		t.Fatalf("expected the call to use the default callTo, got %+v", call.calls)
	}
}

// TestDispatcher_Handle_IncidentCreated_CallSendingDisabled verifies the
// CALL_SENDING_ENABLED killswitch: Handle still succeeds and the Google Chat
// alert still sends, but MakeCall is never invoked.
func TestDispatcher_Handle_IncidentCreated_CallSendingDisabled(t *testing.T) {
	chat := &mockGoogleChatSender{}
	call := &mockCallSender{}
	d := NewDispatcher(&mockEmailSender{}, chat, call, &mockLinkResolver{}, false, nil, false, "", "")

	record := eventbus.Record{Value: []byte(validIncidentRecord)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(chat.calls) != 1 {
		t.Errorf("expected the chat alert to still be sent, got %d calls", len(chat.calls))
	}
	if len(call.calls) != 0 {
		t.Errorf("expected MakeCall to never be invoked while disabled, got %d calls", len(call.calls))
	}
}

// TestDispatcher_Handle_IgnoresSLAEventTypes verifies that
// sla.clock.register/sla.tier_reached records — consumed by
// internal/slaengine's own consumer group, which shares this topic — are a
// silent no-op here, not an error. Erroring would burn this consumer's
// retries and dead-letter an event that was never broken.
func TestDispatcher_Handle_IgnoresSLAEventTypes(t *testing.T) {
	mock := &mockEmailSender{}
	chat := &mockGoogleChatSender{}
	call := &mockCallSender{}
	d := newTestDispatcher(mock, chat, call)

	records := []string{
		`{"type":"sla.clock.register","entityId":"CASE-1","payload":{"caseId":"CASE-1","durations":{"response":"2h"}}}`,
		`{"type":"sla.tier_reached","entityId":"CASE-1","payload":{"caseId":"CASE-1","clockType":"response","tier":"50"}}`,
	}
	for _, r := range records {
		if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(r)}); err != nil {
			t.Errorf("Handle(%s) error = %v, want nil", r, err)
		}
	}
	if len(mock.calls) != 0 || len(chat.calls) != 0 || len(call.calls) != 0 {
		t.Errorf("expected no notification sent, got email=%d chat=%d call=%d", len(mock.calls), len(chat.calls), len(call.calls))
	}
}

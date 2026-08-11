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

const testRecipient = "test-recipient@example.com"

func newTestDispatcher(email emailSender, chat googleChatSender, call callSender) *Dispatcher {
	return NewDispatcher(email, chat, call)
}

func TestDispatcher_Handle_CaseCreated(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"desc","caseLink":"https://x/CASE-1","commentLink":"https://x/CASE-1#c","recipients":["test-recipient@example.com"]}}`)}

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
}

func TestDispatcher_Handle_CommentAdded(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"CASE-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentLink":"https://x#c","caseLink":"https://x","recipients":["test-recipient@example.com"]}}`)}

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

func TestDispatcher_Handle_StatusChanged(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"caseId":"CASE-1","newStatus":"Work In Progress","caseLink":"https://x","commentLink":"https://x#c","recipients":["test-recipient@example.com"]}}`)}

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

	record := eventbus.Record{Value: []byte(`{"type":"case.assigned","entityId":"CASE-1","payload":{"assignerName":"Assigner","assignerEmail":"assigner@example.com","caseId":"CASE-1","caseLink":"https://x","commentLink":"https://x#c","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !strings.Contains(mock.calls[0].htmlBody, "assigner@example.com") {
		t.Error("htmlBody does not contain the assigner's email")
	}
}

// TestDispatcher_Handle_EmptyRecipients exercises events.Validate, the only
// validation boundary this service has left (see Handle's doc comment) —
// Dispatcher.send has its own defensive backstop for the same case, but
// Validate should reject this before send is ever reached.
func TestDispatcher_Handle_EmptyRecipients(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"caseId":"CASE-1","newStatus":"Open","caseLink":"https://x","commentLink":"https://x#c","recipients":[]}}`)}

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

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"caseId":"CASE-1","caseLink":"https://x","commentLink":"https://x#c","recipients":["test-recipient@example.com"]}}`)}

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

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"caseId":"CASE-2","newStatus":"Open","caseLink":"https://x","commentLink":"https://x#c","recipients":["test-recipient@example.com"]}}`)}

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

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"caseId":"CASE-1","newStatus":"Open","caseLink":"https://x","commentLink":"https://x#c","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected the underlying SendEmail error to propagate")
	}
}

const validIncidentRecord = `{"type":"incident.created","entityId":"INC-1","payload":{"product":"api-manager","title":"P1 outage","shortDescription":"Everything is down","incidentLink":"https://x/INC-1","callTo":"+15551234567"}}`

func TestDispatcher_Handle_IncidentCreated(t *testing.T) {
	chat := &mockGoogleChatSender{}
	call := &mockCallSender{}
	d := NewDispatcher(&mockEmailSender{}, chat, call)

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
	d := NewDispatcher(&mockEmailSender{}, chat, call)

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
	d := NewDispatcher(&mockEmailSender{}, chat, call)

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
	d := NewDispatcher(&mockEmailSender{}, chat, call)

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
	d := NewDispatcher(&mockEmailSender{}, chat, call)

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
	d := NewDispatcher(&mockEmailSender{}, chat, call)

	record := eventbus.Record{Value: []byte(validIncidentRecord)}

	err := d.Handle(context.Background(), record)
	if err == nil {
		t.Fatal("expected a combined error")
	}
	if !strings.Contains(err.Error(), "webhook unreachable") || !strings.Contains(err.Error(), "twilio unreachable") {
		t.Errorf("error = %q, want it to mention both underlying failures", err.Error())
	}
}

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

package notify

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/recipients"
)

type sendEmailCall struct {
	to, cc            []string
	subject, htmlBody string
}

type mockEmailSender struct {
	sendEmailFn func(ctx context.Context, to, cc []string, subject, htmlBody string) error
	calls       []sendEmailCall
}

func (m *mockEmailSender) SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error {
	m.calls = append(m.calls, sendEmailCall{to: to, cc: cc, subject: subject, htmlBody: htmlBody})
	if m.sendEmailFn != nil {
		return m.sendEmailFn(ctx, to, cc, subject, htmlBody)
	}
	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(&capturingHandler{})
}

// TestEmailNotifier_Send_InternalOnlyGoesToTo covers the no-Customer case
// (90/60/30 windows, and the no-business-contact notice): Account Owner/
// Renewal Manager/Technical Owner all go in "to", nothing in "cc".
func TestEmailNotifier_Send_InternalOnlyGoesToTo(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	err := n.Send(context.Background(), Notice{
		Subject: "subject",
		Body:    "body",
		Recipients: Recipients{
			AccountOwner:   recipients.Contact{Email: "am@wso2.com"},
			RenewalManager: recipients.Contact{Email: "rm@wso2.com"},
			TechnicalOwner: recipients.Contact{Email: "to@wso2.com"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("SendEmail calls = %d, want 1", len(sender.calls))
	}
	call := sender.calls[0]
	if len(call.cc) != 0 {
		t.Errorf("cc = %v, want empty (no customer in scope)", call.cc)
	}
	wantTo := map[string]bool{"am@wso2.com": true, "rm@wso2.com": true, "to@wso2.com": true}
	if len(call.to) != 3 {
		t.Fatalf("to = %v, want 3 recipients", call.to)
	}
	for _, addr := range call.to {
		if !wantTo[addr] {
			t.Errorf("unexpected to address %q", addr)
		}
	}
}

// TestEmailNotifier_Send_CustomerGoesToToInternalGoesToCC covers the
// resolved-customer case: the customer is the primary "to" recipient, the
// three internal people are copied via "cc".
func TestEmailNotifier_Send_CustomerGoesToToInternalGoesToCC(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	err := n.Send(context.Background(), Notice{
		Subject: "subject",
		Body:    "body",
		Recipients: Recipients{
			AccountOwner:   recipients.Contact{Email: "am@wso2.com"},
			RenewalManager: recipients.Contact{Email: "rm@wso2.com"},
			TechnicalOwner: recipients.Contact{Email: "to@wso2.com"},
			Customer:       &recipients.Contact{Email: "customer@wso2.com"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("SendEmail calls = %d, want 1", len(sender.calls))
	}
	call := sender.calls[0]
	if len(call.to) != 1 || call.to[0] != "customer@wso2.com" {
		t.Errorf("to = %v, want [customer@wso2.com]", call.to)
	}
	if len(call.cc) != 3 {
		t.Errorf("cc = %v, want 3 internal recipients", call.cc)
	}
}

// TestEmailNotifier_Send_OmitsEmptyEmails covers the legitimate-absence
// case: a role with no email on file must not produce an empty-string
// entry in the recipient list sent to the real API.
func TestEmailNotifier_Send_OmitsEmptyEmails(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	err := n.Send(context.Background(), Notice{
		Subject: "subject",
		Body:    "body",
		Recipients: Recipients{
			AccountOwner:   recipients.Contact{Email: "am@wso2.com"},
			RenewalManager: recipients.Contact{Email: ""},
			TechnicalOwner: recipients.Contact{Email: ""},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("SendEmail calls = %d, want 1", len(sender.calls))
	}
	if got := sender.calls[0].to; len(got) != 1 || got[0] != "am@wso2.com" {
		t.Errorf("to = %v, want [am@wso2.com] (empty emails dropped)", got)
	}
}

// TestEmailNotifier_Send_FiltersNonWSO2RecipientsByDefault is the core
// staging safeguard: per explicit direction from Rashmika, no real email
// should reach a non-WSO2 address unless explicitly allowed. Default
// (AllowNonWSO2Recipients: false) must drop the external address while
// still sending to the internal wso2.com one — mixing both in "to" so the
// send still happens (a customer-only scenario would leave "to" empty and
// skip entirely, which is TestEmailNotifier_Send_SkipsWhenNoValidRecipientsRemain's
// job to cover, not this test's).
func TestEmailNotifier_Send_FiltersNonWSO2RecipientsByDefault(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger()} // AllowNonWSO2Recipients defaults to false

	err := n.Send(context.Background(), Notice{
		Subject: "subject",
		Body:    "body",
		Recipients: Recipients{
			AccountOwner:   recipients.Contact{Email: "am@wso2.com"},
			RenewalManager: recipients.Contact{Email: "external-rm@external.example"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("SendEmail calls = %d, want 1", len(sender.calls))
	}
	call := sender.calls[0]
	if len(call.to) != 1 || call.to[0] != "am@wso2.com" {
		t.Errorf("to = %v, want [am@wso2.com] (external address filtered out)", call.to)
	}
}

// TestEmailNotifier_Send_AllowsNonWSO2RecipientsWhenExplicitlyEnabled
// verifies the opt-in override actually works — the intended production
// behavior once this is genuinely live.
func TestEmailNotifier_Send_AllowsNonWSO2RecipientsWhenExplicitlyEnabled(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	err := n.Send(context.Background(), Notice{
		Subject: "subject",
		Body:    "body",
		Recipients: Recipients{
			Customer: &recipients.Contact{Email: "customer@external.example"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("SendEmail calls = %d, want 1", len(sender.calls))
	}
	if got := sender.calls[0].to; len(got) != 1 || got[0] != "customer@external.example" {
		t.Errorf("to = %v, want [customer@external.example]", got)
	}
}

// TestEmailNotifier_Send_SkipsWhenNoValidRecipientsRemain covers the case
// where filtering (empty emails, or the WSO2-only staging safeguard) leaves
// zero "to" candidates — must skip cleanly (no error, no SendEmail call),
// not force a call the real API would reject anyway.
func TestEmailNotifier_Send_SkipsWhenNoValidRecipientsRemain(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger()} // AllowNonWSO2Recipients: false

	err := n.Send(context.Background(), Notice{
		Subject: "subject",
		Body:    "body",
		Recipients: Recipients{
			Customer: &recipients.Contact{Email: "customer@external.example"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(sender.calls) != 0 {
		t.Errorf("SendEmail calls = %d, want 0 (no valid recipients)", len(sender.calls))
	}
}

// TestEmailNotifier_Send_ConvertsBodyToHTML verifies the plain-text Body
// (blank-line paragraph breaks, as every existing notice template uses) is
// converted to simple HTML within the branded wrapper — line breaks become
// <br>, and any HTML-special characters in the content are escaped rather
// than passed through raw.
func TestEmailNotifier_Send_ConvertsBodyToHTML(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	err := n.Send(context.Background(), Notice{
		Subject: "subject",
		Body:    "Dear Team\n\nProject: A & B <Special>",
		Recipients: Recipients{
			AccountOwner: recipients.Contact{Email: "am@wso2.com"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("SendEmail calls = %d, want 1", len(sender.calls))
	}
	const wantBody = "Dear Team<br>\n<br>\nProject: A &amp; B &lt;Special&gt;"
	got := sender.calls[0].htmlBody
	if !strings.Contains(got, wantBody) {
		t.Errorf("htmlBody = %q, want it to contain %q", got, wantBody)
	}
}

// TestEmailNotifier_Send_WrapsBodyInBrandedTemplate verifies the branded
// WSO2 shell (logo, orange accent border, footer disclaimer) wraps every
// outgoing email, per Chamara's real examples — not just the plain
// paragraph-to-<br> conversion on its own.
func TestEmailNotifier_Send_WrapsBodyInBrandedTemplate(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	err := n.Send(context.Background(), Notice{
		Subject:    "subject",
		Body:       "body text",
		Recipients: Recipients{AccountOwner: recipients.Contact{Email: "am@wso2.com"}},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("SendEmail calls = %d, want 1", len(sender.calls))
	}
	got := sender.calls[0].htmlBody

	if !strings.Contains(got, "data:image/png;base64,") {
		t.Error("htmlBody missing the embedded WSO2 logo (data:image/png;base64,...)")
	}
	if !strings.Contains(got, "This automated message was sent by WSO2's support system. Please do not reply to this email.") {
		t.Error("htmlBody missing the standard footer disclaimer")
	}
	if !strings.Contains(got, "body text") {
		t.Error("htmlBody missing the actual notice body content")
	}
}

// TestEmailNotifier_Send_PropagatesSenderError verifies a real send
// failure surfaces as an error rather than being swallowed.
func TestEmailNotifier_Send_PropagatesSenderError(t *testing.T) {
	sender := &mockEmailSender{
		sendEmailFn: func(ctx context.Context, to, cc []string, subject, htmlBody string) error {
			return errors.New("upstream unavailable")
		},
	}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	err := n.Send(context.Background(), Notice{
		Subject:    "subject",
		Body:       "body",
		Recipients: Recipients{AccountOwner: recipients.Contact{Email: "am@wso2.com"}},
	})
	if err == nil {
		t.Fatal("Send() error = nil, want non-nil")
	}
}

// TestEmailNotifier_Delivers_ReturnsTrue distinguishes EmailNotifier from
// LoggingNotifier for recordNoticeSent's actionSendEmailNotification
// marker: this one genuinely attempts real delivery.
func TestEmailNotifier_Delivers_ReturnsTrue(t *testing.T) {
	n := &EmailNotifier{}
	if !n.Delivers() {
		t.Error("Delivers() = false, want true")
	}
}

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
	_ "embed"
	"encoding/base64"
	"fmt"
	"html"
	"log/slog"
	"strings"
)

// wso2LogoPNG is the real WSO2 logo (circular pulse icon + wordmark),
// embedded directly into the compiled binary so it ships with the code —
// no external asset host, no broken image if that host is ever unreachable
// from wherever the email is eventually read. Provided by Chamara,
// confirmed as the same mark used in the real notice examples this
// template is built from.
//
//go:embed assets/wso2-logo.png
var wso2LogoPNG []byte

// wso2LogoDataURI is wso2LogoPNG pre-encoded as a data: URI once at
// package init, ready to drop straight into an <img src="...">. Computed
// here rather than per-send since the underlying bytes never change.
var wso2LogoDataURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(wso2LogoPNG)

// emailHTMLTemplate is the branded shell every outgoing notice is wrapped
// in — logo header, orange accent border around the message body, and the
// standard disclaimer footer — matching real examples Chamara shared
// (screenshots of actual received notices), not a from-scratch design.
// Two placeholders: the logo's data URI, then the notice body already
// converted to simple HTML by plainTextToHTML.
const emailHTMLTemplate = `<div style="background-color:#fdece2;padding:32px 16px;font-family:Arial,Helvetica,sans-serif;">
  <div style="max-width:600px;margin:0 auto;background-color:#ffffff;border-radius:4px;overflow:hidden;">
    <div style="padding:24px 32px 16px 32px;">
      <img src="%s" alt="WSO2" height="28" style="display:block;">
    </div>
    <div style="padding:8px 32px 24px 32px;">
      <div style="border-left:4px solid #ff7300;padding:4px 0 4px 16px;color:#333333;font-size:14px;line-height:1.6;">
        %s
      </div>
    </div>
    <div style="padding:16px 32px;border-top:1px solid #eeeeee;color:#888888;font-size:11px;">
      This automated message was sent by WSO2's support system. Please do not reply to this email.
    </div>
  </div>
</div>`

// emailSender is the minimal send surface EmailNotifier needs. Satisfied by
// *emailservice.Client — declared locally, not imported, so this package
// doesn't need to depend on emailservice's concrete type.
type emailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// EmailNotifier sends real emails via WSO2's internal email notification
// service (internal/emailservice), replacing LoggingNotifier once real
// sending is actually wanted. Recipients map onto the real API's "to"/"cc"
// fields based on which Recipients fields are populated: when Customer is
// present, the customer is the primary "to" recipient and the three
// internal people (Account Owner/Renewal Manager/Technical Owner) are
// copied via "cc"; otherwise (internal-only notices, and the
// no-business-contact notice) all populated internal recipients go in "to".
type EmailNotifier struct {
	Sender emailSender
	Logger *slog.Logger
	// AllowNonWSO2Recipients, when false (the safe default), filters out
	// any recipient whose address doesn't end in "@wso2.com" before
	// sending. This is a hard requirement from Rashmika's team (owners of
	// the email service): a staging/testing environment must never
	// actually email a real customer contact. Only set true in a genuine
	// production environment, once that's a deliberate decision — not
	// something to flip casually to "make a test work".
	AllowNonWSO2Recipients bool
}

// Send builds the to/cc recipient lists, converts Body to simple HTML, and
// calls the real email service. If every recipient is filtered out (empty
// email, or a non-WSO2 address with AllowNonWSO2Recipients false) — or
// there were never any recipients to begin with — this logs and returns
// nil rather than forcing a call the real API would reject anyway (it
// requires at least one "to" address); a project with nobody to notify is
// a legitimate, unremarkable state, not an error, matching the convention
// already established throughout this codebase for absent recipients.
func (n *EmailNotifier) Send(ctx context.Context, notice Notice) error {
	to, cc := recipientsToToCC(notice.Recipients)
	to = n.filterRecipients(to)
	cc = n.filterRecipients(cc)

	if len(to) == 0 {
		n.Logger.InfoContext(ctx, "email skipped: no valid recipients",
			"projectID", notice.ProjectID, "window", notice.Window, "subject", notice.Subject)
		return nil
	}

	htmlBody := renderEmailHTML(notice.Body)

	if err := n.Sender.SendEmail(ctx, to, cc, notice.Subject, htmlBody); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	n.Logger.InfoContext(ctx, "email sent",
		"projectID", notice.ProjectID, "window", notice.Window, "subject", notice.Subject, "to", to, "cc", cc)
	return nil
}

// Delivers reports true: unlike LoggingNotifier, EmailNotifier genuinely
// attempts real delivery. This is a blanket, per-notifier signal, not a
// per-notice one — recordNoticeSent has no way to know from this alone
// whether a specific notice's recipients all happened to get filtered out
// by the WSO2-only staging safeguard; that's a known, accepted imprecision
// in the existing Delivers() contract, not something this method tries to
// work around.
func (n *EmailNotifier) Delivers() bool {
	return true
}

// recipientsToToCC maps Recipients onto the real API's to/cc shape. Empty
// emails (a role with no address on file — a legitimate, unremarkable
// state per recipients.AccountManagerEmail's existing contract) are
// dropped rather than sent through as blank strings.
func recipientsToToCC(r Recipients) (to, cc []string) {
	if r.Customer != nil {
		to = appendIfNonEmpty(to, r.Customer.Email)
		cc = appendIfNonEmpty(cc, r.AccountOwner.Email, r.RenewalManager.Email, r.TechnicalOwner.Email)
		return to, cc
	}
	to = appendIfNonEmpty(to, r.AccountOwner.Email, r.RenewalManager.Email, r.TechnicalOwner.Email)
	return to, nil
}

func appendIfNonEmpty(list []string, emails ...string) []string {
	for _, e := range emails {
		if e != "" {
			list = append(list, e)
		}
	}
	return list
}

// filterRecipients applies the WSO2-only staging safeguard: when
// AllowNonWSO2Recipients is false, only addresses ending in "@wso2.com"
// (case-insensitive) survive.
func (n *EmailNotifier) filterRecipients(emails []string) []string {
	if n.AllowNonWSO2Recipients {
		return emails
	}
	var filtered []string
	for _, e := range emails {
		if strings.HasSuffix(strings.ToLower(e), "@wso2.com") {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// renderEmailHTML wraps a notice's plain-text Body in the branded WSO2
// email shell (emailHTMLTemplate) — logo, orange accent border, footer
// disclaimer — matching real notice examples, not a bare text conversion.
func renderEmailHTML(body string) string {
	return fmt.Sprintf(emailHTMLTemplate, wso2LogoDataURI, plainTextToHTML(body))
}

// plainTextToHTML converts a plain-text notice Body (every existing
// template uses blank-line paragraph breaks and single newlines, never
// HTML) into simple HTML: special characters are escaped first (so a
// project/account name containing "&", "<", etc. can never break the
// resulting markup), then every newline becomes a <br> line break. Used by
// renderEmailHTML to build the message content inside the branded shell.
func plainTextToHTML(s string) string {
	escaped := html.EscapeString(s)
	return strings.ReplaceAll(escaped, "\n", "<br>\n")
}

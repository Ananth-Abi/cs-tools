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
	"fmt"
	"html"
	"log/slog"
	"strings"
)

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

	htmlBody := plainTextToHTML(notice.Body)

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

// plainTextToHTML converts a plain-text notice Body (every existing
// template uses blank-line paragraph breaks and single newlines, never
// HTML) into simple HTML: special characters are escaped first (so a
// project/account name containing "&", "<", etc. can never break the
// resulting markup), then every newline becomes a <br> line break.
func plainTextToHTML(s string) string {
	escaped := html.EscapeString(s)
	return strings.ReplaceAll(escaped, "\n", "<br>\n")
}

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

// Package notify defines the shape of an ACP notice and a logging
// implementation of sending one. Real email sending is not implemented on
// either side yet — deferred pending message-queue design (per the 2026-07-17
// meeting notes) — so LoggingNotifier is not a temporary stand-in for this
// component specifically; it is genuinely the only option available today.
package notify

import (
	"context"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/closure"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/recipients"
)

// Recipients is the structured set of people one Notice should reach.
// AccountOwner (the Account Manager), RenewalManager, and TechnicalOwner are
// always populated for a day-count reminder — an individual Contact's Email
// may legitimately be "" per recipients.AccountManagerEmail's existing
// convention (role assigned but no email on file, or no role assigned at
// all), which is not an error state. Customer is nil except on a resolved
// 15/7/0-window notice; it is also nil (not a zero-value Contact) on the
// separate no-business-contact notice, which names only an Account Owner.
type Recipients struct {
	AccountOwner   recipients.Contact
	RenewalManager recipients.Contact
	TechnicalOwner recipients.Contact
	Customer       *recipients.Contact
}

// Notice is everything a Notifier needs to send (or, today, log) one ACP
// notification. There is no more Kind field distinguishing
// internal/customer/am_nudge audiences — that distinction is now implied by
// Subject's wording and which Recipients fields are populated, per Chamara's
// request that the log stop saying "internal"/"external" explicitly.
type Notice struct {
	ProjectID   string
	ProjectName string
	ProjectKey  string
	StartDate   time.Time
	EndDate     time.Time
	Window      closure.NoticeWindow
	// Subject is the notice's title line — either the day-count reminder
	// ("{N} Days Reminder of Project for {ProjectName} of {AccountName}",
	// [ACP]-prefixed only for the internal-only 90/60/30 windows) or the
	// no-business-contact notice's fixed "[Urgent] [ACP] No Business
	// Contacts Specified for Project {ProjectName}".
	Subject string
	// Body is only populated for the no-business-contact notice today — no
	// body template exists yet for the day-count reminders, so it's left
	// empty there rather than inventing one.
	Body       string
	Recipients Recipients
	// ResolvedVia records which tier of the three-tier customer-contact
	// fallback produced Recipients.Customer (see
	// recipients.ResolveCustomerContact). Left at its zero value ("") when
	// not applicable — a 90/60/30 notice, or the no-business-contact notice
	// itself, neither of which carry a resolved customer contact.
	ResolvedVia recipients.ResolvedVia
}

// LoggingNotifier logs what would have been sent instead of sending it.
type LoggingNotifier struct {
	Logger *slog.Logger
}

// Send logs the notice and always succeeds.
func (n *LoggingNotifier) Send(ctx context.Context, notice Notice) error {
	attrs := []any{
		"subject", notice.Subject,
		"window", notice.Window,
		"projectID", notice.ProjectID,
		"projectName", notice.ProjectName,
		"projectKey", notice.ProjectKey,
		"startDate", notice.StartDate,
		"endDate", notice.EndDate,
		"accountOwner", notice.Recipients.AccountOwner.Email,
		"renewalManager", notice.Recipients.RenewalManager.Email,
		"technicalOwner", notice.Recipients.TechnicalOwner.Email,
		"resolvedVia", notice.ResolvedVia,
	}
	if notice.Recipients.Customer != nil {
		attrs = append(attrs, "customer", notice.Recipients.Customer.Email)
	}
	if notice.Body != "" {
		attrs = append(attrs, "body", notice.Body)
	}

	n.Logger.InfoContext(ctx, "notice", attrs...)
	return nil
}

// Delivers reports false: LoggingNotifier only logs what would have been
// sent, it never actually delivers a notice to anyone.
func (n *LoggingNotifier) Delivers() bool {
	return false
}

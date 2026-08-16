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

// Package recipientlinks resolves, for a list of notification recipients,
// which portal's case link each one should get in their email: a customer
// contact should never be handed a CSM-portal link they can't access (and
// vice versa isn't the point, but keeping the audiences separate is). This
// is a per-recipient decision, not a per-event one — the same
// case.comment_added notification can go to both a customer watcher and an
// internal CSM watcher at once, each needing a different link — so
// internal/dispatch calls this to resolve links, then groups recipients by
// the link they resolved to, before rendering/sending anything (see
// Dispatcher.groupByLink). This does not resolve *who* to notify — every
// case.* payload still carries its own caller-supplied Recipients list, see
// internal/events' package doc — only which link a given recipient gets.
//
// Note the customer portal has no comment-permalink fragment handling today
// (only the CSM portal's frontend reads location.hash to scroll to a
// comment) — a comment-specific fragment on a customer-portal link is
// simply inert there, not an error.
package recipientlinks

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
)

// entityClient abstracts entity.CustomerEntityClient's user-role lookup for
// testability.
type entityClient interface {
	SearchUsersByEmail(ctx context.Context, emails []string) ([]entity.UserRoleInfo, error)
}

// Config holds the role classification and portal base URLs Resolver needs.
// CustomerRoles and CSMRoles need not be exhaustive of every role in the
// system — see ResolveLinks' doc comment for what happens when a
// recipient's roles match neither list.
type Config struct {
	CustomerRoles   []string
	CSMRoles        []string
	CustomerBaseURL string
	CSMBaseURL      string
}

// Resolver resolves recipient emails to portal-appropriate case links.
type Resolver struct {
	entity        entityClient
	customerRoles map[string]bool
	csmRoles      map[string]bool
	customerBase  string
	csmBase       string
}

// New constructs a Resolver.
func New(entity entityClient, cfg Config) *Resolver {
	return &Resolver{
		entity:        entity,
		customerRoles: toSet(cfg.CustomerRoles),
		csmRoles:      toSet(cfg.CSMRoles),
		customerBase:  cfg.CustomerBaseURL,
		csmBase:       cfg.CSMBaseURL,
	}
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}

// RecipientLink pairs a recipient's email with the case link resolved for
// their role.
type RecipientLink struct {
	Email    string
	CaseLink string
}

// ResolveLinks looks up each of emails' roles via entity-service and
// returns the case link appropriate to each: a recipient whose roles
// include one from Config.CustomerRoles gets the customer portal's link
// (<CustomerBaseURL>/projects/{projectID}/support/cases/{caseID}); anyone
// else gets the CSM portal's link (<CSMBaseURL>/cases/{caseID}).
//
// A recipient whose roles match neither CustomerRoles nor CSMRoles falls
// back to their entity-service userType (customer/external -> customer
// portal, anything else -> CSM portal) and logs a warning — the role lists
// are operator-curated and may not be exhaustive, but userType is always
// present. A recipient entity-service has no record for at all logs a
// warning and defaults to the CSM portal link: every real recipient should
// already exist there (case reporters/watchers/assignees are entity-service
// user references before they ever become a plain email address), so a
// miss here is a data anomaly worth surfacing, not an expected case — and
// the CSM link is the safer default of the two to hand someone in that
// situation, versus routing an actual CSM user to a portal they may have no
// account on at all.
//
// The returned links are the bare case link only — appending a
// comment-specific anchor/fragment is internal/dispatch's job (see
// commentLinkFor there), since that varies by event type and this package
// only knows about portal audiences.
func (r *Resolver) ResolveLinks(ctx context.Context, emails []string, projectID, caseID string) ([]RecipientLink, error) {
	if len(emails) == 0 {
		return nil, nil
	}

	users, err := r.entity.SearchUsersByEmail(ctx, emails)
	if err != nil {
		return nil, fmt.Errorf("recipientlinks: search users: %w", err)
	}

	// Keyed lowercase: entity-service and the caller's own recipient list
	// don't necessarily agree on email casing, and an email address is the
	// same address regardless of case.
	byEmail := make(map[string]entity.UserRoleInfo, len(users))
	for _, u := range users {
		byEmail[strings.ToLower(u.Email)] = u
	}

	links := make([]RecipientLink, 0, len(emails))
	for _, email := range emails {
		user, found := byEmail[strings.ToLower(email)]
		links = append(links, RecipientLink{
			Email:    email,
			CaseLink: r.linkFor(ctx, user, found, projectID, caseID),
		})
	}
	return links, nil
}

// linkFor does not take the recipient's email — logging it would put PII
// (an email address) in the logs, which this repo's own convention
// disallows. caseID identifies which notification a warning belongs to
// without identifying who it's about.
func (r *Resolver) linkFor(ctx context.Context, user entity.UserRoleInfo, found bool, projectID, caseID string) string {
	isCustomer := false
	switch {
	case !found:
		slog.WarnContext(ctx, "recipientlinks: recipient not found on entity-service; defaulting to CSM portal link", "caseID", caseID)
	case r.matchesAny(user.Roles, r.customerRoles):
		isCustomer = true
	case r.matchesAny(user.Roles, r.csmRoles):
		// isCustomer already false.
	case user.UserType == "customer" || user.UserType == "external":
		isCustomer = true
		slog.WarnContext(ctx, "recipientlinks: recipient's roles matched neither CUSTOMER_ROLES nor CSM_ROLES; used userType as a fallback",
			"caseID", caseID, "roles", user.Roles, "userType", user.UserType)
	default:
		slog.WarnContext(ctx, "recipientlinks: recipient's roles matched neither CUSTOMER_ROLES nor CSM_ROLES; defaulting to CSM portal link",
			"caseID", caseID, "roles", user.Roles, "userType", user.UserType)
	}

	if isCustomer {
		return fmt.Sprintf("%s/projects/%s/support/cases/%s", r.customerBase, url.PathEscape(projectID), url.PathEscape(caseID))
	}
	return fmt.Sprintf("%s/cases/%s", r.csmBase, url.PathEscape(caseID))
}

func (r *Resolver) matchesAny(roles []string, set map[string]bool) bool {
	for _, role := range roles {
		if set[role] {
			return true
		}
	}
	return false
}

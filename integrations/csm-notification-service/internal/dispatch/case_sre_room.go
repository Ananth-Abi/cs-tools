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
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// isCatastrophic reports whether a case severity is S0, in any of the forms
// entity-service and the paging configuration use for it.
func isCatastrophic(severity string) bool {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "CATASTROPHIC", "S0", "P0":
		return true
	}
	return false
}

// isCustomerCase reports whether a case type is a customer case, the only
// type the paging ladders page (paging/cases.go makes the same check).
func isCustomerCase(caseType string) bool {
	return strings.EqualFold(strings.TrimSpace(caseType), "case")
}

// caseSRERooms is where an S0 case's SRE room card goes: its account SRE
// group's rooms when that group is a known SRE team, else the teamless rooms
// (notifications.CaseSREAudiences, which the SRE ladder's replies use too).
// The SRE ladder posts nothing until a rung finds someone to page, so without
// this card the SRE room hears of an S0 only minutes later. None for any
// other severity.
func (d *Dispatcher) caseSRERooms(ctx context.Context, t events.Type, caseID, severity, sreGroup string) []string {
	if !isCatastrophic(severity) {
		return nil
	}
	audiences := notifications.CaseSREAudiences(d.incidentRoutes, d.incidentChatFanout, sreGroup, d.googleChat.HasAudienceSpace)
	if len(audiences) == 0 {
		slog.InfoContext(ctx, "dispatch: S0 case has no SRE room (no team room, teamless room or shared SRE room), no SRE room card",
			"type", string(t), "caseId", caseID, "sreGroup", sreGroup)
	}
	return audiences
}

// sendCaseSRERoomCard posts an S0 case's card to its SRE rooms, best effort:
// a failure is logged, never returned. The Incident Monitor card and the
// emails of the same record are its must-deliver parts; returning this card's
// error would retry the record, and once the retries run out the dead-letter
// tier would send those parts a second time. The SRE ladder's rung cards
// still reach the room.
func (d *Dispatcher) sendCaseSRERoomCard(ctx context.Context, t events.Type, caseID string, rooms []string, send func(audience string) error) {
	if err := d.sendToEachIncidentSpace(ctx, t, caseID, rooms, send); err != nil {
		slog.WarnContext(ctx, "dispatch: S0 case card did not reach its SRE room",
			"type", string(t), "caseId", caseID, "rooms", rooms, "errSummary", chatErrSummary(err))
	}
}

// caseSRERoomCard is the card an S0 case posts to its SRE rooms: the
// incident's "Reported" card in its case form, keyed by the case id so the
// SRE ladder's rung cards reply under it (paging's sreCaseCard), as an
// incident's do.
//
// The case is named as the SRE ladder's replies name it (paging's
// Trigger.caseRef): the WSO2 case ID, else the case number.
func caseSRERoomCard(caseID, caseNumber, wso2CaseID, sreGroup, severity, title, product, project, caseLink string) notifications.IncidentCreatedAlert {
	ref := strings.TrimSpace(wso2CaseID)
	if ref == "" {
		ref = displayCaseRef(caseNumber, caseID)
	}
	return notifications.IncidentCreatedAlert{
		IncidentID:       caseID,
		Number:           ref,
		Team:             sreGroup,
		Priority:         severity,
		ShortDescription: truncateTitle(title, maxChatTitleLength),
		Service:          product,
		Category:         "case",
		IncidentLink:     caseLink,
		Case:             true,
		Project:          project,
	}
}

// chatErrSummary is a Chat send error safe to log: the HTTP status of each
// failed post, or the error's type. Never the error text, which for a failed
// post carries an excerpt of Google Chat's response body and so can echo
// case content.
func chatErrSummary(err error) string {
	var parts []string
	var walk func(error)
	walk = func(err error) {
		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			for _, e := range joined.Unwrap() {
				walk(e)
			}
			return
		}
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) {
			parts = append(parts, fmt.Sprintf("status %d", apiErr.StatusCode))
			return
		}
		parts = append(parts, fmt.Sprintf("%T", err))
	}
	if err != nil {
		walk(err)
	}
	return strings.Join(parts, "; ")
}

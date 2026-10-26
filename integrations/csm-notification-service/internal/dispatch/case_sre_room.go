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
	"log/slog"
	"strings"

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

// caseSRERooms is where an S0 case's card goes besides Incident Monitor: the
// rooms its account's SRE group maps to on the Alert Settings tab, the same
// rooms an incident of that group gets its "Incident Reported" card in. The
// SRE ladder posts nothing until a rung finds someone to page, so without
// this card the SRE room hears of an S0 only minutes later, or not at all
// when nobody is rostered.
//
// None for any other severity, or for a case whose account has no SRE team:
// such a case pages the teamless rooms, which its rung cards already reach.
func (d *Dispatcher) caseSRERooms(ctx context.Context, t events.Type, caseID, severity, sreGroup string) []string {
	if !isCatastrophic(severity) || strings.TrimSpace(sreGroup) == "" {
		return nil
	}
	audiences, fallback := notifications.IncidentAudiences(d.incidentRoutes, d.incidentChatFanout, sreGroup, d.googleChat.HasAudienceSpace)
	switch {
	case fallback:
		slog.InfoContext(ctx, "dispatch: S0 case's SRE group maps to no SRE team room; card goes to the shared SRE room",
			"type", string(t), "caseId", caseID, "sreGroup", sreGroup)
	case len(audiences) == 0:
		slog.InfoContext(ctx, "dispatch: S0 case's SRE group has no chat space, no SRE room card",
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
			"type", string(t), "caseId", caseID, "rooms", rooms, "err", err)
	}
}

// caseSRERoomCard is the card an S0 case posts to its SRE rooms: the
// incident's "Reported" card in its case form, keyed by the case id so the
// SRE ladder's rung cards reply under it (paging's sreCaseCard), as an
// incident's do.
func caseSRERoomCard(caseID, caseNumber, sreGroup, severity, title, product, caseLink string) notifications.IncidentCreatedAlert {
	return notifications.IncidentCreatedAlert{
		IncidentID:       caseID,
		Number:           displayCaseRef(caseNumber, caseID),
		Team:             sreGroup,
		Priority:         severity,
		ShortDescription: truncateTitle(title, maxChatTitleLength),
		Service:          product,
		Category:         "case",
		IncidentLink:     caseLink,
		Case:             true,
	}
}

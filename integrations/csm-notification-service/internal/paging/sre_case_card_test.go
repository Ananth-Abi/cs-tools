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

package paging

import (
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

func sreCase(sreGroup string) Trigger {
	return Trigger{IncidentID: "case-1", Record: RecordCase, Team: "Phoenix", At: time.Now(),
		Routing: RoutingContext{Ladder: LadderSRE, AssignedCRETeam: "Phoenix", SRETeam: sreGroup}}
}

// An S0 case with an account SRE team replies under the card the dispatcher
// posted to that team's room, as an incident's rungs do.
func TestSRECaseRung_RepliesUnderItsCardInTheSRERoom(t *testing.T) {
	chat := &fakeChat{}
	deliverFirstRung(t, incidentSpaceEngine(chat), sreCase("MS/PC SRE Group"))
	if len(chat.posted) != 1 {
		t.Fatalf("posted %d cards; want 1", len(chat.posted))
	}
	got := chat.posted[0]
	if got.Audience != notifications.IncidentAudience("MS/PC SRE Group") || !got.Reply || got.ThreadKey != notifications.IncidentThreadKey("case-1") {
		t.Errorf("card to %q reply=%v thread=%q; want a reply under the case card in its SRE room", got.Audience, got.Reply, got.ThreadKey)
	}
}

// A case with no account SRE team has no card to reply under: it keeps the
// ladder's own full card, as before.
func TestSRECaseRung_NoSRETeamKeepsTheLaddersOwnCard(t *testing.T) {
	chat := &fakeChat{}
	deliverFirstRung(t, incidentSpaceEngine(chat), sreCase(""))
	if len(chat.posted) != 1 || chat.posted[0].Audience != "SRE Case Paging" || chat.posted[0].Reply ||
		chat.posted[0].ThreadKey != "incident-escalation-case-1" {
		t.Fatalf("posted %+v; want the ladder's own full card, unthreaded", chat.posted)
	}
}

// The CRE ladder is untouched: a case there keeps its own spaces and thread,
// even when the case carries an account SRE team.
func TestSRECaseRung_CRELadderUnchanged(t *testing.T) {
	cre := sreCase("MS/PC SRE Group")
	cre.Routing.Ladder = LadderCRE
	if got := cre.chatThreadKey(); got != "incident-escalation-case-1" {
		t.Errorf("CRE case thread = %q; want incident-escalation-case-1", got)
	}

	e := incidentSpaceEngine(&fakeChat{})
	e.cfg.Kind = LadderCRE
	if rooms := e.incidentRooms(cre); len(rooms) != 0 {
		t.Errorf("CRE engine rooms = %+v; want none", rooms)
	}
	// Even an SRE-ladder trigger is never routed to SRE rooms by the CRE engine.
	if rooms := e.incidentRooms(sreCase("MS/PC SRE Group")); len(rooms) != 0 {
		t.Errorf("CRE engine rooms for an SRE case = %+v; want none", rooms)
	}
}

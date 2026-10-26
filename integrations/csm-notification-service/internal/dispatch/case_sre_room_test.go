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
	"slices"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/chataudience"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

func caseCreatedWith(priority, sreGroup string) eventbus.Record {
	return eventbus.Record{Value: []byte(fmt.Sprintf(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","caseTitle":"Something broke","caseType":"CASE","priority":%q,"product":"api-manager","team":"Phoenix","sreTeamName":%q,"createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`, priority, sreGroup))}
}

// caseCardAudiences is where the Incident Monitor "Case Reported" card went.
func caseCardAudiences(chat *mockGoogleChatSender) []string {
	chat.mu.Lock()
	defer chat.mu.Unlock()
	var out []string
	for _, c := range chat.caseCreatedCalls {
		out = append(out, c.audience)
	}
	return out
}

// roomCards is the SRE room cards: the incident card in its case form.
func roomCards(chat *mockGoogleChatSender) []sentIncidentCreatedAlert {
	chat.mu.Lock()
	defer chat.mu.Unlock()
	return slices.Clone(chat.incidentCreatedCalls)
}

func roomAudiences(chat *mockGoogleChatSender) []string {
	var out []string
	for _, c := range roomCards(chat) {
		out = append(out, c.audience)
	}
	return out
}

func sreRoomDispatcher(chat googleChatSender) *Dispatcher {
	routes := &notifications.IncidentRoutes{}
	routes.Set(map[string][]string{"Default SRE Group": {"apollo", "artemis"}})
	routes.SetTeamless([]string{"apollo", "artemis"})
	return NewDispatcher(&mockEmailSender{}, chat, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, true, "", nil).
		WithIncidentRoutes(routes)
}

func TestDispatcher_S0CaseCardGoesToItsSRERoom(t *testing.T) {
	cases := []struct {
		name     string
		priority string
		sreGroup string
		rooms    []string
	}{
		{"an S0 on a mapped SRE group goes to its team's room", "CATASTROPHIC", "WSO2 IaaS Team",
			[]string{notifications.IncidentAudience("WSO2 IaaS Team")}},
		{"an S0 on an unmapped SRE group goes to the shared SRE room", "CATASTROPHIC", "U2 SRE Group",
			[]string{notifications.IncidentFallbackAudience}},
		{"an S0 on an account with no SRE team goes to the teamless rooms", "CATASTROPHIC", "",
			[]string{notifications.IncidentAudience("apollo"), notifications.IncidentAudience("artemis")}},
		{"an S1 gets no room card", "CRITICAL", "WSO2 IaaS Team", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
			if err := sreRoomDispatcher(chat).Handle(context.Background(), caseCreatedWith(tc.priority, tc.sreGroup)); err != nil {
				t.Fatalf("Handle error = %v", err)
			}
			// The Incident Monitor card is untouched whatever the case.
			if got := caseCardAudiences(chat); !slices.Equal(got, []string{chataudience.IncidentMonitor}) {
				t.Errorf("Case Reported card went to %v; want only Incident Monitor", got)
			}
			if got := roomAudiences(chat); !slices.Equal(got, tc.rooms) {
				t.Errorf("room card went to %v; want %v", got, tc.rooms)
			}
		})
	}
}

func TestDispatcher_S0CaseRoomCardIsTheIncidentCardInCaseForm(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	if err := sreRoomDispatcher(chat).Handle(context.Background(), caseCreatedWith("CATASTROPHIC", "WSO2 IaaS Team")); err != nil {
		t.Fatalf("Handle error = %v", err)
	}
	cards := roomCards(chat)
	if len(cards) != 1 {
		t.Fatalf("room cards = %d; want 1", len(cards))
	}
	a := cards[0].alert
	// IncidentID is the case id, so the SRE ladder's rung cards (thread incident-<caseId>) reply under it.
	if !a.Case || a.IncidentID != "CASE-1" || a.Number != "CS0001001" || a.Team != "WSO2 IaaS Team" ||
		a.Priority != "CATASTROPHIC" || a.ShortDescription != "Something broke" || a.Service != "api-manager" {
		t.Errorf("room card = %+v; want the case form of the incident card for CASE-1", a)
	}
}

func TestDispatcher_SeverityRaisedToS0GoesToItsSRERoom(t *testing.T) {
	for _, tc := range []struct {
		newSeverity string
		rooms       []string
	}{
		{"CATASTROPHIC", []string{notifications.IncidentAudience("WSO2 IaaS Team")}},
		{"CRITICAL", nil},
	} {
		t.Run(tc.newSeverity, func(t *testing.T) {
			chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
			record := eventbus.Record{Value: []byte(fmt.Sprintf(`{"type":"case.severity_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","caseTitle":"Something broke","oldSeverity":"HIGH","newSeverity":%q,"product":"api-manager","team":"Phoenix","sreTeamName":"WSO2 IaaS Team","recipients":["test-recipient@example.com"]}}`, tc.newSeverity))}
			if err := sreRoomDispatcher(chat).Handle(context.Background(), record); err != nil {
				t.Fatalf("Handle error = %v", err)
			}
			chat.mu.Lock()
			var monitor []string
			for _, c := range chat.severityChangedCalls {
				monitor = append(monitor, c.audience)
			}
			chat.mu.Unlock()
			if !slices.Equal(monitor, []string{chataudience.IncidentMonitor}) {
				t.Errorf("Severity changed card went to %v; want only Incident Monitor", monitor)
			}
			if got := roomAudiences(chat); !slices.Equal(got, tc.rooms) {
				t.Errorf("room card went to %v; want %v", got, tc.rooms)
			}
		})
	}
}

// failingSRERoomChat fails every room card.
type failingSRERoomChat struct{ *mockGoogleChatSender }

func (f failingSRERoomChat) SendIncidentCreatedAlert(context.Context, string, notifications.IncidentCreatedAlert) error {
	return errors.New("webhook gone")
}

// A failing SRE room must not fail the record: a retry, and in the end the
// dead-letter tier, would resend the Incident Monitor card and the emails.
func TestDispatcher_S0CaseSRERoomFailureDoesNotFailTheRecord(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	if err := sreRoomDispatcher(failingSRERoomChat{chat}).Handle(context.Background(), caseCreatedWith("CATASTROPHIC", "WSO2 IaaS Team")); err != nil {
		t.Fatalf("Handle error = %v; want nil when only the SRE room card fails", err)
	}
	if got := caseCardAudiences(chat); !slices.Equal(got, []string{chataudience.IncidentMonitor}) {
		t.Errorf("Case Reported card went to %v; want Incident Monitor", got)
	}
}

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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/chataudience"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

func caseCreatedWith(priority, sreGroup string) eventbus.Record {
	return caseCreatedOfType("CASE", priority, sreGroup)
}

func caseCreatedOfType(caseType, priority, sreGroup string) eventbus.Record {
	return eventbus.Record{Value: []byte(fmt.Sprintf(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","wso2CaseId":"WSO2-1001","caseTitle":"Something broke","caseType":%q,"priority":%q,"product":"api-manager","team":"Phoenix","sreTeamName":%q,"createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`, caseType, priority, sreGroup))}
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

// roomAudiences is the rooms the card went to, sorted.
func roomAudiences(chat *mockGoogleChatSender) []string {
	var out []string
	for _, c := range roomCards(chat) {
		out = append(out, c.audience)
	}
	slices.Sort(out)
	return out
}

func sreRoomDispatcher(chat googleChatSender) *Dispatcher {
	routes := &notifications.IncidentRoutes{}
	routes.Set(map[string][]string{"Default SRE Group": {"apollo", "artemis"}})
	routes.SetTeamless([]string{"apollo", "artemis"})
	// WSO2 IaaS Team is the one known SRE team; U2 SRE Group is not one.
	routes.SetKnownSRETeam(func(group string) bool { return strings.EqualFold(group, "WSO2 IaaS Team") })
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
		{"an S0 on an SRE group the ladder does not know goes to the teamless rooms, where it is paged", "CATASTROPHIC", "U2 SRE Group",
			[]string{notifications.IncidentAudience("apollo"), notifications.IncidentAudience("artemis")}},
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
	// Number is the WSO2 case ID, as the SRE ladder's replies name the case (Trigger.caseRef).
	if !a.Case || a.IncidentID != "CASE-1" || a.Number != "WSO2-1001" || a.Team != "WSO2 IaaS Team" ||
		a.Priority != "CATASTROPHIC" || a.ShortDescription != "Something broke" || a.Service != "api-manager" || a.Project != "Proj" {
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

// A Chat error's response excerpt can echo case content, so only its status
// (or its type) is logged.
func TestChatErrSummary_NeverCarriesTheResponseBody(t *testing.T) {
	err := errors.Join(&apierror.Error{StatusCode: 400, Body: `{"text":"Something broke for Customer 3"}`}, errors.New("dial tcp: timeout"))
	got := chatErrSummary(err)
	if got != "status 400; *errors.errorString" {
		t.Errorf("chatErrSummary = %q; want only the status and the type", got)
	}
	if strings.Contains(got, "Customer 3") {
		t.Errorf("chatErrSummary leaked the response body: %q", got)
	}
}

// The SRE room card has its own gate, not the Incident Monitor branch's: only
// a customer case is paged, so only it is carded in an SRE room.
func TestDispatcher_S0RoomCardOnlyForCustomerCases(t *testing.T) {
	for _, tc := range []struct {
		caseType string
		rooms    int
	}{
		{"CASE", 1},
		{"case", 1},
		{"service_request", 0},
		{"security_report_analysis", 0},
	} {
		t.Run(tc.caseType, func(t *testing.T) {
			chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
			if err := sreRoomDispatcher(chat).Handle(context.Background(), caseCreatedOfType(tc.caseType, "CATASTROPHIC", "WSO2 IaaS Team")); err != nil {
				t.Fatalf("Handle error = %v", err)
			}
			if got := len(roomCards(chat)); got != tc.rooms {
				t.Errorf("room cards = %d; want %d", got, tc.rooms)
			}
		})
	}
}

// blockingRoomChat holds every SRE room card until the Incident Monitor card
// has gone out, so the record only completes if the two run side by side.
type blockingRoomChat struct {
	*mockGoogleChatSender
	monitorSent chan struct{}
}

func (b blockingRoomChat) SendCaseCreatedAlert(ctx context.Context, audience, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	err := b.mockGoogleChatSender.SendCaseCreatedAlert(ctx, audience, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink)
	close(b.monitorSent)
	return err
}

func (b blockingRoomChat) SendIncidentCreatedAlert(ctx context.Context, audience string, a notifications.IncidentCreatedAlert) error {
	select {
	case <-b.monitorSent:
	case <-time.After(5 * time.Second):
		return errors.New("the Incident Monitor card waited on the SRE room card")
	}
	return b.mockGoogleChatSender.SendIncidentCreatedAlert(ctx, audience, a)
}

// The SRE room card runs beside the CRE-side work: the Incident Monitor card
// (and the emails before it) are sent without waiting on it, and Handle still
// returns only once the room card is done.
func TestDispatcher_S0RoomCardRunsBesideTheIncidentMonitorCard(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	d := sreRoomDispatcher(blockingRoomChat{mockGoogleChatSender: chat, monitorSent: make(chan struct{})})
	if err := d.Handle(context.Background(), caseCreatedWith("CATASTROPHIC", "WSO2 IaaS Team")); err != nil {
		t.Fatalf("Handle error = %v", err)
	}
	if got := caseCardAudiences(chat); !slices.Equal(got, []string{chataudience.IncidentMonitor}) {
		t.Errorf("Case Reported card went to %v; want Incident Monitor", got)
	}
	if got := roomAudiences(chat); !slices.Equal(got, []string{notifications.IncidentAudience("WSO2 IaaS Team")}) {
		t.Errorf("room card went to %v by the time Handle returned; want the team's room", got)
	}
}

// Every room is posted; one failing room does not stop the others.
func TestDispatcher_S0RoomCardPostsEveryRoom(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	if err := sreRoomDispatcher(chat).Handle(context.Background(), caseCreatedWith("CATASTROPHIC", "")); err != nil {
		t.Fatalf("Handle error = %v", err)
	}
	got := roomAudiences(chat)
	if want := []string{notifications.IncidentAudience("apollo"), notifications.IncidentAudience("artemis")}; !slices.Equal(got, want) {
		t.Errorf("room cards went to %v; want %v", got, want)
	}
}

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

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/chataudience"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

func caseCreatedWith(priority, sreGroup string) eventbus.Record {
	return eventbus.Record{Value: []byte(fmt.Sprintf(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","caseTitle":"Something broke","caseType":"CASE","priority":%q,"product":"api-manager","team":"Phoenix","sreTeamName":%q,"createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`, priority, sreGroup))}
}

func caseCardAudiences(chat *mockGoogleChatSender) []string {
	chat.mu.Lock()
	defer chat.mu.Unlock()
	var out []string
	for _, c := range chat.caseCreatedCalls {
		out = append(out, c.audience)
	}
	return out
}

func sreRoomDispatcher(chat *mockGoogleChatSender) *Dispatcher {
	routes := &notifications.IncidentRoutes{}
	routes.Set(map[string][]string{"Default SRE Group": {"apollo", "artemis"}})
	return NewDispatcher(&mockEmailSender{}, chat, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, true, "", nil).
		WithIncidentRoutes(routes)
}

func TestDispatcher_S0CaseCardGoesToItsSRERoom(t *testing.T) {
	cases := []struct {
		name     string
		priority string
		sreGroup string
		want     []string
	}{
		{"an S0 on a mapped SRE group goes to its team's room too", "CATASTROPHIC", "WSO2 IaaS Team",
			[]string{chataudience.IncidentMonitor, notifications.IncidentAudience("WSO2 IaaS Team")}},
		{"an S0 on an unmapped SRE group goes to the shared SRE room", "CATASTROPHIC", "U2 SRE Group",
			[]string{chataudience.IncidentMonitor, notifications.IncidentFallbackAudience}},
		{"an S0 on an account with no SRE team stays in Incident Monitor", "CATASTROPHIC", "",
			[]string{chataudience.IncidentMonitor}},
		{"an S1 does not go to the SRE room", "CRITICAL", "WSO2 IaaS Team",
			[]string{chataudience.IncidentMonitor}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
			if err := sreRoomDispatcher(chat).Handle(context.Background(), caseCreatedWith(tc.priority, tc.sreGroup)); err != nil {
				t.Fatalf("Handle error = %v", err)
			}
			if got := caseCardAudiences(chat); !slices.Equal(got, tc.want) {
				t.Errorf("case card went to %v; want %v", got, tc.want)
			}
		})
	}
}

func TestDispatcher_SeverityRaisedToS0GoesToItsSRERoom(t *testing.T) {
	for _, tc := range []struct {
		newSeverity string
		want        []string
	}{
		{"CATASTROPHIC", []string{chataudience.IncidentMonitor, notifications.IncidentAudience("WSO2 IaaS Team")}},
		{"CRITICAL", []string{chataudience.IncidentMonitor}},
	} {
		t.Run(tc.newSeverity, func(t *testing.T) {
			chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
			record := eventbus.Record{Value: []byte(fmt.Sprintf(`{"type":"case.severity_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","caseTitle":"Something broke","oldSeverity":"HIGH","newSeverity":%q,"product":"api-manager","team":"Phoenix","sreTeamName":"WSO2 IaaS Team","recipients":["test-recipient@example.com"]}}`, tc.newSeverity))}
			if err := sreRoomDispatcher(chat).Handle(context.Background(), record); err != nil {
				t.Fatalf("Handle error = %v", err)
			}
			chat.mu.Lock()
			var got []string
			for _, c := range chat.severityChangedCalls {
				got = append(got, c.audience)
			}
			chat.mu.Unlock()
			if !slices.Equal(got, tc.want) {
				t.Errorf("severity card went to %v; want %v", got, tc.want)
			}
		})
	}
}

// failingSRERoomChat fails every card sent to an incident (SRE room) audience.
type failingSRERoomChat struct{ *mockGoogleChatSender }

func (f failingSRERoomChat) SendCaseCreatedAlert(ctx context.Context, audience, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	if strings.HasPrefix(audience, notifications.IncidentAudiencePrefix) {
		return errors.New("webhook gone")
	}
	return f.mockGoogleChatSender.SendCaseCreatedAlert(ctx, audience, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink)
}

// A failing SRE room must not fail the record: a retry, and in the end the
// dead-letter tier, would resend the Incident Monitor card and the emails.
func TestDispatcher_S0CaseSRERoomFailureDoesNotFailTheRecord(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	routes := &notifications.IncidentRoutes{}
	d := NewDispatcher(&mockEmailSender{}, failingSRERoomChat{chat}, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, true, "", nil).
		WithIncidentRoutes(routes)
	if err := d.Handle(context.Background(), caseCreatedWith("CATASTROPHIC", "WSO2 IaaS Team")); err != nil {
		t.Fatalf("Handle error = %v; want nil when only the SRE room card fails", err)
	}
	if got := caseCardAudiences(chat); !slices.Equal(got, []string{chataudience.IncidentMonitor}) {
		t.Errorf("case card went to %v; want only Incident Monitor", got)
	}
}

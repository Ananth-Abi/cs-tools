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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// mockEventPublisher is a minimal EventPublisherService test double that
// records every Publish call rather than actually reaching Event Hub.
type mockEventPublisher struct {
	calls []mockPublishCall
	err   error
}

type mockPublishCall struct {
	eventType events.Type
	entityID  string
	payload   json.RawMessage
}

func (m *mockEventPublisher) Publish(_ context.Context, eventType events.Type, entityID string, payload json.RawMessage) error {
	m.calls = append(m.calls, mockPublishCall{eventType, entityID, payload})
	return m.err
}

func (m *mockEventPublisher) Close() {}

// newTestCreateCaseClient stubs both requests publishCaseCreated triggers
// after a successful create: the POST /cases create call itself, then the
// GetCaseByID enrichment (a GET /cases/{id}, plus a GET /cases/{id}/tags
// listCaseTags always issues — stubbed empty here since it's incidental to
// these tests). getCaseBody is served for the GET /cases/{id} call.
func newTestCreateCaseClient(t *testing.T, caseSysid, getCaseBody string) *integrationservice.Client {
	t.Helper()
	return newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"message": "Case created successfully",
				"case": {"id": "` + caseSysid + `", "number": "CS0009001", "createdBy": "jane.doe@example.com", "createdOn": "2026-01-02 10:00:00", "state": {"id": 1, "label": "Open"}}
			}`))
		case strings.HasSuffix(r.URL.Path, "/tags"):
			_, _ = w.Write([]byte(`{"tags":[]}`))
		default:
			_, _ = w.Write([]byte(getCaseBody))
		}
	})
}

// TestSNCaseService_CreateCase_PublishesCaseCreated verifies the happy path:
// after a successful create, publishCaseCreated enriches via GetCaseByID and
// publishes case.created with the reporter's name, project name, case
// details, and the watch list's emails as Recipients.
func TestSNCaseService_CreateCase_PublishesCaseCreated(t *testing.T) {
	const caseSysid = "1111111111111111111111111111aaaa"
	const projectSysid = "2222222222222222222222222222bbbb"
	const watcherSysid = "3333333333333333333333333333cccc"

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-009",
		"number": "CS0009001",
		"title": "Cannot log in",
		"description": "Login fails with a 500",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"severity": {"id": 3, "label": "3 - High"},
		"state": {"id": 1, "label": "Open"},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`

	client := newTestCreateCaseClient(t, caseSysid, getCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher)

	req := domain.CreateCaseRequest{
		Type:              "case",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
		Subject:           "Cannot log in",
		Description:       "Login fails with a 500",
		Severity:          domain.CaseSeverityHigh,
		IssueType:         domain.CaseIssueTypeQuestion,
	}

	resp, err := svc.CreateCase(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(publisher.calls))
	}
	call := publisher.calls[0]
	if call.eventType != events.TypeCaseCreated {
		t.Errorf("eventType = %q, want %q", call.eventType, events.TypeCaseCreated)
	}
	if call.entityID != resp.Case.ID {
		t.Errorf("entityID = %q, want the new case's id %q", call.entityID, resp.Case.ID)
	}

	var payload events.CaseCreatedPayload
	if err := json.Unmarshal(call.payload, &payload); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}
	if payload.ReporterName != "Jane Doe" {
		t.Errorf("reporterName = %q, want %q", payload.ReporterName, "Jane Doe")
	}
	if payload.ProjectName != "Project Zeta" {
		t.Errorf("projectName = %q, want %q", payload.ProjectName, "Project Zeta")
	}
	if payload.CaseID != resp.Case.ID {
		t.Errorf("caseId = %q, want %q", payload.CaseID, resp.Case.ID)
	}
	if payload.CaseTitle != "Cannot log in" {
		t.Errorf("caseTitle = %q, want %q", payload.CaseTitle, "Cannot log in")
	}
	if payload.CaseType != "CASE" {
		t.Errorf("caseType = %q, want %q", payload.CaseType, "CASE")
	}
	if payload.Priority != "HIGH" {
		t.Errorf("priority = %q, want %q", payload.Priority, "HIGH")
	}
	if payload.Description != "Login fails with a 500" {
		t.Errorf("description = %q, want %q", payload.Description, "Login fails with a 500")
	}
	if len(payload.Recipients) != 1 || payload.Recipients[0] != "john.roe@example.com" {
		t.Errorf("recipients = %v, want [john.roe@example.com]", payload.Recipients)
	}
}

// TestSNCaseService_CreateCase_SkipsPublishWhenNoWatchers verifies that a
// case created with no watchers does not publish case.created at all — sending
// one with an empty Recipients list would only be rejected downstream by
// csm-notification-service's events.Validate, so this service skips it
// proactively instead.
func TestSNCaseService_CreateCase_SkipsPublishWhenNoWatchers(t *testing.T) {
	const caseSysid = "4444444444444444444444444444dddd"
	const projectSysid = "5555555555555555555555555555eeee"

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-010",
		"number": "CS0010001",
		"title": "No watchers here",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"}
	}`

	client := newTestCreateCaseClient(t, caseSysid, getCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher)

	req := domain.CreateCaseRequest{
		Type:              "case",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
		Subject:           "No watchers here",
		Description:       "d",
		Severity:          domain.CaseSeverityHigh,
		IssueType:         domain.CaseIssueTypeQuestion,
	}

	if _, err := svc.CreateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call for a case with no watchers, got %d", len(publisher.calls))
	}
}

// TestSNCaseService_CreateCase_PublishFailureDoesNotFailCreateCase verifies
// that neither a Publish error nor a GetCaseByID enrichment error is
// returned to CreateCase's own caller — the case already exists in
// ServiceNow by that point, so a notification-side failure must not be
// reported as a failed case creation.
func TestSNCaseService_CreateCase_PublishFailureDoesNotFailCreateCase(t *testing.T) {
	const caseSysid = "6666666666666666666666666666ffff"
	const projectSysid = "7777777777777777777777777777aaaa"
	const watcherSysid = "8888888888888888888888888888bbbb"

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-011",
		"number": "CS0011001",
		"title": "Publish will fail",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`

	client := newTestCreateCaseClient(t, caseSysid, getCaseBody)
	publisher := &mockEventPublisher{err: errors.New("event hub unreachable")}
	svc := NewServiceNowCaseService(client, nil, publisher)

	req := domain.CreateCaseRequest{
		Type:              "case",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
		Subject:           "Publish will fail",
		Description:       "d",
		Severity:          domain.CaseSeverityHigh,
		IssueType:         domain.CaseIssueTypeQuestion,
	}

	resp, err := svc.CreateCase(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("CreateCase must succeed even when publishing fails, got: %v", err)
	}
	if resp.Case.Number != "CS0009001" {
		t.Fatalf("unexpected case number: %s", resp.Case.Number)
	}
	if len(publisher.calls) != 1 {
		t.Fatalf("expected the publish attempt to still happen, got %d calls", len(publisher.calls))
	}
}

// TestSNCaseService_CreateCase_NoPublisherConfigured verifies that a nil
// publisher (Event Hub not configured) is a silent no-op, not a panic —
// every pre-existing test in this package already relies on this (they
// construct NewServiceNowCaseService with a nil publisher), but this test
// pins it explicitly against a case that does have watchers, so it's
// unambiguous that skipping is due to the nil publisher and not the
// no-watchers path exercised above.
func TestSNCaseService_CreateCase_NoPublisherConfigured(t *testing.T) {
	const caseSysid = "9999999999999999999999999999cccc"

	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"message": "Case created successfully",
			"case": {"id": "` + caseSysid + `", "number": "CS0012001", "createdBy": "jane.doe@example.com", "createdOn": "2026-01-02 10:00:00", "state": {"id": 1, "label": "Open"}}
		}`))
	})
	svc := NewServiceNowCaseService(client, nil, nil)

	req := domain.CreateCaseRequest{
		Type:              "case",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
		Subject:           "No publisher configured",
		Description:       "d",
		Severity:          domain.CaseSeverityHigh,
		IssueType:         domain.CaseIssueTypeQuestion,
	}

	if _, err := svc.CreateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

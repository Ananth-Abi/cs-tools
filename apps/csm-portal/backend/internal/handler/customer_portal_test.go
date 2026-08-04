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

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type portalCall struct {
	operation string
	id        string
	body      []byte
}

type portalClientStub struct{ calls []portalCall }

func (s *portalClientStub) record(operation, id string, body []byte) ([]byte, error) {
	s.calls = append(s.calls, portalCall{operation: operation, id: id, body: append([]byte(nil), body...)})
	return []byte(`{"ok":true}`), nil
}
func (s *portalClientStub) GetMetadata(context.Context) ([]byte, error) {
	return s.record("metadata", "", nil)
}
func (s *portalClientStub) GlobalSearch(_ context.Context, b []byte) ([]byte, error) {
	return s.record("globalSearch", "", b)
}
func (s *portalClientStub) SearchInstances(_ context.Context, b []byte) ([]byte, error) {
	return s.record("instances", "", b)
}
func (s *portalClientStub) CreateCaseAttachment(_ context.Context, b []byte) ([]byte, error) {
	return s.record("createAttachment", "", b)
}
func (s *portalClientStub) SearchCaseAttachments(_ context.Context, b []byte) ([]byte, error) {
	return s.record("searchAttachments", "", b)
}
func (s *portalClientStub) GetAttachment(_ context.Context, id string) ([]byte, error) {
	return s.record("getAttachment", id, nil)
}
func (s *portalClientStub) PatchAttachment(_ context.Context, id string, b []byte) ([]byte, error) {
	return s.record("patchAttachment", id, b)
}
func (s *portalClientStub) GetCaseFeedback(_ context.Context, id string) ([]byte, error) {
	return s.record("getFeedback", id, nil)
}
func (s *portalClientStub) SubmitCaseFeedback(_ context.Context, id string, b []byte) ([]byte, error) {
	return s.record("submitFeedback", id, b)
}
func (s *portalClientStub) SearchConversations(_ context.Context, b []byte) ([]byte, error) {
	return s.record("searchConversations", "", b)
}
func (s *portalClientStub) GetConversation(_ context.Context, id string) ([]byte, error) {
	return s.record("getConversation", id, nil)
}
func (s *portalClientStub) CreateConversation(_ context.Context, b []byte) ([]byte, error) {
	return s.record("createConversation", "", b)
}
func (s *portalClientStub) UpdateConversation(_ context.Context, id string, b []byte) ([]byte, error) {
	return s.record("updateConversation", id, b)
}
func (s *portalClientStub) GetProductVulnerabilityMetadata(context.Context) ([]byte, error) {
	return s.record("vulnerabilityMetadata", "", nil)
}
func (s *portalClientStub) SearchCaseTimeCards(_ context.Context, b []byte) ([]byte, error) {
	return s.record("caseTimeCards", "", b)
}
func (s *portalClientStub) SearchInstanceMetrics(_ context.Context, b []byte) ([]byte, error) {
	return s.record("instanceMetrics", "", b)
}
func (s *portalClientStub) SearchInstanceUsage(_ context.Context, b []byte) ([]byte, error) {
	return s.record("instanceUsage", "", b)
}
func (s *portalClientStub) SearchInstanceMetricsStats(_ context.Context, b []byte) ([]byte, error) {
	return s.record("instanceMetricsStats", "", b)
}
func (s *portalClientStub) SearchInstanceUsageStats(_ context.Context, b []byte) ([]byte, error) {
	return s.record("instanceUsageStats", "", b)
}
func (s *portalClientStub) CreateEscalation(_ context.Context, b []byte) ([]byte, error) {
	return s.record("createEscalation", "", b)
}
func (s *portalClientStub) SearchEscalations(_ context.Context, b []byte) ([]byte, error) {
	return s.record("searchEscalations", "", b)
}
func (s *portalClientStub) SearchDeployedProductMetrics(_ context.Context, id string, b []byte) ([]byte, error) {
	return s.record("deployedProductMetrics", id, b)
}
func (s *portalClientStub) SearchDeployedProductUsageCounts(_ context.Context, id string, b []byte) ([]byte, error) {
	return s.record("deployedProductUsageCounts", id, b)
}

const portalTestID = "11111111-1111-1111-1111-111111111111"

func portalRequest(method, path, body string, pathValues map[string]string) *http.Request {
	r := withUser(httptest.NewRequest(method, path, strings.NewReader(body)))
	for key, value := range pathValues {
		r.SetPathValue(key, value)
	}
	return r
}

func lastPortalPayload(t *testing.T, client *portalClientStub) map[string]any {
	t.Helper()
	if len(client.calls) == 0 {
		t.Fatal("entity client was not called")
	}
	var payload map[string]any
	if err := json.Unmarshal(client.calls[len(client.calls)-1].body, &payload); err != nil {
		t.Fatalf("decode forwarded body: %v", err)
	}
	return payload
}

func TestCustomerPortalHandler_ScopedInstanceRoutesInjectOnlyTheirScope(t *testing.T) {
	tests := []struct {
		name, field string
		call        func(*CustomerPortalHandler, http.ResponseWriter, *http.Request)
	}{
		{"project", "projectIds", (*CustomerPortalHandler).SearchProjectInstances},
		{"deployment", "deploymentIds", (*CustomerPortalHandler).SearchDeploymentInstances},
		{"deployed product", "deployedProductIds", (*CustomerPortalHandler).SearchDeployedProductInstances},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &portalClientStub{}
			h := NewCustomerPortalHandler(client)
			w := httptest.NewRecorder()
			r := portalRequest(http.MethodPost, "/scope/instances/search", `{"filters":{"startDate":"2026-01-01"},"pagination":{"limit":20}}`, map[string]string{"id": portalTestID})
			tc.call(h, w, r)
			assertStatus(t, w, http.StatusOK)
			filters := lastPortalPayload(t, client)["filters"].(map[string]any)
			ids := filters[tc.field].([]any)
			if len(ids) != 1 || ids[0] != portalTestID {
				t.Fatalf("%s = %#v", tc.field, ids)
			}
			if filters["startDate"] != "2026-01-01" {
				t.Errorf("existing filters were not preserved: %#v", filters)
			}
		})
	}
}

func TestCustomerPortalHandler_CreateConversationInjectsProjectID(t *testing.T) {
	client := &portalClientStub{}
	h := NewCustomerPortalHandler(client)
	w := httptest.NewRecorder()
	r := portalRequest(http.MethodPost, "/projects/"+portalTestID+"/conversations", `{"message":"hello"}`, map[string]string{"id": portalTestID})
	h.CreateProjectConversation(w, r)
	assertStatus(t, w, http.StatusOK)
	payload := lastPortalPayload(t, client)
	if payload["projectId"] != portalTestID || payload["initialMessage"] != "hello" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestCustomerPortalHandler_TranslatesLegacyGlobalSearchAndConversationPayloads(t *testing.T) {
	client := &portalClientStub{}
	h := NewCustomerPortalHandler(client)

	w := httptest.NewRecorder()
	r := portalRequest(http.MethodPost, "/search", `{"filters":{"types":["projects"]},"projectsPagination":{"limit":75}}`, nil)
	h.GlobalSearch(w, r)
	assertStatus(t, w, http.StatusOK)
	payload := lastPortalPayload(t, client)
	filters := payload["filters"].(map[string]any)
	if _, exists := filters["types"]; exists {
		t.Error("legacy types field was forwarded")
	}
	if filters["tables"].([]any)[0] != "projects" {
		t.Fatalf("filters = %#v", filters)
	}
	if payload["projectsPagination"].(map[string]any)["limit"] != float64(50) {
		t.Fatalf("pagination = %#v", payload["projectsPagination"])
	}

	w = httptest.NewRecorder()
	r = portalRequest(http.MethodPost, "/projects/"+portalTestID+"/conversations/search", `{"filters":{"stateKeys":[2,3]}}`, map[string]string{"id": portalTestID})
	h.SearchProjectConversations(w, r)
	assertStatus(t, w, http.StatusOK)
	payload = lastPortalPayload(t, client)
	filters = payload["filters"].(map[string]any)
	states := filters["states"].([]any)
	if states[0] != "ACTIVE" || states[1] != "RESOLVED" {
		t.Fatalf("states = %#v", states)
	}
}

func TestCustomerPortalHandler_TranslatesConversationStatusUpdate(t *testing.T) {
	client := &portalClientStub{}
	h := NewCustomerPortalHandler(client)
	w := httptest.NewRecorder()
	r := portalRequest(http.MethodPatch, "/conversations/"+portalTestID, `{"status":"closed"}`, map[string]string{"id": portalTestID})
	h.UpdateConversation(w, r)
	assertStatus(t, w, http.StatusOK)
	if lastPortalPayload(t, client)["state"] != "CLOSED" {
		t.Fatalf("payload = %#v", lastPortalPayload(t, client))
	}
}

func TestCustomerPortalHandler_EscalationValidationAndInjection(t *testing.T) {
	client := &portalClientStub{}
	h := NewCustomerPortalHandler(client)

	w := httptest.NewRecorder()
	r := portalRequest(http.MethodPost, "/cases/"+portalTestID+"/escalations", `{}`, map[string]string{"caseId": portalTestID})
	h.CreateCaseEscalation(w, r)
	assertStatus(t, w, http.StatusBadRequest)

	w = httptest.NewRecorder()
	r = portalRequest(http.MethodPost, "/cases/"+portalTestID+"/escalations", `{"action":"escalate","reason":"Need help"}`, map[string]string{"caseId": portalTestID})
	h.CreateCaseEscalation(w, r)
	assertStatus(t, w, http.StatusCreated)
	payload := lastPortalPayload(t, client)
	if payload["caseId"] != portalTestID || payload["action"] != "ESCALATE" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestCustomerPortalHandler_DeployedProductMetricsInjectsDeploymentID(t *testing.T) {
	client := &portalClientStub{}
	h := NewCustomerPortalHandler(client)
	w := httptest.NewRecorder()
	r := portalRequest(http.MethodPost, "/metrics", `{"startDate":"2026-01-01","endDate":"2026-02-01"}`, map[string]string{"deploymentId": portalTestID, "productId": portalTestID})
	h.SearchDeployedProductMetrics(w, r)
	assertStatus(t, w, http.StatusOK)
	if client.calls[0].id != portalTestID {
		t.Errorf("deployed product id = %q", client.calls[0].id)
	}
	if lastPortalPayload(t, client)["deploymentId"] != portalTestID {
		t.Errorf("deploymentId was not injected")
	}
}

func TestCustomerPortalHandler_RequiresAuthenticationBeforeValidation(t *testing.T) {
	h := NewCustomerPortalHandler(&portalClientStub{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/conversations/not-a-uuid", strings.NewReader(`{}`))
	r.SetPathValue("id", "not-a-uuid")
	h.UpdateConversation(w, r)
	assertStatus(t, w, http.StatusUnauthorized)
}

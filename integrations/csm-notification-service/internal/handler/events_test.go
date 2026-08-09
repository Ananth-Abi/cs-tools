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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockPublisher is a test double for eventPublisher.
type mockPublisher struct {
	err              error
	called           bool
	gotKey, gotValue []byte
}

func (m *mockPublisher) Publish(ctx context.Context, key, value []byte) error {
	m.called = true
	m.gotKey, m.gotValue = key, value
	return m.err
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status = %d, want %d (body=%s)", w.Code, want, w.Body.String())
	}
}

func postEvent(t *testing.T, pub *mockPublisher, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewEventsHandler(pub)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/events", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.PostEvent(w, r)
	return w
}

const validCaseCreated = `{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"n","projectName":"p","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`
const validCommentAdded = `{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"n","projectId":"p","caseTitle":"t","caseComment":"c","commentLink":"https://x#c","caseLink":"https://x","recipients":["r@x.com"]}}`
const validStatusChanged = `{"type":"case.status_changed","entityId":"CASE-1","payload":{"caseId":"CASE-1","newStatus":"Open","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`
const validCaseAssigned = `{"type":"case.assigned","entityId":"CASE-1","payload":{"assignerName":"n","assignerEmail":"e@x.com","caseId":"CASE-1","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`
const validIncidentCreated = `{"type":"incident.created","entityId":"INC-1","payload":{"product":"api-manager","title":"P1 outage","shortDescription":"Everything is down","incidentLink":"https://x/INC-1","callTo":"+15551234567"}}`

func TestPostEvent_ValidEvents(t *testing.T) {
	cases := map[string]struct {
		body string
		key  string
	}{
		"case.created":        {validCaseCreated, "CASE-1"},
		"case.comment_added":  {validCommentAdded, "CASE-1"},
		"case.status_changed": {validStatusChanged, "CASE-1"},
		"case.assigned":       {validCaseAssigned, "CASE-1"},
		"incident.created":    {validIncidentCreated, "INC-1"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			pub := &mockPublisher{}
			w := postEvent(t, pub, c.body)
			assertStatus(t, w, http.StatusAccepted)
			if !pub.called {
				t.Fatal("expected Publish to be called")
			}
			if string(pub.gotKey) != c.key {
				t.Errorf("publish key = %q, want %q", pub.gotKey, c.key)
			}
			if string(pub.gotValue) != c.body {
				t.Errorf("publish value = %q, want original request body %q", pub.gotValue, c.body)
			}
		})
	}
}

func TestPostEvent_RequiresFields(t *testing.T) {
	cases := map[string]string{
		"missing entityId":                        `{"type":"case.created","payload":{"reporterName":"n","projectName":"p","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`,
		"whitespace-only entityId":                `{"type":"case.created","entityId":"   ","payload":{"reporterName":"n","projectName":"p","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`,
		"unknown type":                            `{"type":"case.deleted","entityId":"CASE-1","payload":{}}`,
		"missing payload":                         `{"type":"case.created","entityId":"CASE-1"}`,
		"case.created missing caseTitle":          `{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"n","projectName":"p","caseId":"CASE-1","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`,
		"case.created missing recipients":         `{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"n","projectName":"p","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","caseLink":"https://x","commentLink":"https://x#c"}}`,
		"case.created empty recipients":           `{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"n","projectName":"p","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","caseLink":"https://x","commentLink":"https://x#c","recipients":[]}}`,
		"comment_added missing caseComment":       `{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"n","projectId":"p","caseTitle":"t","commentLink":"https://x#c","caseLink":"https://x","recipients":["r@x.com"]}}`,
		"comment_added missing recipients":        `{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"n","projectId":"p","caseTitle":"t","caseComment":"c","commentLink":"https://x#c","caseLink":"https://x"}}`,
		"status_changed missing newStatus":        `{"type":"case.status_changed","entityId":"CASE-1","payload":{"caseId":"CASE-1","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`,
		"status_changed missing recipients":       `{"type":"case.status_changed","entityId":"CASE-1","payload":{"caseId":"CASE-1","newStatus":"Open","caseLink":"https://x","commentLink":"https://x#c"}}`,
		"assigned missing assignerEmail":          `{"type":"case.assigned","entityId":"CASE-1","payload":{"assignerName":"n","caseId":"CASE-1","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`,
		"assigned missing recipients":             `{"type":"case.assigned","entityId":"CASE-1","payload":{"assignerName":"n","assignerEmail":"e@x.com","caseId":"CASE-1","caseLink":"https://x","commentLink":"https://x#c"}}`,
		"incident missing callTo":                 `{"type":"incident.created","entityId":"INC-1","payload":{"product":"api-manager","title":"t","shortDescription":"d","incidentLink":"https://x/INC-1"}}`,
		"incident malformed callTo":               `{"type":"incident.created","entityId":"INC-1","payload":{"product":"api-manager","title":"t","shortDescription":"d","incidentLink":"https://x/INC-1","callTo":"555-1234"}}`,
		"incident missing product":                `{"type":"incident.created","entityId":"INC-1","payload":{"title":"t","shortDescription":"d","incidentLink":"https://x/INC-1","callTo":"+15551234567"}}`,
		"unknown top-level field":                 `{"type":"case.created","entityId":"CASE-1","extra":true,"payload":{"reporterName":"n","projectName":"p","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`,
		"unknown payload field":                   `{"type":"case.status_changed","entityId":"CASE-1","payload":{"caseId":"CASE-1","newStatus":"Open","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"],"extra":true}}`,
		"empty body":                              ``,
		"invalid json":                            `not json`,
		"case.created blank recipient":            `{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"n","projectName":"p","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","caseLink":"https://x","commentLink":"https://x#c","recipients":[""]}}`,
		"case.created malformed recipient":        `{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"n","projectName":"p","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","caseLink":"https://x","commentLink":"https://x#c","recipients":["not-an-email"]}}`,
		"case.created entityId/caseId mismatch":   `{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"n","projectName":"p","caseId":"CASE-2","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`,
		"status_changed entityId/caseId mismatch": `{"type":"case.status_changed","entityId":"CASE-1","payload":{"caseId":"CASE-2","newStatus":"Open","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`,
		"assigned entityId/caseId mismatch":       `{"type":"case.assigned","entityId":"CASE-1","payload":{"assignerName":"n","assignerEmail":"e@x.com","caseId":"CASE-2","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			pub := &mockPublisher{}
			w := postEvent(t, pub, body)
			assertStatus(t, w, http.StatusBadRequest)
			if pub.called {
				t.Error("Publish should not be called for an invalid event")
			}
		})
	}
}

func TestPostEvent_RejectsTrailingData(t *testing.T) {
	pub := &mockPublisher{}
	w := postEvent(t, pub, validCaseCreated+`{"garbage":true}`)
	assertStatus(t, w, http.StatusBadRequest)
	if pub.called {
		t.Error("Publish should not be called when trailing data is present")
	}
}

func TestPostEvent_RejectsOversizedBody(t *testing.T) {
	pub := &mockPublisher{}
	huge := strings.Repeat("a", maxRequestBodyBytes+1)
	body := `{"type":"case.status_changed","entityId":"CASE-1","payload":{"caseId":"CASE-1","newStatus":"` + huge + `","caseLink":"https://x","commentLink":"https://x#c","recipients":["r@x.com"]}}`
	w := postEvent(t, pub, body)
	assertStatus(t, w, http.StatusRequestEntityTooLarge)
}

func TestPostEvent_MapsPublishFailure(t *testing.T) {
	pub := &mockPublisher{err: errors.New("event hub unreachable")}
	w := postEvent(t, pub, validCaseCreated)
	assertStatus(t, w, http.StatusInternalServerError)
}

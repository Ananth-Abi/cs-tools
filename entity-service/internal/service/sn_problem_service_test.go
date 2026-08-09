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
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSNProblemService_SearchProblems_NumberFilterPassedThrough verifies the
// exact-match Number filter reaches the outgoing payload under the "number" key
// unchanged, alongside the untouched free-text searchQuery.
func TestSNProblemService_SearchProblems_NumberFilterPassedThrough(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/problems/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"problems": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProblemService(client)

	req := domain.SearchProblemsRequest{
		Filters: domain.SearchProblemsFilters{Number: strPtr("PRB0010001")},
	}
	if _, err := svc.SearchProblems(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotFilters, ok := gotBody["filters"].(map[string]any)
	if !ok {
		t.Fatalf("expected filters object in payload, got %+v", gotBody["filters"])
	}
	if gotFilters["number"] != "PRB0010001" {
		t.Fatalf("filters.number: got %v, want %q", gotFilters["number"], "PRB0010001")
	}
	if _, hasSearchQuery := gotFilters["searchQuery"]; hasSearchQuery {
		t.Fatalf("filters.searchQuery: expected omitted (empty), got %v", gotFilters["searchQuery"])
	}
}

// TestSNProblemService_SearchProblems_RejectsOverlongNumber verifies an
// oversized exact-match Number filter is rejected as a *apierror.ValidationError
// before the ServiceNow client is ever called.
func TestSNProblemService_SearchProblems_RejectsOverlongNumber(t *testing.T) {
	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("/problems/search", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"problems": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProblemService(client)

	req := domain.SearchProblemsRequest{
		Filters: domain.SearchProblemsFilters{Number: strPtr(strings.Repeat("x", maxExactNumberLen+1))},
	}
	_, err := svc.SearchProblems(contextWithUserIDToken("token"), req)

	var valErr *apierror.ValidationError
	if err == nil {
		t.Fatal("expected a validation error, got nil")
	}
	if ok := asValidationError(err, &valErr); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
	if called {
		t.Fatal("ServiceNow client was called despite the invalid filter")
	}
}

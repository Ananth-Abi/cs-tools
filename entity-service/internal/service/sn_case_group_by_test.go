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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSNCaseService_GroupCasesBy_CallsGroupByEndpointAndMapsResponse proves
// GroupCasesBy posts to the dedicated /cases/group-by endpoint (not
// /cases/search), forwards groupBy/maxGroups alongside the parsed filter
// payload, and maps the response into the shared domain.GroupByResponse
// shape untouched.
func TestSNCaseService_GroupCasesBy_CallsGroupByEndpointAndMapsResponse(t *testing.T) {
	var gotBody struct {
		Filters struct {
			CaseTypes  []string `json:"caseTypes"`
			StateKeys  []string `json:"stateKeys"`
			ProjectIDs []string `json:"projectIds"`
		} `json:"filters"`
		GroupBy   string `json:"groupBy"`
		MaxGroups int    `json:"maxGroups"`
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/cases/group-by", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"groups": []map[string]any{
				{"key": "acme", "label": "Acme Corp", "count": 12},
				{"key": "globex", "label": "Globex Inc", "count": 7},
			},
			"othersCount":  3,
			"totalRecords": 22,
		})
	})
	// A request that lands here instead means GroupCasesBy called the wrong
	// endpoint (the plain search path) rather than the dedicated group-by one.
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("GroupCasesBy must not call /cases/search")
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	req := domain.GroupCasesByRequest{
		Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{
				{Field: "type", Op: "in", Values: []string{"case"}},
			},
		},
		GroupBy:   "account",
		MaxGroups: 12,
	}

	resp, err := svc.GroupCasesBy(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotBody.GroupBy != "account" {
		t.Fatalf("groupBy = %q, want %q", gotBody.GroupBy, "account")
	}
	if gotBody.MaxGroups != 12 {
		t.Fatalf("maxGroups = %d, want 12", gotBody.MaxGroups)
	}
	// "case" is SN's "default_case" wire value -- same translation SearchCases
	// applies via snCaseTypeMap/domainTypeKeysToSN.
	if len(gotBody.Filters.CaseTypes) != 1 || gotBody.Filters.CaseTypes[0] != "default_case" {
		t.Fatalf("caseTypes = %v, want [default_case] (filters must be parsed/forwarded exactly as search does)", gotBody.Filters.CaseTypes)
	}

	want := domain.GroupByResponse{
		Groups: []domain.GroupByBucket{
			{Key: "acme", Label: "Acme Corp", Count: 12},
			{Key: "globex", Label: "Globex Inc", Count: 7},
		},
		OthersCount:  3,
		TotalRecords: 22,
	}
	if len(resp.Groups) != len(want.Groups) {
		t.Fatalf("groups = %+v, want %+v", resp.Groups, want.Groups)
	}
	for i := range want.Groups {
		if resp.Groups[i] != want.Groups[i] {
			t.Fatalf("groups[%d] = %+v, want %+v", i, resp.Groups[i], want.Groups[i])
		}
	}
	if resp.OthersCount != want.OthersCount {
		t.Fatalf("othersCount = %d, want %d", resp.OthersCount, want.OthersCount)
	}
	if resp.TotalRecords != want.TotalRecords {
		t.Fatalf("totalRecords = %d, want %d", resp.TotalRecords, want.TotalRecords)
	}
	// Sum of groups' counts + othersCount must equal totalRecords, matching
	// the live-verified SN/Ballerina contract's own arithmetic invariant.
	sum := want.OthersCount
	for _, g := range want.Groups {
		sum += g.Count
	}
	if sum != want.TotalRecords {
		t.Fatalf("groups+othersCount = %d, want totalRecords %d", sum, want.TotalRecords)
	}
}

// TestSNCaseService_GroupCasesBy_RejectsBadFilterFieldAndCombo proves a
// filter-parse rejection from ParseCaseFieldFilters propagates through
// GroupCasesBy exactly as it does through SearchCases, and that no request
// reaches ServiceNow when that happens.
func TestSNCaseService_GroupCasesBy_RejectsBadFilterFieldAndCombo(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/group-by", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("GroupCasesBy must not call ServiceNow when filter parsing fails")
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	t.Run("bad field name", func(t *testing.T) {
		req := domain.GroupCasesByRequest{
			Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "bogusField", Op: "in", Values: []string{"x"}}},
			},
			GroupBy: "account",
		}
		_, err := svc.GroupCasesBy(ctx, req)
		if _, ok := err.(*apierror.ValidationError); !ok {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})

	t.Run("bad field+op combo", func(t *testing.T) {
		req := domain.GroupCasesByRequest{
			Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "type", Op: "gte", Values: []string{"case"}}},
			},
			GroupBy: "account",
		}
		_, err := svc.GroupCasesBy(ctx, req)
		if _, ok := err.(*apierror.ValidationError); !ok {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})

	t.Run("missing groupBy", func(t *testing.T) {
		req := domain.GroupCasesByRequest{}
		_, err := svc.GroupCasesBy(ctx, req)
		if _, ok := err.(*apierror.ValidationError); !ok {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})
}

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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSNProductVersionService_SearchProductVersions_PayloadHasNoFiltersKey
// guards against reintroducing a "filters" key on the outbound payload to the
// Choreo POST /products/{id}/versions/search endpoint. The Choreo API's
// ProductVersionSearchPayload is a closed Ballerina record with only a
// "pagination" field: any "filters" key, including an empty "filters": {},
// makes the downstream call fail at bind time with every single call 400ing.
// Go's `omitempty` never suppresses a non-pointer struct field, so this must
// be verified against the actual serialized JSON, not just the Go struct
// definition.
func TestSNProductVersionService_SearchProductVersions_PayloadHasNoFiltersKey(t *testing.T) {
	cases := []struct {
		name        string
		searchQuery string
	}{
		{name: "empty search query", searchQuery: ""},
		{name: "populated search query", searchQuery: "3.2.0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]any
			mux := http.NewServeMux()
			mux.HandleFunc("/products/", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Fatalf("expected POST, got %s", r.Method)
				}
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"versions": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
			})

			client := newTestSNClient(t, mux)
			svc := NewServiceNowProductVersionService(client)

			req := domain.SearchProductVersionsRequest{
				ProductID:   "11111111-2222-3333-4444-555555555555",
				SearchQuery: tc.searchQuery,
				Pagination:  domain.Pagination{Limit: 20, Offset: 0},
			}
			if _, err := svc.SearchProductVersions(contextWithUserIDToken("token"), req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if _, hasFilters := gotBody["filters"]; hasFilters {
				t.Fatalf("expected no \"filters\" key in outbound payload, got %+v", gotBody["filters"])
			}
			if _, hasPagination := gotBody["pagination"]; !hasPagination {
				t.Fatalf("expected \"pagination\" key in outbound payload, got %+v", gotBody)
			}
		})
	}
}

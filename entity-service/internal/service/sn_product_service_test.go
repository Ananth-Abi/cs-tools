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

// TestSNProductService_SearchProducts_DefaultsClassToProductModel guards
// against regressing to the pre-fix behavior where an unfiltered product
// search returned one row per product *version* (e.g. 20 rows all named
// "Identity Server") instead of one row per distinct product. When the
// caller sends no class, the request forwarded to the Choreo product search
// endpoint must carry class=product_model.
func TestSNProductService_SearchProducts_DefaultsClassToProductModel(t *testing.T) {
	var gotFilters snProductFilters
	mux := http.NewServeMux()
	mux.HandleFunc("/products/search", func(w http.ResponseWriter, r *http.Request) {
		var payload snProductSearchPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		gotFilters = payload.Filters
		_ = json.NewEncoder(w).Encode(map[string]any{
			"products":     []map[string]any{},
			"totalRecords": 0, "offset": 0, "limit": 20,
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProductService(client)

	_, err := svc.SearchProducts(contextWithUserIDToken("token"), domain.SearchProductsRequest{
		Pagination: domain.Pagination{Limit: 20, Offset: 0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotFilters.Class != "product_model" {
		t.Fatalf("expected class %q forwarded to SN, got %q", "product_model", gotFilters.Class)
	}
}

// TestSNProductService_SearchProducts_ForwardsExplicitClassUnchanged verifies
// that when the caller explicitly requests version-level rows (class=
// software_model or service_model), the entity service does not override
// that choice with the product_model default.
func TestSNProductService_SearchProducts_ForwardsExplicitClassUnchanged(t *testing.T) {
	for _, class := range []string{"software_model", "service_model", "product_model"} {
		t.Run(class, func(t *testing.T) {
			var gotFilters snProductFilters
			mux := http.NewServeMux()
			mux.HandleFunc("/products/search", func(w http.ResponseWriter, r *http.Request) {
				var payload snProductSearchPayload
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				gotFilters = payload.Filters
				_ = json.NewEncoder(w).Encode(map[string]any{
					"products":     []map[string]any{},
					"totalRecords": 0, "offset": 0, "limit": 20,
				})
			})

			client := newTestSNClient(t, mux)
			svc := NewServiceNowProductService(client)

			_, err := svc.SearchProducts(contextWithUserIDToken("token"), domain.SearchProductsRequest{
				Pagination: domain.Pagination{Limit: 20, Offset: 0},
				Class:      class,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotFilters.Class != class {
				t.Fatalf("expected class %q forwarded unchanged, got %q", class, gotFilters.Class)
			}
		})
	}
}

// TestSNProductService_SearchProducts_RejectsUnknownClass ensures a caller
// cannot forward an arbitrary string to the Choreo product search endpoint.
func TestSNProductService_SearchProducts_RejectsUnknownClass(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/products/search", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("SN endpoint should not be called for an invalid class")
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProductService(client)

	_, err := svc.SearchProducts(contextWithUserIDToken("token"), domain.SearchProductsRequest{
		Pagination: domain.Pagination{Limit: 20, Offset: 0},
		Class:      "bogus_model",
	})
	if err == nil {
		t.Fatal("expected validation error for unknown class, got nil")
	}
}

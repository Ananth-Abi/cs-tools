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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// countsFakeEntityClient serves a fixed deployment list and a fixed deployed
// product list. entityDeploymentClient is embedded (nil) so only the two
// methods under test need implementing.
type countsFakeEntityClient struct {
	entityDeploymentClient
	deployments     []entity.DeploymentView
	deployedProduct []entity.DeployedProductView
	productsErr     error
	productsCalls   int
}

func (f *countsFakeEntityClient) SearchDeployments(_ context.Context, _ entity.SearchDeploymentsRequest) (entity.SearchDeploymentsResponse, error) {
	return entity.SearchDeploymentsResponse{
		Deployments: f.deployments,
		Total:       len(f.deployments),
	}, nil
}

func (f *countsFakeEntityClient) SearchDeployedProducts(_ context.Context, _ entity.SearchDeployedProductsRequest) (entity.SearchDeployedProductsResponse, error) {
	f.productsCalls++
	if f.productsErr != nil {
		return entity.SearchDeployedProductsResponse{}, f.productsErr
	}
	return entity.SearchDeployedProductsResponse{
		DeployedProducts: f.deployedProduct,
		Total:            len(f.deployedProduct),
	}, nil
}

// searchDeploymentsBody runs SearchDeployments through a real ServeMux using
// main.go's exact pattern, and returns the decoded deployments array.
func searchDeploymentsBody(t *testing.T, fake *countsFakeEntityClient) []map[string]any {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{id}/deployments/search", NewDeploymentHandler(fake).SearchDeployments)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, authedRequest(http.MethodPost, "/projects/"+testProjectID+"/deployments/search", "{}"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var got struct {
		Deployments []map[string]any `json:"deployments"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	return got.Deployments
}

// TestSearchDeployments_EmptyProductSearchEmitsZeroCounts covers the case that
// motivated seeding the tally map: the deployed-product search succeeds but
// returns nothing. Every deployment has genuinely been counted, so each must
// report productCount 0 rather than omitting the field — omitting it would be
// indistinguishable from the tally having failed.
func TestSearchDeployments_EmptyProductSearchEmitsZeroCounts(t *testing.T) {
	fake := &countsFakeEntityClient{
		deployments: []entity.DeploymentView{
			{ID: "dep-1", Name: "one"},
			{ID: "dep-2", Name: "two"},
		},
		deployedProduct: nil, // successful search, no products
	}

	deployments := searchDeploymentsBody(t, fake)

	if len(deployments) != 2 {
		t.Fatalf("got %d deployments, want 2", len(deployments))
	}
	for i, d := range deployments {
		got, present := d["productCount"]
		if !present {
			t.Errorf("deployment %d: productCount omitted; a successful tally must report 0", i)
			continue
		}
		if got != float64(0) {
			t.Errorf("deployment %d: productCount = %v, want 0", i, got)
		}
	}
}

// TestSearchDeployments_CountsPerDeployment checks products are attributed to
// the deployment that owns them, and that a deployment with none still reports
// zero in the same response.
func TestSearchDeployments_CountsPerDeployment(t *testing.T) {
	fake := &countsFakeEntityClient{
		deployments: []entity.DeploymentView{
			{ID: "dep-1"}, {ID: "dep-2"}, {ID: "dep-3"},
		},
		deployedProduct: []entity.DeployedProductView{
			{ID: "p1", Deployment: entity.EntityRef{ID: "dep-1"}},
			{ID: "p2", Deployment: entity.EntityRef{ID: "dep-1"}},
			{ID: "p3", Deployment: entity.EntityRef{ID: "dep-3"}},
		},
	}

	deployments := searchDeploymentsBody(t, fake)

	want := map[string]float64{"dep-1": 2, "dep-2": 0, "dep-3": 1}
	for _, d := range deployments {
		id, _ := d["id"].(string)
		if got := d["productCount"]; got != want[id] {
			t.Errorf("%s: productCount = %v, want %v", id, got, want[id])
		}
	}

	// One upstream call for all three deployments — not an N+1.
	if fake.productsCalls != 1 {
		t.Errorf("SearchDeployedProducts called %d times, want 1", fake.productsCalls)
	}
}

// TestSearchDeployments_ProductTallyFailureOmitsCounts pins the best-effort
// contract: when the tally errors the deployment search still succeeds, and the
// counts are omitted entirely rather than reported as a misleading 0.
func TestSearchDeployments_ProductTallyFailureOmitsCounts(t *testing.T) {
	fake := &countsFakeEntityClient{
		deployments: []entity.DeploymentView{{ID: "dep-1"}},
		productsErr: errors.New("upstream unavailable"),
	}

	deployments := searchDeploymentsBody(t, fake)

	if len(deployments) != 1 {
		t.Fatalf("got %d deployments, want 1", len(deployments))
	}
	if _, present := deployments[0]["productCount"]; present {
		t.Errorf("productCount = %v, want it omitted when the tally failed", deployments[0]["productCount"])
	}
}

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

package dto

import (
	"encoding/json"
	"testing"
)

// TestWithDeploymentCounts_UsesFrontendFieldNames is the regression guard for
// the Usage Metrics page rendering blank.
//
// That page filters deployments on `(dep.productCount ?? 0) > 0`. The field is
// productCount — NOT deployedProductCount, which is what entity-service calls
// the equivalent aggregate elsewhere. When the key is absent or misnamed every
// deployment is filtered out, no deployment tab is selected, and every
// downstream metrics query is disabled by its `enabled` guard — so the page
// renders empty with no console error and no network request at all. Renaming
// this key would silently reintroduce that.
func TestWithDeploymentCounts_UsesFrontendFieldNames(t *testing.T) {
	resp := WithDeploymentCounts(
		SearchDeploymentsResponse{Deployments: []DeploymentSummary{{ID: "dep-1"}}},
		map[string]int{"dep-1": 3},
		map[string]int{"dep-1": 5},
	)

	raw, err := json.Marshal(resp.Deployments[0])
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if got["productCount"] != float64(3) {
		t.Errorf(`productCount = %v, want 3 — the Usage Metrics page filters on this exact key`, got["productCount"])
	}
	if got["instanceCount"] != float64(5) {
		t.Errorf("instanceCount = %v, want 5", got["instanceCount"])
	}
	if _, wrong := got["deployedProductCount"]; wrong {
		t.Error(`emitted "deployedProductCount"; the frontend reads "productCount"`)
	}
}

// TestWithDeploymentCounts_AbsentWhenNotCounted checks that a deployment with
// no tally is left without the key rather than reported as zero, so "we did not
// count" stays distinguishable from "counted zero" for any future consumer.
// The current frontend collapses both to 0 via `?? 0`.
func TestWithDeploymentCounts_AbsentWhenNotCounted(t *testing.T) {
	resp := WithDeploymentCounts(
		SearchDeploymentsResponse{Deployments: []DeploymentSummary{{ID: "dep-1"}, {ID: "dep-2"}}},
		map[string]int{"dep-1": 2},
		nil,
	)

	if resp.Deployments[0].ProductCount == nil || *resp.Deployments[0].ProductCount != 2 {
		t.Errorf("dep-1 ProductCount = %v, want 2", resp.Deployments[0].ProductCount)
	}
	if resp.Deployments[1].ProductCount != nil {
		t.Errorf("dep-2 ProductCount = %v, want nil (never counted)", *resp.Deployments[1].ProductCount)
	}

	raw, _ := json.Marshal(resp.Deployments[1])
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if _, present := got["productCount"]; present {
		t.Error("productCount should be omitted entirely when the deployment was never counted")
	}
}

// TestWithDeploymentCounts_NilMapsAreSafe covers the best-effort path: when the
// upstream tally fails the handler passes nil, and the response must still be
// well-formed rather than panicking or emitting zeros.
func TestWithDeploymentCounts_NilMapsAreSafe(t *testing.T) {
	resp := WithDeploymentCounts(
		SearchDeploymentsResponse{Deployments: []DeploymentSummary{{ID: "dep-1"}}},
		nil, nil,
	)
	if resp.Deployments[0].ProductCount != nil || resp.Deployments[0].InstanceCount != nil {
		t.Error("nil count maps must leave both counts unset")
	}
}

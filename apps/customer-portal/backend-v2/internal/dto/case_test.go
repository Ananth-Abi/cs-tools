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
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// TestBuildEntitySearchCasesRequest_AllFiltersTranslate guards against
// reintroducing the bug where this backend sent entity-service's old
// named-filter-field case-search shape directly instead of building the
// generic filter-predicate array entity-service's current /cases/search
// requires (which rejects unknown fields outright) — every filter the
// portal exposes must produce exactly the CaseFieldFilter entry
// entity-service's case_filters.go expects.
func TestBuildEntitySearchCasesRequest_AllFiltersTranslate(t *testing.T) {
	parentID := "parent-id-1"
	req := CaseSearchRequest{
		Filters: CaseSearchFilters{
			Types:           []string{"case", "engagement"},
			SearchQuery:     "login issue",
			ProjectIDs:      []string{"proj-1"},
			DeploymentIDs:   []string{"dep-1"},
			States:          []string{"open"},
			Severities:      []string{"high"},
			IssueTypes:      []string{"question"},
			EngagementTypes: []string{"migration"},
			CreatedBy:       []string{"user-1"},
			WorkStates:      []string{"ongoing"},
			AssignedUserIDs: []string{"eng-1"},
			ProductNames:    []string{"API Manager"},
			Tags:            []string{"urgent"},
			ParentID:        &parentID,
		},
		SortBy:     entity.CaseSort{Field: "createdOn", Order: "desc"},
		Pagination: entity.Pagination{Limit: 20, Offset: 0},
	}

	got := BuildEntitySearchCasesRequest(req)

	if got.Filters.SearchQuery != "login issue" {
		t.Fatalf("SearchQuery = %q", got.Filters.SearchQuery)
	}
	if got.SortBy != req.SortBy {
		t.Fatalf("SortBy = %+v, want %+v", got.SortBy, req.SortBy)
	}
	if got.Pagination != req.Pagination {
		t.Fatalf("Pagination = %+v, want %+v", got.Pagination, req.Pagination)
	}

	want := []entity.CaseFieldFilter{
		{Field: "type", Op: "in", Values: []string{"case", "engagement"}},
		{Field: "projectId", Op: "in", Values: []string{"proj-1"}},
		{Field: "deploymentId", Op: "in", Values: []string{"dep-1"}},
		{Field: "state", Op: "in", Values: []string{"open"}},
		{Field: "severity", Op: "in", Values: []string{"high"}},
		{Field: "issueType", Op: "in", Values: []string{"question"}},
		{Field: "engagementType", Op: "in", Values: []string{"migration"}},
		{Field: "workState", Op: "in", Values: []string{"ongoing"}},
		{Field: "assignedUserId", Op: "in", Values: []string{"eng-1"}},
		{Field: "product", Op: "in", Values: []string{"API Manager"}},
		{Field: "tag", Op: "in", Values: []string{"urgent"}},
		{Field: "createdBy", Op: "in", Values: []string{"user-1"}},
		{Field: "parentId", Op: "eq", Values: []string{"parent-id-1"}},
	}
	if !reflect.DeepEqual(got.Filters.Filters, want) {
		t.Fatalf("Filters = %+v,\nwant     %+v", got.Filters.Filters, want)
	}
}

// TestBuildEntitySearchCasesRequest_CreatedByMe verifies CreatedByMe becomes
// a createdBy+eq filter carrying entity-service's exact current-user
// placeholder string — a mismatch here would make entity-service reject it
// as "createdBy eq only supports values: [...]" instead of resolving the
// caller.
func TestBuildEntitySearchCasesRequest_CreatedByMe(t *testing.T) {
	req := CaseSearchRequest{Filters: CaseSearchFilters{CreatedByMe: true}}

	got := BuildEntitySearchCasesRequest(req)

	want := []entity.CaseFieldFilter{
		{Field: "createdBy", Op: "eq", Values: []string{"__current_user_email__"}},
	}
	if !reflect.DeepEqual(got.Filters.Filters, want) {
		t.Fatalf("Filters = %+v, want %+v", got.Filters.Filters, want)
	}
}

// TestBuildEntitySearchCasesRequest_EmptyFiltersProduceNoPredicates verifies
// an empty/default CaseSearchRequest builds a request with no filter
// predicates at all -- entity-service's own case search already treats a
// missing "type" filter (etc.) as "no restriction", so this must not
// fabricate empty-but-present filter entries.
func TestBuildEntitySearchCasesRequest_EmptyFiltersProduceNoPredicates(t *testing.T) {
	got := BuildEntitySearchCasesRequest(CaseSearchRequest{})

	if len(got.Filters.Filters) != 0 {
		t.Fatalf("expected no filter predicates for an empty request, got %+v", got.Filters.Filters)
	}
	if got.Filters.SearchQuery != "" {
		t.Fatalf("expected empty SearchQuery, got %q", got.Filters.SearchQuery)
	}
}

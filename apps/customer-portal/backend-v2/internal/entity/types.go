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

package entity

import (
	"encoding/json"
	"time"
)

// These types mirror entity-service's wire format 1:1 (see
// cs-tools/entity-service/internal/domain/entity.go) so json.Unmarshal can
// decode its responses directly. They are internal to this package — the
// dto package maps them into the portal's own response contracts before
// anything reaches the frontend.

// Pagination controls which page of results is requested/returned.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// --- users/me ---

// GetUserMeResponse is entity-service's response for GET /users/me.
type GetUserMeResponse struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	FirstName *string  `json:"firstName,omitempty"`
	LastName  string   `json:"lastName"`
	TimeZone  *string  `json:"timeZone,omitempty"`
	Roles     []string `json:"roles"`
}

// --- projects ---

// SearchProjectsRequest is the input for POST /projects/search.
type SearchProjectsRequest struct {
	Pagination    Pagination `json:"pagination"`
	SearchQuery   string     `json:"searchQuery,omitempty"`
	ClosureStatus string     `json:"closureStatus,omitempty"`
	EndDateFrom   string     `json:"endDateFrom,omitempty"`
	EndDateTo     string     `json:"endDateTo,omitempty"`
	SortBy        string     `json:"sortBy,omitempty"`
	SortOrder     string     `json:"sortOrder,omitempty"`
}

// ProjectClosureFields groups the ServiceNow-only closure-tracking fields
// shared by ProjectDetailsView and ProjectView.
type ProjectClosureFields struct {
	ClosureState                   *string         `json:"closureState"`
	EndDateClosureState             *string         `json:"endDateClosureState"`
	InvoiceDueDateClosureState      *string         `json:"invoiceDueDateClosureState"`
	ComplianceViolationClosureState *string         `json:"complianceViolationClosureState"`
	ComplianceViolationDate         *string         `json:"complianceViolationDate"`
	SuspensionProcessState          json.RawMessage `json:"suspensionProcessState"`
}

// ProjectView is a single search result item from POST /projects/search.
type ProjectView struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Key              string     `json:"key"`
	SubscriptionType string     `json:"subscriptionType"`
	EndDate          *time.Time `json:"endDate"`
	CreatedOn        time.Time  `json:"createdOn"`
	ProjectClosureFields
}

// SearchProjectsResponse is entity-service's response for POST /projects/search.
type SearchProjectsResponse struct {
	Projects []ProjectView `json:"projects"`
	Total    int           `json:"total"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
	HasMore  bool          `json:"hasMore"`
}

// ProjectAccountRef is the account summary embedded in ProjectDetailsView.
type ProjectAccountRef struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	ActivationDate      *time.Time `json:"activationDate"`
	Tier                string     `json:"tier"`
	Region              *string    `json:"region"`
	AgentEnabled        bool       `json:"agentEnabled"`
	KbReferencesEnabled bool       `json:"kbReferencesEnabled"`
}

// ProjectDetailsView is entity-service's response for GET /projects/{id}.
type ProjectDetailsView struct {
	ID               string            `json:"id"`
	Account          ProjectAccountRef `json:"account"`
	SfID             string            `json:"sfId"`
	Name             string            `json:"name"`
	Key              string            `json:"key"`
	SubscriptionType string            `json:"subscriptionType"`
	StartDate        time.Time         `json:"startDate"`
	EndDate          time.Time         `json:"endDate"`
	CreatedOn        time.Time         `json:"createdOn"`
	UpdatedOn        time.Time         `json:"updatedOn"`
	ProjectClosureFields
}

// --- cases ---

// CaseSort specifies the sort field and direction for case search results.
type CaseSort struct {
	Field string `json:"field,omitempty"`
	Order string `json:"order,omitempty"`
}

// SearchCasesFilters holds the optional filter criteria for a case search.
// Only the fields the portal currently exposes are included here; extend as
// needed when more filters are surfaced to the frontend.
type SearchCasesFilters struct {
	Types           []string   `json:"types,omitempty"`
	SearchQuery     string     `json:"searchQuery,omitempty"`
	ProjectIDs      []string   `json:"projectIds,omitempty"`
	DeploymentIDs   []string   `json:"deploymentIds,omitempty"`
	States          []string   `json:"states,omitempty"`
	Severities      []string   `json:"severities,omitempty"`
	IssueTypes      []string   `json:"issueTypes,omitempty"`
	EngagementTypes []string   `json:"engagementTypes,omitempty"`
	CreatedBy       []string   `json:"createdBy,omitempty"`
	CreatedByMe     bool       `json:"createdByMe,omitempty"`
	WorkStates      []string   `json:"workStates,omitempty"`
	AssignedUserIDs []string   `json:"assignedUserIds,omitempty"`
	ProductNames    []string   `json:"productNames,omitempty"`
	Tags            []string   `json:"tags,omitempty"`
	ParentID        *string    `json:"parentId,omitempty"`
}

// SearchCasesRequest is the input for POST /cases/search.
type SearchCasesRequest struct {
	Filters    SearchCasesFilters `json:"filters"`
	SortBy     CaseSort           `json:"sortBy"`
	Pagination Pagination         `json:"pagination"`
}

// EntityRef is a compact reference to a named entity (project, deployment, product, etc.).
type EntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AssignedEngineerRef is a compact reference to an assigned support engineer.
type AssignedEngineerRef struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email"`
}

// CaseNumberRef is a compact reference to a case carrying its human-readable number.
type CaseNumberRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

// LinkedServiceRequestRef is a compact reference to a service-request case
// linked to another case as its parent.
type LinkedServiceRequestRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Name   string `json:"name"`
}

// AccountRef is a compact reference to an account.
type AccountRef struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Type    string     `json:"type"`
	CreTeam *EntityRef `json:"creTeam,omitempty"`
	SreTeam *EntityRef `json:"sreTeam,omitempty"`
}

// DeployedProductRef is a compact reference to a deployed product.
type DeployedProductRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// UserRef is a reference to a user with key display fields.
type UserRef struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	UserID string `json:"userId,omitempty"`
	Email  string `json:"email"`
}

// Tag is a free-text label attached to a case.
type Tag struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Color *string `json:"color"`
}

// SearchCaseView is a single search result item from POST /cases/search.
type SearchCaseView struct {
	ID               string               `json:"id"`
	InternalID       string               `json:"internalId"`
	Number           string               `json:"number"`
	CreatedOn        string               `json:"createdOn"`
	CreatedBy        string               `json:"createdBy"`
	Subject          *string              `json:"subject"`
	Description      *string              `json:"description"`
	IssueType        *string              `json:"issueType"`
	State            string               `json:"state"`
	Severity         *string              `json:"severity"`
	Catalog          *EntityRef           `json:"catalog"`
	CatalogItem      *EntityRef           `json:"catalogItem"`
	AssignedTeam     *EntityRef           `json:"assignedTeam"`
	Product          *EntityRef           `json:"product"`
	EngagementType   *string              `json:"engagementType"`
	WorkState        *string              `json:"workState"`
	Type             string               `json:"type"`
	Project          EntityRef            `json:"project"`
	Deployment       *EntityRef           `json:"deployment"`
	DeployedProduct  *EntityRef           `json:"deployedProduct"`
	AssignedEngineer *AssignedEngineerRef `json:"assignedEngineer"`
	ParentCase       *EntityRef           `json:"parentCase"`
	RelatedCase      *EntityRef           `json:"relatedCase"`
	Conversation     *EntityRef           `json:"conversation"`
}

// SearchCasesResponse is entity-service's response for POST /cases/search.
type SearchCasesResponse struct {
	Cases  []SearchCaseView `json:"cases"`
	Total  int              `json:"total"`
	Offset int              `json:"offset"`
	Limit  int              `json:"limit"`
}

// CaseView is entity-service's response for GET /cases/{id}.
type CaseView struct {
	ID                     string                    `json:"id"`
	Number                 string                    `json:"number"`
	InternalID             string                    `json:"internalId"`
	Subject                string                    `json:"subject"`
	Description            string                    `json:"description"`
	Severity               string                    `json:"severity"`
	IssueType              string                    `json:"issueType"`
	State                  string                    `json:"state"`
	WorkState              *string                   `json:"workState"`
	Type                   *string                   `json:"type"`
	EngagementType         *string                   `json:"engagementType"`
	CreatedOn              time.Time                 `json:"createdOn"`
	UpdatedOn              time.Time                 `json:"updatedOn"`
	ClosedOn               *time.Time                `json:"closedOn"`
	CreatedByDetails       UserRef                   `json:"createdBy"`
	ProjectDetails         EntityRef                 `json:"project"`
	DeploymentDetails      *EntityRef                `json:"deployment"`
	DeployedProductDetails *DeployedProductRef       `json:"deployedProduct"`
	ProductDetails         *EntityRef                `json:"product"`
	Catalog                *EntityRef                `json:"catalog"`
	CatalogItem            *EntityRef                `json:"catalogItem"`
	AssignedTeam           *EntityRef                `json:"assignedTeam"`
	Conversation           *EntityRef                `json:"conversation"`
	AssignedEngineer       *AssignedEngineerRef      `json:"assignedEngineer"`
	ParentCase             *CaseNumberRef            `json:"parentCase"`
	RelatedCase            *CaseNumberRef            `json:"relatedCase"`
	AccountDetails         *AccountRef               `json:"account"`
	LinkedServiceRequests  []LinkedServiceRequestRef `json:"linkedServiceRequests"`
	ResolvedOn             *time.Time                `json:"resolvedOn"`
	ResolutionCode         *string                   `json:"resolutionCode"`
	Cause                  *string                   `json:"cause"`
	ResolutionNotes        *string                   `json:"resolutionNotes"`
	// WatchList, AutoclosureStep/AutoclosureStateTime and the Best/MostLikely/
	// WorstCaseFixEta fields are intentionally NOT decoded here — entity-service
	// documents them as CSM-engineer-facing only (see entity.go comments on
	// CaseView), and the customer portal must not surface internal WSO2 support
	// workflow state to end customers.
	FixEta *time.Time `json:"fixEta"`
	Tags   []Tag      `json:"tags"`
}

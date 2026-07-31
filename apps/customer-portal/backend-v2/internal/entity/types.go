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

// PatchUserMeRequest is the request body for PATCH /users/me.
type PatchUserMeRequest struct {
	TimeZone string `json:"timeZone"`
}

// PatchUserMeUpdated contains the key fields returned after a successful user update.
type PatchUserMeUpdated struct {
	ID        string `json:"id"`
	UpdatedBy string `json:"updatedBy"`
	UpdatedOn string `json:"updatedOn"`
}

// PatchUserMeResponse is entity-service's response for PATCH /users/me.
type PatchUserMeResponse struct {
	Message string             `json:"message"`
	User    PatchUserMeUpdated `json:"user"`
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
	ClosureState                    *string         `json:"closureState"`
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

// --- accounts ---
//
// entity-service returns a different wire shape for these two endpoints
// depending on its DATA_SOURCE (postgres vs servicenow) — unlike projects and
// cases, account responses have not been unified upstream. The structs below
// are a superset of both shapes: JSON key names never collide between the two
// data sources (they use different field names for the same concept, e.g.
// "tier" vs "classification", "agentEnabled" vs "hasAgent"), and every date/
// time field is typed as *string here since a Go string field decodes a JSON
// string value regardless of whether the source type was time.Time or a
// plain string — so only the fields the active data source actually
// populates come out non-nil.

// SupportTierRef is a compact reference to a support tier carrying its label
// (ServiceNow data source, GET /accounts/{id} only).
type SupportTierRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// SearchAccountsFilters holds the optional filter criteria for an account search.
type SearchAccountsFilters struct {
	SearchQuery    string `json:"searchQuery,omitempty"`
	Active         *bool  `json:"active,omitempty"`
	Pod            string `json:"pod,omitempty"`
	Classification string `json:"classification,omitempty"`
}

// SearchAccountsRequest is the input for POST /accounts/search.
type SearchAccountsRequest struct {
	Pagination Pagination            `json:"pagination"`
	Filters    SearchAccountsFilters `json:"filters,omitempty"`
}

// AccountSummary is a single search result item from POST /accounts/search —
// a superset of entity-service's Postgres Account and ServiceNow SNAccountView.
type AccountSummary struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Region *string `json:"region,omitempty"`
	// Postgres-only.
	SfID                *string `json:"sfId,omitempty"`
	Tier                *string `json:"tier,omitempty"`
	OwnerID             *string `json:"ownerId,omitempty"`
	TechnicalOwnerID    *string `json:"technicalOwnerId,omitempty"`
	AgentEnabled        *bool   `json:"agentEnabled,omitempty"`
	KbReferencesEnabled *bool   `json:"kbReferencesEnabled,omitempty"`
	// ServiceNow-only.
	Classification  *string    `json:"classification,omitempty"`
	Pod             *string    `json:"pod,omitempty"`
	SupportTier     *string    `json:"supportTier,omitempty"`
	ArrToday        *string    `json:"arrToday,omitempty"`
	Owner           *EntityRef `json:"owner,omitempty"`
	TechnicalOwner  *EntityRef `json:"technicalOwner,omitempty"`
	HasAgent        *bool      `json:"hasAgent,omitempty"`
	HasKbReferences *bool      `json:"hasKbReferences,omitempty"`
	CreatedBy       *string    `json:"createdBy,omitempty"`
	// Shared (identical key/type on both data sources).
	ActivationDate   *string `json:"activationDate,omitempty"`
	DeactivationDate *string `json:"deactivationDate,omitempty"`
	CreatedOn        *string `json:"createdOn,omitempty"`
	UpdatedOn        *string `json:"updatedOn,omitempty"`
}

// SearchAccountsResponse is entity-service's response for POST /accounts/search.
type SearchAccountsResponse struct {
	Accounts []AccountSummary `json:"accounts"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
	HasMore  bool             `json:"hasMore"`
}

// AccountDetail is entity-service's response for GET /accounts/{id} — a
// superset of entity-service's Postgres Account and ServiceNow SNAccountDetail.
type AccountDetail struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Region *string `json:"region,omitempty"`
	// Postgres-only.
	SfID                *string `json:"sfId,omitempty"`
	Tier                *string `json:"tier,omitempty"`
	OwnerID             *string `json:"ownerId,omitempty"`
	TechnicalOwnerID    *string `json:"technicalOwnerId,omitempty"`
	AgentEnabled        *bool   `json:"agentEnabled,omitempty"`
	KbReferencesEnabled *bool   `json:"kbReferencesEnabled,omitempty"`
	// ServiceNow-only.
	Classification  *string         `json:"classification,omitempty"`
	Pod             *string         `json:"pod,omitempty"`
	SupportTier     *SupportTierRef `json:"supportTier,omitempty"`
	ArrToday        *string         `json:"arrToday,omitempty"`
	Owner           *EntityRef      `json:"owner,omitempty"`
	TechnicalOwner  *EntityRef      `json:"technicalOwner,omitempty"`
	HasAgent        *bool           `json:"hasAgent,omitempty"`
	HasKbReferences *bool           `json:"hasKbReferences,omitempty"`
	CreatedBy       *string         `json:"createdBy,omitempty"`
	// Shared (identical key/type on both data sources).
	ActivationDate   *string `json:"activationDate,omitempty"`
	DeactivationDate *string `json:"deactivationDate,omitempty"`
	CreatedOn        *string `json:"createdOn,omitempty"`
	UpdatedOn        *string `json:"updatedOn,omitempty"`
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
	Types           []string `json:"types,omitempty"`
	SearchQuery     string   `json:"searchQuery,omitempty"`
	ProjectIDs      []string `json:"projectIds,omitempty"`
	DeploymentIDs   []string `json:"deploymentIds,omitempty"`
	States          []string `json:"states,omitempty"`
	Severities      []string `json:"severities,omitempty"`
	IssueTypes      []string `json:"issueTypes,omitempty"`
	EngagementTypes []string `json:"engagementTypes,omitempty"`
	CreatedBy       []string `json:"createdBy,omitempty"`
	CreatedByMe     bool     `json:"createdByMe,omitempty"`
	WorkStates      []string `json:"workStates,omitempty"`
	AssignedUserIDs []string `json:"assignedUserIds,omitempty"`
	ProductNames    []string `json:"productNames,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	ParentID        *string  `json:"parentId,omitempty"`
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

// Variable is a key-value pair used in service-request case creation.
type Variable struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// CaseAttachment is a file attachment for security-report-analysis case
// creation. File must be a base64 data URI (e.g. "data:application/pdf;base64,...").
type CaseAttachment struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// CreateCaseRequest is the input for POST /cases. CreatedBy is never
// serialized (json:"-") — entity-service derives the creator from its own
// auth context, not the request body.
type CreateCaseRequest struct {
	CreatedBy         string           `json:"-"`
	Type              string           `json:"type"`
	ProjectID         string           `json:"projectId"`
	DeploymentID      string           `json:"deploymentId"`
	DeployedProductID string           `json:"deployedProductId,omitempty"`
	Subject           string           `json:"subject"`
	Description       string           `json:"description"`
	Severity          string           `json:"severity"`
	IssueType         string           `json:"issueType"`
	CatalogID         string           `json:"catalogId,omitempty"`
	CatalogItemID     string           `json:"catalogItemId,omitempty"`
	Variables         []Variable       `json:"variables,omitempty"`
	RelatedCaseID     string           `json:"relatedCaseId,omitempty"`
	ConversationID    string           `json:"conversationId,omitempty"`
	WatchList         []string         `json:"watchList,omitempty"`
	Attachments       []CaseAttachment `json:"attachments,omitempty"`
}

// CreateCaseDetails carries the key fields of a newly created case.
type CreateCaseDetails struct {
	ID         string    `json:"id"`
	InternalID string    `json:"internalId"`
	Number     string    `json:"number"`
	CreatedBy  string    `json:"createdBy"`
	CreatedOn  time.Time `json:"createdOn"`
	State      string    `json:"state"`
}

// CreateCaseResponse is entity-service's response for POST /cases.
type CreateCaseResponse struct {
	Message string            `json:"message"`
	Case    CreateCaseDetails `json:"case"`
}

// UpdateCaseRequest is the full field set entity-service accepts for
// PATCH /cases/{id}. This is entity-service's raw contract — the portal only
// exposes a customer-safe subset of these fields; see dto.UpdateCaseRequest
// and dto.BuildEntityUpdateCaseRequest for which ones and why.
type UpdateCaseRequest struct {
	ID                 string     `json:"-"`
	State              *string    `json:"state,omitempty"`
	Severity           *string    `json:"severity,omitempty"`
	WorkState          *string    `json:"workState,omitempty"`
	WatchList          []string   `json:"watchList,omitempty"`
	AssigneeEmail      *string    `json:"assigneeEmail,omitempty"`
	ResolutionCode     *string    `json:"resolutionCode,omitempty"`
	Cause              *string    `json:"cause,omitempty"`
	CloseNotes         *string    `json:"closeNotes,omitempty"`
	ParentID           *string    `json:"parentId,omitempty"`
	RelatedCaseID      *string    `json:"relatedCaseId,omitempty"`
	AutocloseHoldUntil *time.Time `json:"autocloseHoldUntil,omitempty"`
	Subject            *string    `json:"subject,omitempty"`
	Description        *string    `json:"description,omitempty"`
	DeploymentID       *string    `json:"deploymentId,omitempty"`
	DeployedProductID  *string    `json:"deployedProductId,omitempty"`
	FixEta             *time.Time `json:"fixEta,omitempty"`
	BestCaseFixEta     *time.Time `json:"bestCaseFixEta,omitempty"`
	MostLikelyFixEta   *time.Time `json:"mostLikelyFixEta,omitempty"`
	WorstCaseFixEta    *time.Time `json:"worstCaseFixEta,omitempty"`
}

// WatchListUser is a user watching a case (ServiceNow data source only).
type WatchListUser struct {
	ID       string `json:"id"`
	UserName string `json:"userName"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
}

// UpdatedCase carries the case fields entity-service returns after a
// successful PATCH /cases/{id}.
type UpdatedCase struct {
	ID             string               `json:"id"`
	UpdatedOn      time.Time            `json:"updatedOn"`
	UpdatedBy      string               `json:"updatedBy,omitempty"`
	State          string               `json:"state,omitempty"`
	Severity       string               `json:"severity,omitempty"`
	WorkState      *string              `json:"workState"`
	WatchList      []WatchListUser      `json:"watchList,omitempty"`
	AssignedTo     *AssignedEngineerRef `json:"assignedTo,omitempty"`
	ResolutionCode *string              `json:"resolutionCode,omitempty"`
	Cause          *string              `json:"cause,omitempty"`
	CloseNotes     *string              `json:"closeNotes,omitempty"`
	ResolvedOn     *time.Time           `json:"resolvedOn,omitempty"`
	ParentCase     *CaseNumberRef       `json:"parentCase,omitempty"`
	// FixEta is the customer-facing fix-commitment date; the internal-only
	// Best/MostLikely/WorstCaseFixEta fields are intentionally not decoded
	// here — see CaseView's doc comment above for why.
	FixEta *time.Time `json:"fixEta,omitempty"`
}

// UpdateCaseResponse is entity-service's response for PATCH /cases/{id}.
type UpdateCaseResponse struct {
	Message string      `json:"message"`
	Case    UpdatedCase `json:"case"`
}

// CommentType classifies a case comment. entity-service supports
// "work_note" and "activity" too, but the customer portal only ever creates
// (and should only ever create) plain "comment" entries — see
// dto.BuildEntityCreateCaseCommentRequest.
type CommentType string

const (
	CommentTypeWorkNote CommentType = "work_note"
	CommentTypeComment  CommentType = "comment"
	CommentTypeActivity CommentType = "activity"
)

// CreateCaseCommentRequest is the input for POST /cases/{id}/comments.
// CaseID and CreatedBy are never serialized (json:"-") — CaseID comes from
// the URL path, CreatedBy from entity-service's own auth context.
type CreateCaseCommentRequest struct {
	CaseID    string      `json:"-"`
	CreatedBy string      `json:"-"`
	Type      CommentType `json:"type"`
	Content   string      `json:"content"`
}

// CaseCommentDetail carries the key fields of a newly created comment.
type CaseCommentDetail struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
}

// CreateCaseCommentResponse is entity-service's response for POST /cases/{id}/comments.
type CreateCaseCommentResponse struct {
	Message string            `json:"message"`
	Comment CaseCommentDetail `json:"comment"`
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

// --- deployments ---

// SearchDeploymentsRequest is the input for POST /deployments/search.
type SearchDeploymentsRequest struct {
	Pagination      Pagination `json:"pagination"`
	SearchQuery     string     `json:"searchQuery,omitempty"`
	ProjectIDs      []string   `json:"projectIds,omitempty"`
	DeploymentTypes []string   `json:"deploymentTypes,omitempty"`
}

// DeploymentView is a single search result item from POST /deployments/search.
type DeploymentView struct {
	ID          string     `json:"id"`
	Number      string     `json:"number"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Description *string    `json:"description"`
	CreatedBy   *EntityRef `json:"createdBy"`
	Project     EntityRef  `json:"project"`
	CreatedOn   time.Time  `json:"createdOn"`
	UpdatedOn   time.Time  `json:"updatedOn"`
}

// SearchDeploymentsResponse is entity-service's response for POST /deployments/search.
type SearchDeploymentsResponse struct {
	Deployments []DeploymentView `json:"deployments"`
	Total       int              `json:"total"`
	Limit       int              `json:"limit"`
	Offset      int              `json:"offset"`
	HasMore     bool             `json:"hasMore"`
}

// CreateDeploymentRequest is the input for POST /deployments.
//
// NOTE: entity-service only supports deployment creation on its ServiceNow
// data source — a Postgres-mode deployment always returns 400 for this
// route (see cs-tools/entity-service/internal/service/deployment_service.go).
type CreateDeploymentRequest struct {
	ProjectID   string  `json:"projectId"`
	Name        string  `json:"name"`
	Type        *string `json:"type,omitempty"`
	Description string  `json:"description,omitempty"`
}

// CreatedDeployment carries the key fields of a newly created deployment.
type CreatedDeployment struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
}

// CreateDeploymentResponse is entity-service's response for POST /deployments.
type CreateDeploymentResponse struct {
	Message    string            `json:"message"`
	Deployment CreatedDeployment `json:"deployment"`
}

// --- deployed products ---

// CreateDeployedProductRequest is the input for POST /deployed-products.
//
// NOTE: entity-service only supports deployed-product creation on its
// ServiceNow data source — a Postgres-mode deployment always returns 400
// for this route (see cs-tools/entity-service/internal/service/deployed_product_service.go).
type CreateDeployedProductRequest struct {
	ProjectID    string   `json:"projectId"`
	DeploymentID string   `json:"deploymentId"`
	ProductID    string   `json:"productId"`
	VersionID    string   `json:"versionId"`
	Cores        *int     `json:"cores,omitempty"`
	TPS          *float64 `json:"tps,omitempty"`
	Description  *string  `json:"description,omitempty"`
}

// CreatedDeployedProduct carries the key fields of a newly created deployed product.
type CreatedDeployedProduct struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
}

// CreateDeployedProductResponse is entity-service's response for POST /deployed-products.
type CreateDeployedProductResponse struct {
	Message         string                 `json:"message"`
	DeployedProduct CreatedDeployedProduct `json:"deployedProduct"`
}

// SearchDeployedProductsRequest is the input for POST /deployed-products/search.
type SearchDeployedProductsRequest struct {
	Pagination    Pagination `json:"pagination"`
	DeploymentIDs []string   `json:"deploymentIds,omitempty"`
}

// DeployedProductVersionRef is the version sub-object in a DeployedProductView.
type DeployedProductVersionRef struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	ReleasedDate   *time.Time `json:"releasedDate"`
	SupportEoLDate *time.Time `json:"supportEoLDate"`
}

// DeployedProductView is a single search result item from POST /deployed-products/search.
// Cores, TPS, and Category are ServiceNow-only fields, always nil on the
// Postgres data source.
type DeployedProductView struct {
	ID         string                     `json:"id"`
	Deployment EntityRef                  `json:"deployment"`
	Product    EntityRef                  `json:"product"`
	Version    *DeployedProductVersionRef `json:"version"`
	Cores      *string                    `json:"cores"`
	TPS        *string                    `json:"tps"`
	Category   *string                    `json:"category"`
	CreatedOn  time.Time                  `json:"createdOn"`
	UpdatedOn  time.Time                  `json:"updatedOn"`
}

// SearchDeployedProductsResponse is entity-service's response for POST /deployed-products/search.
type SearchDeployedProductsResponse struct {
	DeployedProducts []DeployedProductView `json:"deployedProducts"`
	Total            int                   `json:"total"`
	Limit            int                   `json:"limit"`
	Offset           int                   `json:"offset"`
	HasMore          bool                  `json:"hasMore"`
}

// UpdateDeployedProductRequest is the input for PATCH /deployed-products/{id}.
// Either detail fields (Cores, TPS, Description) or Active=false must be
// provided, but not both. Description uses json.RawMessage to preserve three
// states: absent = omit, "null" = clear, `"value"` = set — decoding a client's
// request body directly into this field naturally preserves that semantic.
//
// NOTE: entity-service only supports this route on its ServiceNow data
// source — see CreateDeployedProductRequest's doc comment.
type UpdateDeployedProductRequest struct {
	ID           string          `json:"-"`
	DeploymentID *string         `json:"deploymentId,omitempty"`
	Cores        *int            `json:"cores,omitempty"`
	TPS          *float64        `json:"tps,omitempty"`
	Description  json.RawMessage `json:"description,omitempty"`
	Active       *bool           `json:"active,omitempty"`
}

// UpdatedDeployedProduct carries the fields that may change after an update.
type UpdatedDeployedProduct struct {
	ID        string    `json:"id"`
	UpdatedOn time.Time `json:"updatedOn"`
	UpdatedBy string    `json:"updatedBy"`
}

// UpdateDeployedProductResponse is entity-service's response for PATCH /deployed-products/{id}.
type UpdateDeployedProductResponse struct {
	Message         string                 `json:"message"`
	DeployedProduct UpdatedDeployedProduct `json:"deployedProduct"`
}

// --- attachments ---

// ReferenceType identifies which kind of entity an attachment or comment is
// attached to.
type ReferenceType string

const (
	ReferenceTypeCase          ReferenceType = "case"
	ReferenceTypeConversation  ReferenceType = "conversation"
	ReferenceTypeChangeRequest ReferenceType = "change_request"
	ReferenceTypeDeployment    ReferenceType = "deployment"
	ReferenceTypeIncident      ReferenceType = "incident"
)

// CreateAttachmentRequest is the input for POST /attachments.
type CreateAttachmentRequest struct {
	ReferenceID   string        `json:"referenceId"`
	ReferenceType ReferenceType `json:"referenceType"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	File          string        `json:"file"`
	Description   *string       `json:"description,omitempty"`
}

// AttachmentDetail holds the core fields returned after creating an attachment.
type AttachmentDetail struct {
	ID          string    `json:"id"`
	SizeBytes   int       `json:"sizeBytes"`
	CreatedOn   time.Time `json:"createdOn"`
	CreatedBy   string    `json:"createdBy"`
	DownloadURL string    `json:"downloadUrl"`
}

// CreateAttachmentResponse is entity-service's response for POST /attachments.
type CreateAttachmentResponse struct {
	Message    string           `json:"message"`
	Attachment AttachmentDetail `json:"attachment"`
}

// SearchAttachmentsRequest is the input for POST /attachments/search.
type SearchAttachmentsRequest struct {
	ReferenceID   string        `json:"referenceId"`
	ReferenceType ReferenceType `json:"referenceType"`
	Pagination    Pagination    `json:"pagination"`
}

// Attachment is a single search result item from POST /attachments/search.
type Attachment struct {
	ID            string        `json:"id"`
	ReferenceID   string        `json:"referenceId"`
	ReferenceType ReferenceType `json:"referenceType"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	SizeBytes     int           `json:"sizeBytes"`
	Description   *string       `json:"description"`
	CreatedBy     string        `json:"createdBy"`
	CreatedOn     time.Time     `json:"createdOn"`
	DownloadURL   *string       `json:"downloadUrl"`
	PreviewURL    *string       `json:"previewUrl"`
}

// SearchAttachmentsResponse is entity-service's response for POST /attachments/search.
type SearchAttachmentsResponse struct {
	Attachments []Attachment `json:"attachments"`
	Total       int          `json:"total"`
	Limit       int          `json:"limit"`
	Offset      int          `json:"offset"`
	HasMore     bool         `json:"hasMore"`
}

// DeleteAttachmentResponse is entity-service's response for DELETE /attachments/{id}.
type DeleteAttachmentResponse struct {
	Message string `json:"message"`
}

// --- case activities ---

// ActivityType discriminates the kind of entry in a case's activity feed.
type ActivityType string

const (
	ActivityTypeComment     ActivityType = "comment"
	ActivityTypeAttachment  ActivityType = "attachment"
	ActivityTypeFieldChange ActivityType = "field_change"
)

// FieldChange describes a single field's value change, present only on
// CaseActivity entries with Type == ActivityTypeFieldChange.
type FieldChange struct {
	Field         string `json:"field"`
	FieldLabel    string `json:"fieldLabel"`
	PreviousValue string `json:"previousValue"`
	NewValue      string `json:"newValue"`
}

// CaseActivity is a single entry in a case's activity feed — a discriminated
// union on Type. The shared fields are always present; type-specific fields
// are populated only for the matching Type (CommentType for comments;
// FileName/ContentType/SizeBytes/DownloadURL for attachments; Changes for
// field changes). Field types/omitempty here mirror entity-service's struct
// exactly (see its doc comment) — do not change to pointers.
type CaseActivity struct {
	ID                 string        `json:"id"`
	Type               ActivityType  `json:"type"`
	Content            string        `json:"content"`
	CreatedOn          time.Time     `json:"createdOn"`
	CreatedBy          string        `json:"createdBy"`
	CreatedByFirstName string        `json:"createdByFirstName"`
	CreatedByLastName  string        `json:"createdByLastName"`
	CreatedByFullName  string        `json:"createdByFullName"`
	CommentType        *CommentType  `json:"commentType,omitempty"`
	FileName           string        `json:"fileName,omitempty"`
	ContentType        string        `json:"contentType,omitempty"`
	SizeBytes          int           `json:"sizeBytes,omitempty"`
	DownloadURL        string        `json:"downloadUrl,omitempty"`
	Changes            []FieldChange `json:"changes,omitempty"`
}

// SearchCaseActivitiesRequest is the input for POST /cases/{id}/activities/search.
// CaseID is populated from the URL path parameter and is not part of the JSON body.
type SearchCaseActivitiesRequest struct {
	CaseID              string     `json:"-"`
	Pagination          Pagination `json:"pagination"`
	IncludeFieldChanges *bool      `json:"includeFieldChanges,omitempty"`
}

// SearchCaseActivitiesResponse is entity-service's response for POST /cases/{id}/activities/search.
type SearchCaseActivitiesResponse struct {
	Activity []CaseActivity `json:"activity"`
	Total    int            `json:"total"`
	Limit    int            `json:"limit"`
	Offset   int            `json:"offset"`
	HasMore  bool           `json:"hasMore"`
}

// --- products ---
//
// Like accounts, entity-service returns a different wire shape for these
// routes depending on its DATA_SOURCE. The structs below are a superset of
// both shapes for the same reasons documented on AccountSummary/AccountDetail
// above: no colliding JSON keys, and every ambiguous-typed field (class,
// dates) is typed as *string so either shape decodes cleanly.

// ProductView is a single search result item from POST /products/search.
type ProductView struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Class     *string `json:"class,omitempty"`
	CreatedOn *string `json:"createdOn,omitempty"`
	UpdatedOn *string `json:"updatedOn,omitempty"`
}

// SearchProductsRequest is the input for POST /products/search.
type SearchProductsRequest struct {
	Pagination  Pagination `json:"pagination"`
	SearchQuery string     `json:"searchQuery,omitempty"`
}

// SearchProductsResponse is entity-service's response for POST /products/search.
type SearchProductsResponse struct {
	Products []ProductView `json:"products"`
	Total    int           `json:"total"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
	HasMore  bool          `json:"hasMore"`
}

// SearchProductVersionsRequest is the input for POST /products/{id}/versions/search.
// ProductID is populated from the URL path parameter and is not part of the JSON body.
type SearchProductVersionsRequest struct {
	Pagination  Pagination `json:"pagination"`
	ProductID   string     `json:"-"`
	SearchQuery string     `json:"searchQuery,omitempty"`
}

// ProductVersionView is a single search result item from
// POST /products/{id}/versions/search.
type ProductVersionView struct {
	ID                             string  `json:"id"`
	ProductID                      string  `json:"productId"`
	Version                        string  `json:"version"`
	CurrentSupportStatus           *string `json:"currentSupportStatus,omitempty"`
	ReleaseDate                    *string `json:"releaseDate,omitempty"`
	SupportEOLDate                 *string `json:"supportEolDate,omitempty"`
	EarliestPossibleSupportEOLDate *string `json:"earliestPossibleSupportEolDate,omitempty"`
	CreatedOn                      *string `json:"createdOn,omitempty"`
	UpdatedOn                      *string `json:"updatedOn,omitempty"`
}

// SearchProductVersionsResponse is entity-service's response for POST /products/{id}/versions/search.
type SearchProductVersionsResponse struct {
	ProductVersions []ProductVersionView `json:"productVersions"`
	Total           int                  `json:"total"`
	Limit           int                  `json:"limit"`
	Offset          int                  `json:"offset"`
	HasMore         bool                 `json:"hasMore"`
}

// --- change requests ---
//
// entity-service only supports change requests on its ServiceNow data
// source — DATA_SOURCE=postgres deployments don't register these routes at
// all (404, not a data-shape difference like accounts/products).

// CreateChangeRequestRequest is the input for POST /change-requests. Subject
// is the only required field. entity-service's category/priority/impact/
// type/state/risk fields are plain strings here (not named Go enum types),
// matching this file's convention elsewhere (e.g. CaseView.Severity).
type CreateChangeRequestRequest struct {
	Subject             string  `json:"subject"`
	Category            *string `json:"category,omitempty"`
	ServiceID           *string `json:"serviceId,omitempty"`
	ServiceOfferingID   *string `json:"serviceOfferingId,omitempty"`
	ConfigurationItemID *string `json:"configurationItemId,omitempty"`
	Priority            *string `json:"priority,omitempty"`
	Impact              *string `json:"impact,omitempty"`
	Type                *string `json:"type,omitempty"`
	State               *string `json:"state,omitempty"`
	GroupID             *string `json:"groupId,omitempty"`
	AssignedEngineerID  *string `json:"assignedEngineerId,omitempty"`
	Risk                *string `json:"risk,omitempty"`
	RequestedByID       *string `json:"requestedById,omitempty"`
	Description         *string `json:"description,omitempty"`
	Justification       *string `json:"justification,omitempty"`
	ImplementationPlan  *string `json:"implementationPlan,omitempty"`
	RiskImpactAnalysis  *string `json:"riskImpactAnalysis,omitempty"`
	BackoutPlan         *string `json:"backoutPlan,omitempty"`
	TestPlan            *string `json:"testPlan,omitempty"`
	PlannedStartDate    *string `json:"plannedStartDate,omitempty"`
	PlannedEndDate      *string `json:"plannedEndDate,omitempty"`
	Comment             *string `json:"comment,omitempty"`
	WorkNote            *string `json:"workNote,omitempty"`
}

// ChangeRequestCreated carries the key fields of a newly created change request.
type ChangeRequestCreated struct {
	ID        string `json:"id"`
	Number    string `json:"number"`
	CreatedOn string `json:"createdOn"`
	CreatedBy string `json:"createdBy"`
}

// CreateChangeRequestResponse is entity-service's response for POST /change-requests.
type CreateChangeRequestResponse struct {
	Message       string               `json:"message"`
	ChangeRequest ChangeRequestCreated `json:"changeRequest"`
}

// ChangeRequestSort specifies the sort field and direction for change request search results.
type ChangeRequestSort struct {
	Field string `json:"field,omitempty"`
	Order string `json:"order,omitempty"`
}

// SearchChangeRequestsFilters holds the optional filter criteria for a change request search.
type SearchChangeRequestsFilters struct {
	ProjectIDs      []string   `json:"projectIds,omitempty"`
	SearchQuery     string     `json:"searchQuery,omitempty"`
	States          []string   `json:"states,omitempty"`
	Impacts         []string   `json:"impacts,omitempty"`
	ClosedStartDate *time.Time `json:"closedStartDate,omitempty"`
	ClosedEndDate   *time.Time `json:"closedEndDate,omitempty"`
}

// SearchChangeRequestsRequest is the input for POST /change-requests/search.
type SearchChangeRequestsRequest struct {
	Filters    SearchChangeRequestsFilters `json:"filters"`
	SortBy     ChangeRequestSort           `json:"sortBy"`
	Pagination Pagination                  `json:"pagination"`
}

// SearchChangeRequestView is a single search result item from POST /change-requests/search.
type SearchChangeRequestView struct {
	ID               string     `json:"id"`
	Number           string     `json:"number"`
	Subject          *string    `json:"subject"`
	Description      *string    `json:"description"`
	Project          EntityRef  `json:"project"`
	Case             *EntityRef `json:"case"`
	Deployment       *EntityRef `json:"deployment"`
	DeployedProduct  *EntityRef `json:"deployedProduct"`
	Product          *EntityRef `json:"product"`
	AssignedEngineer *EntityRef `json:"assignedEngineer"`
	AssignedTeam     *EntityRef `json:"assignedTeam"`
	PlannedStartOn   *string    `json:"plannedStartOn"`
	PlannedEndOn     *string    `json:"plannedEndOn"`
	Duration         *string    `json:"duration"`
	Impact           *string    `json:"impact"`
	State            *string    `json:"state"`
	Type             *string    `json:"type"`
	CreatedOn        string     `json:"createdOn"`
	UpdatedOn        string     `json:"updatedOn"`
}

// SearchChangeRequestsResponse is entity-service's response for POST /change-requests/search.
type SearchChangeRequestsResponse struct {
	ChangeRequests []SearchChangeRequestView `json:"changeRequests"`
	Total          int                       `json:"total"`
	Offset         int                       `json:"offset"`
	Limit          int                       `json:"limit"`
}

// ChangeRequest is entity-service's response for GET /change-requests/{id}.
// Embeds SearchChangeRequestView (matching entity-service's own struct
// embedding, which flattens its fields into the same JSON object).
type ChangeRequest struct {
	SearchChangeRequestView
	CreatedBy           string     `json:"createdBy"`
	Justification       *string    `json:"justification"`
	ImpactDescription   *string    `json:"impactDescription"`
	ServiceOutage       *string    `json:"serviceOutage"`
	CommunicationPlan   *string    `json:"communicationPlan"`
	RollbackPlan        *string    `json:"rollbackPlan"`
	TestPlan            *string    `json:"testPlan"`
	HasCustomerApproved bool       `json:"hasCustomerApproved"`
	HasCustomerReviewed bool       `json:"hasCustomerReviewed"`
	ApprovedBy          *EntityRef `json:"approvedBy"`
	ApprovedOn          *string    `json:"approvedOn"`
	LegalNextStates     []string   `json:"legalNextStates"`
}

// PatchChangeRequestRequest is the full field set entity-service accepts for
// PATCH /change-requests/{id}. This is entity-service's raw contract — the
// portal only exposes a customer-safe subset; see
// dto.ChangeRequestUpdateRequest for which ones and why.
type PatchChangeRequestRequest struct {
	Title              *string `json:"title,omitempty"`
	Description        *string `json:"description,omitempty"`
	ProjectID          *string `json:"projectId,omitempty"`
	CaseID             *string `json:"caseId,omitempty"`
	DeploymentID       *string `json:"deploymentId,omitempty"`
	DeployedProductID  *string `json:"deployedProductId,omitempty"`
	AssignedEngineerID *string `json:"assignedEngineerId,omitempty"`
	AssignedTeamID     *string `json:"assignedTeamId,omitempty"`
	PlannedStartOn     *string `json:"plannedStartOn,omitempty"`
	PlannedEndOn       *string `json:"plannedEndOn,omitempty"`
	Impact             *string `json:"impact,omitempty"`
	State              *string `json:"state,omitempty"`
	Type               *string `json:"type,omitempty"`
	Justification      *string `json:"justification,omitempty"`
	ImpactDescription  *string `json:"impactDescription,omitempty"`
	ServiceOutage      *string `json:"serviceOutage,omitempty"`
	CommunicationPlan  *string `json:"communicationPlan,omitempty"`
	RollbackPlan       *string `json:"rollbackPlan,omitempty"`
	TestPlan           *string `json:"testPlan,omitempty"`
	IsCustomerApproved *bool   `json:"isCustomerApproved,omitempty"`
	IsCustomerReviewed *bool   `json:"isCustomerReviewed,omitempty"`
	RequestApproval    *bool   `json:"requestApproval,omitempty"`
}

// PatchChangeRequestResponse is entity-service's response for PATCH /change-requests/{id}.
type PatchChangeRequestResponse struct {
	Message       string        `json:"message"`
	ChangeRequest ChangeRequest `json:"changeRequest"`
}

// ChangeRequestApprover is a single approver's response within an approval
// stage. Status is deliberately a plain string, not a closed enum —
// entity-service documents it as an open set (APPROVED, NOT_REQUIRED,
// REQUESTED, REJECTED, CANCELLED, NO_CONSENSUS, or an unrecognized
// uppercased value).
type ChangeRequestApprover struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	RespondedOn *string `json:"respondedOn"`
}

// ChangeRequestApproval represents a single approval stage (e.g. Assess,
// Authorize, Customer Approval) on a change request.
type ChangeRequestApproval struct {
	Stage        string                  `json:"stage"`
	ApproverType string                  `json:"approverType"`
	ApproverName string                  `json:"approverName"`
	Status       string                  `json:"status"`
	Approvers    []ChangeRequestApprover `json:"approvers"`
}

// ChangeRequestApprovals is entity-service's response for GET /change-requests/{id}/approvals.
type ChangeRequestApprovals struct {
	Approvals []ChangeRequestApproval `json:"approvals"`
}

// ChangeRequestApprovalDecisionRequest is the input for
// POST /change-requests/{id}/approvals/decision. Decision is a plain string
// ("approved" or "rejected" per entity-service's doc comment), not an enum type.
type ChangeRequestApprovalDecisionRequest struct {
	Decision string `json:"decision"`
}

// ChangeRequestApprovalDecisionResponse is entity-service's response for
// POST /change-requests/{id}/approvals/decision.
type ChangeRequestApprovalDecisionResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// --- call requests ---
//
// entity-service only supports call requests on its ServiceNow data source.

// CreateCallRequestRequest is the input for POST /call-requests.
type CreateCallRequestRequest struct {
	CaseID          string   `json:"caseId"`
	Reason          string   `json:"reason"`
	UTCTimes        []string `json:"utcTimes"`
	DurationMinutes int      `json:"durationInMinutes"`
}

// CallRequestCreated carries the key fields of a newly created call request.
type CallRequestCreated struct {
	ID        string `json:"id"`
	CreatedOn string `json:"createdOn"`
	CreatedBy string `json:"createdBy"`
	State     string `json:"state"`
}

// CreateCallRequestResponse is entity-service's response for POST /call-requests.
type CreateCallRequestResponse struct {
	Message     string             `json:"message"`
	CallRequest CallRequestCreated `json:"callRequest"`
}

// CallRequestState holds the state of a call request: ID is the string state
// enum key, Label is the human-readable display label.
type CallRequestState struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// CallRequestCaseRef is a reference to a case embedded in a call request.
type CallRequestCaseRef struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Number *string `json:"number,omitempty"`
}

// SearchCallRequestsFilters holds the optional filter criteria for a call request search.
type SearchCallRequestsFilters struct {
	States []string `json:"states,omitempty"`
}

// SearchCallRequestsRequest is the input for POST /call-requests/search.
type SearchCallRequestsRequest struct {
	CaseID     string                     `json:"caseId"`
	Filters    *SearchCallRequestsFilters `json:"filters,omitempty"`
	Pagination Pagination                 `json:"pagination"`
}

// CallRequestView is a single search result item from POST /call-requests/search.
// Assignee/Notes/Plan/Attendees/ActionItems/ActualDurationMin are agent-side
// fields, populated once a support engineer schedules or concludes the call.
type CallRequestView struct {
	ID                 string             `json:"id"`
	Number             string             `json:"number"`
	Case               CallRequestCaseRef `json:"case"`
	Reason             *string            `json:"reason"`
	PreferredTimes     []string           `json:"preferredTimes"`
	DurationMin        int                `json:"durationMin"`
	ScheduleTime       *string            `json:"scheduleTime"`
	MeetingLink        *string            `json:"meetingLink"`
	CreatedOn          string             `json:"createdOn"`
	UpdatedOn          string             `json:"updatedOn"`
	State              CallRequestState   `json:"state"`
	CancellationReason *string            `json:"cancellationReason,omitempty"`
	Assignee           *string            `json:"assignee,omitempty"`
	Notes              *string            `json:"notes,omitempty"`
	Plan               *string            `json:"plan,omitempty"`
	Attendees          *string            `json:"attendees,omitempty"`
	ActionItems        *string            `json:"actionItems,omitempty"`
	ActualDurationMin  *int               `json:"actualDurationMin,omitempty"`
}

// SearchCallRequestsResponse is entity-service's response for POST /call-requests/search.
type SearchCallRequestsResponse struct {
	CallRequests []CallRequestView `json:"callRequests"`
	Total        int               `json:"total"`
	Offset       int               `json:"offset"`
	Limit        int               `json:"limit"`
}

// UpdateCallRequestRequest is the full field set entity-service accepts for
// PATCH /call-requests/{id}. This is entity-service's raw contract — the
// portal only exposes a customer-safe subset; see
// dto.CallRequestUpdateRequest for which ones and why (the agent-side
// fields below must never be customer-settable).
type UpdateCallRequestRequest struct {
	ID                 string   `json:"-"`
	CaseID             string   `json:"caseId,omitempty"`
	State              string   `json:"state"`
	CancellationReason *string  `json:"cancellationReason,omitempty"`
	UTCTimes           []string `json:"utcTimes,omitempty"`
	DurationMinutes    *int     `json:"durationInMinutes,omitempty"`
	// Agent-side fields, set when an engineer schedules or concludes the call.
	MeetingDate       *string `json:"meetingDate,omitempty"`
	Assignee          *string `json:"assignee,omitempty"`
	Notes             *string `json:"notes,omitempty"`
	Plan              *string `json:"plan,omitempty"`
	Attendees         *string `json:"attendees,omitempty"`
	ActionItems       *string `json:"actionItems,omitempty"`
	ActualDurationMin *int    `json:"actualDurationMin,omitempty"`
}

// CallRequestUpdated carries the fields that may change after an update.
type CallRequestUpdated struct {
	ID        string `json:"id"`
	UpdatedOn string `json:"updatedOn"`
	UpdatedBy string `json:"updatedBy"`
}

// UpdateCallRequestResponse is entity-service's response for PATCH /call-requests/{id}.
type UpdateCallRequestResponse struct {
	Message     string             `json:"message"`
	CallRequest CallRequestUpdated `json:"callRequest"`
}

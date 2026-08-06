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
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// Ref is a compact {id, name} reference to another entity (project,
// deployment, product, assigned team, etc.).
type Ref struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func mapRef(r *entity.EntityRef) *Ref {
	if r == nil {
		return nil
	}
	return &Ref{ID: r.ID, Name: r.Name}
}

// NumberRef is a compact reference to a case carrying its human-readable number.
type NumberRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

func mapNumberRef(r *entity.CaseNumberRef) *NumberRef {
	if r == nil {
		return nil
	}
	return &NumberRef{ID: r.ID, Number: r.Number}
}

// CaseSummary is one item of the portal's response for POST /cases/search.
//
// Deliberately excludes entity-service's InternalID (internal-only
// identifier), Catalog/CatalogItem (CMDB implementation detail), Conversation,
// DeployedProduct, AssignedEngineer, ParentCase/RelatedCase — kept minimal for
// the list view; the full picture is available via GET /cases/{id}.
type CaseSummary struct {
	ID             string  `json:"id"`
	Number         string  `json:"number"`
	Subject        *string `json:"subject,omitempty"`
	Description    *string `json:"description,omitempty"`
	State          string  `json:"state"`
	Severity       *string `json:"severity,omitempty"`
	IssueType      *string `json:"issueType,omitempty"`
	EngagementType *string `json:"engagementType,omitempty"`
	WorkState      *string `json:"workState,omitempty"`
	Type           string  `json:"type"`
	CreatedOn      string  `json:"createdOn"`
	Project        Ref     `json:"project"`
	Deployment     *Ref    `json:"deployment,omitempty"`
	Product        *Ref    `json:"product,omitempty"`
	AssignedTeam   *Ref    `json:"assignedTeam,omitempty"`
}

// SearchCasesResponse is the portal's response for POST /cases/search.
type SearchCasesResponse struct {
	Cases  []CaseSummary `json:"cases"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

// MapSearchCases builds the portal response from entity-service's SearchCasesResponse.
func MapSearchCases(r entity.SearchCasesResponse) SearchCasesResponse {
	cases := make([]CaseSummary, 0, len(r.Cases))
	for _, c := range r.Cases {
		cases = append(cases, CaseSummary{
			ID:             c.ID,
			Number:         c.Number,
			Subject:        c.Subject,
			Description:    c.Description,
			State:          c.State,
			Severity:       c.Severity,
			IssueType:      c.IssueType,
			EngagementType: c.EngagementType,
			WorkState:      c.WorkState,
			Type:           c.Type,
			CreatedOn:      c.CreatedOn,
			Project:        Ref{ID: c.Project.ID, Name: c.Project.Name},
			Deployment:     mapRef(c.Deployment),
			Product:        mapRef(c.Product),
			AssignedTeam:   mapRef(c.AssignedTeam),
		})
	}
	return SearchCasesResponse{
		Cases:  cases,
		Total:  r.Total,
		Limit:  r.Limit,
		Offset: r.Offset,
	}
}

// currentUserFilterPlaceholder must match entity-service's own
// currentUserFilterPlaceholder (case_filters.go) exactly — it's the literal
// values entry a createdBy+eq filter must carry to mean "the authenticated
// caller".
const currentUserFilterPlaceholder = "__current_user_email__"

// CaseSearchFilters holds the optional filter criteria for POST /cases/search.
// This is the portal's own request contract, unchanged from before
// entity-service redesigned its case-search wire format into a generic
// filter-predicate array (matching what the frontend has always sent) —
// only the fields
// the portal currently exposes are included; extend as needed when more
// filters are surfaced to the frontend. See BuildEntitySearchCasesRequest
// for the translation into entity-service's current contract.
type CaseSearchFilters struct {
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

// CaseSearchRequest is the portal's request body for POST /cases/search.
type CaseSearchRequest struct {
	Filters    CaseSearchFilters `json:"filters"`
	SortBy     entity.CaseSort   `json:"sortBy"`
	Pagination entity.Pagination `json:"pagination"`
}

// BuildEntitySearchCasesRequest translates the portal's named-filter-field
// request into entity-service's current generic filter-predicate array
// contract (see entity.CaseFieldFilter) — every filter the portal exposes
// becomes one "field in/eq values" entry; SearchQuery, SortBy, and
// Pagination pass straight through unchanged. CreatedByMe becomes a
// createdBy+eq filter carrying currentUserFilterPlaceholder, exactly as
// entity-service's own case_filters.go expects, resolving the caller's
// identity server-side from the forwarded x-user-id-token. CreatedByMe takes
// precedence over CreatedBy when both are set: entity-service's filters
// array is AND-only, so a createdBy+in filter and a createdBy+eq filter
// together could never both match (barring CreatedBy coincidentally
// containing the caller's own email), silently returning an empty result
// set — CreatedBy is dropped entirely rather than combined.
func BuildEntitySearchCasesRequest(req CaseSearchRequest) entity.SearchCasesRequest {
	var filters []entity.CaseFieldFilter

	addIn := func(field string, values []string) {
		if len(values) > 0 {
			filters = append(filters, entity.CaseFieldFilter{Field: field, Op: "in", Values: values})
		}
	}

	addIn("type", req.Filters.Types)
	addIn("projectId", req.Filters.ProjectIDs)
	addIn("deploymentId", req.Filters.DeploymentIDs)
	addIn("state", req.Filters.States)
	addIn("severity", req.Filters.Severities)
	addIn("issueType", req.Filters.IssueTypes)
	addIn("engagementType", req.Filters.EngagementTypes)
	addIn("workState", req.Filters.WorkStates)
	addIn("assignedUserId", req.Filters.AssignedUserIDs)
	addIn("product", req.Filters.ProductNames)
	addIn("tag", req.Filters.Tags)

	if req.Filters.ParentID != nil {
		filters = append(filters, entity.CaseFieldFilter{Field: "parentId", Op: "eq", Values: []string{*req.Filters.ParentID}})
	}
	// CreatedByMe takes precedence over CreatedBy: entity-service's filters
	// array is AND-only (case_filters.go), so a createdBy+in filter and a
	// createdBy+eq filter together could never both match (barring CreatedBy
	// coincidentally containing the caller's own email) — silently returning
	// an empty result set instead of the caller's own cases. A client
	// shouldn't send both, but if it does, honor the explicit "my cases"
	// intent rather than the list.
	if req.Filters.CreatedByMe {
		filters = append(filters, entity.CaseFieldFilter{Field: "createdBy", Op: "eq", Values: []string{currentUserFilterPlaceholder}})
	} else {
		addIn("createdBy", req.Filters.CreatedBy)
	}

	return entity.SearchCasesRequest{
		Filters: entity.SearchCasesFilters{
			SearchQuery: req.Filters.SearchQuery,
			Filters:     filters,
		},
		SortBy:     req.SortBy,
		Pagination: req.Pagination,
	}
}

// PersonRef is a compact reference to a person (name + email), used for
// case creators and assigned engineers. Internal identifiers (entity-service's
// UserRef.ID/UserID, AssignedEngineerRef.ID) are intentionally dropped.
type PersonRef struct {
	Name  string  `json:"name"`
	Email *string `json:"email,omitempty"`
}

// LinkedServiceRequest is a compact reference to a service-request case
// linked to another case as its parent.
type LinkedServiceRequest struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Name   string `json:"name"`
}

// CaseTag is a free-text label attached to a case.
type CaseTag struct {
	Label string  `json:"label"`
	Color *string `json:"color,omitempty"`
}

// CaseDetails is the portal's response for GET /cases/{id}.
//
// Deliberately excludes entity-service's InternalID, Catalog/CatalogItem
// (CMDB detail), WatchList, AutoclosureStep/AutoclosureStateTime, and
// BestCaseFixEta/MostLikelyFixEta/WorstCaseFixEta — entity-service documents
// the Fix-ETA trio and WatchList as CSM-engineer-facing only, and autoclosure
// state is internal ServiceNow workflow detail not surfaced to entity-service's
// decoded CaseView in the first place (see internal/entity/types.go).
type CaseDetails struct {
	ID                    string                 `json:"id"`
	Number                string                 `json:"number"`
	Subject               string                 `json:"subject"`
	Description           string                 `json:"description"`
	Severity              string                 `json:"severity"`
	IssueType             string                 `json:"issueType"`
	State                 string                 `json:"state"`
	WorkState             *string                `json:"workState,omitempty"`
	Type                  *string                `json:"type,omitempty"`
	EngagementType        *string                `json:"engagementType,omitempty"`
	CreatedOn             time.Time              `json:"createdOn"`
	UpdatedOn             time.Time              `json:"updatedOn"`
	ClosedOn              *time.Time             `json:"closedOn,omitempty"`
	CreatedBy             PersonRef              `json:"createdBy"`
	Project               Ref                    `json:"project"`
	Deployment            *Ref                   `json:"deployment,omitempty"`
	DeployedProduct       *Ref                   `json:"deployedProduct,omitempty"`
	Product               *Ref                   `json:"product,omitempty"`
	AssignedTeam          *Ref                   `json:"assignedTeam,omitempty"`
	Conversation          *Ref                   `json:"conversation,omitempty"`
	AssignedEngineer      *PersonRef             `json:"assignedEngineer,omitempty"`
	ParentCase            *NumberRef             `json:"parentCase,omitempty"`
	RelatedCase           *NumberRef             `json:"relatedCase,omitempty"`
	LinkedServiceRequests []LinkedServiceRequest `json:"linkedServiceRequests,omitempty"`
	ResolvedOn            *time.Time             `json:"resolvedOn,omitempty"`
	ResolutionCode        *string                `json:"resolutionCode,omitempty"`
	Cause                 *string                `json:"cause,omitempty"`
	ResolutionNotes       *string                `json:"resolutionNotes,omitempty"`
	FixEta                *time.Time             `json:"fixEta,omitempty"`
	Tags                  []CaseTag              `json:"tags,omitempty"`
}

// MapCaseDetails builds the portal response from entity-service's CaseView.
func MapCaseDetails(c entity.CaseView) CaseDetails {
	var deployedProduct *Ref
	if c.DeployedProductDetails != nil {
		deployedProduct = &Ref{ID: c.DeployedProductDetails.ID, Name: c.DeployedProductDetails.DisplayName}
	}

	var assignedEngineer *PersonRef
	if c.AssignedEngineer != nil {
		assignedEngineer = &PersonRef{Name: c.AssignedEngineer.Name, Email: c.AssignedEngineer.Email}
	}

	linked := make([]LinkedServiceRequest, 0, len(c.LinkedServiceRequests))
	for _, lsr := range c.LinkedServiceRequests {
		linked = append(linked, LinkedServiceRequest{ID: lsr.ID, Number: lsr.Number, Name: lsr.Name})
	}

	var tags []CaseTag
	if len(c.Tags) > 0 {
		tags = make([]CaseTag, 0, len(c.Tags))
		for _, t := range c.Tags {
			tags = append(tags, CaseTag{Label: t.Label, Color: t.Color})
		}
	}

	email := c.CreatedByDetails.Email
	return CaseDetails{
		ID:              c.ID,
		Number:          c.Number,
		Subject:         c.Subject,
		Description:     c.Description,
		Severity:        c.Severity,
		IssueType:       c.IssueType,
		State:           c.State,
		WorkState:       c.WorkState,
		Type:            c.Type,
		EngagementType:  c.EngagementType,
		CreatedOn:       c.CreatedOn,
		UpdatedOn:       c.UpdatedOn,
		ClosedOn:        c.ClosedOn,
		CreatedBy:       PersonRef{Name: c.CreatedByDetails.Name, Email: &email},
		Project:         Ref{ID: c.ProjectDetails.ID, Name: c.ProjectDetails.Name},
		Deployment:      mapRef(c.DeploymentDetails),
		DeployedProduct: deployedProduct,
		Product:         mapRef(c.ProductDetails),
		AssignedTeam:    mapRef(c.AssignedTeam),
		Conversation:    mapRef(c.Conversation),

		AssignedEngineer:      assignedEngineer,
		ParentCase:            mapNumberRef(c.ParentCase),
		RelatedCase:           mapNumberRef(c.RelatedCase),
		LinkedServiceRequests: linked,
		ResolvedOn:            c.ResolvedOn,
		ResolutionCode:        c.ResolutionCode,
		Cause:                 c.Cause,
		ResolutionNotes:       c.ResolutionNotes,
		FixEta:                c.FixEta,
		Tags:                  tags,
	}
}

// CaseCreateResponse is the portal's response for POST /cases. Deliberately
// excludes entity-service's InternalID, consistent with the other case DTOs.
type CaseCreateResponse struct {
	ID        string    `json:"id"`
	Number    string    `json:"number"`
	CreatedOn time.Time `json:"createdOn"`
	State     string    `json:"state"`
}

// MapCaseCreate builds the portal response from entity-service's CreateCaseResponse.
func MapCaseCreate(r entity.CreateCaseResponse) CaseCreateResponse {
	return CaseCreateResponse{
		ID:        r.Case.ID,
		Number:    r.Case.Number,
		CreatedOn: r.Case.CreatedOn,
		State:     r.Case.State,
	}
}

// UpdateCaseRequest is the portal's request shape for PATCH /cases/{id} — a
// deliberately restricted subset of entity-service's UpdateCaseRequest.
// Excluded fields are internal WSO2 support operations, not customer
// self-service actions: WorkState (CSM engineer work-in-progress tracking),
// AssigneeEmail (support engineer assignment), ParentID/RelatedCaseID/
// DeploymentID/DeployedProductID (case relinking), AutocloseHoldUntil
// (ServiceNow auto-closure workflow control), and FixEta/BestCaseFixEta/
// MostLikelyFixEta/WorstCaseFixEta (fix-commitment dates set by support
// engineers, not the customer). entity-service requires exactly one of
// State/Severity/WorkState/WatchList/AssigneeEmail/ParentID/RelatedCaseID/
// AutocloseHoldUntil/Subject/Description/DeploymentID/DeployedProductID/
// FixEta/BestCaseFixEta/MostLikelyFixEta/WorstCaseFixEta to be set (see
// entity.UpdateCaseRequest's doc comment) — since this portal DTO only
// exposes State/Severity/Subject/Description/WatchList of that set, exactly
// one of those five must be set here too. ResolutionCode/Cause/CloseNotes
// are secondary fields, only accepted alongside a closing State transition,
// and don't count toward the exactly-one rule.
type UpdateCaseRequest struct {
	State          *string  `json:"state,omitempty"`
	Severity       *string  `json:"severity,omitempty"`
	Subject        *string  `json:"subject,omitempty"`
	Description    *string  `json:"description,omitempty"`
	WatchList      []string `json:"watchList,omitempty"`
	ResolutionCode *string  `json:"resolutionCode,omitempty"`
	Cause          *string  `json:"cause,omitempty"`
	CloseNotes     *string  `json:"closeNotes,omitempty"`
}

// BuildEntityUpdateCaseRequest converts the portal's restricted update
// request into entity-service's full request shape, leaving every excluded
// field zero/nil.
func BuildEntityUpdateCaseRequest(id string, req UpdateCaseRequest) entity.UpdateCaseRequest {
	return entity.UpdateCaseRequest{
		ID:             id,
		State:          req.State,
		Severity:       req.Severity,
		Subject:        req.Subject,
		Description:    req.Description,
		WatchList:      req.WatchList,
		ResolutionCode: req.ResolutionCode,
		Cause:          req.Cause,
		CloseNotes:     req.CloseNotes,
	}
}

// CaseUpdateResponse is the portal's response for PATCH /cases/{id}.
// Deliberately excludes entity-service's UpdatedBy (internal actor identity)
// and the Best/MostLikely/WorstCaseFixEta trio, for the same reasons as
// CaseDetails above. WatchList IS included (unlike CaseView/CaseDetails,
// which omit it) — a customer who just updated the watch list needs
// confirmation of who's on it now, so this is a deliberate exception to the
// read-path exclusion rather than an oversight.
type CaseUpdateResponse struct {
	ID             string     `json:"id"`
	UpdatedOn      time.Time  `json:"updatedOn"`
	State          string     `json:"state,omitempty"`
	Severity       string     `json:"severity,omitempty"`
	WorkState      *string    `json:"workState,omitempty"`
	WatchList      []string   `json:"watchList,omitempty"`
	AssignedTo     *PersonRef `json:"assignedTo,omitempty"`
	ResolutionCode *string    `json:"resolutionCode,omitempty"`
	Cause          *string    `json:"cause,omitempty"`
	CloseNotes     *string    `json:"closeNotes,omitempty"`
	ResolvedOn     *time.Time `json:"resolvedOn,omitempty"`
	ParentCase     *NumberRef `json:"parentCase,omitempty"`
	FixEta         *time.Time `json:"fixEta,omitempty"`
}

// MapCaseUpdate builds the portal response from entity-service's UpdateCaseResponse.
func MapCaseUpdate(r entity.UpdateCaseResponse) CaseUpdateResponse {
	c := r.Case

	var watchList []string
	if len(c.WatchList) > 0 {
		watchList = make([]string, 0, len(c.WatchList))
		for _, w := range c.WatchList {
			if w.Email != "" {
				watchList = append(watchList, w.Email)
			} else {
				watchList = append(watchList, w.UserName)
			}
		}
	}

	var assignedTo *PersonRef
	if c.AssignedTo != nil {
		assignedTo = &PersonRef{Name: c.AssignedTo.Name, Email: c.AssignedTo.Email}
	}

	return CaseUpdateResponse{
		ID:             c.ID,
		UpdatedOn:      c.UpdatedOn,
		State:          c.State,
		Severity:       c.Severity,
		WorkState:      c.WorkState,
		WatchList:      watchList,
		AssignedTo:     assignedTo,
		ResolutionCode: c.ResolutionCode,
		Cause:          c.Cause,
		CloseNotes:     c.CloseNotes,
		ResolvedOn:     c.ResolvedOn,
		ParentCase:     mapNumberRef(c.ParentCase),
		FixEta:         c.FixEta,
	}
}

// CaseCommentRequest is the portal's request shape for POST /cases/{id}/comments.
// entity-service also supports "work_note" and "activity" comment types, but
// those are internal WSO2 support annotations — the customer portal only
// ever lets a customer post a plain "comment"; see
// BuildEntityCreateCaseCommentRequest.
type CaseCommentRequest struct {
	Content string `json:"content"`
}

// BuildEntityCreateCaseCommentRequest converts the portal's comment request
// into entity-service's request shape, forcing Type to "comment" regardless
// of anything the caller might otherwise try to set.
func BuildEntityCreateCaseCommentRequest(caseID string, req CaseCommentRequest) entity.CreateCaseCommentRequest {
	return entity.CreateCaseCommentRequest{
		CaseID:  caseID,
		Type:    entity.CommentTypeComment,
		Content: req.Content,
	}
}

// CaseCommentResponse is the portal's response for POST /cases/{id}/comments.
type CaseCommentResponse struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
}

// MapCaseComment builds the portal response from entity-service's CreateCaseCommentResponse.
func MapCaseComment(r entity.CreateCaseCommentResponse) CaseCommentResponse {
	return CaseCommentResponse{
		ID:        r.Comment.ID,
		CreatedOn: r.Comment.CreatedOn,
		CreatedBy: r.Comment.CreatedBy,
	}
}

// CaseFieldChange describes a single field's value change in a case's activity feed.
type CaseFieldChange struct {
	Field         string `json:"field"`
	FieldLabel    string `json:"fieldLabel"`
	PreviousValue string `json:"previousValue"`
	NewValue      string `json:"newValue"`
}

// CaseActivity is one entry in the portal's response for
// POST /cases/{id}/activities/search — a discriminated union on Type, like
// entity-service's CaseActivity. Deliberately excludes entity-service's
// CreatedByFirstName/CreatedByLastName (redundant with CreatedBy, which
// carries the full name) and the raw internal actor ID.
type CaseActivity struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Content     string            `json:"content,omitempty"`
	CreatedOn   time.Time         `json:"createdOn"`
	CreatedBy   string            `json:"createdBy"`
	CommentType *string           `json:"commentType,omitempty"`
	FileName    string            `json:"fileName,omitempty"`
	ContentType string            `json:"contentType,omitempty"`
	SizeBytes   int               `json:"sizeBytes,omitempty"`
	DownloadURL string            `json:"downloadUrl,omitempty"`
	Changes     []CaseFieldChange `json:"changes,omitempty"`
}

// SearchCaseActivitiesResponse is the portal's response for POST /cases/{id}/activities/search.
type SearchCaseActivitiesResponse struct {
	Activity []CaseActivity `json:"activity"`
	Total    int            `json:"total"`
	Limit    int            `json:"limit"`
	Offset   int            `json:"offset"`
	HasMore  bool           `json:"hasMore"`
}

// MapSearchCaseActivities builds the portal response from entity-service's SearchCaseActivitiesResponse.
func MapSearchCaseActivities(r entity.SearchCaseActivitiesResponse) SearchCaseActivitiesResponse {
	items := make([]CaseActivity, 0, len(r.Activity))
	for _, a := range r.Activity {
		var commentType *string
		if a.CommentType != nil {
			s := string(*a.CommentType)
			commentType = &s
		}

		var changes []CaseFieldChange
		if len(a.Changes) > 0 {
			changes = make([]CaseFieldChange, 0, len(a.Changes))
			for _, c := range a.Changes {
				changes = append(changes, CaseFieldChange{
					Field:         c.Field,
					FieldLabel:    c.FieldLabel,
					PreviousValue: c.PreviousValue,
					NewValue:      c.NewValue,
				})
			}
		}

		items = append(items, CaseActivity{
			ID:          a.ID,
			Type:        string(a.Type),
			Content:     a.Content,
			CreatedOn:   a.CreatedOn,
			CreatedBy:   a.CreatedByFullName,
			CommentType: commentType,
			FileName:    a.FileName,
			ContentType: a.ContentType,
			SizeBytes:   a.SizeBytes,
			DownloadURL: a.DownloadURL,
			Changes:     changes,
		})
	}
	return SearchCaseActivitiesResponse{
		Activity: items,
		Total:    r.Total,
		Limit:    r.Limit,
		Offset:   r.Offset,
		HasMore:  r.HasMore,
	}
}

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

import "github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"

// ChangeRequestCreateRequest is the portal's request shape for
// POST /change-requests — a deliberately restricted subset of
// entity-service's CreateChangeRequestRequest. Excluded fields are internal
// WSO2 support operations, not customer self-service actions: GroupID/
// AssignedEngineerID (support team/engineer assignment), State (entity-service
// defaults this; a customer shouldn't set the initial workflow state
// directly), RequestedByID (an arbitrary "on behalf of" WSO2/ServiceNow user
// id — not something an external customer has or should supply), and
// WorkNote (an internal support annotation, same rationale as excluding
// entity-service's "work_note" comment type from case comments).
type ChangeRequestCreateRequest struct {
	Subject             string  `json:"subject"`
	Category            *string `json:"category,omitempty"`
	ServiceID           *string `json:"serviceId,omitempty"`
	ServiceOfferingID   *string `json:"serviceOfferingId,omitempty"`
	ConfigurationItemID *string `json:"configurationItemId,omitempty"`
	Priority            *string `json:"priority,omitempty"`
	Impact              *string `json:"impact,omitempty"`
	Type                *string `json:"type,omitempty"`
	Risk                *string `json:"risk,omitempty"`
	Description         *string `json:"description,omitempty"`
	Justification       *string `json:"justification,omitempty"`
	ImplementationPlan  *string `json:"implementationPlan,omitempty"`
	RiskImpactAnalysis  *string `json:"riskImpactAnalysis,omitempty"`
	BackoutPlan         *string `json:"backoutPlan,omitempty"`
	TestPlan            *string `json:"testPlan,omitempty"`
	PlannedStartDate    *string `json:"plannedStartDate,omitempty"`
	PlannedEndDate      *string `json:"plannedEndDate,omitempty"`
	Comment             *string `json:"comment,omitempty"`
}

// BuildEntityCreateChangeRequestRequest converts the portal's restricted
// create request into entity-service's full request shape, leaving every
// excluded field nil.
func BuildEntityCreateChangeRequestRequest(req ChangeRequestCreateRequest) entity.CreateChangeRequestRequest {
	return entity.CreateChangeRequestRequest{
		Subject:             req.Subject,
		Category:            req.Category,
		ServiceID:           req.ServiceID,
		ServiceOfferingID:   req.ServiceOfferingID,
		ConfigurationItemID: req.ConfigurationItemID,
		Priority:            req.Priority,
		Impact:              req.Impact,
		Type:                req.Type,
		Risk:                req.Risk,
		Description:         req.Description,
		Justification:       req.Justification,
		ImplementationPlan:  req.ImplementationPlan,
		RiskImpactAnalysis:  req.RiskImpactAnalysis,
		BackoutPlan:         req.BackoutPlan,
		TestPlan:            req.TestPlan,
		PlannedStartDate:    req.PlannedStartDate,
		PlannedEndDate:      req.PlannedEndDate,
		Comment:             req.Comment,
	}
}

// ChangeRequestCreateResponse is the portal's response for POST /change-requests.
type ChangeRequestCreateResponse struct {
	ID        string `json:"id"`
	Number    string `json:"number"`
	CreatedOn string `json:"createdOn"`
}

// MapChangeRequestCreate builds the portal response from entity-service's CreateChangeRequestResponse.
func MapChangeRequestCreate(r entity.CreateChangeRequestResponse) ChangeRequestCreateResponse {
	return ChangeRequestCreateResponse{
		ID:        r.ChangeRequest.ID,
		Number:    r.ChangeRequest.Number,
		CreatedOn: r.ChangeRequest.CreatedOn,
	}
}

// ChangeRequestSummary is one item of the portal's response for
// POST /change-requests/search.
type ChangeRequestSummary struct {
	ID               string  `json:"id"`
	Number           string  `json:"number"`
	Subject          *string `json:"subject,omitempty"`
	Description      *string `json:"description,omitempty"`
	Project          Ref     `json:"project"`
	Case             *Ref    `json:"case,omitempty"`
	Deployment       *Ref    `json:"deployment,omitempty"`
	DeployedProduct  *Ref    `json:"deployedProduct,omitempty"`
	Product          *Ref    `json:"product,omitempty"`
	AssignedEngineer *Ref    `json:"assignedEngineer,omitempty"`
	AssignedTeam     *Ref    `json:"assignedTeam,omitempty"`
	PlannedStartOn   *string `json:"plannedStartOn,omitempty"`
	PlannedEndOn     *string `json:"plannedEndOn,omitempty"`
	Duration         *string `json:"duration,omitempty"`
	Impact           *string `json:"impact,omitempty"`
	State            *string `json:"state,omitempty"`
	Type             *string `json:"type,omitempty"`
	CreatedOn        string  `json:"createdOn"`
	UpdatedOn        string  `json:"updatedOn"`
}

// SearchChangeRequestsResponse is the portal's response for POST /change-requests/search.
type SearchChangeRequestsResponse struct {
	ChangeRequests []ChangeRequestSummary `json:"changeRequests"`
	Total          int                    `json:"total"`
	Offset         int                    `json:"offset"`
	Limit          int                    `json:"limit"`
}

func mapChangeRequestSummary(v entity.SearchChangeRequestView) ChangeRequestSummary {
	return ChangeRequestSummary{
		ID:               v.ID,
		Number:           v.Number,
		Subject:          v.Subject,
		Description:      v.Description,
		Project:          Ref{ID: v.Project.ID, Name: v.Project.Name},
		Case:             mapRef(v.Case),
		Deployment:       mapRef(v.Deployment),
		DeployedProduct:  mapRef(v.DeployedProduct),
		Product:          mapRef(v.Product),
		AssignedEngineer: mapRef(v.AssignedEngineer),
		AssignedTeam:     mapRef(v.AssignedTeam),
		PlannedStartOn:   v.PlannedStartOn,
		PlannedEndOn:     v.PlannedEndOn,
		Duration:         v.Duration,
		Impact:           v.Impact,
		State:            v.State,
		Type:             v.Type,
		CreatedOn:        v.CreatedOn,
		UpdatedOn:        v.UpdatedOn,
	}
}

// MapSearchChangeRequests builds the portal response from entity-service's SearchChangeRequestsResponse.
func MapSearchChangeRequests(r entity.SearchChangeRequestsResponse) SearchChangeRequestsResponse {
	items := make([]ChangeRequestSummary, 0, len(r.ChangeRequests))
	for _, v := range r.ChangeRequests {
		items = append(items, mapChangeRequestSummary(v))
	}
	return SearchChangeRequestsResponse{
		ChangeRequests: items,
		Total:          r.Total,
		Offset:         r.Offset,
		Limit:          r.Limit,
	}
}

// ChangeRequestDetails is the portal's response for GET /change-requests/{id}
// and PATCH /change-requests/{id}.
type ChangeRequestDetails struct {
	ChangeRequestSummary
	CreatedBy           string   `json:"createdBy"`
	Justification       *string  `json:"justification,omitempty"`
	ImpactDescription   *string  `json:"impactDescription,omitempty"`
	ServiceOutage       *string  `json:"serviceOutage,omitempty"`
	CommunicationPlan   *string  `json:"communicationPlan,omitempty"`
	RollbackPlan        *string  `json:"rollbackPlan,omitempty"`
	TestPlan            *string  `json:"testPlan,omitempty"`
	HasCustomerApproved bool     `json:"hasCustomerApproved"`
	HasCustomerReviewed bool     `json:"hasCustomerReviewed"`
	ApprovedBy          *Ref     `json:"approvedBy,omitempty"`
	ApprovedOn          *string  `json:"approvedOn,omitempty"`
	LegalNextStates     []string `json:"legalNextStates,omitempty"`
}

// MapChangeRequestDetails builds the portal response from entity-service's ChangeRequest.
func MapChangeRequestDetails(r entity.ChangeRequest) ChangeRequestDetails {
	return ChangeRequestDetails{
		ChangeRequestSummary: mapChangeRequestSummary(r.SearchChangeRequestView),
		CreatedBy:            r.CreatedBy,
		Justification:        r.Justification,
		ImpactDescription:    r.ImpactDescription,
		ServiceOutage:        r.ServiceOutage,
		CommunicationPlan:    r.CommunicationPlan,
		RollbackPlan:         r.RollbackPlan,
		TestPlan:             r.TestPlan,
		HasCustomerApproved:  r.HasCustomerApproved,
		HasCustomerReviewed:  r.HasCustomerReviewed,
		ApprovedBy:           mapRef(r.ApprovedBy),
		ApprovedOn:           r.ApprovedOn,
		LegalNextStates:      r.LegalNextStates,
	}
}

// ChangeRequestUpdateRequest is the portal's request shape for
// PATCH /change-requests/{id} — a deliberately restricted subset of
// entity-service's PatchChangeRequestRequest. Excluded fields are internal
// WSO2 support operations: ProjectID/CaseID/DeploymentID/DeployedProductID
// (change-request relinking), AssignedEngineerID/AssignedTeamID (support
// assignment), and State (state transitions are driven through the
// dedicated approval workflow — IsCustomerApproved/IsCustomerReviewed/
// RequestApproval below — not by setting state directly).
type ChangeRequestUpdateRequest struct {
	Title              *string `json:"title,omitempty"`
	Description        *string `json:"description,omitempty"`
	PlannedStartOn     *string `json:"plannedStartOn,omitempty"`
	PlannedEndOn       *string `json:"plannedEndOn,omitempty"`
	Impact             *string `json:"impact,omitempty"`
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

// BuildEntityPatchChangeRequestRequest converts the portal's restricted
// update request into entity-service's full request shape, leaving every
// excluded field nil.
func BuildEntityPatchChangeRequestRequest(req ChangeRequestUpdateRequest) entity.PatchChangeRequestRequest {
	return entity.PatchChangeRequestRequest{
		Title:              req.Title,
		Description:        req.Description,
		PlannedStartOn:     req.PlannedStartOn,
		PlannedEndOn:       req.PlannedEndOn,
		Impact:             req.Impact,
		Type:               req.Type,
		Justification:      req.Justification,
		ImpactDescription:  req.ImpactDescription,
		ServiceOutage:      req.ServiceOutage,
		CommunicationPlan:  req.CommunicationPlan,
		RollbackPlan:       req.RollbackPlan,
		TestPlan:           req.TestPlan,
		IsCustomerApproved: req.IsCustomerApproved,
		IsCustomerReviewed: req.IsCustomerReviewed,
		RequestApproval:    req.RequestApproval,
	}
}

// ChangeRequestApprover is a single approver's response within an approval stage.
type ChangeRequestApprover struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	RespondedOn *string `json:"respondedOn,omitempty"`
}

// ChangeRequestApproval represents a single approval stage on a change request.
type ChangeRequestApproval struct {
	Stage        string                  `json:"stage"`
	ApproverType string                  `json:"approverType"`
	ApproverName string                  `json:"approverName"`
	Status       string                  `json:"status"`
	Approvers    []ChangeRequestApprover `json:"approvers"`
}

// ChangeRequestApprovals is the portal's response for GET /change-requests/{id}/approvals.
type ChangeRequestApprovals struct {
	Approvals []ChangeRequestApproval `json:"approvals"`
}

// MapChangeRequestApprovals builds the portal response from entity-service's ChangeRequestApprovals.
func MapChangeRequestApprovals(r entity.ChangeRequestApprovals) ChangeRequestApprovals {
	approvals := make([]ChangeRequestApproval, 0, len(r.Approvals))
	for _, a := range r.Approvals {
		approvers := make([]ChangeRequestApprover, 0, len(a.Approvers))
		for _, ap := range a.Approvers {
			approvers = append(approvers, ChangeRequestApprover{
				ID:          ap.ID,
				Name:        ap.Name,
				Status:      ap.Status,
				RespondedOn: ap.RespondedOn,
			})
		}
		approvals = append(approvals, ChangeRequestApproval{
			Stage:        a.Stage,
			ApproverType: a.ApproverType,
			ApproverName: a.ApproverName,
			Status:       a.Status,
			Approvers:    approvers,
		})
	}
	return ChangeRequestApprovals{Approvals: approvals}
}

// ChangeRequestApprovalDecisionResponse is the portal's response for
// POST /change-requests/{id}/approvals/decision.
type ChangeRequestApprovalDecisionResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// MapChangeRequestApprovalDecision builds the portal response from
// entity-service's ChangeRequestApprovalDecisionResponse.
func MapChangeRequestApprovalDecision(r entity.ChangeRequestApprovalDecisionResponse) ChangeRequestApprovalDecisionResponse {
	return ChangeRequestApprovalDecisionResponse{ID: r.ID, State: r.State}
}

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

// ProjectSummary is one item of the portal's response for POST /projects/search.
//
// Deliberately excludes entity-service's endDateClosureState,
// invoiceDueDateClosureState, complianceViolationClosureState,
// complianceViolationDate and suspensionProcessState — these are WSO2-internal
// closure/compliance workflow fields not meant for customer display.
type ProjectSummary struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Key              string     `json:"key"`
	SubscriptionType string     `json:"subscriptionType"`
	EndDate          *time.Time `json:"endDate,omitempty"`
	CreatedOn        time.Time  `json:"createdOn"`
	ClosureState     *string    `json:"closureState,omitempty"`
}

// SearchProjectsResponse is the portal's response for POST /projects/search.
type SearchProjectsResponse struct {
	Projects []ProjectSummary `json:"projects"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
	HasMore  bool              `json:"hasMore"`
}

// MapSearchProjects builds the portal response from entity-service's SearchProjectsResponse.
func MapSearchProjects(r entity.SearchProjectsResponse) SearchProjectsResponse {
	projects := make([]ProjectSummary, 0, len(r.Projects))
	for _, p := range r.Projects {
		projects = append(projects, ProjectSummary{
			ID:               p.ID,
			Name:             p.Name,
			Key:              p.Key,
			SubscriptionType: p.SubscriptionType,
			EndDate:          p.EndDate,
			CreatedOn:        p.CreatedOn,
			ClosureState:     p.ClosureState,
		})
	}
	return SearchProjectsResponse{
		Projects: projects,
		Total:    r.Total,
		Limit:    r.Limit,
		Offset:   r.Offset,
		HasMore:  r.HasMore,
	}
}

// ProjectAccount is the account summary embedded in ProjectDetails.
//
// Deliberately excludes entity-service's AgentEnabled/KbReferencesEnabled
// (internal feature-flags gating WSO2 agent behavior, not customer-facing)
// and ActivationDate (redundant with the project's own dates for this view).
type ProjectAccount struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Tier   string  `json:"tier"`
	Region *string `json:"region,omitempty"`
}

// ProjectDetails is the portal's response for GET /projects/{id}.
//
// Deliberately excludes entity-service's SfID (Salesforce internal ID —
// must never reach the frontend) and the same closure/compliance fields
// dropped from ProjectSummary above.
type ProjectDetails struct {
	ID               string         `json:"id"`
	Account          ProjectAccount `json:"account"`
	Name             string         `json:"name"`
	Key              string         `json:"key"`
	SubscriptionType string         `json:"subscriptionType"`
	StartDate        time.Time      `json:"startDate"`
	EndDate          time.Time      `json:"endDate"`
	CreatedOn        time.Time      `json:"createdOn"`
	UpdatedOn        time.Time      `json:"updatedOn"`
	ClosureState     *string        `json:"closureState,omitempty"`
}

// MapProjectDetails builds the portal response from entity-service's ProjectDetailsView.
func MapProjectDetails(p entity.ProjectDetailsView) ProjectDetails {
	return ProjectDetails{
		ID: p.ID,
		Account: ProjectAccount{
			ID:     p.Account.ID,
			Name:   p.Account.Name,
			Tier:   p.Account.Tier,
			Region: p.Account.Region,
		},
		Name:             p.Name,
		Key:              p.Key,
		SubscriptionType: p.SubscriptionType,
		StartDate:        p.StartDate,
		EndDate:          p.EndDate,
		CreatedOn:        p.CreatedOn,
		UpdatedOn:        p.UpdatedOn,
		ClosureState:     p.ClosureState,
	}
}

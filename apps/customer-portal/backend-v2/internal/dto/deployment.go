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

// DeploymentSummary is one item of the portal's response for
// POST /projects/{id}/deployments/search.
type DeploymentSummary struct {
	ID          string    `json:"id"`
	Number      string    `json:"number"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Description *string   `json:"description,omitempty"`
	CreatedBy   *Ref      `json:"createdBy,omitempty"`
	Project     Ref       `json:"project"`
	CreatedOn   time.Time `json:"createdOn"`
	UpdatedOn   time.Time `json:"updatedOn"`
}

// SearchDeploymentsResponse is the portal's response for
// POST /projects/{id}/deployments/search.
type SearchDeploymentsResponse struct {
	Deployments []DeploymentSummary `json:"deployments"`
	Total       int                 `json:"total"`
	Limit       int                 `json:"limit"`
	Offset      int                 `json:"offset"`
	HasMore     bool                `json:"hasMore"`
}

// MapSearchDeployments builds the portal response from entity-service's SearchDeploymentsResponse.
func MapSearchDeployments(r entity.SearchDeploymentsResponse) SearchDeploymentsResponse {
	deployments := make([]DeploymentSummary, 0, len(r.Deployments))
	for _, d := range r.Deployments {
		deployments = append(deployments, DeploymentSummary{
			ID:          d.ID,
			Number:      d.Number,
			Name:        d.Name,
			Type:        d.Type,
			Description: d.Description,
			CreatedBy:   mapRef(d.CreatedBy),
			Project:     Ref{ID: d.Project.ID, Name: d.Project.Name},
			CreatedOn:   d.CreatedOn,
			UpdatedOn:   d.UpdatedOn,
		})
	}
	return SearchDeploymentsResponse{
		Deployments: deployments,
		Total:       r.Total,
		Limit:       r.Limit,
		Offset:      r.Offset,
		HasMore:     r.HasMore,
	}
}

// DeploymentCreateResponse is the portal's response for POST /deployments.
type DeploymentCreateResponse struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
}

// MapDeploymentCreate builds the portal response from entity-service's CreateDeploymentResponse.
func MapDeploymentCreate(r entity.CreateDeploymentResponse) DeploymentCreateResponse {
	return DeploymentCreateResponse{
		ID:        r.Deployment.ID,
		CreatedOn: r.Deployment.CreatedOn,
	}
}

// DeploymentUpdateResponse is the portal's response for PATCH /deployments/{id}.
// Deliberately excludes entity-service's UpdatedBy (internal actor identity),
// consistent with the other update responses in this package.
type DeploymentUpdateResponse struct {
	ID        string    `json:"id"`
	UpdatedOn time.Time `json:"updatedOn"`
}

// MapDeploymentUpdate builds the portal response from entity-service's UpdateDeploymentResponse.
func MapDeploymentUpdate(r entity.UpdateDeploymentResponse) DeploymentUpdateResponse {
	return DeploymentUpdateResponse{
		ID:        r.Deployment.ID,
		UpdatedOn: r.Deployment.UpdatedOn,
	}
}

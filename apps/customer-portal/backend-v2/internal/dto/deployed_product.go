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

// DeployedProductVersion is the version reference embedded in a deployed product.
type DeployedProductVersion struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	ReleasedDate   *time.Time `json:"releasedDate,omitempty"`
	SupportEoLDate *time.Time `json:"supportEoLDate,omitempty"`
}

// DeployedProductSummary is one item of the portal's response for
// POST /deployed-products/search.
type DeployedProductSummary struct {
	ID         string                  `json:"id"`
	Deployment Ref                     `json:"deployment"`
	Product    Ref                     `json:"product"`
	Version    *DeployedProductVersion `json:"version,omitempty"`
	Cores      *string                 `json:"cores,omitempty"`
	TPS        *string                 `json:"tps,omitempty"`
	Category   *string                 `json:"category,omitempty"`
	CreatedOn  time.Time               `json:"createdOn"`
	UpdatedOn  time.Time               `json:"updatedOn"`
}

// SearchDeployedProductsResponse is the portal's response for POST /deployed-products/search.
type SearchDeployedProductsResponse struct {
	DeployedProducts []DeployedProductSummary `json:"deployedProducts"`
	Total            int                      `json:"total"`
	Limit            int                      `json:"limit"`
	Offset           int                      `json:"offset"`
	HasMore          bool                     `json:"hasMore"`
}

func mapDeployedProductVersion(v *entity.DeployedProductVersionRef) *DeployedProductVersion {
	if v == nil {
		return nil
	}
	return &DeployedProductVersion{
		ID:             v.ID,
		Name:           v.Name,
		ReleasedDate:   v.ReleasedDate,
		SupportEoLDate: v.SupportEoLDate,
	}
}

// MapSearchDeployedProducts builds the portal response from entity-service's SearchDeployedProductsResponse.
func MapSearchDeployedProducts(r entity.SearchDeployedProductsResponse) SearchDeployedProductsResponse {
	items := make([]DeployedProductSummary, 0, len(r.DeployedProducts))
	for _, d := range r.DeployedProducts {
		items = append(items, DeployedProductSummary{
			ID:         d.ID,
			Deployment: Ref{ID: d.Deployment.ID, Name: d.Deployment.Name},
			Product:    Ref{ID: d.Product.ID, Name: d.Product.Name},
			Version:    mapDeployedProductVersion(d.Version),
			Cores:      d.Cores,
			TPS:        d.TPS,
			Category:   d.Category,
			CreatedOn:  d.CreatedOn,
			UpdatedOn:  d.UpdatedOn,
		})
	}
	return SearchDeployedProductsResponse{
		DeployedProducts: items,
		Total:            r.Total,
		Limit:            r.Limit,
		Offset:           r.Offset,
		HasMore:          r.HasMore,
	}
}

// DeployedProductCreateResponse is the portal's response for POST /deployed-products.
type DeployedProductCreateResponse struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
}

// MapDeployedProductCreate builds the portal response from entity-service's CreateDeployedProductResponse.
func MapDeployedProductCreate(r entity.CreateDeployedProductResponse) DeployedProductCreateResponse {
	return DeployedProductCreateResponse{
		ID:        r.DeployedProduct.ID,
		CreatedOn: r.DeployedProduct.CreatedOn,
	}
}

// DeployedProductUpdateResponse is the portal's response for PATCH /deployed-products/{id}.
// Deliberately excludes entity-service's UpdatedBy (internal actor identity),
// consistent with the other update responses in this package.
type DeployedProductUpdateResponse struct {
	ID        string    `json:"id"`
	UpdatedOn time.Time `json:"updatedOn"`
}

// MapDeployedProductUpdate builds the portal response from entity-service's UpdateDeployedProductResponse.
func MapDeployedProductUpdate(r entity.UpdateDeployedProductResponse) DeployedProductUpdateResponse {
	return DeployedProductUpdateResponse{
		ID:        r.DeployedProduct.ID,
		UpdatedOn: r.DeployedProduct.UpdatedOn,
	}
}

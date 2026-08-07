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

// ProductSummary is one item of the portal's response for POST /products/search.
type ProductSummary struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Class     *string `json:"class,omitempty"`
	CreatedOn *string `json:"createdOn,omitempty"`
	UpdatedOn *string `json:"updatedOn,omitempty"`
}

// SearchProductsResponse is the portal's response for POST /products/search
// and GET /products. TotalRecords (not Total) to match the frontend's
// shared pagination envelope.
type SearchProductsResponse struct {
	Products     []ProductSummary `json:"products"`
	TotalRecords int              `json:"totalRecords"`
	Limit        int              `json:"limit"`
	Offset       int              `json:"offset"`
	HasMore      bool             `json:"hasMore"`
}

// MapSearchProducts builds the portal response from entity-service's SearchProductsResponse.
func MapSearchProducts(r entity.SearchProductsResponse) SearchProductsResponse {
	items := make([]ProductSummary, 0, len(r.Products))
	for _, p := range r.Products {
		items = append(items, ProductSummary{
			ID:        p.ID,
			Name:      p.Name,
			Class:     p.Class,
			CreatedOn: p.CreatedOn,
			UpdatedOn: p.UpdatedOn,
		})
	}
	return SearchProductsResponse{
		Products:     items,
		TotalRecords: r.Total,
		Limit:        r.Limit,
		Offset:       r.Offset,
		HasMore:      r.HasMore,
	}
}

// GetProductsRequest is the portal's translated request for GET /products —
// built from the offset/limit query params by the handler. entity-service's
// SearchProductsRequest has no class filter at all (unlike the old
// Ballerina backend's target service, which accepted filters.class), so the
// frontend's `class=product_model` query param has no server-side
// equivalent here — a genuine entity-service gap, not fixable in this dto
// layer alone.
type GetProductsRequest struct {
	Pagination entity.Pagination
}

// BuildEntitySearchProductsRequestFromQuery translates GET /products' query
// params into entity-service's POST /products/search request shape.
func BuildEntitySearchProductsRequestFromQuery(req GetProductsRequest) entity.SearchProductsRequest {
	return entity.SearchProductsRequest{Pagination: req.Pagination}
}

// ProductVersionSummary is one item of the portal's response for
// POST /products/{id}/versions/search. Deliberately excludes entity-service's
// ProductID — already known from the request path, redundant here.
type ProductVersionSummary struct {
	ID                             string  `json:"id"`
	Version                        string  `json:"version"`
	CurrentSupportStatus           *string `json:"currentSupportStatus,omitempty"`
	ReleaseDate                    *string `json:"releaseDate,omitempty"`
	SupportEOLDate                 *string `json:"supportEolDate,omitempty"`
	EarliestPossibleSupportEOLDate *string `json:"earliestPossibleSupportEolDate,omitempty"`
	CreatedOn                      *string `json:"createdOn,omitempty"`
	UpdatedOn                      *string `json:"updatedOn,omitempty"`
}

// SearchProductVersionsResponse is the portal's response for
// POST /products/{id}/versions/search — Versions (not ProductVersions) and
// TotalRecords (not Total) to match the frontend's own
// ProductVersionsSearchResponse type (apps/customer-portal/webapp/src/
// features/project-details/types/products.ts).
type SearchProductVersionsResponse struct {
	Versions     []ProductVersionSummary `json:"versions"`
	TotalRecords int                     `json:"totalRecords"`
	Limit        int                     `json:"limit"`
	Offset       int                     `json:"offset"`
	HasMore      bool                    `json:"hasMore"`
}

// MapSearchProductVersions builds the portal response from entity-service's SearchProductVersionsResponse.
func MapSearchProductVersions(r entity.SearchProductVersionsResponse) SearchProductVersionsResponse {
	items := make([]ProductVersionSummary, 0, len(r.ProductVersions))
	for _, v := range r.ProductVersions {
		items = append(items, ProductVersionSummary{
			ID:                             v.ID,
			Version:                        v.Version,
			CurrentSupportStatus:           v.CurrentSupportStatus,
			ReleaseDate:                    v.ReleaseDate,
			SupportEOLDate:                 v.SupportEOLDate,
			EarliestPossibleSupportEOLDate: v.EarliestPossibleSupportEOLDate,
			CreatedOn:                      v.CreatedOn,
			UpdatedOn:                      v.UpdatedOn,
		})
	}
	return SearchProductVersionsResponse{
		Versions:     items,
		TotalRecords: r.Total,
		Limit:        r.Limit,
		Offset:       r.Offset,
		HasMore:      r.HasMore,
	}
}

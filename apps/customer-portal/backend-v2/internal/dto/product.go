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

// SearchProductsResponse is the portal's response for POST /products/search.
type SearchProductsResponse struct {
	Products []ProductSummary `json:"products"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
	HasMore  bool             `json:"hasMore"`
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
		Products: items,
		Total:    r.Total,
		Limit:    r.Limit,
		Offset:   r.Offset,
		HasMore:  r.HasMore,
	}
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

// SearchProductVersionsResponse is the portal's response for POST /products/{id}/versions/search.
type SearchProductVersionsResponse struct {
	ProductVersions []ProductVersionSummary `json:"productVersions"`
	Total           int                     `json:"total"`
	Limit           int                     `json:"limit"`
	Offset          int                     `json:"offset"`
	HasMore         bool                    `json:"hasMore"`
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
		ProductVersions: items,
		Total:           r.Total,
		Limit:           r.Limit,
		Offset:          r.Offset,
		HasMore:         r.HasMore,
	}
}

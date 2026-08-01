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

// CatalogItem is a single selectable item within a service catalog.
type CatalogItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Catalog is one item of the portal's response for POST /catalogs/search.
type Catalog struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	CatalogItems []CatalogItem `json:"catalogItems"`
}

// SearchCatalogsResponse is the portal's response for POST /catalogs/search.
type SearchCatalogsResponse struct {
	Catalogs []Catalog `json:"catalogs"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
}

// MapSearchCatalogs builds the portal response from entity-service's SearchCatalogsResponse.
func MapSearchCatalogs(r entity.SearchCatalogsResponse) SearchCatalogsResponse {
	catalogs := make([]Catalog, 0, len(r.Catalogs))
	for _, c := range r.Catalogs {
		items := make([]CatalogItem, 0, len(c.CatalogItems))
		for _, item := range c.CatalogItems {
			items = append(items, CatalogItem{ID: item.ID, Name: item.Name})
		}
		catalogs = append(catalogs, Catalog{
			ID:           c.ID,
			Name:         c.Name,
			CatalogItems: items,
		})
	}
	return SearchCatalogsResponse{
		Catalogs: catalogs,
		Total:    r.Total,
		Limit:    r.Limit,
		Offset:   r.Offset,
	}
}

// CatalogItemVariable is the portal's shape for a single catalog-item variable.
type CatalogItemVariable struct {
	ID           string `json:"id"`
	QuestionText string `json:"questionText"`
	Order        int    `json:"order"`
	Type         string `json:"type"`
}

// CatalogItemVariablesResponse is the portal's response for
// GET /catalogs/{catalogId}/items/{catalogItemId}/variables.
type CatalogItemVariablesResponse struct {
	Variables []CatalogItemVariable `json:"variables"`
}

// MapCatalogItemVariables builds the portal response from entity-service's
// GetCatalogItemVariablesResponse.
func MapCatalogItemVariables(r entity.GetCatalogItemVariablesResponse) CatalogItemVariablesResponse {
	variables := make([]CatalogItemVariable, 0, len(r.Variables))
	for _, v := range r.Variables {
		variables = append(variables, CatalogItemVariable{
			ID:           v.ID,
			QuestionText: v.QuestionText,
			Order:        v.Order,
			Type:         v.Type,
		})
	}
	return CatalogItemVariablesResponse{Variables: variables}
}

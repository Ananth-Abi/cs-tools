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

package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityProductClient abstracts the entity-service product operations used by ProductHandler.
type entityProductClient interface {
	SearchProducts(ctx context.Context, req entity.SearchProductsRequest) (entity.SearchProductsResponse, error)
	SearchProductVersions(ctx context.Context, productID string, req entity.SearchProductVersionsRequest) (entity.SearchProductVersionsResponse, error)
}

// ProductHandler handles HTTP requests for product operations.
type ProductHandler struct {
	entity entityProductClient
}

// NewProductHandler creates a ProductHandler backed by the given entity client.
func NewProductHandler(entity entityProductClient) *ProductHandler {
	return &ProductHandler{entity: entity}
}

// SearchProducts handles POST /products/search.
func (h *ProductHandler) SearchProducts(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.SearchProductsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchProducts(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProducts failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search products.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchProducts(result))
}

// SearchProductVersions handles POST /products/{id}/versions/search.
func (h *ProductHandler) SearchProductVersions(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.SearchProductVersionsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchProductVersions(r.Context(), id, req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProductVersions failed", "userID", user.UserID, "productID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search product versions.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchProductVersions(result))
}

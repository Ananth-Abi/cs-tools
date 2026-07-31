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

// entityDeployedProductClient abstracts the entity-service deployed-product
// operations used by DeployedProductHandler.
type entityDeployedProductClient interface {
	SearchDeployedProducts(ctx context.Context, req entity.SearchDeployedProductsRequest) (entity.SearchDeployedProductsResponse, error)
	CreateDeployedProduct(ctx context.Context, req entity.CreateDeployedProductRequest) (entity.CreateDeployedProductResponse, error)
	UpdateDeployedProduct(ctx context.Context, id string, req entity.UpdateDeployedProductRequest) (entity.UpdateDeployedProductResponse, error)
}

// DeployedProductHandler handles HTTP requests for deployed-product operations.
type DeployedProductHandler struct {
	entity entityDeployedProductClient
}

// NewDeployedProductHandler creates a DeployedProductHandler backed by the given entity client.
func NewDeployedProductHandler(entity entityDeployedProductClient) *DeployedProductHandler {
	return &DeployedProductHandler{entity: entity}
}

// SearchDeployedProducts handles POST /deployed-products/search.
func (h *DeployedProductHandler) SearchDeployedProducts(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.SearchDeployedProductsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchDeployedProducts(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployedProducts failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search deployed products.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchDeployedProducts(result))
}

// CreateDeployedProduct handles POST /deployed-products.
//
// NOTE: entity-service only supports this route on its ServiceNow data
// source — see internal/entity/deployed_products.go.
func (h *DeployedProductHandler) CreateDeployedProduct(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.CreateDeployedProductRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateDeployedProduct(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateDeployedProduct failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create deployed product.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapDeployedProductCreate(result))
}

// PatchDeployedProduct handles PATCH /deployed-products/{id}.
//
// NOTE: entity-service only supports this route on its ServiceNow data
// source — see internal/entity/deployed_products.go.
func (h *DeployedProductHandler) PatchDeployedProduct(w http.ResponseWriter, r *http.Request) {
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

	var req entity.UpdateDeployedProductRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	// entity-service requires exactly one of the detail-fields group
	// (cores/tps/description) or active=false — never both, never neither.
	detailFieldsSet := req.Cores != nil || req.TPS != nil || len(req.Description) > 0
	activeSet := req.Active != nil
	if detailFieldsSet == activeSet {
		writeError(w, http.StatusBadRequest, "Provide either cores/tps/description or active, but not both.")
		return
	}

	result, err := h.entity.UpdateDeployedProduct(r.Context(), id, req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateDeployedProduct failed", "userID", user.UserID, "deployedProductID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update deployed product.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapDeployedProductUpdate(result))
}

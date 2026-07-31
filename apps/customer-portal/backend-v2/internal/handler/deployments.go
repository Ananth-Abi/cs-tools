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

// entityDeploymentClient abstracts the entity-service deployment operations
// used by DeploymentHandler.
type entityDeploymentClient interface {
	SearchDeployments(ctx context.Context, req entity.SearchDeploymentsRequest) (entity.SearchDeploymentsResponse, error)
	CreateDeployment(ctx context.Context, req entity.CreateDeploymentRequest) (entity.CreateDeploymentResponse, error)
}

// DeploymentHandler handles HTTP requests for deployment operations.
type DeploymentHandler struct {
	entity entityDeploymentClient
}

// NewDeploymentHandler creates a DeploymentHandler backed by the given entity client.
func NewDeploymentHandler(entity entityDeploymentClient) *DeploymentHandler {
	return &DeploymentHandler{entity: entity}
}

// SearchDeployments handles POST /deployments/search.
func (h *DeploymentHandler) SearchDeployments(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.SearchDeploymentsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchDeployments(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployments failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search deployments.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchDeployments(result))
}

// CreateDeployment handles POST /deployments.
//
// NOTE: entity-service only supports this route on its ServiceNow data
// source — a Postgres-mode deployment returns 400 for every call, which
// mapUpstreamError surfaces as ErrMsgBadRequest.
func (h *DeploymentHandler) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.CreateDeploymentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateDeployment(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateDeployment failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create deployment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapDeploymentCreate(result))
}

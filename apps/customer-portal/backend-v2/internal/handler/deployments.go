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
	SearchDeployedProducts(ctx context.Context, req entity.SearchDeployedProductsRequest) (entity.SearchDeployedProductsResponse, error)
	CreateDeployment(ctx context.Context, req entity.CreateDeploymentRequest) (entity.CreateDeploymentResponse, error)
	UpdateDeployment(ctx context.Context, id string, req entity.UpdateDeploymentRequest) (entity.UpdateDeploymentResponse, error)
	UpdateAttachment(ctx context.Context, id string, req entity.UpdateAttachmentRequest) (entity.UpdateAttachmentResponse, error)
}

// DeploymentHandler handles HTTP requests for deployment operations.
type DeploymentHandler struct {
	entity entityDeploymentClient
}

// NewDeploymentHandler creates a DeploymentHandler backed by the given entity client.
func NewDeploymentHandler(entity entityDeploymentClient) *DeploymentHandler {
	return &DeploymentHandler{entity: entity}
}

// SearchDeployments handles POST /projects/{id}/deployments/search.
func (h *DeploymentHandler) SearchDeployments(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
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
	// ProjectIDs is always forced to the {id} path parameter, never the
	// client-supplied body: project scoping comes exclusively from the URL,
	// same reasoning as dto.BuildEntitySearchCasesRequest's projectID
	// parameter for POST /projects/{id}/cases/search.
	req.ProjectIDs = []string{projectID}

	result, err := h.entity.SearchDeployments(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployments failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search deployments.")
		return
	}

	resp := dto.MapSearchDeployments(result)
	productCounts := h.deployedProductCounts(r.Context(), user.UserID, result.Deployments)
	writeJSONValue(w, http.StatusOK, dto.WithDeploymentCounts(resp, productCounts, nil))
}

// deployedProductCountsPageLimit is the page size used when tallying deployed
// products. entity-service caps limit at 100, so this is the effective maximum.
const deployedProductCountsPageLimit = 100

// deployedProductCounts tallies deployed products per deployment for the
// deployments just returned by a search.
//
// One upstream call (paged) covers every deployment, because
// SearchDeployedProductsRequest accepts DeploymentIDs as a list and each
// DeployedProductView carries its own Deployment ref — so this is not an N+1
// over deployments.
//
// Best-effort by design: a failure here logs and returns nil, leaving the
// counts absent rather than failing the whole deployment search. The frontend
// treats an absent count as 0, so the worst case is the pre-existing behaviour,
// not a broken page. Same graceful-degradation stance as
// dto.BuildProjectDashboardStats.
func (h *DeploymentHandler) deployedProductCounts(ctx context.Context, userID string, deployments []entity.DeploymentView) map[string]int {
	if len(deployments) == 0 {
		return nil
	}
	ids := make([]string, 0, len(deployments))
	for _, d := range deployments {
		ids = append(ids, d.ID)
	}

	counts := make(map[string]int, len(ids))
	for offset := 0; ; {
		page, err := h.entity.SearchDeployedProducts(ctx, entity.SearchDeployedProductsRequest{
			DeploymentIDs: ids,
			Pagination:    entity.Pagination{Limit: deployedProductCountsPageLimit, Offset: offset},
		})
		if err != nil {
			slog.WarnContext(ctx, "entity SearchDeployedProducts failed while tallying deployment product counts",
				"userID", userID, "err", summarizeErr(err))
			return nil
		}
		for _, p := range page.DeployedProducts {
			counts[p.Deployment.ID]++
		}
		offset += len(page.DeployedProducts)
		if len(page.DeployedProducts) == 0 || offset >= page.Total {
			break
		}
	}
	return counts
}

// CreateDeployment handles POST /projects/{id}/deployments.
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

	projectID := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.DeploymentCreateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateDeployment(r.Context(), dto.BuildEntityCreateDeploymentRequest(projectID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateDeployment failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create deployment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapDeploymentCreate(result))
}

// PatchDeployment handles PATCH /projects/{projectId}/deployments/{id}.
// projectId is read from the URL only to match the frontend's nesting — the
// handler never uses it, since entity-service's UpdateDeployment is keyed on
// the deployment's own id alone (same pattern as
// PATCH /cases/{caseId}/call-requests/{id} — see this backend's CLAUDE.md).
func (h *DeploymentHandler) PatchDeployment(w http.ResponseWriter, r *http.Request) {
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

	var portalReq dto.DeploymentUpdateRequest
	if err := json.Unmarshal(body, &portalReq); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	req := dto.BuildEntityUpdateDeploymentRequest(id, portalReq)
	// entity-service requires exactly one of the detail-fields group
	// (name/type/description) or active=false — never both, never neither.
	detailFieldsSet := req.Name != nil || req.Type != nil || len(req.Description) > 0
	activeSet := req.Active != nil
	if detailFieldsSet == activeSet {
		writeError(w, http.StatusBadRequest, "Provide either name/type/description or active, but not both.")
		return
	}
	if activeSet && *req.Active {
		writeError(w, http.StatusBadRequest, "active can only be set to false.")
		return
	}

	result, err := h.entity.UpdateDeployment(r.Context(), id, req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateDeployment failed", "userID", user.UserID, "deploymentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update deployment.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapDeploymentUpdate(result))
}

// PatchDeploymentAttachment handles
// PATCH /deployments/{deploymentId}/attachments/{attachmentId}. referenceId/
// referenceType are injected server-side (deploymentId path param,
// ReferenceTypeDeployment) — the client only supplies name/description.
func (h *DeploymentHandler) PatchDeploymentAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	deploymentID := r.PathValue("deploymentId")
	attachmentID := r.PathValue("attachmentId")
	if !uuidRe.MatchString(deploymentID) || !uuidRe.MatchString(attachmentID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.AttachmentUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	entityReq := dto.BuildEntityUpdateAttachmentRequest(req, deploymentID, entity.ReferenceTypeDeployment)
	result, err := h.entity.UpdateAttachment(r.Context(), attachmentID, entityReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update the attachment.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapUpdatedAttachment(result))
}

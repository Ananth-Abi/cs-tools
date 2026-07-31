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

// entityCaseClient abstracts the entity-service case operations used by CaseHandler.
type entityCaseClient interface {
	SearchCases(ctx context.Context, req entity.SearchCasesRequest) (entity.SearchCasesResponse, error)
	GetCase(ctx context.Context, id string) (entity.CaseView, error)
	CreateCase(ctx context.Context, req entity.CreateCaseRequest) (entity.CreateCaseResponse, error)
	UpdateCase(ctx context.Context, id string, req entity.UpdateCaseRequest) (entity.UpdateCaseResponse, error)
	CreateCaseComment(ctx context.Context, caseID string, req entity.CreateCaseCommentRequest) (entity.CreateCaseCommentResponse, error)
}

// CaseHandler handles HTTP requests for case operations.
type CaseHandler struct {
	entity entityCaseClient
}

// NewCaseHandler creates a CaseHandler backed by the given entity client.
func NewCaseHandler(entity entityCaseClient) *CaseHandler {
	return &CaseHandler{entity: entity}
}

// SearchCases handles POST /cases/search.
func (h *CaseHandler) SearchCases(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.SearchCasesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchCases(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCases failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search cases.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchCases(result))
}

// GetCase handles GET /cases/{id}.
func (h *CaseHandler) GetCase(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetCase(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve case.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapCaseDetails(result))
}

// CreateCase handles POST /cases.
func (h *CaseHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.CreateCaseRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	// CreatedBy is server-set from the authenticated caller, never from the
	// request body (the struct's json:"-" tag means a client-supplied value
	// would be silently dropped anyway, but set it explicitly for clarity).
	req.CreatedBy = user.Email

	result, err := h.entity.CreateCase(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCase failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create case.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapCaseCreate(result))
}

// PatchCase handles PATCH /cases/{id}.
func (h *CaseHandler) PatchCase(w http.ResponseWriter, r *http.Request) {
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

	var req dto.UpdateCaseRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.State == nil && req.Severity == nil && req.Subject == nil && req.Description == nil && len(req.WatchList) == 0 {
		writeError(w, http.StatusBadRequest, "At least one field must be provided for update.")
		return
	}

	result, err := h.entity.UpdateCase(r.Context(), id, dto.BuildEntityUpdateCaseRequest(id, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateCase failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update case.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapCaseUpdate(result))
}

// CreateCaseComment handles POST /cases/{id}/comments.
func (h *CaseHandler) CreateCaseComment(w http.ResponseWriter, r *http.Request) {
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

	var req dto.CaseCommentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateCaseComment(r.Context(), id, dto.BuildEntityCreateCaseCommentRequest(id, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseComment failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to add comment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapCaseComment(result))
}

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
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

type entityCustomerPortalClient interface {
	GetMetadata(context.Context) ([]byte, error)
	GlobalSearch(context.Context, []byte) ([]byte, error)
	SearchInstances(context.Context, []byte) ([]byte, error)
	CreateCaseAttachment(context.Context, []byte) ([]byte, error)
	SearchCaseAttachments(context.Context, []byte) ([]byte, error)
	GetAttachment(context.Context, string) ([]byte, error)
	PatchAttachment(context.Context, string, []byte) ([]byte, error)
	GetCaseFeedback(context.Context, string) ([]byte, error)
	SubmitCaseFeedback(context.Context, string, []byte) ([]byte, error)
	SearchConversations(context.Context, []byte) ([]byte, error)
	GetConversation(context.Context, string) ([]byte, error)
	CreateConversation(context.Context, []byte) ([]byte, error)
	UpdateConversation(context.Context, string, []byte) ([]byte, error)
	GetProductVulnerabilityMetadata(context.Context) ([]byte, error)
	SearchCaseTimeCards(context.Context, []byte) ([]byte, error)
	SearchInstanceMetrics(context.Context, []byte) ([]byte, error)
	SearchInstanceUsage(context.Context, []byte) ([]byte, error)
	SearchInstanceMetricsStats(context.Context, []byte) ([]byte, error)
	SearchInstanceUsageStats(context.Context, []byte) ([]byte, error)
	CreateEscalation(context.Context, []byte) ([]byte, error)
	SearchEscalations(context.Context, []byte) ([]byte, error)
	SearchDeployedProductMetrics(context.Context, string, []byte) ([]byte, error)
	SearchDeployedProductUsageCounts(context.Context, string, []byte) ([]byte, error)
}

// CustomerPortalHandler exposes the customer-facing scoped API surface backed
// by the generic entity-service operations.
type CustomerPortalHandler struct{ entity entityCustomerPortalClient }

type portalValidationError struct{ message string }

func (e *portalValidationError) Error() string { return e.message }

func NewCustomerPortalHandler(entity entityCustomerPortalClient) *CustomerPortalHandler {
	return &CustomerPortalHandler{entity: entity}
}

func mappedBodyCall(call func(context.Context, []byte) ([]byte, error), mapper portalMapper) func(context.Context, []byte) ([]byte, error) {
	return func(ctx context.Context, body []byte) ([]byte, error) {
		result, err := call(ctx, body)
		return mapEntityCall(result, err, mapper)
	}
}

func mappedContextCall(ctx context.Context, call func(context.Context) ([]byte, error), mapper portalMapper) ([]byte, error) {
	result, err := call(ctx)
	return mapEntityCall(result, err, mapper)
}

func mappedIDCall(ctx context.Context, id string, call func(context.Context, string) ([]byte, error), mapper portalMapper) ([]byte, error) {
	result, err := call(ctx, id)
	return mapEntityCall(result, err, mapper)
}

func mappedIDBodyCall(ctx context.Context, id string, body []byte, call func(context.Context, string, []byte) ([]byte, error), mapper portalMapper) ([]byte, error) {
	result, err := call(ctx, id, body)
	return mapEntityCall(result, err, mapper)
}

func authenticated(r *http.Request, w http.ResponseWriter) bool {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return false
	}
	return true
}

func readPortalJSON(w http.ResponseWriter, r *http.Request, allowEmpty bool) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
		} else {
			writeError(w, http.StatusBadRequest, errMsgReadBody)
		}
		return nil, false
	}
	if len(strings.TrimSpace(string(body))) == 0 && allowEmpty {
		return []byte(`{}`), true
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	return body, true
}

func requirePortalUUID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := r.PathValue(name)
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return "", false
	}
	return id, true
}

func injectFilterID(body []byte, field, id string) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	filters, _ := payload["filters"].(map[string]any)
	if filters == nil {
		filters = make(map[string]any)
	}
	filters[field] = []string{id}
	payload["filters"] = filters
	return json.Marshal(payload)
}

func injectStringField(body []byte, field, value string) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	payload[field] = value
	return json.Marshal(payload)
}

func renameJSONField(body []byte, from, to string) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if value, exists := payload[from]; exists {
		payload[to] = value
		delete(payload, from)
	}
	return json.Marshal(payload)
}

func transformGlobalSearchRequest(body []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if filters, ok := payload["filters"].(map[string]any); ok {
		if types, exists := filters["types"]; exists {
			filters["tables"] = types
			delete(filters, "types")
		}
	}
	if pagination, ok := payload["projectsPagination"].(map[string]any); ok {
		if limit, ok := pagination["limit"].(float64); ok && limit > 50 {
			pagination["limit"] = float64(50)
		}
	}
	return json.Marshal(payload)
}

func transformConversationSearchRequest(body []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	filters, _ := payload["filters"].(map[string]any)
	if filters == nil {
		filters = make(map[string]any)
		payload["filters"] = filters
	}
	if keys, exists := filters["stateKeys"].([]any); exists {
		states := make([]string, 0, len(keys))
		for _, raw := range keys {
			key, ok := raw.(float64)
			if !ok {
				return nil, &portalValidationError{message: "stateKeys must contain integers"}
			}
			switch int(key) {
			case 2:
				states = append(states, "ACTIVE")
			case 3:
				states = append(states, "RESOLVED")
			default:
				return nil, &portalValidationError{message: "unsupported conversation state key"}
			}
		}
		filters["states"] = states
		delete(filters, "stateKeys")
	}
	return json.Marshal(payload)
}

func transformConversationUpdateRequest(body []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	status, _ := payload["status"].(string)
	switch status {
	case "closed":
		payload = map[string]any{"state": "CLOSED"}
	case "abandoned":
		payload = map[string]any{"state": "ABANDONED"}
	case "converted":
		payload = map[string]any{"state": "CONVERTED"}
	default:
		return nil, &portalValidationError{message: "status must be closed, abandoned, or converted"}
	}
	return json.Marshal(payload)
}

func (h *CustomerPortalHandler) forwardBody(w http.ResponseWriter, r *http.Request, status int, fallback string, allowEmpty bool, call func(context.Context, []byte) ([]byte, error)) {
	if !authenticated(r, w) {
		return
	}
	body, ok := readPortalJSON(w, r, allowEmpty)
	if !ok {
		return
	}
	result, err := call(r.Context(), body)
	if err != nil {
		var validationErr *portalValidationError
		if errors.As(err, &validationErr) {
			writeError(w, http.StatusBadRequest, validationErr.message)
			return
		}
		slog.ErrorContext(r.Context(), "customer portal entity request failed", "path", r.URL.Path, "err", err)
		mapUpstreamErrorGeneric(w, err, fallback)
		return
	}
	writeJSON(w, status, result)
}

func (h *CustomerPortalHandler) forwardPatch(w http.ResponseWriter, r *http.Request, fallback string, call func(context.Context, []byte) ([]byte, error)) {
	if !authenticated(r, w) {
		return
	}
	body, ok := readPortalJSON(w, r, false)
	if !ok {
		return
	}
	result, err := call(r.Context(), body)
	if err != nil {
		var validationErr *portalValidationError
		if errors.As(err, &validationErr) {
			writeError(w, http.StatusBadRequest, validationErr.message)
			return
		}
		slog.ErrorContext(r.Context(), "customer portal entity update failed", "path", r.URL.Path, "err", err)
		mapUpstreamError(w, err, fallback)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CustomerPortalHandler) forwardScoped(w http.ResponseWriter, r *http.Request, idName, filterField, fallback string, call func(context.Context, []byte) ([]byte, error)) {
	if !authenticated(r, w) {
		return
	}
	id, ok := requirePortalUUID(w, r, idName)
	if !ok {
		return
	}
	body, ok := readPortalJSON(w, r, false)
	if !ok {
		return
	}
	body, err := injectFilterID(body, filterField, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	result, err := call(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "scoped customer portal entity request failed", "path", r.URL.Path, "err", err)
		mapUpstreamErrorGeneric(w, err, fallback)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CustomerPortalHandler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r, w) {
		return
	}
	result, err := mappedContextCall(r.Context(), h.entity.GetMetadata, mapMetadataResponse)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve metadata information.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CustomerPortalHandler) GlobalSearch(w http.ResponseWriter, r *http.Request) {
	h.forwardBody(w, r, http.StatusOK, "Failed to perform global search.", true, func(ctx context.Context, body []byte) ([]byte, error) {
		body, err := transformGlobalSearchRequest(body)
		if err != nil {
			return nil, err
		}
		return mappedBodyCall(h.entity.GlobalSearch, mapGlobalSearchResponse)(ctx, body)
	})
}

func (h *CustomerPortalHandler) SearchProjectInstances(w http.ResponseWriter, r *http.Request) {
	h.forwardScoped(w, r, "id", "projectIds", "Failed to search instances for the project.", mappedBodyCall(h.entity.SearchInstances, mapInstancesResponse))
}
func (h *CustomerPortalHandler) SearchDeploymentInstances(w http.ResponseWriter, r *http.Request) {
	h.forwardScoped(w, r, "id", "deploymentIds", "Failed to search instances for the deployment.", mappedBodyCall(h.entity.SearchInstances, mapInstancesResponse))
}
func (h *CustomerPortalHandler) SearchDeployedProductInstances(w http.ResponseWriter, r *http.Request) {
	h.forwardScoped(w, r, "id", "deployedProductIds", "Failed to search instances for the deployed product.", mappedBodyCall(h.entity.SearchInstances, mapInstancesResponse))
}

func (h *CustomerPortalHandler) SearchProjectConversations(w http.ResponseWriter, r *http.Request) {
	h.forwardScoped(w, r, "id", "projectIds", "Failed to search conversations for the project.", func(ctx context.Context, body []byte) ([]byte, error) {
		body, err := transformConversationSearchRequest(body)
		if err != nil {
			return nil, err
		}
		return mappedBodyCall(h.entity.SearchConversations, mapConversationSearchResponse)(ctx, body)
	})
}
func (h *CustomerPortalHandler) CreateProjectConversation(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r, w) {
		return
	}
	id, ok := requirePortalUUID(w, r, "id")
	if !ok {
		return
	}
	body, ok := readPortalJSON(w, r, false)
	if !ok {
		return
	}
	body, err := renameJSONField(body, "message", "initialMessage")
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	body, err = injectStringField(body, "projectId", id)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	result, err := h.entity.CreateConversation(r.Context(), body)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to create conversation.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *CustomerPortalHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r, w) {
		return
	}
	id, ok := requirePortalUUID(w, r, "id")
	if !ok {
		return
	}
	result, err := mappedIDCall(r.Context(), id, h.entity.GetConversation, mapConversationResponse)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve conversation.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *CustomerPortalHandler) UpdateConversation(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r, w) {
		return
	}
	id, ok := requirePortalUUID(w, r, "id")
	if !ok {
		return
	}
	h.forwardPatch(w, r, "Failed to update conversation.", func(ctx context.Context, body []byte) ([]byte, error) {
		body, err := transformConversationUpdateRequest(body)
		if err != nil {
			return nil, err
		}
		return h.entity.UpdateConversation(ctx, id, body)
	})
}

func (h *CustomerPortalHandler) GetCaseFeedback(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r, w) {
		return
	}
	id, ok := requirePortalUUID(w, r, "id")
	if !ok {
		return
	}
	result, err := mappedIDCall(r.Context(), id, h.entity.GetCaseFeedback, mapFeedbackResponse)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve case feedback.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *CustomerPortalHandler) SubmitCaseFeedback(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r, w) {
		return
	}
	id, ok := requirePortalUUID(w, r, "id")
	if !ok {
		return
	}
	h.forwardBody(w, r, http.StatusCreated, "Failed to submit case feedback.", false, func(ctx context.Context, body []byte) ([]byte, error) {
		return mappedIDBodyCall(ctx, id, body, h.entity.SubmitCaseFeedback, mapSubmittedFeedbackResponse)
	})
}

func (h *CustomerPortalHandler) searchAttachments(w http.ResponseWriter, r *http.Request, referenceType string) {
	if !authenticated(r, w) {
		return
	}
	id, ok := requirePortalUUID(w, r, "id")
	if !ok {
		return
	}
	pagination := map[string]int{}
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			writeError(w, http.StatusBadRequest, "limit must be an integer between 1 and 100")
			return
		}
		pagination["limit"] = limit
	}
	if value := r.URL.Query().Get("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil || offset < 0 {
			writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		pagination["offset"] = offset
	}
	body, _ := json.Marshal(map[string]any{"referenceId": id, "referenceType": referenceType, "pagination": pagination})
	result, err := mappedBodyCall(h.entity.SearchCaseAttachments, mapAttachmentsResponse)(r.Context(), body)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve attachments.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *CustomerPortalHandler) GetCaseAttachments(w http.ResponseWriter, r *http.Request) {
	h.searchAttachments(w, r, "case")
}
func (h *CustomerPortalHandler) GetDeploymentAttachments(w http.ResponseWriter, r *http.Request) {
	h.searchAttachments(w, r, "deployment")
}

func (h *CustomerPortalHandler) createAttachment(w http.ResponseWriter, r *http.Request, referenceType string) {
	if !authenticated(r, w) {
		return
	}
	id, ok := requirePortalUUID(w, r, "id")
	if !ok {
		return
	}
	body, ok := readPortalJSON(w, r, false)
	if !ok {
		return
	}
	body, err := renameJSONField(body, "content", "file")
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	body, err = injectReferenceFields(body, id, referenceType)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	result, err := mappedBodyCall(h.entity.CreateCaseAttachment, mapCreatedAttachmentResponse)(r.Context(), body)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to create a new attachment.")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
func (h *CustomerPortalHandler) CreateCaseAttachment(w http.ResponseWriter, r *http.Request) {
	h.createAttachment(w, r, "case")
}
func (h *CustomerPortalHandler) CreateDeploymentAttachment(w http.ResponseWriter, r *http.Request) {
	h.createAttachment(w, r, "deployment")
}

func (h *CustomerPortalHandler) patchAttachment(w http.ResponseWriter, r *http.Request, parentName, referenceType string) {
	if !authenticated(r, w) {
		return
	}
	parentID, ok := requirePortalUUID(w, r, parentName)
	if !ok {
		return
	}
	attachmentID, ok := requirePortalUUID(w, r, "attachmentId")
	if !ok {
		return
	}
	h.forwardPatch(w, r, "Failed to update the attachment.", func(ctx context.Context, body []byte) ([]byte, error) {
		body, err := injectReferenceFields(body, parentID, referenceType)
		if err != nil {
			return nil, err
		}
		return mappedIDBodyCall(ctx, attachmentID, body, h.entity.PatchAttachment, mapUpdatedAttachmentResponse)
	})
}
func (h *CustomerPortalHandler) PatchCaseAttachment(w http.ResponseWriter, r *http.Request) {
	h.patchAttachment(w, r, "caseId", "case")
}
func (h *CustomerPortalHandler) PatchDeploymentAttachment(w http.ResponseWriter, r *http.Request) {
	h.patchAttachment(w, r, "deploymentId", "deployment")
}
func (h *CustomerPortalHandler) GetAttachment(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r, w) {
		return
	}
	id, ok := requirePortalUUID(w, r, "id")
	if !ok {
		return
	}
	result, err := mappedIDCall(r.Context(), id, h.entity.GetAttachment, mapAttachmentResponse)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve attachment.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CustomerPortalHandler) GetProductVulnerabilityMetadata(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r, w) {
		return
	}
	result, err := mappedContextCall(r.Context(), h.entity.GetProductVulnerabilityMetadata, mapVulnerabilityMetadataResponse)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve product vulnerability metadata.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CustomerPortalHandler) SearchProjectCaseTimeCards(w http.ResponseWriter, r *http.Request) {
	h.forwardScoped(w, r, "id", "projectIds", "Failed to search time cards grouped by cases.", mappedBodyCall(h.entity.SearchCaseTimeCards, mapCaseTimeCardsResponse))
}

func (h *CustomerPortalHandler) scopedAnalytics(w http.ResponseWriter, r *http.Request, field, fallback string, call func(context.Context, []byte) ([]byte, error)) {
	h.forwardScoped(w, r, "id", field, fallback, call)
}
func (h *CustomerPortalHandler) SearchProjectInstanceMetrics(w http.ResponseWriter, r *http.Request) {
	h.scopedAnalytics(w, r, "projectIds", "Failed to search instance metrics for the project.", mappedBodyCall(h.entity.SearchInstanceMetrics, mapInstanceMetricsResponse))
}
func (h *CustomerPortalHandler) SearchDeploymentInstanceMetrics(w http.ResponseWriter, r *http.Request) {
	h.scopedAnalytics(w, r, "deploymentIds", "Failed to search instance metrics for the deployment.", mappedBodyCall(h.entity.SearchInstanceMetrics, mapInstanceMetricsResponse))
}
func (h *CustomerPortalHandler) SearchDeployedProductInstanceMetrics(w http.ResponseWriter, r *http.Request) {
	h.scopedAnalytics(w, r, "deployedProductIds", "Failed to search instance metrics for the deployed product.", mappedBodyCall(h.entity.SearchInstanceMetrics, mapInstanceMetricsResponse))
}
func (h *CustomerPortalHandler) SearchProjectInstanceUsage(w http.ResponseWriter, r *http.Request) {
	h.scopedAnalytics(w, r, "projectIds", "Failed to search instance usage for the project.", mappedBodyCall(h.entity.SearchInstanceUsage, mapInstanceUsageResponse))
}
func (h *CustomerPortalHandler) SearchDeploymentInstanceUsage(w http.ResponseWriter, r *http.Request) {
	h.scopedAnalytics(w, r, "deploymentIds", "Failed to search instance usage for the deployment.", mappedBodyCall(h.entity.SearchInstanceUsage, mapInstanceUsageResponse))
}
func (h *CustomerPortalHandler) SearchDeployedProductInstanceUsage(w http.ResponseWriter, r *http.Request) {
	h.scopedAnalytics(w, r, "deployedProductIds", "Failed to search instance usage for the deployed product.", mappedBodyCall(h.entity.SearchInstanceUsage, mapInstanceUsageResponse))
}
func (h *CustomerPortalHandler) SearchProjectInstanceMetricsStats(w http.ResponseWriter, r *http.Request) {
	h.scopedAnalytics(w, r, "projectIds", "Failed to search instance metric stats for the project.", mappedBodyCall(h.entity.SearchInstanceMetricsStats, mapStatsResponse))
}
func (h *CustomerPortalHandler) SearchDeploymentInstanceMetricsStats(w http.ResponseWriter, r *http.Request) {
	h.scopedAnalytics(w, r, "deploymentIds", "Failed to search instance metric stats for the deployment.", mappedBodyCall(h.entity.SearchInstanceMetricsStats, mapStatsResponse))
}
func (h *CustomerPortalHandler) SearchDeployedProductInstanceMetricsStats(w http.ResponseWriter, r *http.Request) {
	h.scopedAnalytics(w, r, "deployedProductIds", "Failed to search instance metric stats for the deployed product.", mappedBodyCall(h.entity.SearchInstanceMetricsStats, mapStatsResponse))
}
func (h *CustomerPortalHandler) SearchProjectInstanceUsageStats(w http.ResponseWriter, r *http.Request) {
	h.scopedAnalytics(w, r, "projectIds", "Failed to search instance usage stats for the project.", mappedBodyCall(h.entity.SearchInstanceUsageStats, mapStatsResponse))
}
func (h *CustomerPortalHandler) SearchDeploymentInstanceUsageStats(w http.ResponseWriter, r *http.Request) {
	h.scopedAnalytics(w, r, "deploymentIds", "Failed to search instance usage stats for the deployment.", mappedBodyCall(h.entity.SearchInstanceUsageStats, mapStatsResponse))
}
func (h *CustomerPortalHandler) SearchDeployedProductInstanceUsageStats(w http.ResponseWriter, r *http.Request) {
	h.scopedAnalytics(w, r, "deployedProductIds", "Failed to search instance usage stats for the deployed product.", mappedBodyCall(h.entity.SearchInstanceUsageStats, mapStatsResponse))
}

func (h *CustomerPortalHandler) CreateCaseEscalation(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r, w) {
		return
	}
	caseID, ok := requirePortalUUID(w, r, "caseId")
	if !ok {
		return
	}
	body, ok := readPortalJSON(w, r, false)
	if !ok {
		return
	}
	var request struct {
		Action *string `json:"action"`
		Reason *string `json:"reason"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	action := "ESCALATE"
	if request.Action != nil {
		action = strings.ToUpper(*request.Action)
	}
	if action != "ESCALATE" && action != "DEESCALATE" {
		writeError(w, http.StatusBadRequest, "action must be ESCALATE or DEESCALATE")
		return
	}
	if action == "ESCALATE" && (request.Reason == nil || strings.TrimSpace(*request.Reason) == "") {
		writeError(w, http.StatusBadRequest, "reason is required when escalating a case")
		return
	}
	body, err := injectStringField(body, "action", action)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	body, err = injectStringField(body, "caseId", caseID)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	result, err := mappedBodyCall(h.entity.CreateEscalation, mapCreatedEscalationResponse)(r.Context(), body)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to create escalation.")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
func (h *CustomerPortalHandler) SearchCaseEscalations(w http.ResponseWriter, r *http.Request) {
	h.forwardScoped(w, r, "caseId", "caseIds", "Failed to search escalations.", mappedBodyCall(h.entity.SearchEscalations, mapEscalationsResponse))
}

func (h *CustomerPortalHandler) deployedProductMetrics(w http.ResponseWriter, r *http.Request, usage bool) {
	if !authenticated(r, w) {
		return
	}
	deploymentID, ok := requirePortalUUID(w, r, "deploymentId")
	if !ok {
		return
	}
	productID, ok := requirePortalUUID(w, r, "productId")
	if !ok {
		return
	}
	body, ok := readPortalJSON(w, r, false)
	if !ok {
		return
	}
	body, err := injectStringField(body, "deploymentId", deploymentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	var result []byte
	if usage {
		result, err = mappedIDBodyCall(r.Context(), productID, body, h.entity.SearchDeployedProductUsageCounts, mapDeployedProductMetricsResponse)
	} else {
		result, err = mappedIDBodyCall(r.Context(), productID, body, h.entity.SearchDeployedProductMetrics, mapDeployedProductMetricsResponse)
	}
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve metrics for the deployed product.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *CustomerPortalHandler) SearchDeployedProductMetrics(w http.ResponseWriter, r *http.Request) {
	h.deployedProductMetrics(w, r, false)
}
func (h *CustomerPortalHandler) SearchDeployedProductUsageCounts(w http.ResponseWriter, r *http.Request) {
	h.deployedProductMetrics(w, r, true)
}

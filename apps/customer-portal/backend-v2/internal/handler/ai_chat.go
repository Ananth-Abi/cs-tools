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
	"strconv"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/aichatagent"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// aiChatAgentClient abstracts the upstream AI chat agent operations used by AIChatHandler.
type aiChatAgentClient interface {
	CreateCaseClassification(ctx context.Context, req aichatagent.CaseClassificationPayload) (aichatagent.CaseClassificationResponse, error)
	CreateChat(ctx context.Context, req aichatagent.ChatPayload) (aichatagent.ChatResponse, error)
	GetRecommendations(ctx context.Context, req aichatagent.RecommendationRequest) (aichatagent.RecommendationResponse, error)
	GetConversationSummary(ctx context.Context, projectID, conversationID string) (aichatagent.ConversationSummaryResponse, error)
}

// entityConversationClient abstracts the entity-service conversation and
// comment operations used by AIChatHandler.
type entityConversationClient interface {
	SearchConversations(ctx context.Context, req entity.SearchConversationsRequest) (entity.SearchConversationsResponse, error)
	SearchComments(ctx context.Context, req entity.SearchCommentsRequest) (entity.SearchCommentsResponse, error)
	CreateComment(ctx context.Context, req entity.CreateCommentRequest) (entity.CreateCommentResponse, error)
}

// AIChatHandler handles HTTP requests for the AI chat feature: case
// classification, KB article recommendations, conversation search/messages,
// and conversation summaries. See WebSocketHandler for the real-time chat
// proxy, and its doc comment for the entity-service gaps (no
// createConversation/updateConversation, no comment createdBy override) that
// this handler works around or skips the same way.
type AIChatHandler struct {
	ai     aiChatAgentClient
	entity entityConversationClient
}

// NewAIChatHandler creates an AIChatHandler backed by the given AI chat agent and entity clients.
func NewAIChatHandler(ai aiChatAgentClient, entityClient entityConversationClient) *AIChatHandler {
	return &AIChatHandler{ai: ai, entity: entityClient}
}

// ClassifyCase handles POST /cases/classify.
func (h *AIChatHandler) ClassifyCase(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.CaseClassificationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.ai.CreateCaseClassification(r.Context(), dto.BuildCaseClassificationPayload(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "aichatagent CreateCaseClassification failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to classify chat message.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapCaseClassification(result))
}

// SearchRecommendations handles POST /conversations/recommendations/search.
func (h *AIChatHandler) SearchRecommendations(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.RecommendationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.ai.GetRecommendations(r.Context(), dto.BuildRecommendationRequest(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "aichatagent GetRecommendations failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve recommendations.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapRecommendationResponse(result))
}

// SearchConversations handles POST /projects/{id}/conversations/search.
func (h *AIChatHandler) SearchConversations(w http.ResponseWriter, r *http.Request) {
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

	var req entity.SearchConversationsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	req.Filters.ProjectIDs = []string{projectID}

	result, err := h.entity.SearchConversations(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchConversations failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve conversations.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchConversations(result))
}

// GetConversationMessages handles GET /conversations/{id}/messages.
// Backed by the generic comment search (referenceType=conversation), the
// same route the portal uses for POST /comments/search.
func (h *AIChatHandler) GetConversationMessages(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	conversationID := r.PathValue("id")
	if conversationID == "" || !uuidRe.MatchString(conversationID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	limit, offset, ok := parseLimitOffset(w, r)
	if !ok {
		return
	}

	req := dto.BuildEntitySearchCommentsRequest(dto.CommentSearchRequest{
		ReferenceID:   conversationID,
		ReferenceType: string(entity.ReferenceTypeConversation),
		Pagination:    entity.Pagination{Limit: limit, Offset: offset},
	})

	result, err := h.entity.SearchComments(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchComments failed", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve conversation messages.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchComments(result))
}

// SendConversationMessage handles
// POST /projects/{projectId}/conversations/{conversationId}/messages — a
// follow-up message on an existing conversation. See WebSocketHandler's doc
// comment for why the AI agent's reply is not saved as a comment here and
// the conversation's resolved state is not updated.
func (h *AIChatHandler) SendConversationMessage(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("projectId")
	conversationID := r.PathValue("conversationId")
	if !uuidRe.MatchString(projectID) || !uuidRe.MatchString(conversationID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.ChatMessageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	chatResp, err := h.ai.CreateChat(r.Context(), aichatagent.ChatPayload{
		Message:        req.Message,
		AccountID:      projectID,
		ConversationID: conversationID,
		EnvProducts:    req.EnvProducts,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "aichatagent CreateChat failed", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to process conversation message.")
		return
	}

	if _, err := h.entity.CreateComment(r.Context(), entity.CreateCommentRequest{
		ReferenceID:   conversationID,
		ReferenceType: entity.ReferenceTypeConversation,
		Type:          entity.CommentTypeComment,
		Content:       req.Message,
	}); err != nil {
		slog.ErrorContext(r.Context(), "entity CreateComment failed for conversation message", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
	}

	writeJSONValue(w, http.StatusOK, dto.MapChatResponse(chatResp))
}

// GetConversationSummary handles GET /projects/{id}/conversations/{conversationId}/summary.
func (h *AIChatHandler) GetConversationSummary(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	conversationID := r.PathValue("conversationId")
	if !uuidRe.MatchString(projectID) || !uuidRe.MatchString(conversationID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.ai.GetConversationSummary(r.Context(), projectID, conversationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "aichatagent GetConversationSummary failed", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve conversation summary.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapConversationSummary(result))
}

// parseLimitOffset parses the limit/offset query parameters, defaulting to
// 20/0. Writes a 400 response and returns ok=false on an invalid value.
// maxConversationMessagesLimit caps GetConversationMessages' limit query
// param, matching this backend's other search endpoints' page-size ceiling.
const maxConversationMessagesLimit = 100

func parseLimitOffset(w http.ResponseWriter, r *http.Request) (limit, offset int, ok bool) {
	limit, offset = 20, 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > maxConversationMessagesLimit {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return 0, 0, false
		}
		limit = v
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return 0, 0, false
		}
		offset = v
	}
	return limit, offset, true
}

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
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/aichatagent"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// wsStreamer abstracts the AI chat agent's WebSocket proxy operation used by
// WebSocketHandler.
type wsStreamer interface {
	StreamChat(ctx context.Context, sessionID, payload string, caller aichatagent.BrowserConn) (map[string]json.RawMessage, error)
}

// wsMaxMessageBytes bounds the size of a single WebSocket frame this handler
// will read, on both the browser connection (here) and the upstream AI agent
// connection (internal/aichatagent/ws.go) — protects against a peer forcing
// a large allocation via an oversized frame.
const wsMaxMessageBytes = 64 << 10 // 64 KiB

// wsIdleTimeout bounds how long this handler waits for the next frame from
// an idle peer before closing the connection.
const wsIdleTimeout = 5 * time.Minute

// entityCommentCreator is the subset of entityCommentClient needed to persist
// a conversation message as a comment.
type entityCommentCreator interface {
	CreateComment(ctx context.Context, req entity.CreateCommentRequest) (entity.CreateCommentResponse, error)
}

// WebSocketHandler proxies real-time chat messages between the browser and
// the upstream AI chat agent for an existing conversation.
//
// NOTE: entity-service has no createConversation, so unlike the Ballerina
// backend this is rewriting, a WebSocket message that doesn't carry an
// existing conversationId cannot start a brand-new conversation here — the
// caller must first create one via a future entity-service-backed endpoint.
// TODO(entity-service): once entity-service gains createConversation, wire
// the same "no conversationId → create one" branch the Ballerina backend has.
// TODO(entity-service): the AI agent's own reply is not persisted as a
// comment here (unlike Ballerina, which tags it with a special "chat agent"
// createdBy) because entity-service's CreateCommentRequest always attributes
// the comment to the caller's own identity — there is no createdBy override.
// TODO(entity-service): marking the conversation "resolved" when the AI
// agent reports resolved=true is skipped — entity-service has no
// updateConversation yet.
type WebSocketHandler struct {
	ai      wsStreamer
	entity  entityCommentCreator
	upgrade websocket.Upgrader
}

// NewWebSocketHandler creates a WebSocketHandler backed by the given AI chat
// agent WebSocket client and entity client. allowedOrigins restricts which
// browser Origins may open this connection (defense in depth against
// cross-site WebSocket hijacking) — pass nil/empty to allow any origin,
// e.g. for local development.
func NewWebSocketHandler(ai wsStreamer, entityClient entityCommentCreator, allowedOrigins []string) *WebSocketHandler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	return &WebSocketHandler{
		ai:     ai,
		entity: entityClient,
		upgrade: websocket.Upgrader{
			// Primary authorization is still the same JWT middleware chain as
			// every other route (see cmd/server/main.go); this Origin check is
			// defense in depth against cross-site WebSocket hijacking. A
			// non-browser caller (e.g. a server-to-server client) sends no
			// Origin header at all and is allowed through either way.
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == "" || len(allowed) == 0 || allowed[origin]
			},
		},
	}
}

// wsEvent is the JSON envelope used for events this handler sends directly
// to the browser (ping/pong, errors) — matches the upstream AI chat agent's
// own event shape so the frontend handles both uniformly.
type wsEvent struct {
	Type           string `json:"type"`
	Message        string `json:"message,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	TS             string `json:"ts,omitempty"`
}

// HandleWebSocket handles GET /ws?sessionId={projectId}. The query parameter
// is named sessionId for wire compatibility with the Ballerina backend this
// is rewriting, but it actually carries the project ID — the AI agent's own
// per-conversation session key is derived below as "{projectId}:{conversationId}".
func (h *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.URL.Query().Get("sessionId")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	conn, err := h.upgrade.Upgrade(w, r, nil)
	if err != nil {
		slog.ErrorContext(r.Context(), "websocket upgrade failed", "userID", user.UserID, "err", summarizeErr(err))
		return
	}
	defer conn.Close()

	// The server's ReadTimeout/WriteTimeout (see cmd/server/main.go) can leave
	// deadlines on the connection Hijack handed off for this upgrade; clear
	// them so they don't kill an otherwise-idle-but-healthy chat session, and
	// rely on wsIdleTimeout below instead.
	underlying := conn.UnderlyingConn()
	_ = underlying.SetReadDeadline(time.Time{})
	_ = underlying.SetWriteDeadline(time.Time{})

	conn.SetReadLimit(wsMaxMessageBytes)

	for {
		if err := conn.SetReadDeadline(time.Now().Add(wsIdleTimeout)); err != nil {
			slog.WarnContext(r.Context(), "websocket set read deadline failed", "userID", user.UserID, "err", summarizeErr(err))
			return
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				slog.WarnContext(r.Context(), "websocket read error", "userID", user.UserID, "err", summarizeErr(err))
			}
			return
		}
		h.handleMessage(r.Context(), conn, user, projectID, data)
	}
}

func (h *WebSocketHandler) handleMessage(ctx context.Context, conn *websocket.Conn, user *middleware.UserInfo, projectID string, data []byte) {
	trimmed := strings.TrimSpace(strings.ToLower(string(data)))
	var parsed map[string]any
	_ = json.Unmarshal(data, &parsed)

	isPing := trimmed == "ping"
	if !isPing {
		if t, _ := parsed["type"].(string); t == "ping" {
			isPing = true
		}
	}
	if isPing {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		_ = writeWSJSON(conn, wsEvent{Type: "pong", TS: ts})
		return
	}

	conversationID, _ := parsed["conversationId"].(string)
	if conversationID == "" || !uuidRe.MatchString(conversationID) {
		_ = writeWSJSON(conn, wsEvent{
			Type: "error",
			Message: "Starting a new conversation over this connection isn't supported yet — " +
				"include the conversationId of an existing conversation to resume it.",
		})
		return
	}

	userMessage, _ := parsed["message"].(string)
	// Forward only the fields the upstream contract defines — never the raw
	// client-supplied map verbatim, which could otherwise let a client smuggle
	// extra keys (e.g. its own "accountId"/"sessionId") the agent might trust.
	upstreamPayload := map[string]any{
		"message":        userMessage,
		"conversationId": conversationID,
	}
	if envProducts, ok := parsed["envProducts"]; ok {
		upstreamPayload["envProducts"] = envProducts
	}
	enriched, err := json.Marshal(upstreamPayload)
	if err != nil {
		_ = writeWSJSON(conn, wsEvent{Type: "error", Message: "Failed to process message."})
		return
	}

	agentSessionID := projectID + ":" + conversationID
	result, err := h.ai.StreamChat(ctx, agentSessionID, string(enriched), conn)
	if err != nil {
		slog.ErrorContext(ctx, "aichatagent StreamChat failed", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		_ = writeWSJSON(conn, wsEvent{Type: "error", Message: "Failed to process message."})
		return
	}

	if userMessage != "" {
		_, err := h.entity.CreateComment(ctx, entity.CreateCommentRequest{
			ReferenceID:   conversationID,
			ReferenceType: entity.ReferenceTypeConversation,
			Type:          entity.CommentTypeComment,
			Content:       userMessage,
		})
		if err != nil {
			slog.ErrorContext(ctx, "entity CreateComment failed for conversation message", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		}
	}

	_ = result // the AI agent's reply is not persisted as a comment — see WebSocketHandler's doc comment.
}

func writeWSJSON(conn *websocket.Conn, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, payload)
}

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
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// caseActivityStreamHeartbeat is how often StreamCaseActivities writes a
// comment-only SSE ping to keep the connection alive through intermediate
// proxies that would otherwise time out an idle response.
const caseActivityStreamHeartbeat = 15 * time.Second

// StreamCaseActivities handles GET /cases/{id}/activities/stream: a
// long-lived Server-Sent Events connection that emits a `case_updated` event
// whenever internal/caseevents.Handler observes a case.comment_added or
// case.status_changed record for this case on any backend replica (see
// internal/stream.BroadcastHub). It is registered on the dedicated :9092
// listener (see cmd/server/main.go) so the main :8080 listener's
// WriteTimeout/IdleTimeout can't kill the connection, but it sits behind the
// same middleware.Auth chain as every other endpoint — there is no separate
// auth mechanism for streaming, unlike the ticket-based design this
// superseded; the browser connects with its normal x-jwt-assertion/
// x-user-id-token headers via a fetch-backed EventSource polyfill (native
// EventSource cannot set custom headers).
//
// The broadcast payload is a minimal {caseId, type, timestamp} — never
// comment text or field values (see events.CommentAddedPayload/
// StatusChangedPayload) — so a caller subscribed to a case it would not
// otherwise be authorized to read learns only that *something* changed, not
// what. The browser's subsequent refetch through the existing REST endpoints
// is what's actually authorized per-user. This mirrors an existing tradeoff,
// not a new one: internal/caseevents.Handler already receives every case
// event system-wide with no per-user filtering.
func (h *CaseHandler) StreamCaseActivities(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" || !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	if h.hub == nil {
		writeError(w, http.StatusServiceUnavailable, "Live updates are not available right now.")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Nginx/Choreo-gateway hint to disable response buffering for this
	// endpoint; harmless (ignored) on stacks that don't recognise it.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := h.hub.Register(caseID)
	defer h.hub.Unregister(caseID, ch)

	ctx := r.Context()
	ticker := time.NewTicker(caseActivityStreamHeartbeat)
	defer ticker.Stop()

	slog.InfoContext(ctx, "case activity stream connected", "userID", user.UserID, "caseID", caseID)

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "case activity stream disconnected", "userID", user.UserID, "caseID", caseID)
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case payload, ok := <-ch:
			if !ok {
				return
			}
			// payload is always compact, single-line JSON built by
			// internal/caseevents.Handler (see BroadcastHub.Publish's
			// caller) — safe to write as one `data:` line, since
			// json.Marshal escapes any literal newline in a string value
			// rather than emitting one.
			if _, err := fmt.Fprintf(w, "event: case_updated\ndata: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

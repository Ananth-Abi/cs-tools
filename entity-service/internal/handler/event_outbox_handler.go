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

// Package handler is declared in user_handler.go.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// EventOutboxHandler handles HTTP requests for the event_outbox resource —
// see domain.EventOutbox's doc comment for what it's for.
type EventOutboxHandler struct {
	svc service.EventOutboxService
}

// NewEventOutboxHandler constructs an EventOutboxHandler with the given service.
func NewEventOutboxHandler(svc service.EventOutboxService) *EventOutboxHandler {
	return &EventOutboxHandler{svc: svc}
}

// CreateEventOutbox handles POST /event-outbox.
func (h *EventOutboxHandler) CreateEventOutbox(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateEventOutboxRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.CreateEventOutbox(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchEventOutbox handles POST /event-outbox/search.
func (h *EventOutboxHandler) SearchEventOutbox(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchEventOutboxRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchEventOutbox(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// UpdateEventOutboxStatus handles PATCH /event-outbox/{id} — the single
// endpoint behind all three legal transitions (claim, mark dispatched,
// release a failed claim). See domain.UpdateEventOutboxStatusRequest's doc
// comment for the request shape and which status values are legal, and
// service.EventOutboxService.UpdateEventOutboxStatus for the guard each
// transition enforces.
func (h *EventOutboxHandler) UpdateEventOutboxStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req domain.UpdateEventOutboxStatusRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.UpdateEventOutboxStatus(r.Context(), id, req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

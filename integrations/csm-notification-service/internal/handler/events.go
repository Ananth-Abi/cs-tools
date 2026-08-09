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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// eventPublisher abstracts eventbus.Producer for testability.
type eventPublisher interface {
	Publish(ctx context.Context, key, value []byte) error
}

// EventsHandler handles HTTP requests that submit a domain event to be
// published to the event bus.
type EventsHandler struct {
	publisher eventPublisher
}

// NewEventsHandler creates an EventsHandler backed by the given publisher.
func NewEventsHandler(publisher eventPublisher) *EventsHandler {
	return &EventsHandler{publisher: publisher}
}

// PostEvent handles POST /events — the entry point csm-portal-backend and
// customer-portal-backend call when something notification-worthy happens
// (a case is created, a comment is added, ...). It validates the event and
// publishes it to the event bus for this service's own consumer to react to
// asynchronously; it does not send anything itself.
func (h *EventsHandler) PostEvent(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	var env events.Envelope
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&env); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	// Decode only consumes the first JSON value; reject trailing values or
	// malformed trailing bytes rather than silently ignoring them.
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	env.EntityID = strings.TrimSpace(env.EntityID)
	if env.EntityID == "" || !env.Type.IsKnown() {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if err := validateEventPayload(env.Type, env.Payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// Publish the original request body as received (already validated
	// above) — the consumer decodes it the same way. Keyed by EntityID so
	// every event about the same case/incident lands on the same partition
	// and is processed in the order it was published.
	if err := h.publisher.Publish(r.Context(), []byte(env.EntityID), body); err != nil {
		slog.ErrorContext(r.Context(), "failed to publish event", "type", env.Type, "entityId", env.EntityID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	writeJSONValue(w, http.StatusAccepted, map[string]string{"message": "event accepted"})
}

// validateEventPayload decodes raw as t's matching payload type (rejecting
// unknown fields) and checks its required fields are non-empty. This is
// deliberately duplicated per type rather than done via reflection — each
// type's required fields are exactly the ones its Render* function in
// internal/notifications needs.
func validateEventPayload(t events.Type, raw json.RawMessage) error {
	switch t {
	case events.TypeCaseCreated:
		var p events.CaseCreatedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.ReporterName == "" || p.ProjectName == "" || p.CaseID == "" || p.CaseTitle == "" ||
			p.CaseType == "" || p.Priority == "" || p.CreatedAt == "" || p.Description == "" ||
			p.CaseLink == "" || p.CommentLink == "" || len(p.Recipients) == 0 {
			return fmt.Errorf("handler: missing required field for %s", t)
		}
	case events.TypeCommentAdded:
		var p events.CommentAddedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.Name == "" || p.ProjectID == "" || p.CaseTitle == "" || p.CaseComment == "" ||
			p.CommentLink == "" || p.CaseLink == "" || len(p.Recipients) == 0 {
			return fmt.Errorf("handler: missing required field for %s", t)
		}
	case events.TypeStatusChanged:
		var p events.StatusChangedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.CaseID == "" || p.NewStatus == "" || p.CaseLink == "" || p.CommentLink == "" || len(p.Recipients) == 0 {
			return fmt.Errorf("handler: missing required field for %s", t)
		}
	case events.TypeCaseAssigned:
		var p events.CaseAssignedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.AssignerName == "" || p.AssignerEmail == "" || p.CaseID == "" ||
			p.CaseLink == "" || p.CommentLink == "" || len(p.Recipients) == 0 {
			return fmt.Errorf("handler: missing required field for %s", t)
		}
	case events.TypeIncidentCreated:
		var p events.IncidentCreatedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.Product == "" || p.Title == "" || p.ShortDescription == "" ||
			p.IncidentLink == "" || p.CallTo == "" {
			return fmt.Errorf("handler: missing required field for %s", t)
		}
	default:
		return fmt.Errorf("handler: unknown event type %q", t)
	}
	return nil
}

// decodeStrict unmarshals raw into v, rejecting any field not present in v's
// struct definition.
func decodeStrict(raw json.RawMessage, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(v)
}

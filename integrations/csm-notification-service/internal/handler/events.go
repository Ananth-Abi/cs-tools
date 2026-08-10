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
	"regexp"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/outbox"
)

// emailPattern is a deliberately loose "does this look like an email
// address" check — local@domain.tld — not full RFC 5322 validation. Good
// enough to catch the actually-costly mistake (a blank or clearly-malformed
// recipient that would burn all of handleAttempts' retries downstream before
// being dropped), without trying to be a real email validator.
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// e164Pattern matches E.164 phone numbers (e.g. "+14155552671") — a leading
// "+", a non-zero first digit, then up to 14 more digits.
var e164Pattern = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// validRecipients reports whether every entry in recipients looks like an
// email address, and there's at least one. A single malformed entry fails
// the whole event — better to reject at the boundary than let the consumer
// retry a delivery that cannot succeed.
func validRecipients(recipients []string) bool {
	if len(recipients) == 0 {
		return false
	}
	for _, r := range recipients {
		if !emailPattern.MatchString(r) {
			return false
		}
	}
	return true
}

// eventPublisher abstracts eventbus.Producer for testability.
type eventPublisher interface {
	Publish(ctx context.Context, key, value []byte) error
}

// entityServiceClient abstracts entityservice.Client for testability.
type entityServiceClient interface {
	UpdateEventOutboxStatus(ctx context.Context, id, status string) error
}

// EventsHandler handles HTTP requests that submit a domain event to be
// published to the event bus.
type EventsHandler struct {
	publisher     eventPublisher
	entityService entityServiceClient
}

// NewEventsHandler creates an EventsHandler backed by the given publisher.
// entityService may be nil — when it is, or when a request has no EventID,
// the outbox claim/dispatch/release calls below are skipped entirely and the
// event is published unconditionally, matching this service's behavior
// before entity-service's event_outbox existed.
func NewEventsHandler(publisher eventPublisher, entityService entityServiceClient) *EventsHandler {
	return &EventsHandler{publisher: publisher, entityService: entityService}
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
	if err := validateEventPayload(env.EntityID, env.Type, env.Payload); err != nil {
		// The response is intentionally the generic ErrMsgBadRequest — err's
		// detail (which field, which mismatch) isn't for the caller — but
		// it's still worth this service's own logs, or a caller integrating
		// against this API for the first time is a black box to debug.
		slog.DebugContext(r.Context(), "rejected event", "type", env.Type, "entityId", env.EntityID, "err", err)
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// Publish the original request body as received (already validated
	// above) — the consumer decodes it the same way. Keyed by EntityID so
	// every event about the same case/incident lands on the same partition
	// and is processed in the order it was published. outbox.Dispatch claims
	// the event_outbox row named by env.EventID before publishing (skipping
	// publish on a conflict — already claimed by a racing caller or
	// entity-service's own polling fallback) — see its doc comment; a 202 is
	// the right response either way, since from this request's perspective
	// the event either got published just now or was already handled.
	if _, err := outbox.Dispatch(r.Context(), h.publisher, h.entityService, env.EventID, []byte(env.EntityID), body); err != nil {
		slog.ErrorContext(r.Context(), "failed to dispatch event", "type", env.Type, "entityId", env.EntityID, "eventId", env.EventID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	writeJSONValue(w, http.StatusAccepted, map[string]string{"message": "event accepted"})
}

// validateEventPayload decodes raw as t's matching payload type (rejecting
// unknown fields) and checks its required fields are non-empty. This is
// deliberately duplicated per type rather than done via reflection — each
// type's required fields are exactly the ones its Render* function in
// internal/notifications needs. entityID is the envelope's own EntityID
// (already trimmed by the caller) — for the three case.* types that carry
// their own CaseID, it must match: entityID is the Kafka partition key (see
// events.Envelope's doc comment), so a payload whose CaseID disagrees with
// it would be keyed under the wrong case's partition, breaking that other
// case's ordering guarantee.
func validateEventPayload(entityID string, t events.Type, raw json.RawMessage) error {
	switch t {
	case events.TypeCaseCreated:
		var p events.CaseCreatedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.ReporterName == "" || p.ProjectName == "" || p.CaseID == "" || p.CaseTitle == "" ||
			p.CaseType == "" || p.Priority == "" || p.CreatedAt == "" || p.Description == "" ||
			p.CaseLink == "" || p.CommentLink == "" || !validRecipients(p.Recipients) {
			return fmt.Errorf("handler: missing required field for %s", t)
		}
		if p.CaseID != entityID {
			return fmt.Errorf("handler: payload caseId %q does not match entityId %q", p.CaseID, entityID)
		}
	case events.TypeCommentAdded:
		var p events.CommentAddedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.Name == "" || p.ProjectID == "" || p.CaseTitle == "" || p.CaseComment == "" ||
			p.CommentLink == "" || p.CaseLink == "" || !validRecipients(p.Recipients) {
			return fmt.Errorf("handler: missing required field for %s", t)
		}
	case events.TypeStatusChanged:
		var p events.StatusChangedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.CaseID == "" || p.NewStatus == "" || p.CaseLink == "" || p.CommentLink == "" || !validRecipients(p.Recipients) {
			return fmt.Errorf("handler: missing required field for %s", t)
		}
		if p.CaseID != entityID {
			return fmt.Errorf("handler: payload caseId %q does not match entityId %q", p.CaseID, entityID)
		}
	case events.TypeCaseAssigned:
		var p events.CaseAssignedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.AssignerName == "" || p.AssignerEmail == "" || p.CaseID == "" ||
			p.CaseLink == "" || p.CommentLink == "" || !validRecipients(p.Recipients) {
			return fmt.Errorf("handler: missing required field for %s", t)
		}
		if p.CaseID != entityID {
			return fmt.Errorf("handler: payload caseId %q does not match entityId %q", p.CaseID, entityID)
		}
	case events.TypeIncidentCreated:
		var p events.IncidentCreatedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.Product == "" || p.Title == "" || p.ShortDescription == "" ||
			p.IncidentLink == "" || !e164Pattern.MatchString(p.CallTo) {
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

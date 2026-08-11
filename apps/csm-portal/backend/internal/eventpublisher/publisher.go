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

// Package eventpublisher is the producer side of the domain-event pipeline
// csm-notification-service consumes from: it builds the wire envelope
// {type, entityId, payload} that service's internal/events.Envelope
// expects, publishes it to Event Hub via internal/eventbus, and — if Event
// Hub doesn't acknowledge the publish — durably records that failure via
// entity-service's POST /event-publish-failures instead of losing the event
// silently. csm-notification-service never talks to that table itself
// anymore (it's a pure Kafka consumer); this package is the one writer.
package eventpublisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// kafkaProducer abstracts eventbus.Producer for testability.
type kafkaProducer interface {
	Publish(ctx context.Context, key, value []byte) error
}

// entityClient abstracts entity.CustomerEntityClient's event-publish-failure
// call for testability.
type entityClient interface {
	CreateEventPublishFailure(ctx context.Context, body []byte) ([]byte, error)
}

// Publisher publishes domain events for csm-notification-service to consume.
type Publisher struct {
	kafka  kafkaProducer
	entity entityClient
}

// New constructs a Publisher.
func New(kafka kafkaProducer, entity entityClient) *Publisher {
	return &Publisher{kafka: kafka, entity: entity}
}

// envelope is the wire shape csm-notification-service's internal/events.
// Envelope expects — kept in sync with that type by hand, since the two
// live in separate Go modules and neither imports the other.
type envelope struct {
	Type     string          `json:"type"`
	EntityID string          `json:"entityId"`
	Payload  json.RawMessage `json:"payload"`
}

// Publish builds the envelope for eventType/entityID/payload and publishes
// it to Event Hub, keyed by entityID (so every event about the same
// case/incident stays ordered on the same partition — see
// eventbus.Producer.Publish).
//
// If the publish itself fails (Event Hub never acknowledges it), Publish
// makes a best-effort call to entity-service to durably record the failure
// (for manual remediation later — see domain.EventPublishFailure on the
// entity-service side), then still returns the original publish error: from
// the caller's perspective the event was not delivered to the bus, and
// recording that fact is a side effect, not a substitute for delivery. If
// the failure-recording call *also* fails, that's logged here (not
// returned) rather than compounding the error the caller already has to
// handle.
func (p *Publisher) Publish(ctx context.Context, eventType, entityID string, payload json.RawMessage) error {
	body, err := json.Marshal(envelope{Type: eventType, EntityID: entityID, Payload: payload})
	if err != nil {
		return fmt.Errorf("eventpublisher: encode envelope: %w", err)
	}

	pubErr := p.kafka.Publish(ctx, []byte(entityID), body)
	if pubErr == nil {
		return nil
	}

	failureBody, err := json.Marshal(struct {
		EventType string          `json:"eventType"`
		EntityID  string          `json:"entityId"`
		Payload   json.RawMessage `json:"payload"`
		Error     string          `json:"error"`
	}{EventType: eventType, EntityID: entityID, Payload: payload, Error: pubErr.Error()})
	if err != nil {
		slog.ErrorContext(ctx, "eventpublisher: publish failed and could not encode the failure record", "eventType", eventType, "entityId", entityID, "publishErr", pubErr, "encodeErr", err)
		return fmt.Errorf("eventpublisher: publish %s for entity %s: %w", eventType, entityID, pubErr)
	}

	if _, recErr := p.entity.CreateEventPublishFailure(ctx, failureBody); recErr != nil {
		slog.ErrorContext(ctx, "eventpublisher: publish failed and recording the failure also failed", "eventType", eventType, "entityId", entityID, "publishErr", pubErr, "recordErr", recErr)
	}

	return fmt.Errorf("eventpublisher: publish %s for entity %s: %w", eventType, entityID, pubErr)
}

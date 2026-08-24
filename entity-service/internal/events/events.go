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

// Package events defines the wire shape of every record on the case-events
// Kafka topic that internal/service.EventPublisherService produces to. It's
// kept in sync by hand with csm-notification-service's own
// internal/events.Envelope and apps/csm-portal/backend's own copy of this
// same package, since all three live in separate Go modules and none of them
// import each other — every one of them must agree on this shape for any two
// to make sense of each other.
package events

import "encoding/json"

// Type identifies which kind of domain event Envelope.Payload holds. Values
// mirror csm-notification-service's internal/events.Type constants exactly.
type Type string

const (
	TypeCaseCreated     Type = "case.created"
	TypeCommentAdded    Type = "case.comment_added"
	TypeStatusChanged   Type = "case.status_changed"
	TypeCaseAssigned    Type = "case.assigned"
	TypeIncidentCreated Type = "incident.created"
)

// Envelope is the wire shape of every record on the case-events topic.
// EntityID is whatever the event is about (a case ID for the case.* types,
// an incident ID for incident.created) and is also the Kafka partition key
// (see eventbus.Producer.Publish) — every event about the same case/incident
// lands on the same partition and is processed in publish order.
type Envelope struct {
	Type     Type            `json:"type"`
	EntityID string          `json:"entityId"`
	Payload  json.RawMessage `json:"payload"`
}

// CommentAddedPayload is the Payload shape for TypeCommentAdded.
// Deliberately minimal: no comment body, no author — Envelope already
// carries EntityID (the case), so this is only what search/replay by
// timestamp needs.
type CommentAddedPayload struct {
	Timestamp string `json:"timestamp"`
}

// StatusChangedPayload is the Payload shape for TypeStatusChanged.
type StatusChangedPayload struct {
	Timestamp string `json:"timestamp"`
	NewStatus string `json:"newStatus"`
}

// CaseCreatedPayload is the Payload shape for TypeCaseCreated — mirrors
// csm-notification-service's own CaseCreatedPayload (its internal/events/
// validate.go is the schema authority; keep this in sync by hand the same
// way Envelope above is kept in sync). That payload also has optional
// Product/IncidentImpactDescription fields; they're omitted here rather than
// always encoded empty, since this service has no data source for either
// yet — omitting an optional field and encoding it empty are equivalent on
// the wire (see the notification service's decodeStrict).
type CaseCreatedPayload struct {
	ReporterName string   `json:"reporterName"`
	ProjectName  string   `json:"projectName"`
	ProjectID    string   `json:"projectId"`
	CaseID       string   `json:"caseId"`
	CaseTitle    string   `json:"caseTitle"`
	CaseType     string   `json:"caseType"`
	Priority     string   `json:"priority"`
	CreatedAt    string   `json:"createdAt"`
	Description  string   `json:"description"`
	Recipients   []string `json:"recipients"`
}

// IncidentCreatedPayload is the Payload shape for TypeIncidentCreated —
// mirrors csm-notification-service's own IncidentCreatedPayload, which also
// has Product (Google Chat space) and CallTo (on-call phone number) fields.
// Both are deliberately omitted here: this service has no product→Chat-space
// mapping or on-call number of its own to supply, and
// csm-notification-service's dispatch.Dispatcher substitutes its own
// configured defaults (DEFAULT_CHAT_PRODUCT/INCIDENT_DEFAULT_CALL_TO) when
// either is absent — see that service's events.Validate, which accepts this.
type IncidentCreatedPayload struct {
	Title            string `json:"title"`
	ShortDescription string `json:"shortDescription"`
	IncidentLink     string `json:"incidentLink"`
}

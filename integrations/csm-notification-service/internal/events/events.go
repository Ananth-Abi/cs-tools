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

// Package events defines the domain events csm-portal-backend and
// customer-portal-backend publish to this service (via POST /events), and
// that this service's consumer reads back off the event bus to decide what
// notification to send.
//
// v1 payloads are deliberately denormalized: they carry every display value
// a template needs (names, titles, links), plus who to notify (Recipients),
// rather than just IDs, so the consumer can render and send without looking
// anything up first. That's a stopgap for not having an entity-service
// client wired in yet — once one exists, payloads can shrink to IDs and the
// consumer can resolve the rest, including Recipients, itself instead of
// requiring every caller to already know the destination addresses.
package events

import "encoding/json"

// Type identifies which of the event types below Envelope.Payload holds.
type Type string

const (
	TypeCaseCreated     Type = "case.created"
	TypeCommentAdded    Type = "case.comment_added"
	TypeStatusChanged   Type = "case.status_changed"
	TypeCaseAssigned    Type = "case.assigned"
	TypeIncidentCreated Type = "incident.created"
)

// KnownTypes lists every Type this service accepts, in the order they're
// checked — used both for request validation and for generating docs/errors
// that enumerate valid values.
var KnownTypes = []Type{TypeCaseCreated, TypeCommentAdded, TypeStatusChanged, TypeCaseAssigned, TypeIncidentCreated}

// Envelope is the wire shape for both POST /events and the record published
// to the event bus: Payload's shape depends on Type (see the Type constants'
// matching Payload struct below). EntityID is whatever this event is about —
// a case ID for the case.* types, an incident ID for incident.created — and
// is duplicated at the envelope level (also present inside most payloads)
// because it's used as the Kafka record's partition key — see
// eventbus.Producer.Publish — so it must be readable without unmarshaling
// Payload first. Everything with the same EntityID lands on the same
// partition and is processed in publish order.
//
// EventID is optional and currently unused by this service — nothing
// validates, indexes, or dedupes on it yet. It exists so a caller can start
// generating one now (e.g. a UUID per event), while this schema has no
// external consumers to migrate: retrying a POST /events call that the
// broker actually acked (e.g. after a client-side timeout) publishes a
// second, distinct Kafka record that today's per-channel idempotency
// tracking can't recognize as a duplicate, since that tracking keys on
// topic/partition/offset, which differ for the retried copy. EventID is the
// natural key for detecting that case, and for future dead-letter
// correlation — once a durable store exists to actually use it.
type Envelope struct {
	Type     Type            `json:"type"`
	EntityID string          `json:"entityId"`
	EventID  string          `json:"eventId,omitempty"`
	Payload  json.RawMessage `json:"payload"`
}

// IsKnown reports whether t is one of KnownTypes.
func (t Type) IsKnown() bool {
	for _, known := range KnownTypes {
		if t == known {
			return true
		}
	}
	return false
}

// CaseCreatedPayload is TypeCaseCreated's payload — one field per
// notifications.CaseCreatedEmailData value, since case.created currently has
// exactly one reaction (the case-created email). Recipients is who to email
// — the caller (e.g. csm-portal-backend) already knows the audience (case
// watchers, assignee, reporter) at publish time, so it's supplied here
// rather than resolved by this service, which has no entity-service client.
type CaseCreatedPayload struct {
	ReporterName              string   `json:"reporterName"`
	ProjectName               string   `json:"projectName"`
	CaseID                    string   `json:"caseId"`
	CaseTitle                 string   `json:"caseTitle"`
	CaseType                  string   `json:"caseType"`
	Priority                  string   `json:"priority"`
	Product                   string   `json:"product,omitempty"`
	CreatedAt                 string   `json:"createdAt"`
	Description               string   `json:"description"`
	IncidentImpactDescription string   `json:"incidentImpactDescription,omitempty"`
	CaseLink                  string   `json:"caseLink"`
	CommentLink               string   `json:"commentLink"`
	Recipients                []string `json:"recipients"`
}

// CommentAddedPayload is TypeCommentAdded's payload. See CaseCreatedPayload's
// doc comment for why Recipients is here.
type CommentAddedPayload struct {
	Name        string   `json:"name"`
	ProjectID   string   `json:"projectId"`
	CaseTitle   string   `json:"caseTitle"`
	CaseComment string   `json:"caseComment"`
	CommentLink string   `json:"commentLink"`
	CaseLink    string   `json:"caseLink"`
	Recipients  []string `json:"recipients"`
}

// StatusChangedPayload is TypeStatusChanged's payload. See
// CaseCreatedPayload's doc comment for why Recipients is here.
type StatusChangedPayload struct {
	CaseID      string   `json:"caseId"`
	NewStatus   string   `json:"newStatus"`
	CaseLink    string   `json:"caseLink"`
	CommentLink string   `json:"commentLink"`
	Recipients  []string `json:"recipients"`
}

// CaseAssignedPayload is TypeCaseAssigned's payload. See CaseCreatedPayload's
// doc comment for why Recipients is here.
type CaseAssignedPayload struct {
	AssignerName  string   `json:"assignerName"`
	AssignerEmail string   `json:"assignerEmail"`
	CaseID        string   `json:"caseId"`
	CaseLink      string   `json:"caseLink"`
	CommentLink   string   `json:"commentLink"`
	Recipients    []string `json:"recipients"`
}

// IncidentCreatedPayload is TypeIncidentCreated's payload. Unlike the case.*
// events above, this one has two reactions, not one: a Google Chat alert
// (Product/Title/ShortDescription/IncidentLink map directly onto
// GoogleChatClient.SendIncidentAlert's params) and a Twilio voice call to
// CallTo, reading Title and ShortDescription aloud.
type IncidentCreatedPayload struct {
	// Product selects which configured Google Chat space receives the alert
	// (e.g. "api-manager"); matched case/whitespace-insensitively against
	// GOOGLE_CHAT_SPACES.
	Product          string `json:"product"`
	Title            string `json:"title"`
	ShortDescription string `json:"shortDescription"`
	// IncidentLink is the "Open in Portal" button target on the Chat card.
	IncidentLink string `json:"incidentLink"`
	// CallTo is the on-call phone number (E.164, e.g. "+14155552671") the
	// voice call is placed to.
	CallTo string `json:"callTo"`
}

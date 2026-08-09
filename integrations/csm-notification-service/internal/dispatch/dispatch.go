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

// Package dispatch is the consumer side of the event bus: it turns a
// published events.Envelope back into an actual notification send, by
// rendering the matching HTML template (internal/notifications) and calling
// the matching channel client.
package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// emailSender abstracts notifications.EmailClient for testability.
type emailSender interface {
	SendEmail(ctx context.Context, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error
}

// googleChatSender abstracts notifications.GoogleChatClient for testability.
type googleChatSender interface {
	SendIncidentAlert(ctx context.Context, product, title, shortDescription, portalURL string) error
}

// callSender abstracts notifications.TwilioClient's MakeCall for testability.
type callSender interface {
	MakeCall(ctx context.Context, to, message string) error
}

// Dispatcher turns a published events.Envelope into an actual notification
// send.
//
// Every case.* payload carries its own Recipients list (who to email) —
// this service has no entity-service client to resolve watchers/assignee/
// reporter itself, so the caller (e.g. csm-portal-backend) supplies the
// audience directly at publish time. incident.created carries no recipients
// field; its Google Chat/call reactions already have their own real
// destination, a space and a phone number, in the event payload itself.
type Dispatcher struct {
	email      emailSender
	googleChat googleChatSender
	call       callSender

	// doneMu/done track which (record, channel) pairs have already
	// succeeded — see handleIncidentCreated's doc comment for why this
	// exists. In-memory only: this is a stopgap for not having a durable
	// idempotency store yet, not a substitute for one. It's lost on
	// restart, which is fine — a restart-triggered redelivery duplicating
	// one already-succeeded channel is the same accepted at-least-once
	// trade-off documented elsewhere in this package; what this map fixes
	// is the much more likely case, retries within a single process's
	// handling of one record.
	doneMu sync.Mutex
	done   map[string]bool
}

// NewDispatcher constructs a Dispatcher.
func NewDispatcher(email emailSender, googleChat googleChatSender, call callSender) *Dispatcher {
	return &Dispatcher{
		email:      email,
		googleChat: googleChat,
		call:       call,
		done:       make(map[string]bool),
	}
}

// alreadyDone reports whether key has previously succeeded, without marking
// anything.
func (d *Dispatcher) alreadyDone(key string) bool {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	return d.done[key]
}

// markDone records key as succeeded.
func (d *Dispatcher) markDone(key string) {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	d.done[key] = true
}

// forget removes key — called once every channel for a record has
// succeeded, so the map doesn't hold onto entries forever.
func (d *Dispatcher) forget(key string) {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	delete(d.done, key)
}

// Handle implements eventbus.Handle. A non-nil return causes the caller
// (eventbus.Consumer) to retry — see its package doc for the retry policy.
func (d *Dispatcher) Handle(ctx context.Context, record eventbus.Record) error {
	var env events.Envelope
	if err := json.Unmarshal(record.Value, &env); err != nil {
		return fmt.Errorf("dispatch: decode envelope: %w", err)
	}

	switch env.Type {
	case events.TypeCaseCreated:
		return d.handleCaseCreated(ctx, env.Payload)
	case events.TypeCommentAdded:
		return d.handleCommentAdded(ctx, env.Payload)
	case events.TypeStatusChanged:
		return d.handleStatusChanged(ctx, env.Payload)
	case events.TypeCaseAssigned:
		return d.handleCaseAssigned(ctx, env.Payload)
	case events.TypeIncidentCreated:
		return d.handleIncidentCreated(ctx, record, env.Payload)
	default:
		return fmt.Errorf("dispatch: unknown event type %q", env.Type)
	}
}

func (d *Dispatcher) handleCaseCreated(ctx context.Context, raw json.RawMessage) error {
	var p events.CaseCreatedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.created payload: %w", err)
	}
	htmlBody := notifications.RenderCaseCreatedEmail(notifications.CaseCreatedEmailData{
		ReporterName:              p.ReporterName,
		ProjectName:               p.ProjectName,
		CaseID:                    p.CaseID,
		CaseTitle:                 p.CaseTitle,
		CaseType:                  p.CaseType,
		Priority:                  p.Priority,
		Product:                   p.Product,
		CreatedAt:                 p.CreatedAt,
		Description:               p.Description,
		IncidentImpactDescription: p.IncidentImpactDescription,
		CaseLink:                  p.CaseLink,
		CommentLink:               p.CommentLink,
	})
	return d.send(ctx, p.Recipients, fmt.Sprintf("[%s] %s", p.CaseID, p.CaseTitle), htmlBody)
}

func (d *Dispatcher) handleCommentAdded(ctx context.Context, raw json.RawMessage) error {
	var p events.CommentAddedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.comment_added payload: %w", err)
	}
	htmlBody := notifications.RenderCommentAddedEmail(p.Name, p.ProjectID, p.CaseTitle, p.CaseComment, p.CommentLink, p.CaseLink)
	return d.send(ctx, p.Recipients, "Re: "+p.CaseTitle, htmlBody)
}

func (d *Dispatcher) handleStatusChanged(ctx context.Context, raw json.RawMessage) error {
	var p events.StatusChangedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.status_changed payload: %w", err)
	}
	htmlBody := notifications.RenderStatusChangedEmail(p.CaseID, p.NewStatus, p.CaseLink, p.CommentLink)
	return d.send(ctx, p.Recipients, fmt.Sprintf("[%s] Status changed to %s", p.CaseID, p.NewStatus), htmlBody)
}

func (d *Dispatcher) handleCaseAssigned(ctx context.Context, raw json.RawMessage) error {
	var p events.CaseAssignedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.assigned payload: %w", err)
	}
	htmlBody := notifications.RenderCaseAssignedEmail(p.AssignerName, p.AssignerEmail, p.CaseID, p.CaseLink, p.CommentLink)
	return d.send(ctx, p.Recipients, fmt.Sprintf("[%s] Case assigned", p.CaseID), htmlBody)
}

// send emails recipients — sourced from the triggering event's own payload
// (see the Dispatcher doc comment), not any fixed/configured list. A caller
// that publishes an event with an empty Recipients slice should have been
// rejected already by handler.validateEventPayload; this check is a
// defensive backstop, not the primary guard.
func (d *Dispatcher) send(ctx context.Context, recipients []string, subject, htmlBody string) error {
	if len(recipients) == 0 {
		return fmt.Errorf("dispatch: event payload has no recipients")
	}
	return d.email.SendEmail(ctx, recipients, nil, nil, nil, subject, htmlBody, nil)
}

// handleIncidentCreated has two independent reactions, unlike every other
// event type here: a Google Chat alert and a voice call. Both are attempted
// even if one fails, and their errors are combined — a Chat outage shouldn't
// suppress the call, or vice versa.
//
// This is also the one handler that needs its own idempotency tracking:
// eventbus.Consumer retries this whole function on any error, and without
// tracking which side already succeeded, a Twilio failure alone would cause
// the (already-successful) Chat alert to be resent on every retry too —
// paging on-call once but posting 3 duplicate Chat cards, or vice versa.
// alreadyDone/markDone key on this specific record (topic/partition/offset,
// unique per record) plus which channel, so a retry only re-attempts the
// channel that's still actually failing.
func (d *Dispatcher) handleIncidentCreated(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.IncidentCreatedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode incident.created payload: %w", err)
	}

	base := record.Topic + "/" + strconv.FormatInt(int64(record.Partition), 10) + "/" + strconv.FormatInt(record.Offset, 10)
	chatKey, callKey := base+"/chat", base+"/call"

	var chatErr error
	if !d.alreadyDone(chatKey) {
		chatErr = d.googleChat.SendIncidentAlert(ctx, p.Product, p.Title, p.ShortDescription, p.IncidentLink)
		if chatErr == nil {
			d.markDone(chatKey)
		}
	}

	var callErr error
	if !d.alreadyDone(callKey) {
		message := fmt.Sprintf("New incident: %s. %s", p.Title, p.ShortDescription)
		callErr = d.call.MakeCall(ctx, p.CallTo, message)
		if callErr == nil {
			d.markDone(callKey)
		}
	}

	if chatErr == nil && callErr == nil {
		d.forget(chatKey)
		d.forget(callKey)
	}
	return errors.Join(chatErr, callErr)
}

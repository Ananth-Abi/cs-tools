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
	"log/slog"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
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

// linkResolver abstracts recipientlinks.Resolver for testability.
type linkResolver interface {
	ResolveLinks(ctx context.Context, emails []string, projectID, caseID string) ([]recipientlinks.RecipientLink, error)
	CSMLink(caseID string) string
}

// Dispatcher turns a published events.Envelope into an actual notification
// send.
//
// Every case.* payload carries its own Recipients list (who to email) — this
// service resolves which portal link each recipient gets (via links, see
// groupByLink), not who to notify: there's no entity-service lookup here for
// watchers/assignee/reporter, so the caller (e.g. csm-portal-backend)
// supplies the audience directly at publish time. incident.created carries
// no recipients field; its Google Chat/call reactions already have their
// own real destination, a space and a phone number, in the event payload
// itself, and don't go through links at all.
type Dispatcher struct {
	email      emailSender
	googleChat googleChatSender
	call       callSender
	links      linkResolver

	// emailDebugMode/emailDebugRecipients (EMAIL_DEBUG_MODE/
	// EMAIL_DEBUG_RECIPIENTS) redirect sendPerGroup's actual SendEmail calls
	// for the four case.* types to emailDebugRecipients instead of each
	// group's real resolved recipients, without touching Twilio or Google
	// Chat — real emails still go out, just to a safe test list rather than
	// real watchers/customers, so a dev/staging deployment can be exercised
	// end-to-end without risking a real mailbox. Link resolution
	// (groupByLink) still runs either way, so this doesn't mask a broken
	// recipientlinks/entity-service path — only the final recipient list is
	// swapped. If emailDebugMode is true but emailDebugRecipients is empty
	// (misconfigured), sendPerGroup logs and skips that group rather than
	// calling SendEmail with zero recipients.
	emailDebugMode       bool
	emailDebugRecipients []string

	// callSendingEnabled is the same kind of killswitch (CALL_SENDING_ENABLED)
	// for incident.created's Twilio call specifically — see
	// handleIncidentCreated's own doc comment. Doesn't affect the Google
	// Chat alert.
	callSendingEnabled bool

	// defaultChatProduct/defaultOnCallNumber are handleCaseCreated's and
	// handleIncidentCreated's fallback values for their payload's own
	// Product/CallTo when a publisher omits them — see handleIncidentCreated's
	// doc comment for why a publisher (e.g. entity-service) might not know
	// either value itself. defaultChatProduct applies to both event types'
	// Google Chat alert (case.created has no call reaction, hence no
	// case.created-specific default for defaultOnCallNumber).
	defaultChatProduct  string
	defaultOnCallNumber string

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

// NewDispatcher constructs a Dispatcher. See Dispatcher.emailDebugMode's and
// Dispatcher.callSendingEnabled's doc comments for what those two
// controls do, and Dispatcher.defaultChatProduct/defaultOnCallNumber's doc
// comment for the Google Chat/call fallback values.
func NewDispatcher(email emailSender, googleChat googleChatSender, call callSender, links linkResolver, emailDebugMode bool, emailDebugRecipients []string, callSendingEnabled bool, defaultChatProduct, defaultOnCallNumber string) *Dispatcher {
	return &Dispatcher{
		email:                email,
		googleChat:           googleChat,
		call:                 call,
		links:                links,
		emailDebugMode:       emailDebugMode,
		emailDebugRecipients: emailDebugRecipients,
		callSendingEnabled:   callSendingEnabled,
		defaultChatProduct:   defaultChatProduct,
		defaultOnCallNumber:  defaultOnCallNumber,
		done:                 make(map[string]bool),
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
	if !env.Type.IsKnown() {
		return fmt.Errorf("dispatch: unknown event type %q", env.Type)
	}
	// The only validation boundary left in this service: callers publish
	// directly to the event bus now (see events.Validate's doc comment), so
	// nothing has checked this record's required fields before it reaches
	// here.
	if err := events.Validate(env.EntityID, env.Type, env.Payload); err != nil {
		return fmt.Errorf("dispatch: invalid payload: %w", err)
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
	case events.TypeSLAClockRegister, events.TypeSLATierReached:
		// internal/slaengine's own consumer group (a different group ID, so
		// it gets its own full copy of this same topic) is what reacts to
		// these — nothing for the notification dispatcher to do. Returning
		// nil (not an error) is required here: erroring would burn this
		// consumer's retries and dead-letter an event that was never broken,
		// just not this consumer's concern.
		return nil
	default:
		return fmt.Errorf("dispatch: unknown event type %q", env.Type)
	}
}

// handleCaseCreated has two independent reactions, like handleIncidentCreated
// below: the case-created email (per resolved recipient link) and a Google
// Chat alert to the shared internal Chat space, via the same
// GoogleChatClient.SendIncidentAlert incident.created uses — its doc comment
// already covers "a newly created incident/case" for exactly this reuse. The
// Chat alert always targets the CSM portal's case link (links.CSMLink), not
// a per-recipient link, since there's no per-recipient audience for a Chat
// post the way there is for email. Product falls back to
// Dispatcher.defaultChatProduct when the payload omits it, the same as
// handleIncidentCreated's Product fallback — see that function's doc comment.
//
// Unlike handleIncidentCreated, this has no per-channel idempotency
// tracking: a retry that resends an already-succeeded Chat alert alongside a
// genuinely-failing email is the same accepted at-least-once trade-off
// sendPerGroup's own doc comment already accepts for case.* email
// duplication — not worth adding here either.
func (d *Dispatcher) handleCaseCreated(ctx context.Context, raw json.RawMessage) error {
	var p events.CaseCreatedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.created payload: %w", err)
	}

	var errs []error

	groups, err := d.groupByLink(ctx, p.Recipients, p.ProjectID, p.CaseID)
	if err != nil {
		errs = append(errs, err)
	} else {
		subject := fmt.Sprintf("[%s] %s", p.CaseID, p.CaseTitle)
		emailErr := d.sendPerGroup(ctx, groups, subject, func(caseLink string) string {
			return notifications.RenderCaseCreatedEmail(notifications.CaseCreatedEmailData{
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
				CaseLink:                  caseLink,
				CommentLink:               commentLinkFor(caseLink, ""),
			})
		})
		if emailErr != nil {
			errs = append(errs, emailErr)
		}
	}

	product := p.Product
	if product == "" {
		product = d.defaultChatProduct
	}
	if chatErr := d.googleChat.SendIncidentAlert(ctx, product, p.CaseTitle, p.Description, d.links.CSMLink(p.CaseID)); chatErr != nil {
		errs = append(errs, chatErr)
	}

	return errors.Join(errs...)
}

func (d *Dispatcher) handleCommentAdded(ctx context.Context, raw json.RawMessage) error {
	var p events.CommentAddedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.comment_added payload: %w", err)
	}
	groups, err := d.groupByLink(ctx, p.Recipients, p.ProjectID, p.CaseID)
	if err != nil {
		return err
	}
	subject := "Re: " + p.CaseTitle
	return d.sendPerGroup(ctx, groups, subject, func(caseLink string) string {
		return notifications.RenderCommentAddedEmail(p.Name, p.ProjectID, p.CaseTitle, p.CaseComment, commentLinkFor(caseLink, p.CommentID), caseLink)
	})
}

func (d *Dispatcher) handleStatusChanged(ctx context.Context, raw json.RawMessage) error {
	var p events.StatusChangedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.status_changed payload: %w", err)
	}
	groups, err := d.groupByLink(ctx, p.Recipients, p.ProjectID, p.CaseID)
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("[%s] Status changed to %s", p.CaseID, p.NewStatus)
	return d.sendPerGroup(ctx, groups, subject, func(caseLink string) string {
		return notifications.RenderStatusChangedEmail(p.CaseID, p.NewStatus, caseLink, commentLinkFor(caseLink, ""))
	})
}

func (d *Dispatcher) handleCaseAssigned(ctx context.Context, raw json.RawMessage) error {
	var p events.CaseAssignedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode case.assigned payload: %w", err)
	}
	groups, err := d.groupByLink(ctx, p.Recipients, p.ProjectID, p.CaseID)
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("[%s] Case assigned", p.CaseID)
	return d.sendPerGroup(ctx, groups, subject, func(caseLink string) string {
		return notifications.RenderCaseAssignedEmail(p.AssignerName, p.AssignerEmail, p.CaseID, caseLink, commentLinkFor(caseLink, ""))
	})
}

// groupByLink resolves each recipient's own case link (see
// recipientlinks.Resolver.ResolveLinks) and buckets recipients by the link
// they resolved to — at most two buckets today, customer portal vs CSM
// portal, so recipients sharing a link still go out in one SendEmail call
// rather than one per person. Recipients is sourced from the triggering
// event's own payload (see the Dispatcher doc comment), not any
// fixed/configured list. An empty Recipients slice should have been
// rejected already by events.Validate; the explicit check here is a
// defensive backstop, not the primary guard.
func (d *Dispatcher) groupByLink(ctx context.Context, recipients []string, projectID, caseID string) (map[string][]string, error) {
	if len(recipients) == 0 {
		return nil, fmt.Errorf("dispatch: event payload has no recipients")
	}
	links, err := d.links.ResolveLinks(ctx, recipients, projectID, caseID)
	if err != nil {
		return nil, fmt.Errorf("dispatch: resolve recipient links: %w", err)
	}
	groups := make(map[string][]string, 2)
	for _, l := range links {
		groups[l.CaseLink] = append(groups[l.CaseLink], l.Email)
	}
	return groups, nil
}

// commentLinkFor appends the comment permalink fragment to a resolved case
// link. Fragments are client-side only, so the same suffix works regardless
// of which portal's URL shape caseLink has — see recipientlinks' package doc
// for why the customer portal simply ignores it today rather than erroring.
// An empty commentID (every case.* type except case.comment_added, which
// has no comment to link to) yields the bare case link.
func commentLinkFor(caseLink, commentID string) string {
	if commentID == "" {
		return caseLink
	}
	return caseLink + "#" + url.PathEscape(commentID)
}

// maskPhone redacts all but the last 4 characters of an E.164 phone number
// for logging — this repo's own convention is to log only ids and sanitised
// summaries, not raw PII, and a phone number is PII the same way a recipient
// email address is (see internal/recipientlinks' own equivalent reasoning).
// A number with 4 or fewer characters (never valid E.164, but defensive
// against a malformed default) is masked entirely rather than echoed as-is.
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return strings.Repeat("*", len(phone))
	}
	return strings.Repeat("*", len(phone)-4) + phone[len(phone)-4:]
}

// sendPerGroup renders and sends one email per distinct resolved link, in
// sorted link order (deterministic, rather than Go's randomized map
// iteration). render is called once per group with that group's own case
// link, so each group's body carries the portal link its recipients can
// actually open.
//
// When emailDebugMode is true, each group's real recipients are replaced
// with emailDebugRecipients before sending — the email still actually goes
// out (unlike the old EMAIL_SENDING_ENABLED=false log-only killswitch this
// replaced), just to a safe configured test list instead of real
// watchers/customers. A group is skipped entirely (logged, not an error) if
// emailDebugMode is true but emailDebugRecipients is empty — sending to zero
// recipients would either be rejected by the email provider or silently do
// nothing, neither of which is better than not calling it at all.
//
// Partial failure here is at-least-once, not tracked with idempotency
// state: if one group's SendEmail fails after another group already
// succeeded, Handle's non-nil return causes eventbus.Consumer to retry the
// whole record, re-sending the group(s) that already succeeded too. That's
// the same trade-off handleIncidentCreated's done map exists to avoid for
// its two channels — email duplication is cheap enough, and today's
// recipient/group counts small enough, that this case accepts the
// duplication rather than adding the same tracking here.
func (d *Dispatcher) sendPerGroup(ctx context.Context, groups map[string][]string, subject string, render func(caseLink string) string) error {
	var errs []error
	for _, caseLink := range slices.Sorted(maps.Keys(groups)) {
		to := groups[caseLink]
		if d.emailDebugMode {
			if len(d.emailDebugRecipients) == 0 {
				slog.WarnContext(ctx, "dispatch: EMAIL_DEBUG_MODE=true but EMAIL_DEBUG_RECIPIENTS is empty; not sending",
					"subject", subject)
				continue
			}
			slog.InfoContext(ctx, "dispatch: EMAIL_DEBUG_MODE=true; redirecting email to configured debug recipients",
				"subject", subject, "realRecipientCount", len(to), "debugRecipientCount", len(d.emailDebugRecipients))
			to = d.emailDebugRecipients
		}
		if err := d.email.SendEmail(ctx, to, nil, nil, nil, subject, render(caseLink), nil); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// handleIncidentCreated has two independent reactions, unlike every other
// event type here except handleCaseCreated: a Google Chat alert and a voice
// call. Both are attempted even if one fails, and their errors are combined
// — a Chat outage shouldn't suppress the call, or vice versa.
//
// Product/CallTo fall back to Dispatcher's configured defaults
// (defaultChatProduct/defaultOnCallNumber) when the payload's own value
// is empty — a publisher that has no way to determine either (e.g.
// entity-service, which knows nothing about Chat-space routing or on-call
// rotations) can omit them entirely; events.Validate allows this. A
// publisher that does know the right values per incident can still supply
// them and takes precedence over the defaults. If a resolved value is still
// empty (payload and default both unset), that one channel is skipped
// (logged, treated as succeeded) instead of calling SendIncidentAlert/
// MakeCall with an empty product/destination — both would just return a
// real error (an unmapped product, an empty call destination), which would
// otherwise burn all of eventbus.Consumer's retries and dead-letter an
// incident whose only problem is a missing operator default, not a
// transient failure.
//
// callSendingEnabled gates only the MakeCall step (CALL_SENDING_ENABLED):
// when false, this logs what would have been called instead of calling, and
// still marks the call "done" so a disabled call doesn't retry forever —
// the same log-only shape sendPerGroup's email sending used to have before
// EMAIL_DEBUG_MODE replaced it with a redirect-to-a-test-list behavior (see
// sendPerGroup's doc comment); calls have no equivalent debug-recipient
// concept, so this keeps the simpler disable-entirely shape. The Google
// Chat alert is unaffected either way.
//
// This is also the one handler that needs its own idempotency tracking:
// eventbus.Consumer retries this whole function on any error, and without
// tracking which side already succeeded, a Twilio failure alone would cause
// the (already-successful) Chat alert to be resent on every retry too —
// paging on-call once but posting 3 duplicate Chat cards, or vice versa.
// alreadyDone/markDone key on this specific record (topic/partition/offset,
// unique per record) plus which channel, so a retry only re-attempts the
// channel that's still actually failing. Both keys are released once either
// both channels have succeeded, or record.IsFinalAttempt is true — the
// latter matters because a channel that never succeeds (e.g. Twilio stays
// down for all 3 attempts) would otherwise never hit the "both succeeded"
// branch, and its key would sit in d.done forever: IsFinalAttempt tells us
// eventbus.Consumer is about to commit and move on regardless of outcome,
// so there's no future retry left to protect against, and it's safe to
// stop tracking.
func (d *Dispatcher) handleIncidentCreated(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.IncidentCreatedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode incident.created payload: %w", err)
	}

	base := record.Topic + "/" + strconv.FormatInt(int64(record.Partition), 10) + "/" + strconv.FormatInt(record.Offset, 10)
	chatKey, callKey := base+"/chat", base+"/call"

	product := p.Product
	if product == "" {
		product = d.defaultChatProduct
	}
	callTo := p.CallTo
	if callTo == "" {
		callTo = d.defaultOnCallNumber
	}

	var chatErr error
	if !d.alreadyDone(chatKey) {
		if product == "" {
			slog.WarnContext(ctx, "dispatch: no product for incident.created (payload and DEFAULT_CHAT_PRODUCT both empty); skipping Google Chat alert")
			d.markDone(chatKey)
		} else {
			chatErr = d.googleChat.SendIncidentAlert(ctx, product, p.Title, p.ShortDescription, p.IncidentLink)
			if chatErr == nil {
				d.markDone(chatKey)
			}
		}
	}

	var callErr error
	if !d.alreadyDone(callKey) {
		switch {
		case !d.callSendingEnabled:
			slog.InfoContext(ctx, "dispatch: call sending disabled (CALL_SENDING_ENABLED=false); not calling", "to", maskPhone(callTo))
			d.markDone(callKey)
		case callTo == "":
			slog.WarnContext(ctx, "dispatch: no callTo for incident.created (payload and INCIDENT_DEFAULT_CALL_TO both empty); skipping call")
			d.markDone(callKey)
		default:
			message := fmt.Sprintf("New incident: %s. %s", p.Title, p.ShortDescription)
			callErr = d.call.MakeCall(ctx, callTo, message)
			if callErr == nil {
				d.markDone(callKey)
			}
		}
	}

	if (chatErr == nil && callErr == nil) || record.IsFinalAttempt {
		d.forget(chatKey)
		d.forget(callKey)
	}
	return errors.Join(chatErr, callErr)
}

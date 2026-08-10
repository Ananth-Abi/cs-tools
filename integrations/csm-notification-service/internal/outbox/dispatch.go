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

// Package outbox implements the claim-before-publish sequence that both
// PostEvent (the immediate-dispatch path) and Poller (its polling fallback)
// use to talk to entity-service's event_outbox, so the two never duplicate
// this logic or drift out of sync with each other.
package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

// Status values accepted by StatusUpdater.UpdateEventOutboxStatus — see
// domain.UpdateEventOutboxStatusRequest on the entity-service side for the
// current-status guard each one requires there.
const (
	StatusWaiting     = "waiting"
	StatusDispatching = "dispatching"
	StatusDispatched  = "dispatched"
)

// Publisher abstracts eventbus.Producer for testability.
type Publisher interface {
	Publish(ctx context.Context, key, value []byte) error
}

// StatusUpdater abstracts entityservice.Client's outbox status calls for
// testability.
type StatusUpdater interface {
	UpdateEventOutboxStatus(ctx context.Context, id, status string) error
}

// IsConflict reports whether err is entity-service's 409 response to
// UpdateEventOutboxStatus — the expected outcome when another caller
// already claimed or dispatched the row, not a failure worth retrying.
func IsConflict(err error) bool {
	var apiErr *apierror.Error
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict
}

// Dispatch claims the event_outbox row named by eventID (when set and es is
// non-nil) before publishing value keyed by key via pub — the claim must
// happen before Publish, not after: entity-service's
// UPDATE ... WHERE status = 'waiting' is the lock two racing callers (an
// immediate POST /events call and Poller's fallback sweep, or a caller
// retrying a call the broker already acked) resolve against. Delaying it
// until after Publish would leave both racers free to publish during the
// unprotected window beforehand.
//
// On successful publish the row is marked "dispatched"; on publish failure
// the claim is released back to "waiting" so a later attempt can pick it up
// again. Both of those follow-up calls run on a context.WithoutCancel copy
// of ctx, not ctx itself — ctx may be an HTTP request context that's about
// to be canceled (the request is finishing either way), and this cleanup
// call matters more than the original request's deadline. They're still
// best-effort — their own failure is only logged, not returned, since the
// event has already durably published (or definitively failed to) by the
// time they run.
//
// KNOWN GAP: if the release-to-"waiting" or mark-"dispatched" call fails
// even with an uncanceled cleanup context (entity-service down, network
// partition, etc.), the row is left stranded in "dispatching" — Poller only
// searches "waiting" rows, so nothing will retry it. This is the same
// unaddressed gap documented on entity-service's EventOutboxRepository.Claim
// (a claim lease + fencing token is the real fix); not built here for the
// same reason.
//
// Returns published=false, err=nil without attempting to publish when the
// claim conflicted (someone else already claimed or dispatched eventID) —
// not an error, just nothing left for this call to do. Returns
// published=true with no claim attempted at all when eventID is empty or es
// is nil — there is then nothing to deduplicate against.
func Dispatch(ctx context.Context, pub Publisher, es StatusUpdater, eventID string, key, value []byte) (published bool, err error) {
	claimed := eventID != "" && es != nil
	if claimed {
		if err := es.UpdateEventOutboxStatus(ctx, eventID, StatusDispatching); err != nil {
			if IsConflict(err) {
				return false, nil
			}
			return false, fmt.Errorf("claim event_outbox %s: %w", eventID, err)
		}
	}

	if err := pub.Publish(ctx, key, value); err != nil {
		if claimed {
			cleanupCtx := context.WithoutCancel(ctx)
			if releaseErr := es.UpdateEventOutboxStatus(cleanupCtx, eventID, StatusWaiting); releaseErr != nil {
				slog.ErrorContext(ctx, "failed to release event_outbox claim after publish failure", "eventId", eventID, "err", releaseErr)
			}
		}
		return false, fmt.Errorf("publish event %s: %w", eventID, err)
	}

	if claimed {
		cleanupCtx := context.WithoutCancel(ctx)
		if err := es.UpdateEventOutboxStatus(cleanupCtx, eventID, StatusDispatched); err != nil {
			slog.ErrorContext(ctx, "failed to mark event_outbox row dispatched", "eventId", eventID, "err", err)
		}
	}
	return true, nil
}

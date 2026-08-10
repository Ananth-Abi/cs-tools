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

package outbox

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entityservice"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// EntityServiceSearcher abstracts entityservice.Client's outbox search, on
// top of the status updates StatusUpdater already covers, for testability.
type EntityServiceSearcher interface {
	StatusUpdater
	SearchWaitingEventOutbox(ctx context.Context, limit int) ([]entityservice.EventOutboxRow, error)
}

// Poller periodically searches entity-service for event_outbox rows still
// "waiting" and dispatches the ones old enough to no longer be a live race
// with an in-flight POST /events call for the same row, via Dispatch — the
// same claim ("dispatching") -> publish -> "dispatched"/"waiting" sequence
// PostEvent uses. This is the fallback for a row whose immediate dispatch
// never happened at all: the caller's own POST /events call failed, was
// never made, or this service was down when it would have been.
type Poller struct {
	Publisher     Publisher
	EntityService EntityServiceSearcher
	// Interval is how often to poll.
	Interval time.Duration
	// MinAge is how old a "waiting" row must be before the Poller will
	// claim it — a grace period for the immediate-dispatch path to win the
	// claim first in the overwhelmingly common case. Claiming a row younger
	// than MinAge would still be race-safe (Dispatch's underlying
	// entity-service UPDATE ... WHERE status = 'waiting' is the actual
	// lock); this just avoids the Poller routinely beating the fast path to
	// rows it would have claimed anyway a moment later.
	MinAge time.Duration
	// Limit caps how many waiting rows are fetched per tick.
	Limit int
}

// Run polls at p.Interval until ctx is canceled.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollOnce(ctx)
		}
	}
}

// pollOnce runs a single search-and-dispatch pass.
func (p *Poller) pollOnce(ctx context.Context) {
	rows, err := p.EntityService.SearchWaitingEventOutbox(ctx, p.Limit)
	if err != nil {
		slog.ErrorContext(ctx, "event_outbox poll: search failed", "err", err)
		return
	}

	var dispatched, skipped, failed int
	for _, row := range rows {
		// Rows are returned oldest-first, so once one row is too fresh,
		// every row after it is too.
		if time.Since(row.CreatedOn) < p.MinAge {
			break
		}

		value, err := json.Marshal(events.Envelope{
			Type:     events.Type(row.EventType),
			EntityID: row.EntityID,
			EventID:  row.ID,
			Payload:  row.Payload,
		})
		if err != nil {
			slog.ErrorContext(ctx, "event_outbox poll: encode envelope failed", "eventId", row.ID, "eventType", row.EventType, "err", err)
			failed++
			continue
		}

		published, err := Dispatch(ctx, p.Publisher, p.EntityService, row.ID, []byte(row.EntityID), value)
		if err != nil {
			slog.ErrorContext(ctx, "event_outbox poll: dispatch failed", "eventId", row.ID, "eventType", row.EventType, "entityId", row.EntityID, "err", err)
			failed++
			continue
		}
		if !published {
			// Claimed by another caller between SearchWaitingEventOutbox and
			// this row's turn in the loop — not an error.
			skipped++
			continue
		}
		slog.InfoContext(ctx, "event_outbox poll: dispatched stale row", "eventId", row.ID, "eventType", row.EventType, "entityId", row.EntityID)
		dispatched++
	}

	if dispatched > 0 || failed > 0 {
		slog.InfoContext(ctx, "event_outbox poll complete", "found", len(rows), "dispatched", dispatched, "skipped", skipped, "failed", failed)
	}
}

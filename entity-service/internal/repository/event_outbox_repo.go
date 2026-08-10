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

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// EventOutboxRepository defines the persistence operations for the
// event_outbox table.
type EventOutboxRepository interface {
	// Create inserts a new row in "waiting" status.
	Create(ctx context.Context, req domain.CreateEventOutboxRequest) (domain.EventOutbox, error)
	// Claim atomically transitions id from "waiting" to "dispatching" and
	// returns the claimed row. This is the operation that closes the outbox
	// race: whichever of two racing callers (e.g. an immediate-dispatch path
	// and its own polling fallback) calls Claim first gets claimed=this row;
	// the other's UPDATE matches zero rows.
	//
	// Returns a *apierror.ConflictError if id could not be claimed — either
	// it was already claimed/dispatched by another caller, or id doesn't
	// exist. Both cases are reported identically: from the caller's
	// perspective the correct action is the same either way (stop, don't
	// retry), and every real caller already has this id from a prior Create
	// or SearchWaiting, so "doesn't exist" is not an expected case in
	// practice — not worth a second query to distinguish it from a lost race.
	//
	// KNOWN GAP: there is no claim lease or expiry. If a caller crashes after
	// Claim succeeds but before calling MarkDispatched or ReleaseFailed, the
	// row stays "dispatching" forever — SearchWaiting only returns "waiting"
	// rows, so nothing will ever pick it back up. Closing this needs a lease
	// (e.g. a claimed_at-based expiry) plus a fencing token so a recovered
	// claim can't be finalized by both the original and the recovering
	// caller. Deliberately not built yet — flagged rather than fixed pending
	// a scoping decision.
	Claim(ctx context.Context, id string) (domain.EventOutbox, error)
	// SearchWaiting returns rows still "waiting", oldest first, together
	// with the total count of waiting rows before pagination — the polling
	// fallback's candidate list.
	SearchWaiting(ctx context.Context, req domain.SearchEventOutboxRequest) ([]domain.EventOutbox, int, error)
	// MarkDispatched transitions id from "dispatching" to "dispatched" and
	// returns the updated row. Returns a *apierror.ConflictError if id is
	// not currently "dispatching" (e.g. called twice, or called without a
	// preceding Claim).
	MarkDispatched(ctx context.Context, id string) (domain.EventOutbox, error)
	// ReleaseFailed transitions id from "dispatching" back to "waiting"
	// (incrementing attempts) after a failed dispatch attempt, and returns
	// the updated row, so the polling fallback retries it later instead of
	// it staying claimed forever. Returns a *apierror.ConflictError if id
	// is not currently "dispatching".
	ReleaseFailed(ctx context.Context, id string) (domain.EventOutbox, error)
}

type eventOutboxRepo struct {
	db *pgxpool.Pool
}

// NewEventOutboxRepository constructs an EventOutboxRepository backed by the
// given connection pool.
func NewEventOutboxRepository(db *pgxpool.Pool) EventOutboxRepository {
	return &eventOutboxRepo{db: db}
}

// eventOutboxColumns is the column list shared by every query that returns a
// full row, kept in one place so Create/Claim/SearchWaiting can't drift out
// of sync with scanEventOutbox's field order.
const eventOutboxColumns = `id, event_type, entity_id, payload, status, attempts, created_at, claimed_at, dispatched_at`

func scanEventOutbox(row pgx.Row) (domain.EventOutbox, error) {
	var eo domain.EventOutbox
	if err := row.Scan(
		&eo.ID, &eo.EventType, &eo.EntityID, &eo.Payload, &eo.Status, &eo.Attempts,
		&eo.CreatedOn, &eo.ClaimedOn, &eo.DispatchedOn,
	); err != nil {
		return domain.EventOutbox{}, err
	}
	return eo, nil
}

// Create implements EventOutboxRepository.
func (r *eventOutboxRepo) Create(ctx context.Context, req domain.CreateEventOutboxRequest) (domain.EventOutbox, error) {
	query := `
		INSERT INTO event_outbox (event_type, entity_id, payload)
		VALUES ($1, $2, $3)
		RETURNING ` + eventOutboxColumns

	eo, err := scanEventOutbox(r.db.QueryRow(ctx, query, req.EventType, req.EntityID, req.Payload))
	if err != nil {
		return domain.EventOutbox{}, fmt.Errorf("create event_outbox: %w", err)
	}
	return eo, nil
}

// Claim implements EventOutboxRepository.
func (r *eventOutboxRepo) Claim(ctx context.Context, id string) (domain.EventOutbox, error) {
	query := `
		UPDATE event_outbox
		SET status = 'dispatching'::event_outbox_status_enum, claimed_at = NOW()
		WHERE id = $1 AND status = 'waiting'::event_outbox_status_enum
		RETURNING ` + eventOutboxColumns

	eo, err := scanEventOutbox(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EventOutbox{}, &apierror.ConflictError{Msg: "event_outbox row is not claimable: already claimed, already dispatched, or does not exist"}
		}
		return domain.EventOutbox{}, fmt.Errorf("claim event_outbox %s: %w", id, err)
	}
	return eo, nil
}

// SearchWaiting implements EventOutboxRepository.
func (r *eventOutboxRepo) SearchWaiting(ctx context.Context, req domain.SearchEventOutboxRequest) ([]domain.EventOutbox, int, error) {
	status := domain.EventOutboxStatusWaiting
	if req.Filters.Status != nil {
		status = *req.Filters.Status
	}

	countQuery := `SELECT COUNT(*) FROM event_outbox WHERE status = $1::event_outbox_status_enum`
	dataQuery := `
		SELECT ` + eventOutboxColumns + `
		FROM event_outbox
		WHERE status = $1::event_outbox_status_enum
		ORDER BY created_at ASC, id
		LIMIT $2 OFFSET $3`

	var total int
	var rowsOut []domain.EventOutbox

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, string(status)).Scan(&total); err != nil {
			return fmt.Errorf("count event_outbox: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, string(status), req.Pagination.Limit, req.Pagination.Offset)
		if err != nil {
			return fmt.Errorf("query event_outbox: %w", err)
		}
		defer rows.Close()

		result := make([]domain.EventOutbox, 0, req.Pagination.Limit)
		for rows.Next() {
			eo, err := scanEventOutbox(rows)
			if err != nil {
				return fmt.Errorf("scan event_outbox: %w", err)
			}
			result = append(result, eo)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate event_outbox: %w", err)
		}
		rowsOut = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return rowsOut, total, nil
}

// MarkDispatched implements EventOutboxRepository.
func (r *eventOutboxRepo) MarkDispatched(ctx context.Context, id string) (domain.EventOutbox, error) {
	query := `
		UPDATE event_outbox
		SET status = 'dispatched'::event_outbox_status_enum, dispatched_at = NOW()
		WHERE id = $1 AND status = 'dispatching'::event_outbox_status_enum
		RETURNING ` + eventOutboxColumns

	eo, err := scanEventOutbox(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EventOutbox{}, &apierror.ConflictError{Msg: "event_outbox row is not currently dispatching"}
		}
		return domain.EventOutbox{}, fmt.Errorf("mark event_outbox %s dispatched: %w", id, err)
	}
	return eo, nil
}

// ReleaseFailed implements EventOutboxRepository.
func (r *eventOutboxRepo) ReleaseFailed(ctx context.Context, id string) (domain.EventOutbox, error) {
	query := `
		UPDATE event_outbox
		SET status = 'waiting'::event_outbox_status_enum, attempts = attempts + 1
		WHERE id = $1 AND status = 'dispatching'::event_outbox_status_enum
		RETURNING ` + eventOutboxColumns

	eo, err := scanEventOutbox(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EventOutbox{}, &apierror.ConflictError{Msg: "event_outbox row is not currently dispatching"}
		}
		return domain.EventOutbox{}, fmt.Errorf("release event_outbox %s: %w", id, err)
	}
	return eo, nil
}

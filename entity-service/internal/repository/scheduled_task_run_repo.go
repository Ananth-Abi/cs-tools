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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// defaultStaleClaimAfter is used when a caller's
// ClaimScheduledTaskRunRequest.StaleClaimAfterSeconds is zero — see that
// field's own doc comment.
const defaultStaleClaimAfter = time.Hour

// ScheduledTaskRunRepository defines the persistence operations for the
// scheduled_task_run table — see domain.ScheduledTaskRun's doc comment for
// what it's for.
type ScheduledTaskRunRepository interface {
	// Attempt atomically decides whether req.TaskName/req.PeriodKey may run
	// right now, and claims it if so:
	//
	//   - No row exists for (TaskName, PeriodKey) — a period this task
	//     hasn't seen before. Any other still-open row for TaskName (one
	//     with neither SucceededOn nor SupersededOn set) is marked
	//     superseded first, then a fresh row is inserted for this period
	//     and claimed. Returns allowed=true.
	//   - A row exists and is SucceededOn, or SupersededOn, or its
	//     NextRetryOn is still in the future — allowed=false, that row
	//     returned unchanged.
	//   - A row exists, isn't succeeded/superseded, and either its
	//     NextRetryOn has arrived or it looks like an orphaned claim (see
	//     req.StaleClaimAfterSeconds) — attempt_count is bumped and
	//     allowed=true.
	//
	// Concurrent callers racing for the exact same (TaskName, PeriodKey)
	// are serialized by this table's own UNIQUE constraint: at most one can
	// ever see allowed=true for a given claim.
	Attempt(ctx context.Context, req domain.ClaimScheduledTaskRunRequest) (run domain.ScheduledTaskRun, allowed bool, err error)
	// Complete marks the row succeeded and clears NextRetryOn/LastError.
	// Returns a *apierror.NotFoundError if id doesn't exist.
	Complete(ctx context.Context, id string) (domain.ScheduledTaskRun, error)
	// Fail records a failed attempt: sets LastError/NextRetryOn. Deliberately
	// does not touch SucceededOn/SupersededOn — the row stays eligible for
	// another attempt (or for being superseded, once the next period comes
	// due, by a future Attempt call for a different period). Returns a
	// *apierror.NotFoundError if id doesn't exist.
	Fail(ctx context.Context, id string, req domain.FailScheduledTaskRunRequest) (domain.ScheduledTaskRun, error)
	// List returns every row matching statusFilter ("failed", "succeeded",
	// "superseded"), or every row if statusFilter is empty. statusFilter is
	// assumed already validated by the service layer.
	List(ctx context.Context, statusFilter string) ([]domain.ScheduledTaskRun, error)
	// DeleteResolvedBefore deletes every succeeded or superseded row
	// created before cutoff and returns how many rows were removed. A row
	// that is still open (neither succeeded nor superseded) is never
	// deleted, regardless of age — it represents a genuinely unresolved
	// problem, not history to archive.
	DeleteResolvedBefore(ctx context.Context, cutoff time.Time) (int, error)
}

type scheduledTaskRunRepo struct {
	db *pgxpool.Pool
}

// NewScheduledTaskRunRepository constructs a ScheduledTaskRunRepository
// backed by the given connection pool.
func NewScheduledTaskRunRepository(db *pgxpool.Pool) ScheduledTaskRunRepository {
	return &scheduledTaskRunRepo{db: db}
}

// scheduledTaskRunColumns is the column list shared by every query that
// returns a full row, kept in one place so the various methods below can't
// drift out of sync with scanScheduledTaskRun's field order.
const scheduledTaskRunColumns = `id, task_name, period_key, attempt_count, last_error, next_retry_at, first_attempted_at, last_attempted_at, succeeded_at, superseded_at`

func scanScheduledTaskRun(row pgx.Row) (domain.ScheduledTaskRun, error) {
	var run domain.ScheduledTaskRun
	if err := row.Scan(
		&run.ID, &run.TaskName, &run.PeriodKey, &run.AttemptCount, &run.LastError, &run.NextRetryOn,
		&run.FirstAttemptedOn, &run.LastAttemptedOn, &run.SucceededOn, &run.SupersededOn,
	); err != nil {
		return domain.ScheduledTaskRun{}, err
	}
	return run, nil
}

// Attempt implements ScheduledTaskRunRepository.
func (r *scheduledTaskRunRepo) Attempt(ctx context.Context, req domain.ClaimScheduledTaskRunRequest) (domain.ScheduledTaskRun, bool, error) {
	staleAfter := defaultStaleClaimAfter
	if req.StaleClaimAfterSeconds > 0 {
		staleAfter = time.Duration(req.StaleClaimAfterSeconds) * time.Second
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.ScheduledTaskRun{}, false, fmt.Errorf("attempt scheduled_task_run: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := scanScheduledTaskRun(tx.QueryRow(ctx,
		`SELECT `+scheduledTaskRunColumns+` FROM scheduled_task_run WHERE task_name = $1 AND period_key = $2 FOR UPDATE`,
		req.TaskName, req.PeriodKey))

	var run domain.ScheduledTaskRun
	var allowed bool

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// A period this task hasn't seen before: retire whatever's still
		// open for it, then claim this one fresh. The UNIQUE(task_name,
		// period_key) constraint is what makes this race-safe against a
		// concurrent Attempt for the exact same period — see the
		// ON CONFLICT DO NOTHING below.
		if _, err := tx.Exec(ctx,
			`UPDATE scheduled_task_run SET superseded_at = NOW(), updated_at = NOW()
			 WHERE task_name = $1 AND succeeded_at IS NULL AND superseded_at IS NULL`,
			req.TaskName); err != nil {
			return domain.ScheduledTaskRun{}, false, fmt.Errorf("attempt scheduled_task_run: supersede: %w", err)
		}

		inserted, insertErr := scanScheduledTaskRun(tx.QueryRow(ctx,
			`INSERT INTO scheduled_task_run (task_name, period_key, attempt_count, first_attempted_at, last_attempted_at)
			 VALUES ($1, $2, 1, NOW(), NOW())
			 ON CONFLICT (task_name, period_key) DO NOTHING
			 RETURNING `+scheduledTaskRunColumns,
			req.TaskName, req.PeriodKey))
		switch {
		case errors.Is(insertErr, pgx.ErrNoRows):
			// Lost a race with a concurrent Attempt for this exact period —
			// the winner's own call is the one that gets to run it.
			run, err = scanScheduledTaskRun(tx.QueryRow(ctx,
				`SELECT `+scheduledTaskRunColumns+` FROM scheduled_task_run WHERE task_name = $1 AND period_key = $2`,
				req.TaskName, req.PeriodKey))
			if err != nil {
				return domain.ScheduledTaskRun{}, false, fmt.Errorf("attempt scheduled_task_run: re-read after lost race: %w", err)
			}
			allowed = false
		case insertErr != nil:
			return domain.ScheduledTaskRun{}, false, fmt.Errorf("attempt scheduled_task_run: insert: %w", insertErr)
		default:
			run = inserted
			allowed = true
		}

	case err != nil:
		return domain.ScheduledTaskRun{}, false, fmt.Errorf("attempt scheduled_task_run: select: %w", err)

	default:
		now := time.Now()
		open := existing.SucceededOn == nil && existing.SupersededOn == nil
		dueRetry := open && existing.NextRetryOn != nil && !existing.NextRetryOn.After(now)
		orphanedClaim := open && existing.NextRetryOn == nil && now.Sub(existing.LastAttemptedOn) > staleAfter

		if !dueRetry && !orphanedClaim {
			run, allowed = existing, false
			break
		}

		run, err = scanScheduledTaskRun(tx.QueryRow(ctx,
			`UPDATE scheduled_task_run
			 SET attempt_count = attempt_count + 1, next_retry_at = NULL, last_attempted_at = NOW(), updated_at = NOW()
			 WHERE id = $1
			 RETURNING `+scheduledTaskRunColumns,
			existing.ID))
		if err != nil {
			return domain.ScheduledTaskRun{}, false, fmt.Errorf("attempt scheduled_task_run: claim retry: %w", err)
		}
		allowed = true
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ScheduledTaskRun{}, false, fmt.Errorf("attempt scheduled_task_run: commit: %w", err)
	}
	return run, allowed, nil
}

// Complete implements ScheduledTaskRunRepository.
func (r *scheduledTaskRunRepo) Complete(ctx context.Context, id string) (domain.ScheduledTaskRun, error) {
	run, err := scanScheduledTaskRun(r.db.QueryRow(ctx,
		`UPDATE scheduled_task_run
		 SET succeeded_at = NOW(), next_retry_at = NULL, last_error = NULL, updated_at = NOW()
		 WHERE id = $1
		 RETURNING `+scheduledTaskRunColumns, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ScheduledTaskRun{}, &apierror.NotFoundError{Msg: "scheduled_task_run not found: " + id}
		}
		return domain.ScheduledTaskRun{}, fmt.Errorf("complete scheduled_task_run: %w", err)
	}
	return run, nil
}

// Fail implements ScheduledTaskRunRepository.
func (r *scheduledTaskRunRepo) Fail(ctx context.Context, id string, req domain.FailScheduledTaskRunRequest) (domain.ScheduledTaskRun, error) {
	run, err := scanScheduledTaskRun(r.db.QueryRow(ctx,
		`UPDATE scheduled_task_run
		 SET last_error = $2, next_retry_at = $3, updated_at = NOW()
		 WHERE id = $1
		 RETURNING `+scheduledTaskRunColumns, id, req.Error, req.NextRetryOn))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ScheduledTaskRun{}, &apierror.NotFoundError{Msg: "scheduled_task_run not found: " + id}
		}
		return domain.ScheduledTaskRun{}, fmt.Errorf("fail scheduled_task_run: %w", err)
	}
	return run, nil
}

// List implements ScheduledTaskRunRepository.
func (r *scheduledTaskRunRepo) List(ctx context.Context, statusFilter string) ([]domain.ScheduledTaskRun, error) {
	query := `SELECT ` + scheduledTaskRunColumns + ` FROM scheduled_task_run`
	switch statusFilter {
	case "":
		query += ` ORDER BY created_at DESC`
	case "failed":
		query += ` WHERE succeeded_at IS NULL AND superseded_at IS NULL AND next_retry_at IS NOT NULL ORDER BY next_retry_at`
	case "succeeded":
		query += ` WHERE succeeded_at IS NOT NULL ORDER BY succeeded_at DESC`
	case "superseded":
		query += ` WHERE superseded_at IS NOT NULL ORDER BY superseded_at DESC`
	default:
		// The service layer validates statusFilter before this is ever
		// called; reaching here means a caller inside this package skipped
		// that, which is a programming error, not a request-level one.
		return nil, fmt.Errorf("list scheduled_task_run: unrecognized status filter %q", statusFilter)
	}

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list scheduled_task_run: %w", err)
	}
	defer rows.Close()

	runs := []domain.ScheduledTaskRun{}
	for rows.Next() {
		run, err := scanScheduledTaskRun(rows)
		if err != nil {
			return nil, fmt.Errorf("list scheduled_task_run: scan: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list scheduled_task_run: %w", err)
	}
	return runs, nil
}

// DeleteResolvedBefore implements ScheduledTaskRunRepository.
func (r *scheduledTaskRunRepo) DeleteResolvedBefore(ctx context.Context, cutoff time.Time) (int, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM scheduled_task_run
		 WHERE (succeeded_at IS NOT NULL OR superseded_at IS NOT NULL) AND created_at < $1`,
		cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete scheduled_task_run: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

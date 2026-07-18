package taskengine

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) CreateJob(ctx context.Context, input CreateJobInput, window time.Time, maximumRate int) (Job, bool, error) {
	if store == nil || store.pool == nil || input.ID.IsZero() || input.OwnerUserID == (auth.Identifier{}) ||
		input.DraftID.IsZero() || !validTaskIdempotency(input.IdempotencyKey) || input.CatalogVersion == "" ||
		input.Now.IsZero() || !input.ExpiresAt.After(input.Now) || window.IsZero() || maximumRate < 1 {
		return Job{}, false, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Job{}, false, fmt.Errorf("begin task job creation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var active bool
	if err := tx.QueryRow(ctx, `SELECT active FROM app_users WHERE id=$1 FOR UPDATE`, taskAuthUUID(input.OwnerUserID)).Scan(&active); errors.Is(err, pgx.ErrNoRows) || !active {
		return Job{}, false, ErrForbidden
	} else if err != nil {
		return Job{}, false, fmt.Errorf("serialize task job creation: %w", err)
	}
	existing, err := scanJob(tx.QueryRow(ctx, `SELECT `+jobColumns+` FROM task_jobs
WHERE owner_user_id=$1 AND idempotency_key=$2 FOR UPDATE`, taskAuthUUID(input.OwnerUserID), input.IdempotencyKey))
	if err == nil {
		if subtle.ConstantTimeCompare(existing.RequestFingerprint[:], input.RequestFingerprint[:]) != 1 {
			return Job{}, false, ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return Job{}, false, taskPostgresError("commit task idempotency replay", err)
		}
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, fmt.Errorf("load task idempotency replay: %w", err)
	}
	var draftOK bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM task_drafts
WHERE id=$1 AND owner_user_id=$2 AND state='REVIEWED' AND catalog_version=$3 AND expires_at>$4)`,
		taskUUID(input.DraftID), taskAuthUUID(input.OwnerUserID), input.CatalogVersion, input.Now).Scan(&draftOK); err != nil {
		return Job{}, false, fmt.Errorf("verify reviewed task draft: %w", err)
	}
	if !draftOK {
		return Job{}, false, ErrConflict
	}
	var requestCount int
	if err := tx.QueryRow(ctx, `INSERT INTO task_usage_windows(owner_user_id,window_started_at,request_count,updated_at)
VALUES($1,$2,1,$3)
ON CONFLICT(owner_user_id) DO UPDATE SET
 window_started_at=CASE WHEN task_usage_windows.window_started_at=$2 THEN task_usage_windows.window_started_at ELSE $2 END,
 request_count=CASE WHEN task_usage_windows.window_started_at=$2 THEN task_usage_windows.request_count+1 ELSE 1 END,
 updated_at=$3
RETURNING request_count`, taskAuthUUID(input.OwnerUserID), window, input.Now).Scan(&requestCount); err != nil {
		return Job{}, false, fmt.Errorf("consume task usage window: %w", err)
	}
	if requestCount > maximumRate {
		return Job{}, false, ErrRateLimited
	}
	created, err := scanJob(tx.QueryRow(ctx, `INSERT INTO task_jobs
(id,owner_user_id,draft_id,retry_of_job_id,idempotency_key,request_fingerprint,catalog_version,state,
 next_event_sequence,expires_at,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,'QUEUED',2,$8,$9,$9)
RETURNING `+jobColumns, taskUUID(input.ID), taskAuthUUID(input.OwnerUserID), taskUUID(input.DraftID),
		optionalTaskUUID(input.RetryOfJobID), input.IdempotencyKey, input.RequestFingerprint[:], input.CatalogVersion,
		input.ExpiresAt, input.Now))
	if err != nil {
		return Job{}, false, taskPostgresError("create task job", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO task_job_events(job_id,sequence,event_kind,created_at)
VALUES($1,1,'JOB_ACCEPTED',$2)`, taskUUID(input.ID), input.Now); err != nil {
		return Job{}, false, taskPostgresError("record task accepted event", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, false, taskPostgresError("commit task job creation", err)
	}
	return created, true, nil
}

func (store *PostgresStore) AttachRiverJob(ctx context.Context, id Identifier, owner auth.Identifier, riverJobID int64, now time.Time) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) || riverJobID <= 0 || now.IsZero() {
		return Job{}, ErrInvalidInput
	}
	value, err := scanJob(store.pool.QueryRow(ctx, `UPDATE task_jobs SET river_job_id=$3,version=version+1,updated_at=$4
WHERE id=$1 AND owner_user_id=$2 AND state='QUEUED' AND river_job_id IS NULL
RETURNING `+jobColumns, taskUUID(id), taskAuthUUID(owner), riverJobID, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrConflict
	}
	if err != nil {
		return Job{}, taskPostgresError("attach task queue job", err)
	}
	return value, nil
}

func (store *PostgresStore) ListJobs(ctx context.Context, owner auth.Identifier, limit, offset int) (JobPage, error) {
	if store == nil || store.pool == nil || owner == (auth.Identifier{}) || limit < 1 || limit > MaximumJobPage || offset < 0 {
		return JobPage{}, ErrInvalidInput
	}
	rows, err := store.pool.Query(ctx, `SELECT `+jobColumns+` FROM task_jobs
WHERE owner_user_id=$1 AND expires_at>now()
ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, taskAuthUUID(owner), limit, offset)
	if err != nil {
		return JobPage{}, fmt.Errorf("list task jobs: %w", err)
	}
	defer rows.Close()
	page := JobPage{Jobs: make([]Job, 0), Limit: limit, Offset: offset}
	for rows.Next() {
		value, err := scanJob(rows)
		if err != nil {
			return JobPage{}, fmt.Errorf("scan task job: %w", err)
		}
		page.Jobs = append(page.Jobs, value)
	}
	if err := rows.Err(); err != nil {
		return JobPage{}, fmt.Errorf("iterate task jobs: %w", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM task_jobs WHERE owner_user_id=$1 AND expires_at>now()`, taskAuthUUID(owner)).Scan(&page.Total); err != nil {
		return JobPage{}, fmt.Errorf("count task jobs: %w", err)
	}
	return page, nil
}

func (store *PostgresStore) GetJob(ctx context.Context, id Identifier, owner auth.Identifier) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) {
		return Job{}, ErrNotFound
	}
	value, err := scanJob(store.pool.QueryRow(ctx, `SELECT `+jobColumns+` FROM task_jobs WHERE id=$1 AND owner_user_id=$2`, taskUUID(id), taskAuthUUID(owner)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("load task job: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) GetJobForWorker(ctx context.Context, id Identifier) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() {
		return Job{}, ErrNotFound
	}
	value, err := scanJob(store.pool.QueryRow(ctx, `SELECT `+jobColumns+` FROM task_jobs WHERE id=$1`, taskUUID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("load task worker job: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) ClaimJob(ctx context.Context, id Identifier, now time.Time) (Job, bool, error) {
	if store == nil || store.pool == nil || id.IsZero() || now.IsZero() {
		return Job{}, false, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Job{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, err := scanJob(tx.QueryRow(ctx, `SELECT `+jobColumns+` FROM task_jobs WHERE id=$1 FOR UPDATE`, taskUUID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, ErrNotFound
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("lock task job: %w", err)
	}
	if current.State.Terminal() {
		if err := tx.Commit(ctx); err != nil {
			return Job{}, false, err
		}
		return current, false, nil
	}
	if current.CancelRequestedAt != nil {
		cancelled, err := terminalTaskJobTx(ctx, tx, current, JobCancelled, "cancelled", now)
		if err != nil {
			return Job{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Job{}, false, err
		}
		return cancelled, false, nil
	}
	if current.AttemptCount >= MaximumAttempts {
		failed, err := terminalTaskJobTx(ctx, tx, current, JobFailed, "attempt_limit", now)
		if err != nil {
			return Job{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Job{}, false, err
		}
		return failed, false, nil
	}
	sequence := current.Version
	_ = sequence
	claimed, err := scanJob(tx.QueryRow(ctx, `UPDATE task_jobs SET
state='RUNNING',attempt_count=attempt_count+1,started_at=COALESCE(started_at,$2),
next_event_sequence=next_event_sequence+1,version=version+1,updated_at=$2
WHERE id=$1 AND state IN ('QUEUED','RUNNING')
RETURNING `+jobColumns, taskUUID(id), now))
	if err != nil {
		return Job{}, false, taskPostgresError("claim task job", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO task_job_events(job_id,sequence,event_kind,created_at)
VALUES($1,$2,'JOB_STARTED',$3)`, taskUUID(id), currentVersionEventSequence(current), now); err != nil {
		return Job{}, false, taskPostgresError("record task started event", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, false, err
	}
	return claimed, true, nil
}

func currentVersionEventSequence(job Job) int64 {
	// The persisted next sequence is not exposed on Job. Counting events under the
	// row lock gives a monotonic sequence without trusting client state.
	return job.Version + 1
}

func (store *PostgresStore) RecordProgress(ctx context.Context, id Identifier, kind EventKind, current, total, candidates int, now time.Time) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || now.IsZero() || kind.Terminal() || current < 0 || total < 0 || candidates < 0 {
		return Job{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Job{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var sequence int64
	if err := tx.QueryRow(ctx, `SELECT next_event_sequence FROM task_jobs WHERE id=$1 AND state='RUNNING' FOR UPDATE`, taskUUID(id)).Scan(&sequence); errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrConflict
	} else if err != nil {
		return Job{}, err
	}
	value, err := scanJob(tx.QueryRow(ctx, `UPDATE task_jobs SET
progress_current=$2,progress_total=$3,candidate_count=$4,next_event_sequence=next_event_sequence+1,
version=version+1,updated_at=$5 WHERE id=$1 AND state='RUNNING'
RETURNING `+jobColumns, taskUUID(id), current, total, candidates, now))
	if err != nil {
		return Job{}, taskPostgresError("record task progress", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO task_job_events
(job_id,sequence,event_kind,progress_current,progress_total,candidate_count,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7)`, taskUUID(id), sequence, string(kind), current, total, candidates, now); err != nil {
		return Job{}, taskPostgresError("insert task progress event", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, err
	}
	return value, nil
}

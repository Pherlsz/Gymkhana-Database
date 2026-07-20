package ocr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) ClaimJob(ctx context.Context, id Identifier, now time.Time) (Job, bool, error) {
	if store == nil || store.pool == nil || id.IsZero() || now.IsZero() {
		return Job{}, false, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Job{}, false, fmt.Errorf("begin OCR claim: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, sequence, err := lockJob(ctx, tx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, ErrNotFound
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("lock OCR job for claim: %w", err)
	}
	if (current.State != JobQueued && current.State != JobRunning) || current.CancelRequestedAt != nil || current.AttemptCount >= MaximumAttempts {
		if err := tx.Commit(ctx); err != nil {
			return Job{}, false, normalizePostgresError(err)
		}
		return current, false, nil
	}
	if current.State == JobRunning {
		claimed, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs
SET attempt_count=attempt_count+1,version=version+1,updated_at=$2
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), now))
		if err != nil {
			return Job{}, false, fmt.Errorf("reclaim OCR job: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return Job{}, false, normalizePostgresError(err)
		}
		return claimed, true, nil
	}
	claimed, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs
SET state='RUNNING',started_at=$2,attempt_count=attempt_count+1,next_event_sequence=next_event_sequence+1,version=version+1,updated_at=$2
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), now))
	if err != nil {
		return Job{}, false, fmt.Errorf("claim OCR job: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ocr_job_events(job_id,sequence,event_kind,created_at)
VALUES($1,$2,'JOB_STARTED',$3)`, databaseUUID(id), sequence, now); err != nil {
		return Job{}, false, fmt.Errorf("insert OCR started event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, false, normalizePostgresError(err)
	}
	return claimed, true, nil
}

func (store *PostgresStore) RecordSourceValidated(ctx context.Context, id Identifier, pages int, pixels int64, now time.Time) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || pages < 1 || pages > MaximumPages || pixels < 0 || pixels > MaximumPixels || now.IsZero() {
		return Job{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Job{}, fmt.Errorf("begin OCR source validation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, sequence, err := lockJob(ctx, tx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("lock OCR source validation: %w", err)
	}
	if current.State != JobRunning || current.CancelRequestedAt != nil {
		return Job{}, ErrInvalidState
	}
	updated, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs
SET page_count=$2,pixel_count=$3,next_event_sequence=next_event_sequence+1,version=version+1,updated_at=$4
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), pages, pixels, now))
	if err != nil {
		return Job{}, fmt.Errorf("record OCR source validation: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ocr_job_events(job_id,sequence,event_kind,page_count,created_at)
VALUES($1,$2,'SOURCE_VALIDATED',$3,$4)`, databaseUUID(id), sequence, pages, now); err != nil {
		return Job{}, fmt.Errorf("insert OCR source validation event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, normalizePostgresError(err)
	}
	return updated, nil
}

func (store *PostgresStore) AddProviderUsage(ctx context.Context, id Identifier, owner auth.Identifier, usage, maximum int64, now time.Time) (Job, bool, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) || usage < 0 || usage > MaximumProviderUsage || maximum < 1 || maximum > MaximumProviderUsage || now.IsZero() {
		return Job{}, false, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Job{}, false, fmt.Errorf("begin OCR provider accounting: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	job, _, err := lockJob(ctx, tx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, ErrNotFound
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("lock OCR provider accounting: %w", err)
	}
	if job.OwnerUserID != owner || job.State != JobRunning {
		return Job{}, false, ErrInvalidState
	}
	var current int64
	err = tx.QueryRow(ctx, `SELECT provider_usage FROM ocr_usage_windows WHERE owner_user_id=$1 FOR UPDATE`, authDatabaseUUID(owner)).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, ErrInvalidState
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("lock OCR usage window: %w", err)
	}
	newUsage := current + usage
	if _, err := tx.Exec(ctx, `UPDATE ocr_usage_windows SET provider_usage=$2,updated_at=$3 WHERE owner_user_id=$1`, authDatabaseUUID(owner), newUsage, now); err != nil {
		return Job{}, false, fmt.Errorf("record OCR usage window: %w", err)
	}
	updated, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs SET provider_usage=provider_usage+$2,version=version+1,updated_at=$3
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), usage, now))
	if err != nil {
		return Job{}, false, fmt.Errorf("record OCR job provider usage: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, false, normalizePostgresError(err)
	}
	return updated, newUsage > maximum, nil
}

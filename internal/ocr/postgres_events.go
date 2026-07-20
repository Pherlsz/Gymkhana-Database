package ocr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) RequestCancellation(ctx context.Context, id Identifier, owner auth.Identifier, now time.Time) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) || now.IsZero() {
		return Job{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Job{}, fmt.Errorf("begin OCR cancellation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, sequence, err := lockJob(ctx, tx, id)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && current.OwnerUserID != owner {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("lock OCR cancellation: %w", err)
	}
	if current.State == JobCancelled {
		if err := tx.Commit(ctx); err != nil {
			return Job{}, normalizePostgresError(err)
		}
		return current, nil
	}
	if current.State.Terminal() {
		return Job{}, ErrInvalidState
	}
	if current.State == JobRunning {
		updated, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs
SET cancel_requested_at=COALESCE(cancel_requested_at,$2),version=version+1,updated_at=$2
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), now))
		if err != nil {
			return Job{}, fmt.Errorf("request running OCR cancellation: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return Job{}, normalizePostgresError(err)
		}
		return updated, nil
	}
	updated, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs
SET state='CANCELLED',cancel_requested_at=$2,completed_at=$2,error_code='cancelled',
    next_event_sequence=next_event_sequence+1,version=version+1,updated_at=$2
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), now))
	if err != nil {
		return Job{}, fmt.Errorf("cancel queued OCR job: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ocr_job_events(job_id,sequence,event_kind,error_code,created_at)
VALUES($1,$2,'JOB_CANCELLED','cancelled',$3)`, databaseUUID(id), sequence, now); err != nil {
		return Job{}, fmt.Errorf("insert OCR cancellation event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, normalizePostgresError(err)
	}
	return updated, nil
}

func (store *PostgresStore) ListEvents(ctx context.Context, id Identifier, owner auth.Identifier, after int64, limit int) (EventPage, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) || after < 0 || limit < 1 || limit > MaximumEventPage {
		return EventPage{}, ErrInvalidInput
	}
	job, err := store.GetJob(ctx, id, owner)
	if err != nil {
		return EventPage{}, err
	}
	rows, err := store.pool.Query(ctx, `SELECT job_id,sequence,event_kind,page_count,suggestion_count,error_code,created_at
FROM ocr_job_events WHERE job_id=$1 AND sequence>$2 ORDER BY sequence LIMIT $3`, databaseUUID(id), after, limit+1)
	if err != nil {
		return EventPage{}, fmt.Errorf("list OCR events: %w", err)
	}
	defer rows.Close()
	values := make([]JobEvent, 0, limit+1)
	for rows.Next() {
		var value JobEvent
		var jobID pgtype.UUID
		var pageCount, suggestionCount pgtype.Int4
		var errorCode pgtype.Text
		if err := rows.Scan(&jobID, &value.Sequence, &value.Kind, &pageCount, &suggestionCount, &errorCode, &value.CreatedAt); err != nil {
			return EventPage{}, fmt.Errorf("scan OCR event: %w", err)
		}
		value.JobID = Identifier(jobID.Bytes)
		if pageCount.Valid {
			mapped := int(pageCount.Int32)
			value.PageCount = &mapped
		}
		if suggestionCount.Valid {
			mapped := int(suggestionCount.Int32)
			value.SuggestionCount = &mapped
		}
		if errorCode.Valid {
			value.ErrorCode = errorCode.String
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return EventPage{}, fmt.Errorf("iterate OCR events: %w", err)
	}
	more := len(values) > limit
	if more {
		values = values[:limit]
	}
	page := EventPage{Events: values, LastSequence: after, Terminal: !more && job.State.Terminal()}
	if len(values) > 0 {
		page.LastSequence = values[len(values)-1].Sequence
	}
	return page, nil
}

func (store *PostgresStore) RecoverStaleJobs(ctx context.Context, staleBefore, now time.Time, limit int) (int, error) {
	if store == nil || store.pool == nil || staleBefore.IsZero() || now.IsZero() || !staleBefore.Before(now) || limit < 1 || limit > 1000 {
		return 0, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin OCR recovery: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `SELECT id,next_event_sequence FROM ocr_jobs
WHERE state='RUNNING' AND updated_at<$1 ORDER BY updated_at,id LIMIT $2 FOR UPDATE SKIP LOCKED`, staleBefore, limit)
	if err != nil {
		return 0, fmt.Errorf("select stale OCR jobs: %w", err)
	}
	type staleJob struct {
		id       Identifier
		sequence int64
	}
	values := make([]staleJob, 0)
	for rows.Next() {
		var id pgtype.UUID
		var sequence int64
		if err := rows.Scan(&id, &sequence); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan stale OCR job: %w", err)
		}
		values = append(values, staleJob{id: Identifier(id.Bytes), sequence: sequence})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate stale OCR jobs: %w", err)
	}
	rows.Close()
	for _, value := range values {
		if _, err := tx.Exec(ctx, `UPDATE ocr_jobs
SET state='FAILED',error_code='worker_interrupted',completed_at=$2,cancel_requested_at=NULL,
    next_event_sequence=next_event_sequence+1,version=version+1,updated_at=$2 WHERE id=$1`, databaseUUID(value.id), now); err != nil {
			return 0, fmt.Errorf("recover stale OCR job: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO ocr_job_events(job_id,sequence,event_kind,error_code,created_at)
VALUES($1,$2,'JOB_FAILED','worker_interrupted',$3)`, databaseUUID(value.id), value.sequence, now); err != nil {
			return 0, fmt.Errorf("insert OCR recovery event: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, normalizePostgresError(err)
	}
	return len(values), nil
}

func (store *PostgresStore) SaveAudit(ctx context.Context, event AuditEvent) error {
	if store == nil || store.pool == nil || event.ID.IsZero() || event.EventType == "" || event.Outcome == "" ||
		len(event.ErrorCode) > 80 || len(event.RequestID) > 128 || event.CreatedAt.IsZero() {
		return ErrInvalidInput
	}
	_, err := store.pool.Exec(ctx, `INSERT INTO ocr_audit_events
(id,actor_user_id,job_id,suggestion_id,field_key,review_action,event_type,outcome,affected_count,error_code,request_id,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, databaseUUID(event.ID), optionalAuthUUID(event.ActorUserID),
		optionalDatabaseUUID(event.JobID), optionalDatabaseUUID(event.SuggestionID), optionalText(event.FieldKey),
		optionalText(string(event.ReviewAction)), event.EventType, event.Outcome, optionalInteger(event.AffectedCount),
		optionalText(event.ErrorCode), event.RequestID, event.CreatedAt)
	if err != nil {
		return fmt.Errorf("record OCR audit event: %w", err)
	}
	return nil
}

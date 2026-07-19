package taskengine

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) RecoverStaleJobs(ctx context.Context, staleBefore, now time.Time, limit int) (int, error) {
	if store == nil || store.pool == nil || staleBefore.IsZero() || now.IsZero() || !staleBefore.Before(now) || limit < 1 || limit > 1000 {
		return 0, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `SELECT id FROM task_jobs
WHERE state='RUNNING' AND updated_at<$1
ORDER BY updated_at,id FOR UPDATE SKIP LOCKED LIMIT $2`, staleBefore, limit)
	if err != nil {
		return 0, fmt.Errorf("list stale task jobs: %w", err)
	}
	ids := make([]Identifier, 0)
	for rows.Next() {
		var raw pgtype.UUID
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return 0, err
		}
		if !raw.Valid {
			rows.Close()
			return 0, ErrUnsafeResult
		}
		ids = append(ids, Identifier(raw.Bytes))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	for _, id := range ids {
		current, _, err := lockTaskJob(ctx, tx, id)
		if err != nil {
			return 0, err
		}
		if _, err := terminalTaskJobTx(ctx, tx, current, JobFailed, "worker_interrupted", now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (store *PostgresStore) DeleteExpired(ctx context.Context, now time.Time, limit int) (int, error) {
	if store == nil || store.pool == nil || now.IsZero() || limit < 1 || limit > 1000 {
		return 0, ErrInvalidInput
	}
	command, err := store.pool.Exec(ctx, `DELETE FROM task_drafts WHERE id IN (
SELECT id FROM task_drafts WHERE expires_at<=$1 ORDER BY expires_at,id LIMIT $2
)`, now, limit)
	if err != nil {
		return 0, taskPostgresError("delete expired task drafts", err)
	}
	return int(command.RowsAffected()), nil
}

func (store *PostgresStore) SaveAudit(ctx context.Context, event AuditEvent) error {
	if store == nil || store.pool == nil || event.ID.IsZero() || event.EventType == "" || event.Outcome == "" || event.CreatedAt.IsZero() {
		return ErrInvalidInput
	}
	_, err := store.pool.Exec(ctx, `INSERT INTO task_audit_events
(id,actor_user_id,draft_id,job_id,event_type,outcome,affected_count,error_code,request_id,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10)`, taskUUID(event.ID), optionalTaskAuthUUID(event.ActorUserID),
		optionalTaskUUID(event.DraftID), optionalTaskUUID(event.JobID), string(event.EventType), string(event.Outcome),
		event.AffectedCount, event.ErrorCode, event.RequestID, event.CreatedAt)
	if err != nil {
		return taskPostgresError("save task audit", err)
	}
	return nil
}

package aichat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) ListRunEvents(ctx context.Context, runID Identifier, owner auth.Identifier, after int64, limit int) (EventPage, error) {
	run, err := store.GetRun(ctx, runID, owner)
	if err != nil {
		return EventPage{}, err
	}
	rows, err := store.pool.Query(ctx, `SELECT run_id,sequence,event_kind,text_delta,tool_step_id,result_reference_id,error_code,created_at
FROM ai_chat_run_events WHERE run_id=$1 AND sequence>$2 ORDER BY sequence LIMIT $3`, chatUUID(runID), after, limit+1)
	if err != nil {
		return EventPage{}, fmt.Errorf("list AI Chat run events: %w", err)
	}
	defer rows.Close()
	page := EventPage{Events: make([]RunEvent, 0, limit), LastSequence: after}
	hasMore := false
	for rows.Next() {
		event, err := scanRunEvent(rows)
		if err != nil {
			return EventPage{}, fmt.Errorf("scan AI Chat run event: %w", err)
		}
		if len(page.Events) == limit {
			hasMore = true
			continue
		}
		page.Events = append(page.Events, event)
		page.LastSequence = event.Sequence
	}
	if err := rows.Err(); err != nil {
		return EventPage{}, fmt.Errorf("iterate AI Chat run events: %w", err)
	}
	page.Terminal = run.State.Terminal() && !hasMore
	return page, nil
}

func (store *PostgresStore) GetResultReference(ctx context.Context, id Identifier, owner auth.Identifier) (ResultReference, error) {
	value, err := scanResultReference(store.pool.QueryRow(ctx, `SELECT id,thread_id,run_id,owner_user_id,reference_kind,query_execution_id,
logical_request,context_fingerprint,label,row_count,column_count,expires_at,created_at
FROM ai_chat_result_references WHERE id=$1 AND owner_user_id=$2`, chatUUID(id), chatAuthUUID(owner)))
	if errors.Is(err, pgx.ErrNoRows) {
		return ResultReference{}, ErrNotFound
	}
	if err != nil {
		return ResultReference{}, fmt.Errorf("load AI Chat result reference: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) SetActiveResultReference(ctx context.Context, threadID Identifier, owner auth.Identifier, referenceID *Identifier, now time.Time) (Thread, error) {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Thread{}, fmt.Errorf("begin AI Chat context update: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var lockedThread pgtype.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM ai_chat_threads WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, chatUUID(threadID), chatAuthUUID(owner)).Scan(&lockedThread); errors.Is(err, pgx.ErrNoRows) {
		return Thread{}, ErrNotFound
	} else if err != nil {
		return Thread{}, fmt.Errorf("lock AI Chat context: %w", err)
	}
	if referenceID != nil {
		var found bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ai_chat_result_references WHERE id=$1 AND thread_id=$2 AND owner_user_id=$3 AND expires_at>$4)`,
			chatUUID(*referenceID), chatUUID(threadID), chatAuthUUID(owner), now).Scan(&found); err != nil {
			return Thread{}, fmt.Errorf("validate AI Chat result context: %w", err)
		}
		if !found {
			return Thread{}, ErrStaleContext
		}
	}
	thread, err := scanThread(tx.QueryRow(ctx, `UPDATE ai_chat_threads SET active_result_reference_id=$3,version=version+1,updated_at=$4
WHERE id=$1 AND owner_user_id=$2
RETURNING id,owner_user_id,title,active_result_reference_id,retention_expires_at,version,created_at,updated_at`,
		chatUUID(threadID), chatAuthUUID(owner), optionalChatUUID(referenceID), now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Thread{}, ErrNotFound
	}
	if err != nil {
		return Thread{}, fmt.Errorf("update AI Chat result context: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Thread{}, normalizePostgresError(err)
	}
	return thread, nil
}

func (store *PostgresStore) CleanupExpired(ctx context.Context, now time.Time, limit int) (int, error) {
	command, err := store.pool.Exec(ctx, `WITH victims AS (
  SELECT thread.id FROM ai_chat_threads thread
  WHERE thread.retention_expires_at<=$1
    AND NOT EXISTS(SELECT 1 FROM ai_chat_runs run WHERE run.thread_id=thread.id AND run.state IN ('QUEUED','RUNNING','TOOL_RUNNING'))
  ORDER BY thread.retention_expires_at,thread.id FOR UPDATE SKIP LOCKED LIMIT $2
)
DELETE FROM ai_chat_threads thread USING victims WHERE thread.id=victims.id`, now, limit)
	if err != nil {
		return 0, fmt.Errorf("clean expired AI Chat threads: %w", err)
	}
	return int(command.RowsAffected()), nil
}

func (store *PostgresStore) SaveAudit(ctx context.Context, event AuditEvent) error {
	_, err := store.pool.Exec(ctx, `INSERT INTO ai_chat_audit_events
(id,actor_user_id,thread_id,run_id,result_reference_id,event_type,outcome,tool_kind,affected_count,error_code,request_id,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, chatUUID(event.ID), optionalAuthUUID(event.ActorUserID),
		optionalChatUUID(event.ThreadID), optionalChatUUID(event.RunID), optionalChatUUID(event.ResultReferenceID), event.EventType,
		event.Outcome, optionalToolKind(event.ToolKind), optionalInteger(event.AffectedCount), optionalText(event.ErrorCode), event.RequestID, event.CreatedAt)
	if err != nil {
		return fmt.Errorf("save AI Chat audit event: %w", err)
	}
	return nil
}

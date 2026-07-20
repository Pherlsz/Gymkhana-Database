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

func loadRunByIdempotency(ctx context.Context, tx pgx.Tx, owner auth.Identifier, key string) (Run, bool, error) {
	value, err := scanRun(tx.QueryRow(ctx, runSelect+` WHERE owner_user_id=$1 AND idempotency_key=$2`, chatAuthUUID(owner), key))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, false, nil
	}
	if err != nil {
		return Run{}, false, fmt.Errorf("load idempotent AI Chat run: %w", err)
	}
	return value, true, nil
}

func loadRunMessage(ctx context.Context, tx pgx.Tx, runID Identifier, role MessageRole) (Message, error) {
	value, err := scanMessage(tx.QueryRow(ctx, `SELECT id,thread_id,run_id,sequence,role,content,created_at
FROM ai_chat_messages WHERE run_id=$1 AND role=$2`, chatUUID(runID), role))
	if err != nil {
		return Message{}, fmt.Errorf("load AI Chat run message: %w", err)
	}
	return value, nil
}

func reserveRunWindow(ctx context.Context, tx pgx.Tx, owner auth.Identifier, window time.Time, maximumRequests int, maximumUsage int64, now time.Time) error {
	command, err := tx.Exec(ctx, `INSERT INTO ai_chat_usage_windows(owner_user_id,window_started_at,request_count,updated_at)
VALUES($1,$2,1,$3) ON CONFLICT(owner_user_id) DO NOTHING`, chatAuthUUID(owner), window, now)
	if err != nil {
		return fmt.Errorf("initialize AI Chat usage window: %w", err)
	}
	if command.RowsAffected() == 1 {
		return nil
	}
	var existingWindow time.Time
	var count int
	var inputUsage, outputUsage int64
	if err := tx.QueryRow(ctx, `SELECT window_started_at,request_count,input_usage,output_usage FROM ai_chat_usage_windows WHERE owner_user_id=$1 FOR UPDATE`, chatAuthUUID(owner)).Scan(&existingWindow, &count, &inputUsage, &outputUsage); err != nil {
		return fmt.Errorf("lock AI Chat usage window: %w", err)
	}
	if !existingWindow.Equal(window) {
		_, err := tx.Exec(ctx, `UPDATE ai_chat_usage_windows SET window_started_at=$2,request_count=1,tool_call_count=0,input_usage=0,output_usage=0,result_bytes=0,updated_at=$3 WHERE owner_user_id=$1`,
			chatAuthUUID(owner), window, now)
		return err
	}
	if inputUsage+outputUsage >= maximumUsage {
		return ErrQuotaExceeded
	}
	if count >= maximumRequests {
		return ErrRateLimited
	}
	if _, err := tx.Exec(ctx, `UPDATE ai_chat_usage_windows SET request_count=request_count+1,updated_at=$2 WHERE owner_user_id=$1`, chatAuthUUID(owner), now); err != nil {
		return fmt.Errorf("reserve AI Chat run rate: %w", err)
	}
	return nil
}

func lockRun(ctx context.Context, tx pgx.Tx, id Identifier, owner auth.Identifier) (Run, error) {
	value, err := scanRun(tx.QueryRow(ctx, runSelect+` WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, chatUUID(id), chatAuthUUID(owner)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	if err != nil {
		return Run{}, fmt.Errorf("lock AI Chat run: %w", err)
	}
	return value, nil
}

func transitionRunTerminal(ctx context.Context, tx pgx.Tx, run Run, state RunState, code string, now time.Time) (Run, error) {
	cancelRequested := any(nil)
	eventKind := EventRunFailed
	if state == RunCancelled {
		cancelRequested, eventKind = now, EventRunCancelled
	}
	value, err := scanRun(tx.QueryRow(ctx, `UPDATE ai_chat_runs SET state=$3,error_code=$4,cancel_requested_at=COALESCE(cancel_requested_at,$5),completed_at=$6,version=version+1,updated_at=$6
WHERE id=$1 AND owner_user_id=$2 AND state IN ('QUEUED','RUNNING','TOOL_RUNNING')
RETURNING id,thread_id,owner_user_id,retry_of_run_id,idempotency_key,request_fingerprint,state,
tool_call_count,input_usage,output_usage,result_bytes,error_code,cancel_requested_at,started_at,completed_at,version,created_at,updated_at`,
		chatUUID(run.ID), chatAuthUUID(run.OwnerUserID), state, code, cancelRequested, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrConflict
	}
	if err != nil {
		return Run{}, normalizePostgresError(fmt.Errorf("finish AI Chat run: %w", err))
	}
	if _, err := appendEventTx(ctx, tx, RunEvent{RunID: run.ID, Kind: eventKind, ErrorCode: code, CreatedAt: now}); err != nil {
		return Run{}, err
	}
	return value, nil
}

func appendEventTx(ctx context.Context, tx pgx.Tx, event RunEvent) (RunEvent, error) {
	if err := tx.QueryRow(ctx, `UPDATE ai_chat_runs SET next_event_sequence=next_event_sequence+1
WHERE id=$1 RETURNING next_event_sequence-1`, chatUUID(event.RunID)).Scan(&event.Sequence); err != nil {
		return RunEvent{}, fmt.Errorf("allocate AI Chat event sequence: %w", err)
	}
	value, err := scanRunEvent(tx.QueryRow(ctx, `INSERT INTO ai_chat_run_events
(run_id,sequence,event_kind,text_delta,tool_step_id,result_reference_id,error_code,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING run_id,sequence,event_kind,text_delta,tool_step_id,result_reference_id,error_code,created_at`,
		chatUUID(event.RunID), event.Sequence, event.Kind, optionalText(event.TextDelta), optionalChatUUID(event.ToolStepID),
		optionalChatUUID(event.ResultReferenceID), optionalText(event.ErrorCode), event.CreatedAt))
	if err != nil {
		return RunEvent{}, normalizePostgresError(fmt.Errorf("append AI Chat run event: %w", err))
	}
	return value, nil
}

func insertResultReference(ctx context.Context, tx pgx.Tx, input CreateResultReferenceInput) (ResultReference, error) {
	value, err := scanResultReference(tx.QueryRow(ctx, `INSERT INTO ai_chat_result_references
(id,thread_id,run_id,owner_user_id,reference_kind,query_execution_id,logical_request,context_fingerprint,label,row_count,column_count,expires_at,created_at)
SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13
WHERE EXISTS(SELECT 1 FROM ai_chat_runs WHERE id=$3 AND thread_id=$2 AND owner_user_id=$4)
RETURNING id,thread_id,run_id,owner_user_id,reference_kind,query_execution_id,logical_request,context_fingerprint,label,row_count,column_count,expires_at,created_at`,
		chatUUID(input.ID), chatUUID(input.ThreadID), chatUUID(input.RunID), chatAuthUUID(input.OwnerUserID), input.Kind,
		optionalChatUUID(input.QueryExecutionID), input.LogicalRequest, input.ContextFingerprint[:], input.Label,
		input.RowCount, input.ColumnCount, input.ExpiresAt, input.Now))
	if errors.Is(err, pgx.ErrNoRows) {
		return ResultReference{}, ErrConflict
	}
	if err != nil {
		return ResultReference{}, normalizePostgresError(fmt.Errorf("insert AI Chat result reference: %w", err))
	}
	return value, nil
}

func normalizePostgresError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	if errors.Is(err, pgx.ErrTxCommitRollback) {
		return ErrConflict
	}
	var databaseError interface{ SQLState() string }
	if errors.As(err, &databaseError) {
		switch databaseError.SQLState() {
		case "57014":
			return ErrTimeout
		case "23505", "40001", "40P01":
			return ErrConflict
		case "23503":
			return ErrStaleContext
		case "23514", "22001", "22P02":
			return ErrInvalidInput
		}
	}
	return err
}

func chatUUID(value Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: !value.IsZero()}
}

func chatAuthUUID(value auth.Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: value != (auth.Identifier{})}
}

func optionalChatUUID(value *Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return chatUUID(*value)
}

func optionalAuthUUID(value *auth.Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return chatAuthUUID(*value)
}

func chatIdentifier(value pgtype.UUID) Identifier { return Identifier(value.Bytes) }

func optionalIdentifier(value pgtype.UUID) *Identifier {
	if !value.Valid {
		return nil
	}
	identifier := chatIdentifier(value)
	return &identifier
}

func sameOptionalIdentifier(left, right *Identifier) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func optionalText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func optionalInteger(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func optionalToolKind(value *ToolKind) any {
	if value == nil {
		return nil
	}
	return *value
}

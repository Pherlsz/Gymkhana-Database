package aichat

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) CreateRun(ctx context.Context, input CreateRunInput, window time.Time, maximumRequests int, maximumUsage int64) (RunCreation, error) {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return RunCreation{}, fmt.Errorf("begin AI Chat run creation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var lockedOwner pgtype.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM app_users WHERE id=$1 FOR UPDATE`, chatAuthUUID(input.OwnerUserID)).Scan(&lockedOwner); err != nil {
		return RunCreation{}, fmt.Errorf("serialize AI Chat owner run creation: %w", err)
	}
	if existing, found, err := loadRunByIdempotency(ctx, tx, input.OwnerUserID, input.IdempotencyKey); err != nil {
		return RunCreation{}, err
	} else if found {
		if subtle.ConstantTimeCompare(existing.RequestFingerprint[:], input.RequestFingerprint[:]) != 1 {
			return RunCreation{}, ErrConflict
		}
		message, err := loadRunMessage(ctx, tx, existing.ID, MessageUser)
		if err != nil {
			return RunCreation{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return RunCreation{}, normalizePostgresError(err)
		}
		return RunCreation{Run: existing, UserMessage: message, Created: false}, nil
	}
	var active pgtype.UUID
	var messageSequence int64
	var retentionExpiresAt time.Time
	err = tx.QueryRow(ctx, `SELECT active_result_reference_id,next_message_sequence,retention_expires_at FROM ai_chat_threads
WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, chatUUID(input.ThreadID), chatAuthUUID(input.OwnerUserID)).Scan(&active, &messageSequence, &retentionExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RunCreation{}, ErrNotFound
	}
	if err != nil {
		return RunCreation{}, fmt.Errorf("lock AI Chat thread for run: %w", err)
	}
	if !retentionExpiresAt.After(input.Now) {
		return RunCreation{}, ErrStaleContext
	}
	if !sameOptionalIdentifier(optionalIdentifier(active), input.ActiveResultReferenceID) {
		return RunCreation{}, ErrStaleContext
	}
	if input.RetryOfRunID != nil {
		var retryThread pgtype.UUID
		var retryOwner pgtype.UUID
		var retryState RunState
		err := tx.QueryRow(ctx, `SELECT thread_id,owner_user_id,state FROM ai_chat_runs WHERE id=$1`, chatUUID(*input.RetryOfRunID)).Scan(&retryThread, &retryOwner, &retryState)
		if errors.Is(err, pgx.ErrNoRows) {
			return RunCreation{}, ErrNotFound
		}
		if err != nil {
			return RunCreation{}, fmt.Errorf("load retried AI Chat run: %w", err)
		}
		if chatIdentifier(retryThread) != input.ThreadID || auth.Identifier(retryOwner.Bytes) != input.OwnerUserID || !retryState.Terminal() {
			return RunCreation{}, ErrInvalidState
		}
	}
	if err := reserveRunWindow(ctx, tx, input.OwnerUserID, window, maximumRequests, maximumUsage, input.Now); err != nil {
		return RunCreation{}, err
	}
	run, err := scanRun(tx.QueryRow(ctx, `INSERT INTO ai_chat_runs
(id,thread_id,owner_user_id,retry_of_run_id,idempotency_key,request_fingerprint,state,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,'QUEUED',$7,$7)
RETURNING id,thread_id,owner_user_id,retry_of_run_id,idempotency_key,request_fingerprint,state,
tool_call_count,input_usage,output_usage,result_bytes,error_code,cancel_requested_at,started_at,completed_at,version,created_at,updated_at`,
		chatUUID(input.ID), chatUUID(input.ThreadID), chatAuthUUID(input.OwnerUserID), optionalChatUUID(input.RetryOfRunID),
		input.IdempotencyKey, input.RequestFingerprint[:], input.Now))
	if err != nil {
		return RunCreation{}, normalizePostgresError(fmt.Errorf("insert AI Chat run: %w", err))
	}
	message, err := scanMessage(tx.QueryRow(ctx, `INSERT INTO ai_chat_messages
(id,thread_id,run_id,sequence,role,content,created_at)
VALUES($1,$2,$3,$4,'USER',$5,$6)
RETURNING id,thread_id,run_id,sequence,role,content,created_at`, chatUUID(input.MessageID), chatUUID(input.ThreadID),
		chatUUID(input.ID), messageSequence, input.Content, input.Now))
	if err != nil {
		return RunCreation{}, fmt.Errorf("insert AI Chat user message: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE ai_chat_threads SET next_message_sequence=next_message_sequence+1,updated_at=$2 WHERE id=$1`, chatUUID(input.ThreadID), input.Now); err != nil {
		return RunCreation{}, fmt.Errorf("advance AI Chat message sequence: %w", err)
	}
	if _, err := appendEventTx(ctx, tx, RunEvent{RunID: input.ID, Kind: EventRunAccepted, CreatedAt: input.Now}); err != nil {
		return RunCreation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RunCreation{}, normalizePostgresError(err)
	}
	return RunCreation{Run: run, UserMessage: message, Created: true}, nil
}

package aichat

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

type rowScanner interface{ Scan(...any) error }

const threadSelect = `SELECT id,owner_user_id,title,active_result_reference_id,
retention_expires_at,version,created_at,updated_at FROM ai_chat_threads`

const runSelect = `SELECT id,thread_id,owner_user_id,retry_of_run_id,idempotency_key,
request_fingerprint,state,tool_call_count,input_usage,output_usage,result_bytes,error_code,
cancel_requested_at,started_at,completed_at,version,created_at,updated_at FROM ai_chat_runs`

func (store *PostgresStore) CurrentUser(ctx context.Context, id auth.Identifier) (auth.User, error) {
	if store == nil || store.pool == nil || id == (auth.Identifier{}) {
		return auth.User{}, ErrForbidden
	}
	var value auth.User
	var databaseID pgtype.UUID
	var avatar pgtype.Text
	err := store.pool.QueryRow(ctx, `SELECT id,email,display_name,avatar_url,role,active
FROM app_users WHERE id=$1`, chatAuthUUID(id)).Scan(
		&databaseID, &value.Email, &value.DisplayName, &avatar, &value.Role, &value.Active,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, ErrForbidden
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("load current AI Chat user: %w", err)
	}
	value.ID = auth.Identifier(databaseID.Bytes)
	if avatar.Valid {
		value.AvatarURL = avatar.String
	}
	return value, nil
}

func (store *PostgresStore) CreateThread(ctx context.Context, input CreateThreadInput) (Thread, error) {
	if store == nil || store.pool == nil {
		return Thread{}, ErrInvalidSetup
	}
	return scanThread(store.pool.QueryRow(ctx, `INSERT INTO ai_chat_threads
(id,owner_user_id,title,retention_expires_at,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$5)
RETURNING id,owner_user_id,title,active_result_reference_id,retention_expires_at,version,created_at,updated_at`,
		chatUUID(input.ID), chatAuthUUID(input.OwnerUserID), input.Title, input.RetentionExpiresAt, input.Now))
}

func (store *PostgresStore) ListThreads(ctx context.Context, owner auth.Identifier, now time.Time, limit, offset int) (ThreadPage, error) {
	if store == nil || store.pool == nil {
		return ThreadPage{}, ErrInvalidSetup
	}
	rows, err := store.pool.Query(ctx, `SELECT id,owner_user_id,title,active_result_reference_id,
retention_expires_at,version,created_at,updated_at,count(*) OVER () AS total_count
FROM ai_chat_threads
WHERE owner_user_id=$1 AND retention_expires_at>$2
ORDER BY updated_at DESC,id DESC LIMIT $3 OFFSET $4`, chatAuthUUID(owner), now, limit, offset)
	if err != nil {
		return ThreadPage{}, fmt.Errorf("list AI Chat threads: %w", err)
	}
	defer rows.Close()
	page := ThreadPage{Threads: make([]Thread, 0), Limit: limit, Offset: offset}
	for rows.Next() {
		value, err := scanThreadWithTotal(rows, &page.Total)
		if err != nil {
			return ThreadPage{}, fmt.Errorf("scan AI Chat thread: %w", err)
		}
		page.Threads = append(page.Threads, value)
	}
	if err := rows.Err(); err != nil {
		return ThreadPage{}, fmt.Errorf("iterate AI Chat threads: %w", err)
	}
	return page, nil
}

func (store *PostgresStore) GetThread(ctx context.Context, id Identifier, owner auth.Identifier) (Thread, error) {
	if store == nil || store.pool == nil {
		return Thread{}, ErrInvalidSetup
	}
	value, err := scanThread(store.pool.QueryRow(ctx, threadSelect+` WHERE id=$1 AND owner_user_id=$2`, chatUUID(id), chatAuthUUID(owner)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Thread{}, ErrNotFound
	}
	if err != nil {
		return Thread{}, fmt.Errorf("load AI Chat thread: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) RenameThread(ctx context.Context, id Identifier, owner auth.Identifier, title string, version int64, now time.Time) (Thread, error) {
	value, err := scanThread(store.pool.QueryRow(ctx, `UPDATE ai_chat_threads SET title=$3,version=version+1,updated_at=$4
WHERE id=$1 AND owner_user_id=$2 AND version=$5
RETURNING id,owner_user_id,title,active_result_reference_id,retention_expires_at,version,created_at,updated_at`,
		chatUUID(id), chatAuthUUID(owner), title, now, version))
	if errors.Is(err, pgx.ErrNoRows) {
		_, loadErr := store.GetThread(ctx, id, owner)
		if loadErr == nil {
			return Thread{}, ErrConflict
		}
		if errors.Is(loadErr, ErrNotFound) {
			return Thread{}, ErrNotFound
		}
		return Thread{}, loadErr
	}
	if err != nil {
		return Thread{}, fmt.Errorf("rename AI Chat thread: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) DeleteThread(ctx context.Context, id Identifier, owner auth.Identifier) error {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin AI Chat thread deletion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var locked pgtype.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM ai_chat_threads WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, chatUUID(id), chatAuthUUID(owner)).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock AI Chat thread for deletion: %w", err)
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ai_chat_runs WHERE thread_id=$1 AND state IN ('QUEUED','RUNNING','TOOL_RUNNING'))`, chatUUID(id)).Scan(&active); err != nil {
		return fmt.Errorf("check active AI Chat run: %w", err)
	}
	if active {
		return ErrConflict
	}
	if _, err := tx.Exec(ctx, `DELETE FROM ai_chat_threads WHERE id=$1`, chatUUID(id)); err != nil {
		return fmt.Errorf("delete AI Chat thread: %w", err)
	}
	return normalizePostgresError(tx.Commit(ctx))
}

func (store *PostgresStore) ListMessages(ctx context.Context, threadID Identifier, owner auth.Identifier, limit, offset int) (MessagePage, error) {
	rows, err := store.pool.Query(ctx, `SELECT message.id,message.thread_id,message.run_id,message.sequence,message.role,message.content,message.created_at,
COALESCE((SELECT jsonb_agg(reference.id ORDER BY reference.created_at,reference.id)
  FROM ai_chat_result_references reference
  WHERE reference.run_id=message.run_id AND message.role='ASSISTANT'),'[]'::jsonb),
count(*) OVER () AS total_count
FROM ai_chat_messages message
JOIN ai_chat_threads thread ON thread.id=message.thread_id AND thread.owner_user_id=$2
WHERE message.thread_id=$1
ORDER BY message.sequence,message.id LIMIT $3 OFFSET $4`, chatUUID(threadID), chatAuthUUID(owner), limit, offset)
	if err != nil {
		return MessagePage{}, fmt.Errorf("list AI Chat messages: %w", err)
	}
	defer rows.Close()
	page := MessagePage{Messages: make([]Message, 0), Limit: limit, Offset: offset}
	for rows.Next() {
		value, err := scanMessageWithTotal(rows, &page.Total)
		if err != nil {
			return MessagePage{}, fmt.Errorf("scan AI Chat message: %w", err)
		}
		page.Messages = append(page.Messages, value)
	}
	if err := rows.Err(); err != nil {
		return MessagePage{}, fmt.Errorf("iterate AI Chat messages: %w", err)
	}
	if page.Total == 0 && len(page.Messages) == 0 {
		if _, err := store.GetThread(ctx, threadID, owner); err != nil {
			return MessagePage{}, err
		}
	}
	return page, nil
}

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

func (store *PostgresStore) GetRun(ctx context.Context, id Identifier, owner auth.Identifier) (Run, error) {
	run, err := scanRun(store.pool.QueryRow(ctx, runSelect+` WHERE id=$1 AND owner_user_id=$2`, chatUUID(id), chatAuthUUID(owner)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	if err != nil {
		return Run{}, fmt.Errorf("load AI Chat run: %w", err)
	}
	return run, nil
}

func (store *PostgresStore) RequestCancellation(ctx context.Context, id Identifier, owner auth.Identifier, now time.Time) (Run, error) {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Run{}, fmt.Errorf("begin AI Chat cancellation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	run, err := lockRun(ctx, tx, id, owner)
	if err != nil {
		return Run{}, err
	}
	if run.State.Terminal() {
		if err := tx.Commit(ctx); err != nil {
			return Run{}, normalizePostgresError(err)
		}
		return run, nil
	}
	if run.State == RunQueued {
		run, err = transitionRunTerminal(ctx, tx, run, RunCancelled, "cancelled", now)
		if err != nil {
			return Run{}, err
		}
	} else {
		run, err = scanRun(tx.QueryRow(ctx, `UPDATE ai_chat_runs SET cancel_requested_at=COALESCE(cancel_requested_at,$3),version=version+1,updated_at=$3
WHERE id=$1 AND owner_user_id=$2
RETURNING id,thread_id,owner_user_id,retry_of_run_id,idempotency_key,request_fingerprint,state,
tool_call_count,input_usage,output_usage,result_bytes,error_code,cancel_requested_at,started_at,completed_at,version,created_at,updated_at`,
			chatUUID(id), chatAuthUUID(owner), now))
		if err != nil {
			return Run{}, fmt.Errorf("request AI Chat cancellation: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, normalizePostgresError(err)
	}
	return run, nil
}

func (store *PostgresStore) StartRun(ctx context.Context, id Identifier, owner auth.Identifier, now time.Time) (Run, error) {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Run{}, fmt.Errorf("begin AI Chat run start: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	run, err := lockRun(ctx, tx, id, owner)
	if err != nil {
		return Run{}, err
	}
	if run.State == RunCancelled || run.CancelRequestedAt != nil {
		return Run{}, ErrCancelled
	}
	if run.State != RunQueued {
		return Run{}, ErrInvalidState
	}
	run, err = scanRun(tx.QueryRow(ctx, `UPDATE ai_chat_runs SET state='RUNNING',started_at=$3,version=version+1,updated_at=$3
WHERE id=$1 AND owner_user_id=$2 AND state='QUEUED'
RETURNING id,thread_id,owner_user_id,retry_of_run_id,idempotency_key,request_fingerprint,state,
tool_call_count,input_usage,output_usage,result_bytes,error_code,cancel_requested_at,started_at,completed_at,version,created_at,updated_at`,
		chatUUID(id), chatAuthUUID(owner), now))
	if err != nil {
		return Run{}, normalizePostgresError(fmt.Errorf("start AI Chat run: %w", err))
	}
	if _, err := appendEventTx(ctx, tx, RunEvent{RunID: id, Kind: EventRunStarted, CreatedAt: now}); err != nil {
		return Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, normalizePostgresError(err)
	}
	return run, nil
}

func (store *PostgresStore) AppendTextDelta(ctx context.Context, id Identifier, owner auth.Identifier, delta string, now time.Time) (RunEvent, error) {
	// Hot-path: chamado a cada delta de streaming. Usa CTE atômica para alocar
	// a sequência e inserir o evento sem abrir uma transação explícita nem fazer
	// um SELECT FOR UPDATE separado. O WHERE state='RUNNING' garante a mesma
	// invariante de estado que o lockRun anterior.
	var event RunEvent
	err := store.pool.QueryRow(ctx, `
WITH seq AS (
  UPDATE ai_chat_runs
  SET next_event_sequence = next_event_sequence + 1
  WHERE id = $1 AND owner_user_id = $2
    AND state = 'RUNNING' AND cancel_requested_at IS NULL
  RETURNING next_event_sequence - 1 AS seq
),
ins AS (
  INSERT INTO ai_chat_run_events (run_id, sequence, event_kind, text_delta, created_at)
  SELECT $1, seq.seq, 'TEXT_DELTA', $3, $4 FROM seq
  RETURNING run_id, sequence, event_kind, text_delta, tool_step_id,
            result_reference_id, error_code, created_at
)
SELECT run_id, sequence, event_kind, text_delta, tool_step_id,
       result_reference_id, error_code, created_at FROM ins`,
		chatUUID(id), chatAuthUUID(owner), delta, now,
	).Scan(&event.RunID, &event.Sequence, &event.Kind, &event.TextDelta,
		new(pgtype.UUID), new(pgtype.UUID), new(pgtype.Text), &event.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// A CTE não retornou linhas: o run não está em estado RUNNING ou foi cancelado.
		// Distingue entre not-found e cancelled consultando o estado atual.
		run, loadErr := store.GetRun(ctx, id, owner)
		if loadErr != nil {
			return RunEvent{}, loadErr
		}
		if run.CancelRequestedAt != nil || run.State == RunCancelled {
			return RunEvent{}, ErrCancelled
		}
		return RunEvent{}, ErrInvalidState
	}
	if err != nil {
		return RunEvent{}, normalizePostgresError(fmt.Errorf("append AI Chat text delta: %w", err))
	}
	return event, nil
}

func (store *PostgresStore) AddRunUsage(ctx context.Context, id Identifier, owner auth.Identifier, inputUsage, outputUsage, maximum int64, now time.Time) (Run, bool, error) {
	if inputUsage < 0 || outputUsage < 0 || maximum < 1 {
		return Run{}, false, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Run{}, false, fmt.Errorf("begin AI Chat usage accounting: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	run, err := lockRun(ctx, tx, id, owner)
	if err != nil {
		return Run{}, false, err
	}
	if !run.State.Active() {
		return Run{}, false, ErrInvalidState
	}
	var windowInput, windowOutput int64
	if err := tx.QueryRow(ctx, `UPDATE ai_chat_usage_windows SET input_usage=input_usage+$2,output_usage=output_usage+$3,updated_at=$4
WHERE owner_user_id=$1 RETURNING input_usage,output_usage`, chatAuthUUID(owner), inputUsage, outputUsage, now).Scan(&windowInput, &windowOutput); err != nil {
		return Run{}, false, fmt.Errorf("account AI Chat usage window: %w", err)
	}
	run, err = scanRun(tx.QueryRow(ctx, `UPDATE ai_chat_runs SET input_usage=input_usage+$3,output_usage=output_usage+$4,version=version+1,updated_at=$5
WHERE id=$1 AND owner_user_id=$2
RETURNING id,thread_id,owner_user_id,retry_of_run_id,idempotency_key,request_fingerprint,state,
tool_call_count,input_usage,output_usage,result_bytes,error_code,cancel_requested_at,started_at,completed_at,version,created_at,updated_at`,
		chatUUID(id), chatAuthUUID(owner), inputUsage, outputUsage, now))
	if err != nil {
		return Run{}, false, fmt.Errorf("account AI Chat run usage: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, false, normalizePostgresError(err)
	}
	return run, windowInput+windowOutput > maximum, nil
}

func (store *PostgresStore) BeginTool(ctx context.Context, input BeginToolInput) (ToolStep, error) {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ToolStep{}, fmt.Errorf("begin AI Chat tool step: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	run, err := lockRun(ctx, tx, input.RunID, input.OwnerUserID)
	if err != nil {
		return ToolStep{}, err
	}
	if run.CancelRequestedAt != nil {
		return ToolStep{}, ErrCancelled
	}
	if run.State != RunRunning {
		return ToolStep{}, ErrInvalidState
	}
	if run.ToolCallCount >= MaximumToolCalls {
		return ToolStep{}, ErrQuotaExceeded
	}
	sequence := run.ToolCallCount + 1
	step, err := scanToolStep(tx.QueryRow(ctx, `INSERT INTO ai_chat_tool_steps
(id,run_id,sequence,tool_kind,state,arguments_fingerprint,started_at)
VALUES($1,$2,$3,$4,'RUNNING',$5,$6)
RETURNING id,run_id,sequence,tool_kind,state,arguments_fingerprint,result_reference_id,row_count,result_bytes,error_code,started_at,completed_at`,
		chatUUID(input.ID), chatUUID(input.RunID), sequence, input.Kind, input.ArgumentsFingerprint[:], input.Now))
	if err != nil {
		return ToolStep{}, normalizePostgresError(fmt.Errorf("insert AI Chat tool step: %w", err))
	}
	if _, err := tx.Exec(ctx, `UPDATE ai_chat_runs SET state='TOOL_RUNNING',tool_call_count=$2,version=version+1,updated_at=$3 WHERE id=$1`,
		chatUUID(input.RunID), sequence, input.Now); err != nil {
		return ToolStep{}, fmt.Errorf("mark AI Chat tool running: %w", err)
	}
	stepID := input.ID
	if _, err := appendEventTx(ctx, tx, RunEvent{RunID: input.RunID, Kind: EventToolStarted, ToolStepID: &stepID, CreatedAt: input.Now}); err != nil {
		return ToolStep{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ToolStep{}, normalizePostgresError(err)
	}
	return step, nil
}

func (store *PostgresStore) CompleteTool(ctx context.Context, input CompleteToolInput) (ToolStep, *ResultReference, error) {
	if input.RowCount < 0 || input.RowCount > MaximumToolRows || input.ResultBytes < 0 || input.ResultBytes > MaximumToolResultBytes {
		return ToolStep{}, nil, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ToolStep{}, nil, fmt.Errorf("begin AI Chat tool completion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	run, err := lockRun(ctx, tx, input.RunID, input.OwnerUserID)
	if err != nil {
		return ToolStep{}, nil, err
	}
	if run.CancelRequestedAt != nil {
		return ToolStep{}, nil, ErrCancelled
	}
	if run.State != RunToolRunning {
		return ToolStep{}, nil, ErrInvalidState
	}
	if input.ResultBytes < 0 || run.ResultBytes+int64(input.ResultBytes) > MaximumToolResultBytes {
		return ToolStep{}, nil, ErrQuotaExceeded
	}
	var reference *ResultReference
	var referenceID *Identifier
	if input.ResultReference != nil {
		value, err := insertResultReference(ctx, tx, *input.ResultReference)
		if err != nil {
			return ToolStep{}, nil, err
		}
		reference = &value
		referenceID = &value.ID
	}
	step, err := scanToolStep(tx.QueryRow(ctx, `UPDATE ai_chat_tool_steps SET state='COMPLETED',result_reference_id=$4,row_count=$5,result_bytes=$6,completed_at=$7
WHERE id=$1 AND run_id=$2 AND state='RUNNING' AND EXISTS(SELECT 1 FROM ai_chat_runs WHERE id=$2 AND owner_user_id=$3)
RETURNING id,run_id,sequence,tool_kind,state,arguments_fingerprint,result_reference_id,row_count,result_bytes,error_code,started_at,completed_at`,
		chatUUID(input.StepID), chatUUID(input.RunID), chatAuthUUID(input.OwnerUserID), optionalChatUUID(referenceID), input.RowCount, input.ResultBytes, input.Now))
	if errors.Is(err, pgx.ErrNoRows) {
		return ToolStep{}, nil, ErrInvalidState
	}
	if err != nil {
		return ToolStep{}, nil, fmt.Errorf("complete AI Chat tool step: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE ai_chat_runs SET state='RUNNING',result_bytes=result_bytes+$2,version=version+1,updated_at=$3 WHERE id=$1`,
		chatUUID(input.RunID), input.ResultBytes, input.Now); err != nil {
		return ToolStep{}, nil, normalizePostgresError(fmt.Errorf("resume AI Chat after tool: %w", err))
	}
	if _, err := tx.Exec(ctx, `UPDATE ai_chat_usage_windows SET tool_call_count=tool_call_count+1,result_bytes=result_bytes+$2,updated_at=$3 WHERE owner_user_id=$1`,
		chatAuthUUID(input.OwnerUserID), input.ResultBytes, input.Now); err != nil {
		return ToolStep{}, nil, fmt.Errorf("account AI Chat tool usage: %w", err)
	}
	stepID := input.StepID
	if _, err := appendEventTx(ctx, tx, RunEvent{RunID: input.RunID, Kind: EventToolCompleted, ToolStepID: &stepID, CreatedAt: input.Now}); err != nil {
		return ToolStep{}, nil, err
	}
	if reference != nil {
		if _, err := tx.Exec(ctx, `UPDATE ai_chat_threads SET active_result_reference_id=$2,version=version+1,updated_at=$3 WHERE id=$1`,
			chatUUID(run.ThreadID), chatUUID(reference.ID), input.Now); err != nil {
			return ToolStep{}, nil, fmt.Errorf("activate AI Chat result reference: %w", err)
		}
		if _, err := appendEventTx(ctx, tx, RunEvent{RunID: input.RunID, Kind: EventResultReference, ResultReferenceID: referenceID, CreatedAt: input.Now}); err != nil {
			return ToolStep{}, nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ToolStep{}, nil, normalizePostgresError(err)
	}
	return step, reference, nil
}

func (store *PostgresStore) FailTool(ctx context.Context, stepID, runID Identifier, owner auth.Identifier, code string, now time.Time) (ToolStep, error) {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ToolStep{}, fmt.Errorf("begin AI Chat tool failure: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	run, err := lockRun(ctx, tx, runID, owner)
	if err != nil {
		return ToolStep{}, err
	}
	if run.State != RunToolRunning {
		return ToolStep{}, ErrInvalidState
	}
	state := ToolStepFailed
	if run.CancelRequestedAt != nil {
		state, code = ToolStepCancelled, "cancelled"
	}
	step, err := scanToolStep(tx.QueryRow(ctx, `UPDATE ai_chat_tool_steps SET state=$4,error_code=$5,completed_at=$6
WHERE id=$1 AND run_id=$2 AND state='RUNNING' AND EXISTS(SELECT 1 FROM ai_chat_runs WHERE id=$2 AND owner_user_id=$3)
RETURNING id,run_id,sequence,tool_kind,state,arguments_fingerprint,result_reference_id,row_count,result_bytes,error_code,started_at,completed_at`,
		chatUUID(stepID), chatUUID(runID), chatAuthUUID(owner), state, code, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return ToolStep{}, ErrInvalidState
	}
	if err != nil {
		return ToolStep{}, fmt.Errorf("fail AI Chat tool step: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE ai_chat_runs SET state='RUNNING',version=version+1,updated_at=$2 WHERE id=$1`, chatUUID(runID), now); err != nil {
		return ToolStep{}, fmt.Errorf("resume failed AI Chat tool: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ToolStep{}, normalizePostgresError(err)
	}
	return step, nil
}

func (store *PostgresStore) CompleteRun(ctx context.Context, input CompleteRunInput) (Run, Message, error) {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Run{}, Message{}, fmt.Errorf("begin AI Chat run completion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	run, err := lockRun(ctx, tx, input.RunID, input.OwnerUserID)
	if err != nil {
		return Run{}, Message{}, err
	}
	if run.CancelRequestedAt != nil {
		if _, transitionErr := transitionRunTerminal(ctx, tx, run, RunCancelled, "cancelled", input.Now); transitionErr != nil {
			return Run{}, Message{}, transitionErr
		}
		if err := tx.Commit(ctx); err != nil {
			return Run{}, Message{}, normalizePostgresError(err)
		}
		return Run{}, Message{}, ErrCancelled
	}
	if run.State != RunRunning {
		return Run{}, Message{}, ErrInvalidState
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `SELECT next_message_sequence FROM ai_chat_threads WHERE id=$1 FOR UPDATE`, chatUUID(run.ThreadID)).Scan(&sequence); err != nil {
		return Run{}, Message{}, fmt.Errorf("lock AI Chat assistant message sequence: %w", err)
	}
	message, err := scanMessage(tx.QueryRow(ctx, `INSERT INTO ai_chat_messages
(id,thread_id,run_id,sequence,role,content,created_at) VALUES($1,$2,$3,$4,'ASSISTANT',$5,$6)
RETURNING id,thread_id,run_id,sequence,role,content,created_at`, chatUUID(input.MessageID), chatUUID(run.ThreadID),
		chatUUID(input.RunID), sequence, input.Content, input.Now))
	if err != nil {
		return Run{}, Message{}, normalizePostgresError(fmt.Errorf("insert AI Chat assistant message: %w", err))
	}
	if _, err := tx.Exec(ctx, `UPDATE ai_chat_threads SET next_message_sequence=next_message_sequence+1,updated_at=$2 WHERE id=$1`, chatUUID(run.ThreadID), input.Now); err != nil {
		return Run{}, Message{}, fmt.Errorf("advance AI Chat assistant message sequence: %w", err)
	}
	run, err = scanRun(tx.QueryRow(ctx, `UPDATE ai_chat_runs SET state='COMPLETED',input_usage=$3,output_usage=$4,completed_at=$5,version=version+1,updated_at=$5
WHERE id=$1 AND owner_user_id=$2 AND state='RUNNING' AND cancel_requested_at IS NULL
RETURNING id,thread_id,owner_user_id,retry_of_run_id,idempotency_key,request_fingerprint,state,
tool_call_count,input_usage,output_usage,result_bytes,error_code,cancel_requested_at,started_at,completed_at,version,created_at,updated_at`,
		chatUUID(input.RunID), chatAuthUUID(input.OwnerUserID), input.InputUsage, input.OutputUsage, input.Now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, Message{}, ErrConflict
	}
	if err != nil {
		return Run{}, Message{}, fmt.Errorf("complete AI Chat run: %w", err)
	}
	if _, err := appendEventTx(ctx, tx, RunEvent{RunID: input.RunID, Kind: EventRunCompleted, CreatedAt: input.Now}); err != nil {
		return Run{}, Message{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, Message{}, normalizePostgresError(err)
	}
	return run, message, nil
}

func (store *PostgresStore) FailRun(ctx context.Context, id Identifier, owner auth.Identifier, code string, now time.Time) (Run, error) {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Run{}, fmt.Errorf("begin AI Chat run failure: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	run, err := lockRun(ctx, tx, id, owner)
	if err != nil {
		return Run{}, err
	}
	if run.State.Terminal() {
		if err := tx.Commit(ctx); err != nil {
			return Run{}, normalizePostgresError(err)
		}
		return run, nil
	}
	state := RunFailed
	if run.CancelRequestedAt != nil {
		state, code = RunCancelled, "cancelled"
	}
	run, err = transitionRunTerminal(ctx, tx, run, state, code, now)
	if err != nil {
		return Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, normalizePostgresError(err)
	}
	return run, nil
}

func (store *PostgresStore) FailStaleRuns(ctx context.Context, before, now time.Time, limit int) (int, error) {
	if store == nil || store.pool == nil || before.IsZero() || now.IsZero() || !before.Before(now) || limit < 1 {
		return 0, ErrInvalidInput
	}
	command, err := store.pool.Exec(ctx, `WITH victims AS (
  SELECT run.id FROM ai_chat_runs run
  WHERE run.state IN ('QUEUED','RUNNING','TOOL_RUNNING') AND run.created_at<=$1
  ORDER BY run.created_at,run.id FOR UPDATE SKIP LOCKED LIMIT $3
), failed_steps AS (
  UPDATE ai_chat_tool_steps step SET
    state=CASE WHEN run.cancel_requested_at IS NULL THEN 'FAILED' ELSE 'CANCELLED' END,
    error_code=CASE WHEN run.cancel_requested_at IS NULL THEN 'timeout' ELSE 'cancelled' END,
    completed_at=COALESCE(step.completed_at,$2)
  FROM ai_chat_runs run,victims
  WHERE step.run_id=run.id AND run.id=victims.id AND step.state='RUNNING'
  RETURNING step.id
), terminal AS (
  UPDATE ai_chat_runs run SET
    state=CASE WHEN run.cancel_requested_at IS NULL THEN 'FAILED' ELSE 'CANCELLED' END,
    error_code=CASE WHEN run.cancel_requested_at IS NULL THEN 'timeout' ELSE 'cancelled' END,
    completed_at=$2,updated_at=$2,version=run.version+1,
    next_event_sequence=run.next_event_sequence+1
  FROM victims
  WHERE run.id=victims.id AND run.state IN ('QUEUED','RUNNING','TOOL_RUNNING')
  RETURNING run.id,run.next_event_sequence-1 AS sequence,run.state,run.error_code
)
INSERT INTO ai_chat_run_events(run_id,sequence,event_kind,error_code,created_at)
SELECT id,sequence,CASE WHEN state='CANCELLED' THEN 'RUN_CANCELLED' ELSE 'RUN_FAILED' END,error_code,$2
FROM terminal`, before, now, limit)
	if err != nil {
		return 0, fmt.Errorf("recover stale AI Chat runs: %w", err)
	}
	return int(command.RowsAffected()), nil
}

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

func scanThread(row rowScanner) (Thread, error) {
	var value Thread
	var id, owner, active pgtype.UUID
	if err := row.Scan(&id, &owner, &value.Title, &active, &value.RetentionExpiresAt, &value.Version, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return Thread{}, err
	}
	value.ID, value.OwnerUserID = chatIdentifier(id), auth.Identifier(owner.Bytes)
	value.ActiveResultReferenceID = optionalIdentifier(active)
	return value, nil
}

// scanThreadWithTotal lê a coluna extra count(*) OVER () emitida pela window
// function em ListThreads, eliminando a segunda query de COUNT.
func scanThreadWithTotal(row rowScanner, total *int) (Thread, error) {
	var value Thread
	var id, owner, active pgtype.UUID
	if err := row.Scan(&id, &owner, &value.Title, &active, &value.RetentionExpiresAt, &value.Version, &value.CreatedAt, &value.UpdatedAt, total); err != nil {
		return Thread{}, err
	}
	value.ID, value.OwnerUserID = chatIdentifier(id), auth.Identifier(owner.Bytes)
	value.ActiveResultReferenceID = optionalIdentifier(active)
	return value, nil
}

func scanRun(row rowScanner) (Run, error) {
	var value Run
	var id, thread, owner, retry pgtype.UUID
	var fingerprint []byte
	var errorCode pgtype.Text
	var cancel, started, completed pgtype.Timestamptz
	if err := row.Scan(&id, &thread, &owner, &retry, &value.IdempotencyKey, &fingerprint, &value.State,
		&value.ToolCallCount, &value.InputUsage, &value.OutputUsage, &value.ResultBytes, &errorCode,
		&cancel, &started, &completed, &value.Version, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return Run{}, err
	}
	if len(fingerprint) != 32 {
		return Run{}, ErrInvalidState
	}
	value.ID, value.ThreadID, value.OwnerUserID = chatIdentifier(id), chatIdentifier(thread), auth.Identifier(owner.Bytes)
	value.RetryOfRunID = optionalIdentifier(retry)
	copy(value.RequestFingerprint[:], fingerprint)
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	if cancel.Valid {
		value.CancelRequestedAt = &cancel.Time
	}
	if started.Valid {
		value.StartedAt = &started.Time
	}
	if completed.Valid {
		value.CompletedAt = &completed.Time
	}
	return value, nil
}

func scanMessage(row rowScanner) (Message, error) {
	var value Message
	var id, thread, run pgtype.UUID
	if err := row.Scan(&id, &thread, &run, &value.Sequence, &value.Role, &value.Content, &value.CreatedAt); err != nil {
		return Message{}, err
	}
	value.ID, value.ThreadID, value.RunID = chatIdentifier(id), chatIdentifier(thread), chatIdentifier(run)
	return value, nil
}

func scanMessageWithReferences(row rowScanner) (Message, error) {
	var value Message
	var id, thread, run pgtype.UUID
	var encodedReferences []byte
	if err := row.Scan(&id, &thread, &run, &value.Sequence, &value.Role, &value.Content, &value.CreatedAt, &encodedReferences); err != nil {
		return Message{}, err
	}
	value.ID, value.ThreadID, value.RunID = chatIdentifier(id), chatIdentifier(thread), chatIdentifier(run)
	var references []string
	if err := json.Unmarshal(encodedReferences, &references); err != nil || len(references) > MaximumToolCalls {
		return Message{}, ErrInvalidState
	}
	value.ResultReferenceIDs = make([]Identifier, 0, len(references))
	for _, raw := range references {
		reference, err := ParseIdentifier(raw)
		if err != nil {
			return Message{}, ErrInvalidState
		}
		value.ResultReferenceIDs = append(value.ResultReferenceIDs, reference)
	}
	return value, nil
}

// scanMessageWithTotal lê a coluna extra count(*) OVER () emitida pela window
// function em ListMessages, eliminando a segunda query de COUNT.
func scanMessageWithTotal(row rowScanner, total *int) (Message, error) {
	var value Message
	var id, thread, run pgtype.UUID
	var encodedReferences []byte
	if err := row.Scan(&id, &thread, &run, &value.Sequence, &value.Role, &value.Content, &value.CreatedAt, &encodedReferences, total); err != nil {
		return Message{}, err
	}
	value.ID, value.ThreadID, value.RunID = chatIdentifier(id), chatIdentifier(thread), chatIdentifier(run)
	var references []string
	if err := json.Unmarshal(encodedReferences, &references); err != nil || len(references) > MaximumToolCalls {
		return Message{}, ErrInvalidState
	}
	value.ResultReferenceIDs = make([]Identifier, 0, len(references))
	for _, raw := range references {
		reference, err := ParseIdentifier(raw)
		if err != nil {
			return Message{}, ErrInvalidState
		}
		value.ResultReferenceIDs = append(value.ResultReferenceIDs, reference)
	}
	return value, nil
}

func scanToolStep(row rowScanner) (ToolStep, error) {
	var value ToolStep
	var id, run, reference pgtype.UUID
	var fingerprint []byte
	var errorCode pgtype.Text
	var completed pgtype.Timestamptz
	if err := row.Scan(&id, &run, &value.Sequence, &value.Kind, &value.State, &fingerprint, &reference,
		&value.RowCount, &value.ResultBytes, &errorCode, &value.StartedAt, &completed); err != nil {
		return ToolStep{}, err
	}
	if len(fingerprint) != 32 {
		return ToolStep{}, ErrInvalidState
	}
	value.ID, value.RunID = chatIdentifier(id), chatIdentifier(run)
	copy(value.ArgumentsFingerprint[:], fingerprint)
	value.ResultReferenceID = optionalIdentifier(reference)
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	if completed.Valid {
		value.CompletedAt = &completed.Time
	}
	return value, nil
}

func scanResultReference(row rowScanner) (ResultReference, error) {
	var value ResultReference
	var id, thread, run, owner, execution pgtype.UUID
	var fingerprint []byte
	if err := row.Scan(&id, &thread, &run, &owner, &value.Kind, &execution, &value.LogicalRequest, &fingerprint,
		&value.Label, &value.RowCount, &value.ColumnCount, &value.ExpiresAt, &value.CreatedAt); err != nil {
		return ResultReference{}, err
	}
	if len(fingerprint) != 32 {
		return ResultReference{}, ErrInvalidState
	}
	value.ID, value.ThreadID, value.RunID = chatIdentifier(id), chatIdentifier(thread), chatIdentifier(run)
	value.OwnerUserID, value.QueryExecutionID = auth.Identifier(owner.Bytes), optionalIdentifier(execution)
	copy(value.ContextFingerprint[:], fingerprint)
	return value, nil
}

func scanRunEvent(row rowScanner) (RunEvent, error) {
	var value RunEvent
	var run, tool, reference pgtype.UUID
	var text, errorCode pgtype.Text
	if err := row.Scan(&run, &value.Sequence, &value.Kind, &text, &tool, &reference, &errorCode, &value.CreatedAt); err != nil {
		return RunEvent{}, err
	}
	value.RunID, value.ToolStepID, value.ResultReferenceID = chatIdentifier(run), optionalIdentifier(tool), optionalIdentifier(reference)
	if text.Valid {
		value.TextDelta = text.String
	}
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	return value, nil
}

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

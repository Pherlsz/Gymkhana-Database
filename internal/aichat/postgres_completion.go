package aichat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

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

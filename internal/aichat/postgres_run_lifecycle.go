package aichat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

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
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return RunEvent{}, fmt.Errorf("begin AI Chat text event: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	run, err := lockRun(ctx, tx, id, owner)
	if err != nil {
		return RunEvent{}, err
	}
	if run.CancelRequestedAt != nil || run.State == RunCancelled {
		return RunEvent{}, ErrCancelled
	}
	if run.State != RunRunning {
		return RunEvent{}, ErrInvalidState
	}
	event, err := appendEventTx(ctx, tx, RunEvent{RunID: id, Kind: EventTextDelta, TextDelta: delta, CreatedAt: now})
	if err != nil {
		return RunEvent{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RunEvent{}, normalizePostgresError(err)
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

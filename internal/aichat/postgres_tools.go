package aichat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

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

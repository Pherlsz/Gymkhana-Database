package ocr

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) ListSuggestions(ctx context.Context, jobID Identifier, owner auth.Identifier, limit, offset int) ([]Suggestion, int, error) {
	if store == nil || store.pool == nil || jobID.IsZero() || owner == (auth.Identifier{}) ||
		limit < 1 || limit > MaximumSuggestions || offset < 0 {
		return nil, 0, ErrInvalidInput
	}
	where := ` WHERE job_id=$1 AND EXISTS (
  SELECT 1 FROM ocr_jobs job WHERE job.id=ocr_suggestions.job_id AND job.owner_user_id=$2
)`
	rows, err := store.pool.Query(ctx, `SELECT `+suggestionColumns+` FROM ocr_suggestions`+where+`
ORDER BY ordinal,id LIMIT $3 OFFSET $4`, databaseUUID(jobID), authDatabaseUUID(owner), limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list OCR suggestions: %w", err)
	}
	defer rows.Close()
	values := make([]Suggestion, 0)
	for rows.Next() {
		value, err := scanSuggestion(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan OCR suggestion: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate OCR suggestions: %w", err)
	}
	var total int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM ocr_suggestions`+where, databaseUUID(jobID), authDatabaseUUID(owner)).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count OCR suggestions: %w", err)
	}
	return values, total, nil
}

func (store *PostgresStore) GetSuggestion(ctx context.Context, id Identifier, owner auth.Identifier) (Suggestion, Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) {
		return Suggestion{}, Job{}, ErrNotFound
	}
	value, err := scanSuggestion(store.pool.QueryRow(ctx, `SELECT `+suggestionColumns+` FROM ocr_suggestions
WHERE id=$1 AND EXISTS (
  SELECT 1 FROM ocr_jobs job WHERE job.id=ocr_suggestions.job_id AND job.owner_user_id=$2
)`, databaseUUID(id), authDatabaseUUID(owner)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Suggestion{}, Job{}, ErrNotFound
	}
	if err != nil {
		return Suggestion{}, Job{}, fmt.Errorf("load OCR suggestion: %w", err)
	}
	job, err := store.GetJob(ctx, value.JobID, owner)
	if err != nil {
		return Suggestion{}, Job{}, err
	}
	return value, job, nil
}

func (store *PostgresStore) ReviewSuggestion(ctx context.Context, input ReviewSuggestionInput) (Suggestion, error) {
	if store == nil || store.pool == nil || input.SuggestionID.IsZero() || input.OwnerUserID == (auth.Identifier{}) ||
		(input.Action != ReviewAccept && input.Action != ReviewReject) || input.TargetVersion < 1 || input.Version < 1 || input.Now.IsZero() ||
		(input.Action == ReviewAccept && input.Value == nil) || (input.Action == ReviewReject && input.Value != nil) {
		return Suggestion{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Suggestion{}, fmt.Errorf("begin OCR suggestion review: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, err := scanSuggestion(tx.QueryRow(ctx, `SELECT `+suggestionColumns+` FROM ocr_suggestions
WHERE id=$1 AND EXISTS (
  SELECT 1 FROM ocr_jobs job WHERE job.id=ocr_suggestions.job_id AND job.owner_user_id=$2
) FOR UPDATE`, databaseUUID(input.SuggestionID), authDatabaseUUID(input.OwnerUserID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Suggestion{}, ErrNotFound
	}
	if err != nil {
		return Suggestion{}, fmt.Errorf("lock OCR suggestion review: %w", err)
	}
	if current.Version != input.Version {
		return Suggestion{}, ErrConflict
	}
	if current.ReviewState == ReviewApplied || current.ReviewState == ReviewStale {
		return Suggestion{}, ErrInvalidState
	}
	state := ReviewAccepted
	if input.Action == ReviewReject {
		state = ReviewRejected
	}
	updated, err := scanSuggestion(tx.QueryRow(ctx, `UPDATE ocr_suggestions
SET review_state=$2,reviewed_value=$3,reviewed_by_user_id=$4,reviewed_at=$5,applied_at=NULL,
    target_version=$6,version=version+1,updated_at=$5
WHERE id=$1 RETURNING `+suggestionColumns, databaseUUID(input.SuggestionID), state, input.Value,
		authDatabaseUUID(input.OwnerUserID), input.Now, input.TargetVersion))
	if err != nil {
		return Suggestion{}, normalizePostgresError(fmt.Errorf("review OCR suggestion: %w", err))
	}
	if err := tx.Commit(ctx); err != nil {
		return Suggestion{}, normalizePostgresError(err)
	}
	return updated, nil
}

func (store *PostgresStore) GetSuggestionsForApply(ctx context.Context, jobID Identifier, owner auth.Identifier, selections []ApplySelection) ([]Suggestion, error) {
	if store == nil || store.pool == nil || jobID.IsZero() || owner == (auth.Identifier{}) || len(selections) == 0 || len(selections) > MaximumSuggestions {
		return nil, ErrInvalidInput
	}
	job, err := store.GetJob(ctx, jobID, owner)
	if err != nil {
		return nil, err
	}
	if job.State != JobCompleted {
		return nil, ErrInvalidState
	}
	values := make([]Suggestion, 0, len(selections))
	for _, selection := range selections {
		if selection.SuggestionID.IsZero() || selection.Version < 1 {
			return nil, ErrInvalidInput
		}
		value, err := scanSuggestion(store.pool.QueryRow(ctx, `SELECT `+suggestionColumns+` FROM ocr_suggestions
WHERE id=$1 AND job_id=$2`, databaseUUID(selection.SuggestionID), databaseUUID(jobID)))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("load OCR suggestion for application: %w", err)
		}
		if value.Version != selection.Version {
			return nil, ErrConflict
		}
		if value.ReviewState != ReviewAccepted || value.ReviewedValue == nil {
			return nil, ErrInvalidState
		}
		values = append(values, value)
	}
	return values, nil
}

func (store *PostgresStore) CreateApplyReceipt(ctx context.Context, input CreateApplyReceiptInput) (ApplyReceipt, bool, error) {
	if store == nil || store.pool == nil || input.ID.IsZero() || input.JobID.IsZero() || input.OwnerUserID == (auth.Identifier{}) ||
		!validIdempotencyKey(input.IdempotencyKey) || input.Now.IsZero() {
		return ApplyReceipt{}, false, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ApplyReceipt{}, false, fmt.Errorf("begin OCR application receipt: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	existing, err := scanApplyReceipt(tx.QueryRow(ctx, `SELECT `+applyReceiptColumns+` FROM ocr_apply_receipts
WHERE owner_user_id=$1 AND idempotency_key=$2 FOR UPDATE`, authDatabaseUUID(input.OwnerUserID), input.IdempotencyKey))
	if err == nil {
		if subtle.ConstantTimeCompare(existing.RequestFingerprint[:], input.RequestFingerprint[:]) != 1 || existing.JobID != input.JobID {
			return ApplyReceipt{}, false, ErrConflict
		}
		existing.Results, err = listApplyResults(ctx, tx, existing.ID)
		if err != nil {
			return ApplyReceipt{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ApplyReceipt{}, false, normalizePostgresError(err)
		}
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ApplyReceipt{}, false, fmt.Errorf("load OCR application replay: %w", err)
	}
	var jobState JobState
	err = tx.QueryRow(ctx, `SELECT state FROM ocr_jobs WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`,
		databaseUUID(input.JobID), authDatabaseUUID(input.OwnerUserID)).Scan(&jobState)
	if errors.Is(err, pgx.ErrNoRows) {
		return ApplyReceipt{}, false, ErrNotFound
	}
	if err != nil {
		return ApplyReceipt{}, false, fmt.Errorf("lock OCR job for application: %w", err)
	}
	if jobState != JobCompleted {
		return ApplyReceipt{}, false, ErrInvalidState
	}
	created, err := scanApplyReceipt(tx.QueryRow(ctx, `INSERT INTO ocr_apply_receipts
(id,job_id,owner_user_id,idempotency_key,request_fingerprint,state,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,'RUNNING',$6,$6) RETURNING `+applyReceiptColumns,
		databaseUUID(input.ID), databaseUUID(input.JobID), authDatabaseUUID(input.OwnerUserID), input.IdempotencyKey,
		input.RequestFingerprint[:], input.Now))
	if err != nil {
		return ApplyReceipt{}, false, normalizePostgresError(fmt.Errorf("insert OCR application receipt: %w", err))
	}
	if err := tx.Commit(ctx); err != nil {
		return ApplyReceipt{}, false, normalizePostgresError(err)
	}
	return created, true, nil
}

func (store *PostgresStore) ResumeApplyReceipt(ctx context.Context, id Identifier, owner auth.Identifier, staleBefore, now time.Time) (ApplyReceipt, bool, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) || staleBefore.IsZero() || now.IsZero() || !staleBefore.Before(now) {
		return ApplyReceipt{}, false, ErrInvalidInput
	}
	value, err := scanApplyReceipt(store.pool.QueryRow(ctx, `UPDATE ocr_apply_receipts
SET updated_at=$4 WHERE id=$1 AND owner_user_id=$2 AND state='RUNNING' AND updated_at<$3
RETURNING `+applyReceiptColumns, databaseUUID(id), authDatabaseUUID(owner), staleBefore, now))
	if err == nil {
		value.Results, err = listApplyResults(ctx, store.pool, id)
		if err != nil {
			return ApplyReceipt{}, false, err
		}
		return value, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ApplyReceipt{}, false, fmt.Errorf("resume OCR application receipt: %w", err)
	}
	value, err = scanApplyReceipt(store.pool.QueryRow(ctx, `SELECT `+applyReceiptColumns+` FROM ocr_apply_receipts
WHERE id=$1 AND owner_user_id=$2`, databaseUUID(id), authDatabaseUUID(owner)))
	if errors.Is(err, pgx.ErrNoRows) {
		return ApplyReceipt{}, false, ErrNotFound
	}
	if err != nil {
		return ApplyReceipt{}, false, fmt.Errorf("load OCR application receipt: %w", err)
	}
	if value.State == ApplyCompleted {
		value.Results, err = listApplyResults(ctx, store.pool, id)
		if err != nil {
			return ApplyReceipt{}, false, err
		}
	}
	return value, false, nil
}

func (store *PostgresStore) SaveApplyResults(ctx context.Context, receiptID Identifier, inputs []ApplyResultInput, now time.Time) ([]ApplyResult, error) {
	if store == nil || store.pool == nil || receiptID.IsZero() || len(inputs) == 0 || len(inputs) > MaximumSuggestions || now.IsZero() {
		return nil, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin OCR application results: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	receipt, err := scanApplyReceipt(tx.QueryRow(ctx, `SELECT `+applyReceiptColumns+` FROM ocr_apply_receipts WHERE id=$1 FOR UPDATE`, databaseUUID(receiptID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock OCR application receipt: %w", err)
	}
	if receipt.State != ApplyRunning {
		return nil, ErrInvalidState
	}
	results := make([]ApplyResult, 0, len(inputs))
	for _, input := range inputs {
		if input.SuggestionID.IsZero() || (input.Outcome != ApplyApplied && input.Outcome != ApplyStale && input.Outcome != ApplyFailed) ||
			(input.Outcome == ApplyApplied && (input.TargetVersion == nil || *input.TargetVersion < 1 || input.ErrorCode != "")) ||
			(input.Outcome != ApplyApplied && (input.TargetVersion != nil || input.ErrorCode == "" || len(input.ErrorCode) > 80)) {
			return nil, ErrInvalidInput
		}
		existing, loadErr := scanApplyResult(tx.QueryRow(ctx, `SELECT receipt_id,suggestion_id,outcome,target_version,error_code,created_at
FROM ocr_apply_results WHERE receipt_id=$1 AND suggestion_id=$2`, databaseUUID(receiptID), databaseUUID(input.SuggestionID)))
		if loadErr == nil {
			if !sameApplyResult(existing, input) {
				return nil, ErrConflict
			}
			results = append(results, existing)
			continue
		}
		if !errors.Is(loadErr, pgx.ErrNoRows) {
			return nil, fmt.Errorf("load OCR application result replay: %w", loadErr)
		}
		var reviewState ReviewState
		err := tx.QueryRow(ctx, `SELECT review_state FROM ocr_suggestions
WHERE id=$1 AND job_id=$2 FOR UPDATE`, databaseUUID(input.SuggestionID), databaseUUID(receipt.JobID)).Scan(&reviewState)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("lock applied OCR suggestion: %w", err)
		}
		if reviewState != ReviewAccepted {
			return nil, ErrConflict
		}
		created, err := scanApplyResult(tx.QueryRow(ctx, `INSERT INTO ocr_apply_results
(receipt_id,suggestion_id,outcome,target_version,error_code,created_at)
VALUES($1,$2,$3,$4,$5,$6)
RETURNING receipt_id,suggestion_id,outcome,target_version,error_code,created_at`, databaseUUID(receiptID),
			databaseUUID(input.SuggestionID), input.Outcome, input.TargetVersion, optionalText(input.ErrorCode), now))
		if err != nil {
			return nil, normalizePostgresError(fmt.Errorf("insert OCR application result: %w", err))
		}
		switch input.Outcome {
		case ApplyApplied:
			_, err = tx.Exec(ctx, `UPDATE ocr_suggestions SET review_state='APPLIED',applied_at=$2,version=version+1,updated_at=$2
WHERE id=$1 AND review_state='ACCEPTED'`, databaseUUID(input.SuggestionID), now)
		case ApplyStale:
			_, err = tx.Exec(ctx, `UPDATE ocr_suggestions SET review_state='STALE',version=version+1,updated_at=$2
WHERE id=$1 AND review_state='ACCEPTED'`, databaseUUID(input.SuggestionID), now)
		}
		if err != nil {
			return nil, fmt.Errorf("advance OCR suggestion application state: %w", err)
		}
		results = append(results, created)
	}
	if _, err := tx.Exec(ctx, `UPDATE ocr_apply_receipts SET updated_at=$2 WHERE id=$1`, databaseUUID(receiptID), now); err != nil {
		return nil, fmt.Errorf("renew OCR application receipt: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, normalizePostgresError(err)
	}
	return results, nil
}

func (store *PostgresStore) CompleteApplyReceipt(ctx context.Context, id Identifier, owner auth.Identifier, now time.Time) (ApplyReceipt, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) || now.IsZero() {
		return ApplyReceipt{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ApplyReceipt{}, fmt.Errorf("begin OCR application completion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	value, err := scanApplyReceipt(tx.QueryRow(ctx, `SELECT `+applyReceiptColumns+` FROM ocr_apply_receipts
WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, databaseUUID(id), authDatabaseUUID(owner)))
	if errors.Is(err, pgx.ErrNoRows) {
		return ApplyReceipt{}, ErrNotFound
	}
	if err != nil {
		return ApplyReceipt{}, fmt.Errorf("lock OCR application completion: %w", err)
	}
	if value.State == ApplyRunning {
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM ocr_apply_results WHERE receipt_id=$1`, databaseUUID(id)).Scan(&count); err != nil {
			return ApplyReceipt{}, fmt.Errorf("count OCR application results: %w", err)
		}
		if count == 0 {
			return ApplyReceipt{}, ErrInvalidState
		}
		value, err = scanApplyReceipt(tx.QueryRow(ctx, `UPDATE ocr_apply_receipts
SET state='COMPLETED',completed_at=$3,updated_at=$3 WHERE id=$1 AND owner_user_id=$2
RETURNING `+applyReceiptColumns, databaseUUID(id), authDatabaseUUID(owner), now))
		if err != nil {
			return ApplyReceipt{}, fmt.Errorf("complete OCR application receipt: %w", err)
		}
	}
	value.Results, err = listApplyResults(ctx, tx, id)
	if err != nil {
		return ApplyReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ApplyReceipt{}, normalizePostgresError(err)
	}
	return value, nil
}

type applyResultQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func listApplyResults(ctx context.Context, query applyResultQuerier, receiptID Identifier) ([]ApplyResult, error) {
	rows, err := query.Query(ctx, `SELECT receipt_id,suggestion_id,outcome,target_version,error_code,created_at
FROM ocr_apply_results WHERE receipt_id=$1 ORDER BY created_at,suggestion_id`, databaseUUID(receiptID))
	if err != nil {
		return nil, fmt.Errorf("list OCR application results: %w", err)
	}
	defer rows.Close()
	values := make([]ApplyResult, 0)
	for rows.Next() {
		value, err := scanApplyResult(rows)
		if err != nil {
			return nil, fmt.Errorf("scan OCR application result: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate OCR application results: %w", err)
	}
	return values, nil
}

func sameApplyResult(existing ApplyResult, input ApplyResultInput) bool {
	if existing.SuggestionID != input.SuggestionID || existing.Outcome != input.Outcome || existing.ErrorCode != input.ErrorCode {
		return false
	}
	if existing.TargetVersion == nil || input.TargetVersion == nil {
		return existing.TargetVersion == nil && input.TargetVersion == nil
	}
	return *existing.TargetVersion == *input.TargetVersion
}

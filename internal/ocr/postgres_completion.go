package ocr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) CompleteJob(ctx context.Context, input CompleteJobInput) (Job, error) {
	if store == nil || store.pool == nil || input.JobID.IsZero() || input.PageCount < 1 || input.PageCount > MaximumPages ||
		input.PixelCount < 0 || input.PixelCount > MaximumPixels || len(input.Suggestions) > MaximumSuggestions || input.Now.IsZero() {
		return Job{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Job{}, fmt.Errorf("begin OCR completion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, sequence, err := lockJob(ctx, tx, input.JobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("lock OCR completion: %w", err)
	}
	if current.State != JobRunning || current.CancelRequestedAt != nil {
		return Job{}, ErrInvalidState
	}
	for _, suggestion := range input.Suggestions {
		if suggestion.ID.IsZero() || suggestion.Ordinal < 1 || suggestion.Ordinal > MaximumSuggestions ||
			!validFieldSchema(suggestion.Field) || suggestion.ProposedValue == "" {
			return Job{}, ErrInvalidInput
		}
		regionX, regionY, regionWidth, regionHeight := optionalRegion(suggestion.Evidence.Region)
		_, err := tx.Exec(ctx, `INSERT INTO ocr_suggestions
(id,job_id,ordinal,target_kind,target_id,target_version,field_key,field_label,value_kind,proposed_value,
 confidence,evidence_page,region_x,region_y,region_width,region_height,evidence_excerpt,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$18)`,
			databaseUUID(suggestion.ID), databaseUUID(input.JobID), suggestion.Ordinal, suggestion.Field.Target.Kind,
			databaseUUID(suggestion.Field.Target.ID), suggestion.Field.TargetVersion, suggestion.Field.Key, suggestion.Field.Label,
			suggestion.Field.Kind, suggestion.ProposedValue, optionalInt(suggestion.Evidence.Confidence), suggestion.Evidence.Page,
			regionX, regionY, regionWidth, regionHeight, optionalText(suggestion.Evidence.Excerpt), input.Now)
		if err != nil {
			return Job{}, normalizePostgresError(fmt.Errorf("insert OCR suggestion: %w", err))
		}
	}
	completed, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs
SET state='COMPLETED',page_count=$2,pixel_count=$3,suggestion_count=$4,completed_at=$5,
    next_event_sequence=next_event_sequence+2,version=version+1,updated_at=$5
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(input.JobID), input.PageCount, input.PixelCount, len(input.Suggestions), input.Now))
	if err != nil {
		return Job{}, fmt.Errorf("complete OCR job: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ocr_job_events(job_id,sequence,event_kind,suggestion_count,created_at)
VALUES($1,$2,'SUGGESTIONS_READY',$3,$4),($1,$2+1,'JOB_COMPLETED',NULL,$4)`,
		databaseUUID(input.JobID), sequence, len(input.Suggestions), input.Now); err != nil {
		return Job{}, fmt.Errorf("insert OCR completion events: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, normalizePostgresError(err)
	}
	return completed, nil
}

func (store *PostgresStore) FailJob(ctx context.Context, id Identifier, errorCode string, state JobState, now time.Time) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || len(errorCode) < 1 || len(errorCode) > 80 ||
		(state != JobFailed && state != JobCancelled) || now.IsZero() {
		return Job{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Job{}, fmt.Errorf("begin OCR failure: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, sequence, err := lockJob(ctx, tx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("lock OCR failure: %w", err)
	}
	if current.State.Terminal() {
		if current.State != state {
			return Job{}, ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return Job{}, normalizePostgresError(err)
		}
		return current, nil
	}
	eventKind := EventJobFailed
	if state == JobCancelled {
		eventKind = EventJobCancelled
	}
	updated, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs
SET state=$2,error_code=$3,completed_at=$4,
    cancel_requested_at=CASE WHEN $2='CANCELLED' THEN COALESCE(cancel_requested_at,$4) ELSE NULL END,
    next_event_sequence=next_event_sequence+1,version=version+1,updated_at=$4
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), state, errorCode, now))
	if err != nil {
		return Job{}, fmt.Errorf("finish OCR job with failure: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ocr_job_events(job_id,sequence,event_kind,error_code,created_at)
VALUES($1,$2,$3,$4,$5)`, databaseUUID(id), sequence, eventKind, errorCode, now); err != nil {
		return Job{}, fmt.Errorf("insert OCR failure event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, normalizePostgresError(err)
	}
	return updated, nil
}

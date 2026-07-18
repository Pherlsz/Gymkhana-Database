package ocr

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (store *PostgresStore) CurrentUser(ctx context.Context, id auth.Identifier) (auth.User, error) {
	if store == nil || store.pool == nil || id == (auth.Identifier{}) {
		return auth.User{}, ErrForbidden
	}
	var value auth.User
	var databaseID pgtype.UUID
	var avatar pgtype.Text
	err := store.pool.QueryRow(ctx, `SELECT id,github_user_id,github_login,display_name,avatar_url,role,active
FROM app_users WHERE id=$1`, authDatabaseUUID(id)).Scan(
		&databaseID, &value.GitHubUserID, &value.Login, &value.DisplayName, &avatar, &value.Role, &value.Active,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, ErrForbidden
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("load current OCR user: %w", err)
	}
	value.ID = auth.Identifier(databaseID.Bytes)
	if avatar.Valid {
		value.AvatarURL = avatar.String
	}
	return value, nil
}

func (store *PostgresStore) CreateJob(
	ctx context.Context,
	input CreateJobInput,
	window time.Time,
	maximumRate int,
	maximumProviderUsage int64,
) (Job, bool, error) {
	if store == nil || store.pool == nil || input.ID.IsZero() || input.OwnerUserID == (auth.Identifier{}) ||
		input.AttachmentID.IsZero() || !validIdempotencyKey(input.IdempotencyKey) || input.Now.IsZero() ||
		window.IsZero() || maximumRate < 1 || maximumProviderUsage < 1 || input.SourceBytes < 1 ||
		!SupportedMIME(input.SourceMIME) {
		return Job{}, false, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Job{}, false, fmt.Errorf("begin OCR job creation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var userExists bool
	err = tx.QueryRow(ctx, `SELECT true FROM app_users WHERE id=$1 AND active FOR UPDATE`, authDatabaseUUID(input.OwnerUserID)).Scan(&userExists)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, ErrForbidden
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("serialize OCR job creation: %w", err)
	}

	existing, err := scanJob(tx.QueryRow(ctx, `SELECT `+jobColumns+` FROM ocr_jobs
WHERE owner_user_id=$1 AND idempotency_key=$2 FOR UPDATE`, authDatabaseUUID(input.OwnerUserID), input.IdempotencyKey))
	if err == nil {
		if subtle.ConstantTimeCompare(existing.RequestFingerprint[:], input.RequestFingerprint[:]) != 1 {
			return Job{}, false, ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return Job{}, false, normalizePostgresError(err)
		}
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, fmt.Errorf("load OCR idempotency replay: %w", err)
	}

	var activeSource bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ocr_jobs
WHERE owner_user_id=$1 AND attachment_id=$2 AND state IN ('QUEUED','RUNNING'))`,
		authDatabaseUUID(input.OwnerUserID), attachmentDatabaseUUID(input.AttachmentID)).Scan(&activeSource)
	if err != nil {
		return Job{}, false, fmt.Errorf("check active OCR source: %w", err)
	}
	if activeSource {
		return Job{}, false, ErrConflict
	}

	var requestCount int
	var providerUsage int64
	err = tx.QueryRow(ctx, `INSERT INTO ocr_usage_windows
(owner_user_id,window_started_at,request_count,provider_usage,source_bytes,updated_at)
VALUES($1,$2,1,0,$3,$4)
ON CONFLICT (owner_user_id) DO UPDATE SET
  window_started_at=CASE WHEN ocr_usage_windows.window_started_at=$2 THEN ocr_usage_windows.window_started_at ELSE $2 END,
  request_count=CASE WHEN ocr_usage_windows.window_started_at=$2 THEN ocr_usage_windows.request_count+1 ELSE 1 END,
  provider_usage=CASE WHEN ocr_usage_windows.window_started_at=$2 THEN ocr_usage_windows.provider_usage ELSE 0 END,
  source_bytes=CASE WHEN ocr_usage_windows.window_started_at=$2 THEN ocr_usage_windows.source_bytes+$3 ELSE $3 END,
  updated_at=$4
RETURNING request_count,provider_usage`, authDatabaseUUID(input.OwnerUserID), window, input.SourceBytes, input.Now).Scan(&requestCount, &providerUsage)
	if err != nil {
		return Job{}, false, fmt.Errorf("consume OCR usage window: %w", err)
	}
	if requestCount > maximumRate {
		return Job{}, false, ErrRateLimited
	}
	if providerUsage >= maximumProviderUsage {
		return Job{}, false, ErrQuotaExceeded
	}

	created, err := scanJob(tx.QueryRow(ctx, `INSERT INTO ocr_jobs
(id,owner_user_id,attachment_id,retry_of_job_id,idempotency_key,request_fingerprint,
 source_sha256,catalog_fingerprint,schema_version,source_mime,source_bytes,state,next_event_sequence,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'QUEUED',2,$12,$12)
RETURNING `+jobColumns,
		databaseUUID(input.ID), authDatabaseUUID(input.OwnerUserID), attachmentDatabaseUUID(input.AttachmentID),
		optionalDatabaseUUID(input.RetryOfJobID), input.IdempotencyKey, input.RequestFingerprint[:], input.SourceSHA256[:],
		input.CatalogFingerprint[:], SchemaVersion, input.SourceMIME, input.SourceBytes, input.Now))
	if err != nil {
		return Job{}, false, normalizePostgresError(fmt.Errorf("insert OCR job: %w", err))
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ocr_job_events(job_id,sequence,event_kind,created_at)
VALUES($1,1,'JOB_ACCEPTED',$2)`, databaseUUID(input.ID), input.Now); err != nil {
		return Job{}, false, fmt.Errorf("insert OCR accepted event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, false, normalizePostgresError(err)
	}
	return created, true, nil
}

func (store *PostgresStore) AttachRiverJob(ctx context.Context, id Identifier, owner auth.Identifier, riverJobID int64, now time.Time) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) || riverJobID <= 0 || now.IsZero() {
		return Job{}, ErrInvalidInput
	}
	value, err := scanJob(store.pool.QueryRow(ctx, `UPDATE ocr_jobs
SET river_job_id=$3,version=version+1,updated_at=$4
WHERE id=$1 AND owner_user_id=$2 AND state='QUEUED' AND river_job_id IS NULL
RETURNING `+jobColumns, databaseUUID(id), authDatabaseUUID(owner), riverJobID, now))
	if errors.Is(err, pgx.ErrNoRows) {
		if _, loadErr := store.GetJob(ctx, id, owner); errors.Is(loadErr, ErrNotFound) {
			return Job{}, ErrNotFound
		}
		return Job{}, ErrConflict
	}
	if err != nil {
		return Job{}, fmt.Errorf("attach OCR queue job: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) ListJobs(ctx context.Context, owner auth.Identifier, limit, offset int) (JobPage, error) {
	if store == nil || store.pool == nil || owner == (auth.Identifier{}) || limit < 1 || limit > MaximumJobPage || offset < 0 {
		return JobPage{}, ErrInvalidInput
	}
	where := ` WHERE owner_user_id=$1 AND EXISTS (
  SELECT 1 FROM attachments source WHERE source.id=ocr_jobs.attachment_id AND source.lifecycle_state='ACTIVE'
)`
	rows, err := store.pool.Query(ctx, `SELECT `+jobColumns+` FROM ocr_jobs`+where+`
ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, authDatabaseUUID(owner), limit, offset)
	if err != nil {
		return JobPage{}, fmt.Errorf("list OCR jobs: %w", err)
	}
	defer rows.Close()
	page := JobPage{Jobs: make([]Job, 0), Limit: limit, Offset: offset}
	for rows.Next() {
		value, err := scanJob(rows)
		if err != nil {
			return JobPage{}, fmt.Errorf("scan OCR job: %w", err)
		}
		page.Jobs = append(page.Jobs, value)
	}
	if err := rows.Err(); err != nil {
		return JobPage{}, fmt.Errorf("iterate OCR jobs: %w", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM ocr_jobs`+where, authDatabaseUUID(owner)).Scan(&page.Total); err != nil {
		return JobPage{}, fmt.Errorf("count OCR jobs: %w", err)
	}
	return page, nil
}

func (store *PostgresStore) GetJob(ctx context.Context, id Identifier, owner auth.Identifier) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) {
		return Job{}, ErrNotFound
	}
	value, err := scanJob(store.pool.QueryRow(ctx, `SELECT `+jobColumns+` FROM ocr_jobs WHERE id=$1 AND owner_user_id=$2`, databaseUUID(id), authDatabaseUUID(owner)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("load OCR job: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) GetJobForWorker(ctx context.Context, id Identifier) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() {
		return Job{}, ErrNotFound
	}
	value, err := scanJob(store.pool.QueryRow(ctx, `SELECT `+jobColumns+` FROM ocr_jobs WHERE id=$1`, databaseUUID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("load OCR worker job: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) ClaimJob(ctx context.Context, id Identifier, now time.Time) (Job, bool, error) {
	if store == nil || store.pool == nil || id.IsZero() || now.IsZero() {
		return Job{}, false, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Job{}, false, fmt.Errorf("begin OCR claim: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, sequence, err := lockJob(ctx, tx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, ErrNotFound
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("lock OCR job for claim: %w", err)
	}
	if (current.State != JobQueued && current.State != JobRunning) || current.CancelRequestedAt != nil || current.AttemptCount >= MaximumAttempts {
		if err := tx.Commit(ctx); err != nil {
			return Job{}, false, normalizePostgresError(err)
		}
		return current, false, nil
	}
	if current.State == JobRunning {
		claimed, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs
SET attempt_count=attempt_count+1,version=version+1,updated_at=$2
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), now))
		if err != nil {
			return Job{}, false, fmt.Errorf("reclaim OCR job: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return Job{}, false, normalizePostgresError(err)
		}
		return claimed, true, nil
	}
	claimed, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs
SET state='RUNNING',started_at=$2,attempt_count=attempt_count+1,next_event_sequence=next_event_sequence+1,version=version+1,updated_at=$2
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), now))
	if err != nil {
		return Job{}, false, fmt.Errorf("claim OCR job: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ocr_job_events(job_id,sequence,event_kind,created_at)
VALUES($1,$2,'JOB_STARTED',$3)`, databaseUUID(id), sequence, now); err != nil {
		return Job{}, false, fmt.Errorf("insert OCR started event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, false, normalizePostgresError(err)
	}
	return claimed, true, nil
}

func (store *PostgresStore) RecordSourceValidated(ctx context.Context, id Identifier, pages int, pixels int64, now time.Time) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || pages < 1 || pages > MaximumPages || pixels < 0 || pixels > MaximumPixels || now.IsZero() {
		return Job{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Job{}, fmt.Errorf("begin OCR source validation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, sequence, err := lockJob(ctx, tx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("lock OCR source validation: %w", err)
	}
	if current.State != JobRunning || current.CancelRequestedAt != nil {
		return Job{}, ErrInvalidState
	}
	updated, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs
SET page_count=$2,pixel_count=$3,next_event_sequence=next_event_sequence+1,version=version+1,updated_at=$4
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), pages, pixels, now))
	if err != nil {
		return Job{}, fmt.Errorf("record OCR source validation: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ocr_job_events(job_id,sequence,event_kind,page_count,created_at)
VALUES($1,$2,'SOURCE_VALIDATED',$3,$4)`, databaseUUID(id), sequence, pages, now); err != nil {
		return Job{}, fmt.Errorf("insert OCR source validation event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, normalizePostgresError(err)
	}
	return updated, nil
}

func (store *PostgresStore) AddProviderUsage(ctx context.Context, id Identifier, owner auth.Identifier, usage, maximum int64, now time.Time) (Job, bool, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) || usage < 0 || usage > MaximumProviderUsage || maximum < 1 || maximum > MaximumProviderUsage || now.IsZero() {
		return Job{}, false, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Job{}, false, fmt.Errorf("begin OCR provider accounting: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	job, _, err := lockJob(ctx, tx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, ErrNotFound
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("lock OCR provider accounting: %w", err)
	}
	// Provider usage is billable once a response is returned, even when a
	// cancellation was requested while the provider call was in flight.
	if job.OwnerUserID != owner || job.State != JobRunning {
		return Job{}, false, ErrInvalidState
	}
	var current int64
	err = tx.QueryRow(ctx, `SELECT provider_usage FROM ocr_usage_windows WHERE owner_user_id=$1 FOR UPDATE`, authDatabaseUUID(owner)).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, ErrInvalidState
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("lock OCR usage window: %w", err)
	}
	newUsage := current + usage
	if _, err := tx.Exec(ctx, `UPDATE ocr_usage_windows SET provider_usage=$2,updated_at=$3 WHERE owner_user_id=$1`, authDatabaseUUID(owner), newUsage, now); err != nil {
		return Job{}, false, fmt.Errorf("record OCR usage window: %w", err)
	}
	updated, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs SET provider_usage=provider_usage+$2,version=version+1,updated_at=$3
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), usage, now))
	if err != nil {
		return Job{}, false, fmt.Errorf("record OCR job provider usage: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, false, normalizePostgresError(err)
	}
	return updated, newUsage > maximum, nil
}

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

func (store *PostgresStore) RequestCancellation(ctx context.Context, id Identifier, owner auth.Identifier, now time.Time) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) || now.IsZero() {
		return Job{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Job{}, fmt.Errorf("begin OCR cancellation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, sequence, err := lockJob(ctx, tx, id)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && current.OwnerUserID != owner {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("lock OCR cancellation: %w", err)
	}
	if current.State == JobCancelled {
		if err := tx.Commit(ctx); err != nil {
			return Job{}, normalizePostgresError(err)
		}
		return current, nil
	}
	if current.State.Terminal() {
		return Job{}, ErrInvalidState
	}
	if current.State == JobRunning {
		updated, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs
SET cancel_requested_at=COALESCE(cancel_requested_at,$2),version=version+1,updated_at=$2
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), now))
		if err != nil {
			return Job{}, fmt.Errorf("request running OCR cancellation: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return Job{}, normalizePostgresError(err)
		}
		return updated, nil
	}
	updated, err := scanJob(tx.QueryRow(ctx, `UPDATE ocr_jobs
SET state='CANCELLED',cancel_requested_at=$2,completed_at=$2,error_code='cancelled',
    next_event_sequence=next_event_sequence+1,version=version+1,updated_at=$2
WHERE id=$1 RETURNING `+jobColumns, databaseUUID(id), now))
	if err != nil {
		return Job{}, fmt.Errorf("cancel queued OCR job: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ocr_job_events(job_id,sequence,event_kind,error_code,created_at)
VALUES($1,$2,'JOB_CANCELLED','cancelled',$3)`, databaseUUID(id), sequence, now); err != nil {
		return Job{}, fmt.Errorf("insert OCR cancellation event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, normalizePostgresError(err)
	}
	return updated, nil
}

func (store *PostgresStore) ListEvents(ctx context.Context, id Identifier, owner auth.Identifier, after int64, limit int) (EventPage, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) || after < 0 || limit < 1 || limit > MaximumEventPage {
		return EventPage{}, ErrInvalidInput
	}
	job, err := store.GetJob(ctx, id, owner)
	if err != nil {
		return EventPage{}, err
	}
	rows, err := store.pool.Query(ctx, `SELECT job_id,sequence,event_kind,page_count,suggestion_count,error_code,created_at
FROM ocr_job_events WHERE job_id=$1 AND sequence>$2 ORDER BY sequence LIMIT $3`, databaseUUID(id), after, limit+1)
	if err != nil {
		return EventPage{}, fmt.Errorf("list OCR events: %w", err)
	}
	defer rows.Close()
	values := make([]JobEvent, 0, limit+1)
	for rows.Next() {
		var value JobEvent
		var jobID pgtype.UUID
		var pageCount, suggestionCount pgtype.Int4
		var errorCode pgtype.Text
		if err := rows.Scan(&jobID, &value.Sequence, &value.Kind, &pageCount, &suggestionCount, &errorCode, &value.CreatedAt); err != nil {
			return EventPage{}, fmt.Errorf("scan OCR event: %w", err)
		}
		value.JobID = Identifier(jobID.Bytes)
		if pageCount.Valid {
			mapped := int(pageCount.Int32)
			value.PageCount = &mapped
		}
		if suggestionCount.Valid {
			mapped := int(suggestionCount.Int32)
			value.SuggestionCount = &mapped
		}
		if errorCode.Valid {
			value.ErrorCode = errorCode.String
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return EventPage{}, fmt.Errorf("iterate OCR events: %w", err)
	}
	more := len(values) > limit
	if more {
		values = values[:limit]
	}
	page := EventPage{Events: values, LastSequence: after, Terminal: !more && job.State.Terminal()}
	if len(values) > 0 {
		page.LastSequence = values[len(values)-1].Sequence
	}
	return page, nil
}

func (store *PostgresStore) RecoverStaleJobs(ctx context.Context, staleBefore, now time.Time, limit int) (int, error) {
	if store == nil || store.pool == nil || staleBefore.IsZero() || now.IsZero() || !staleBefore.Before(now) || limit < 1 || limit > 1000 {
		return 0, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin OCR recovery: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `SELECT id,next_event_sequence FROM ocr_jobs
WHERE state='RUNNING' AND updated_at<$1 ORDER BY updated_at,id LIMIT $2 FOR UPDATE SKIP LOCKED`, staleBefore, limit)
	if err != nil {
		return 0, fmt.Errorf("select stale OCR jobs: %w", err)
	}
	type staleJob struct {
		id       Identifier
		sequence int64
	}
	values := make([]staleJob, 0)
	for rows.Next() {
		var id pgtype.UUID
		var sequence int64
		if err := rows.Scan(&id, &sequence); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan stale OCR job: %w", err)
		}
		values = append(values, staleJob{id: Identifier(id.Bytes), sequence: sequence})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate stale OCR jobs: %w", err)
	}
	rows.Close()
	for _, value := range values {
		if _, err := tx.Exec(ctx, `UPDATE ocr_jobs
SET state='FAILED',error_code='worker_interrupted',completed_at=$2,cancel_requested_at=NULL,
    next_event_sequence=next_event_sequence+1,version=version+1,updated_at=$2 WHERE id=$1`, databaseUUID(value.id), now); err != nil {
			return 0, fmt.Errorf("recover stale OCR job: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO ocr_job_events(job_id,sequence,event_kind,error_code,created_at)
VALUES($1,$2,'JOB_FAILED','worker_interrupted',$3)`, databaseUUID(value.id), value.sequence, now); err != nil {
			return 0, fmt.Errorf("insert OCR recovery event: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, normalizePostgresError(err)
	}
	return len(values), nil
}

func (store *PostgresStore) SaveAudit(ctx context.Context, event AuditEvent) error {
	if store == nil || store.pool == nil || event.ID.IsZero() || event.EventType == "" || event.Outcome == "" ||
		len(event.ErrorCode) > 80 || len(event.RequestID) > 128 || event.CreatedAt.IsZero() {
		return ErrInvalidInput
	}
	_, err := store.pool.Exec(ctx, `INSERT INTO ocr_audit_events
(id,actor_user_id,job_id,suggestion_id,field_key,review_action,event_type,outcome,affected_count,error_code,request_id,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, databaseUUID(event.ID), optionalAuthUUID(event.ActorUserID),
		optionalDatabaseUUID(event.JobID), optionalDatabaseUUID(event.SuggestionID), optionalText(event.FieldKey),
		optionalText(string(event.ReviewAction)), event.EventType, event.Outcome, optionalInteger(event.AffectedCount),
		optionalText(event.ErrorCode), event.RequestID, event.CreatedAt)
	if err != nil {
		return fmt.Errorf("record OCR audit event: %w", err)
	}
	return nil
}

func lockJob(ctx context.Context, tx pgx.Tx, id Identifier) (Job, int64, error) {
	value, err := scanJob(tx.QueryRow(ctx, `SELECT `+jobColumns+` FROM ocr_jobs WHERE id=$1 FOR UPDATE`, databaseUUID(id)))
	if err != nil {
		return Job{}, 0, err
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `SELECT next_event_sequence FROM ocr_jobs WHERE id=$1`, databaseUUID(id)).Scan(&sequence); err != nil {
		return Job{}, 0, err
	}
	return value, sequence, nil
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
	databaseCode := ""
	var databaseError interface{ SQLState() string }
	if errors.As(err, &databaseError) {
		databaseCode = databaseError.SQLState()
	}
	if databaseCode == "" {
		for _, code := range []string{"57014", "23505", "40001", "40P01", "23503", "23514", "23P01"} {
			if strings.Contains(err.Error(), "(SQLSTATE "+code+")") {
				databaseCode = code
				break
			}
		}
	}
	switch databaseCode {
	case "57014":
		return ErrTimeout
	case "23505", "40001", "40P01":
		return ErrConflict
	case "23503", "23P01":
		return ErrInvalidState
	case "23514":
		return ErrInvalidInput
	default:
		return err
	}
}

func optionalAuthUUID(value *auth.Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return authDatabaseUUID(*value)
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

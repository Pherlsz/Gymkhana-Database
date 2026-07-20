package ocr

import (
	"context"
	"crypto/subtle"
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

func (store *PostgresStore) CurrentUser(ctx context.Context, id auth.Identifier) (auth.User, error) {
	if store == nil || store.pool == nil || id == (auth.Identifier{}) {
		return auth.User{}, ErrForbidden
	}
	var value auth.User
	var databaseID pgtype.UUID
	var subject, avatar pgtype.Text
	err := store.pool.QueryRow(ctx, `SELECT id,google_subject,email,display_name,avatar_url,role,active
FROM app_users WHERE id=$1`, authDatabaseUUID(id)).Scan(
		&databaseID, &subject, &value.Email, &value.DisplayName, &avatar, &value.Role, &value.Active,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, ErrForbidden
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("load current OCR user: %w", err)
	}
	value.ID = auth.Identifier(databaseID.Bytes)
	if subject.Valid {
		value.GoogleSubject = subject.String
	}
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

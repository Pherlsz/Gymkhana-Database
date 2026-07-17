package matching

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

type rowScanner interface {
	Scan(...any) error
}

const analysisSelect = `SELECT id, actor_user_id, idempotency_key, state, river_job_id,
       profiles_scanned, candidate_count, refreshed_count, error_code,
       cancel_requested_at, started_at, completed_at, expires_at, version, created_at, updated_at
  FROM matching_analyses`

func scanAnalysis(row rowScanner) (Analysis, error) {
	var value Analysis
	var id, actorID pgtype.UUID
	var riverJobID pgtype.Int8
	var errorCode pgtype.Text
	var cancelledAt, startedAt, completedAt pgtype.Timestamptz
	if err := row.Scan(&id, &actorID, &value.IdempotencyKey, &value.State, &riverJobID,
		&value.ProfilesScanned, &value.CandidateCount, &value.RefreshedCount, &errorCode,
		&cancelledAt, &startedAt, &completedAt, &value.ExpiresAt, &value.Version,
		&value.CreatedAt, &value.UpdatedAt); err != nil {
		return Analysis{}, err
	}
	value.ID = matchingIdentifier(id)
	value.ActorUserID = authIdentifier(actorID)
	if riverJobID.Valid {
		value.RiverJobID = riverJobID.Int64
	}
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	if cancelledAt.Valid {
		value.CancelRequestedAt = &cancelledAt.Time
	}
	if startedAt.Valid {
		value.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		value.CompletedAt = &completedAt.Time
	}
	return value, nil
}

func (store *PostgresStore) CreateAnalysis(ctx context.Context, input CreateAnalysisInput, window time.Time, maximumRate int) (Analysis, bool, error) {
	return store.createAnalysis(ctx, input, window, maximumRate, nil)
}

func (store *PostgresStore) CreateAnalysisWithJob(
	ctx context.Context,
	input CreateAnalysisInput,
	window time.Time,
	maximumRate int,
	jobs AnalysisJobInserter,
) (Analysis, bool, error) {
	if jobs == nil {
		return Analysis{}, false, ErrInvalidInput
	}
	return store.createAnalysis(ctx, input, window, maximumRate, jobs)
}

func (store *PostgresStore) createAnalysis(
	ctx context.Context,
	input CreateAnalysisInput,
	window time.Time,
	maximumRate int,
	jobs AnalysisJobInserter,
) (Analysis, bool, error) {
	if store == nil || store.pool == nil || input.ID.IsZero() || input.ActorUserID == (auth.Identifier{}) ||
		!validIdempotencyKey(input.IdempotencyKey) || input.ExpiresAt.IsZero() || window.IsZero() || maximumRate < 1 {
		return Analysis{}, false, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Analysis{}, false, fmt.Errorf("begin matching analysis creation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var actorExists bool
	if err := tx.QueryRow(ctx, `SELECT true FROM app_users WHERE id=$1 FOR UPDATE`, matchingAuthUUID(input.ActorUserID)).Scan(&actorExists); errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, false, ErrForbidden
	} else if err != nil {
		return Analysis{}, false, fmt.Errorf("serialize matching analysis creation: %w", err)
	}
	existing, err := scanAnalysis(tx.QueryRow(ctx, analysisSelect+` WHERE actor_user_id=$1 AND idempotency_key=$2 FOR UPDATE`, matchingAuthUUID(input.ActorUserID), input.IdempotencyKey))
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return Analysis{}, false, fmt.Errorf("commit matching analysis replay: %w", err)
		}
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, false, fmt.Errorf("load matching analysis replay: %w", err)
	}
	var requestCount int
	if err := tx.QueryRow(ctx, `INSERT INTO matching_rate_limits
(actor_user_id, window_started_at, request_count, updated_at)
VALUES($1,$2,1,$3)
ON CONFLICT (actor_user_id) DO UPDATE SET
  window_started_at = CASE WHEN matching_rate_limits.window_started_at=$2 THEN matching_rate_limits.window_started_at ELSE $2 END,
  request_count = CASE WHEN matching_rate_limits.window_started_at=$2 THEN matching_rate_limits.request_count+1 ELSE 1 END,
  updated_at=$3
RETURNING request_count`, matchingAuthUUID(input.ActorUserID), window, time.Now().UTC()).Scan(&requestCount); err != nil {
		return Analysis{}, false, fmt.Errorf("consume matching analysis rate: %w", err)
	}
	if requestCount > maximumRate {
		return Analysis{}, false, ErrRateLimited
	}
	created, err := scanAnalysis(tx.QueryRow(ctx, `INSERT INTO matching_analyses
(id, actor_user_id, idempotency_key, state, expires_at)
VALUES($1,$2,$3,'QUEUED',$4)
RETURNING id, actor_user_id, idempotency_key, state, river_job_id,
          profiles_scanned, candidate_count, refreshed_count, error_code,
          cancel_requested_at, started_at, completed_at, expires_at, version, created_at, updated_at`,
		matchingUUID(input.ID), matchingAuthUUID(input.ActorUserID), input.IdempotencyKey, input.ExpiresAt))
	if err != nil {
		return Analysis{}, false, normalizePostgresError(err)
	}
	if jobs != nil {
		jobID, enqueueErr := jobs.EnqueueAnalysisTx(ctx, tx, input.ID)
		if enqueueErr != nil {
			return Analysis{}, false, fmt.Errorf("enqueue matching analysis transactionally: %w", enqueueErr)
		}
		created, err = scanAnalysis(tx.QueryRow(ctx, `UPDATE matching_analyses
SET river_job_id=$2, version=version+1, updated_at=now()
WHERE id=$1 AND river_job_id IS NULL
RETURNING id, actor_user_id, idempotency_key, state, river_job_id,
          profiles_scanned, candidate_count, refreshed_count, error_code,
          cancel_requested_at, started_at, completed_at, expires_at, version, created_at, updated_at`,
			matchingUUID(input.ID), jobID))
		if err != nil {
			return Analysis{}, false, fmt.Errorf("attach transactional matching analysis job: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Analysis{}, false, normalizePostgresError(err)
	}
	return created, true, nil
}

func (store *PostgresStore) AttachAnalysisJob(ctx context.Context, id Identifier, actorID auth.Identifier, jobID int64) (Analysis, error) {
	if store == nil || store.pool == nil || id.IsZero() || actorID == (auth.Identifier{}) || jobID <= 0 {
		return Analysis{}, ErrInvalidInput
	}
	value, err := scanAnalysis(store.pool.QueryRow(ctx, `UPDATE matching_analyses
SET river_job_id=$3, version=version+1, updated_at=now()
WHERE id=$1 AND actor_user_id=$2 AND river_job_id IS NULL
RETURNING id, actor_user_id, idempotency_key, state, river_job_id,
          profiles_scanned, candidate_count, refreshed_count, error_code,
          cancel_requested_at, started_at, completed_at, expires_at, version, created_at, updated_at`,
		matchingUUID(id), matchingAuthUUID(actorID), jobID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, ErrConflict
	}
	if err != nil {
		return Analysis{}, fmt.Errorf("attach matching analysis job: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) GetAnalysis(ctx context.Context, id Identifier, actorID auth.Identifier) (Analysis, error) {
	if store == nil || store.pool == nil || id.IsZero() || actorID == (auth.Identifier{}) {
		return Analysis{}, ErrNotFound
	}
	value, err := scanAnalysis(store.pool.QueryRow(ctx, analysisSelect+` WHERE id=$1 AND actor_user_id=$2`, matchingUUID(id), matchingAuthUUID(actorID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, ErrNotFound
	}
	if err != nil {
		return Analysis{}, fmt.Errorf("load matching analysis: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) RequestAnalysisCancellation(ctx context.Context, id Identifier, actorID auth.Identifier, now time.Time) (Analysis, error) {
	if store == nil || store.pool == nil || id.IsZero() || actorID == (auth.Identifier{}) || now.IsZero() {
		return Analysis{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Analysis{}, fmt.Errorf("begin matching cancellation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	value, err := scanAnalysis(tx.QueryRow(ctx, analysisSelect+` WHERE id=$1 AND actor_user_id=$2 FOR UPDATE`, matchingUUID(id), matchingAuthUUID(actorID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, ErrNotFound
	}
	if err != nil {
		return Analysis{}, fmt.Errorf("lock matching analysis for cancellation: %w", err)
	}
	if value.State == AnalysisCancelled {
		if err := tx.Commit(ctx); err != nil {
			return Analysis{}, err
		}
		return value, nil
	}
	if value.State.Terminal() {
		return Analysis{}, ErrInvalidState
	}
	value, err = scanAnalysis(tx.QueryRow(ctx, `UPDATE matching_analyses
SET state='CANCELLED', cancel_requested_at=$2, completed_at=$2, error_code='cancelled', version=version+1, updated_at=$2
WHERE id=$1
RETURNING id, actor_user_id, idempotency_key, state, river_job_id,
          profiles_scanned, candidate_count, refreshed_count, error_code,
          cancel_requested_at, started_at, completed_at, expires_at, version, created_at, updated_at`, matchingUUID(id), now))
	if err != nil {
		return Analysis{}, fmt.Errorf("cancel matching analysis: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Analysis{}, fmt.Errorf("commit matching cancellation: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) ClaimAnalysis(ctx context.Context, id Identifier, now time.Time) (Analysis, bool, error) {
	if store == nil || store.pool == nil || id.IsZero() || now.IsZero() {
		return Analysis{}, false, ErrInvalidInput
	}
	value, err := scanAnalysis(store.pool.QueryRow(ctx, `UPDATE matching_analyses
SET state='RUNNING', started_at=COALESCE(started_at,$2), error_code=NULL, version=version+1, updated_at=$2
WHERE id=$1 AND state IN ('QUEUED','RUNNING') AND cancel_requested_at IS NULL
RETURNING id, actor_user_id, idempotency_key, state, river_job_id,
          profiles_scanned, candidate_count, refreshed_count, error_code,
          cancel_requested_at, started_at, completed_at, expires_at, version, created_at, updated_at`, matchingUUID(id), now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, false, nil
	}
	if err != nil {
		return Analysis{}, false, fmt.Errorf("claim matching analysis: %w", err)
	}
	return value, true, nil
}

func (store *PostgresStore) CompleteAnalysis(ctx context.Context, id Identifier, stats AnalysisStats, now time.Time) error {
	if store == nil || store.pool == nil || id.IsZero() || now.IsZero() || stats.ProfilesScanned < 0 || stats.CandidateCount < 0 || stats.RefreshedCount < 0 {
		return ErrInvalidInput
	}
	command, err := store.pool.Exec(ctx, `UPDATE matching_analyses
SET state='COMPLETED', profiles_scanned=$2, candidate_count=$3, refreshed_count=$4,
    completed_at=$5, error_code=NULL, version=version+1, updated_at=$5
WHERE id=$1 AND state='RUNNING' AND cancel_requested_at IS NULL`,
		matchingUUID(id), stats.ProfilesScanned, stats.CandidateCount, stats.RefreshedCount, now)
	if err != nil {
		return fmt.Errorf("complete matching analysis: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (store *PostgresStore) FailAnalysis(ctx context.Context, id Identifier, errorCode string, state AnalysisState, now time.Time) error {
	if store == nil || store.pool == nil || id.IsZero() || now.IsZero() || len(errorCode) < 1 || len(errorCode) > 80 ||
		(state != AnalysisFailed && state != AnalysisCancelled) {
		return ErrInvalidInput
	}
	command, err := store.pool.Exec(ctx, `UPDATE matching_analyses
SET state=$2, error_code=$3, completed_at=COALESCE(completed_at,$4),
    cancel_requested_at=CASE WHEN $2='CANCELLED' THEN COALESCE(cancel_requested_at,$4) ELSE cancel_requested_at END,
    version=version+1, updated_at=$4
WHERE id=$1 AND state IN ('QUEUED','RUNNING')`, matchingUUID(id), state, errorCode, now)
	if err != nil {
		return fmt.Errorf("fail matching analysis: %w", err)
	}
	if command.RowsAffected() == 0 {
		var existing AnalysisState
		if err := store.pool.QueryRow(ctx, `SELECT state FROM matching_analyses WHERE id=$1`, matchingUUID(id)).Scan(&existing); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if existing != state {
			return ErrConflict
		}
	}
	return nil
}

func (store *PostgresStore) CleanupAnalyses(ctx context.Context, now time.Time, limit int) (int, error) {
	if store == nil || store.pool == nil || now.IsZero() || limit < 1 || limit > 1_000 {
		return 0, ErrInvalidInput
	}
	command, err := store.pool.Exec(ctx, `WITH expired AS (
  SELECT id FROM matching_analyses
  WHERE expires_at <= $1 AND state IN ('COMPLETED','FAILED','CANCELLED')
  ORDER BY expires_at, id
  LIMIT $2
  FOR UPDATE SKIP LOCKED
)
DELETE FROM matching_analyses analysis USING expired WHERE analysis.id=expired.id`, now, limit)
	if err != nil {
		return 0, fmt.Errorf("cleanup matching analyses: %w", err)
	}
	return int(command.RowsAffected()), nil
}

func (store *PostgresStore) RecordAudit(ctx context.Context, event AuditEvent) error {
	if store == nil || store.pool == nil || event.ID.IsZero() || event.EventType == "" || event.Outcome == "" ||
		len(event.RequestID) < 1 || len(event.RequestID) > 128 || len(event.ErrorCode) > 80 {
		return ErrInvalidInput
	}
	_, err := store.pool.Exec(ctx, `INSERT INTO matching_audit_events
(id, actor_user_id, analysis_id, case_id, event_type, outcome, score_band, affected_count, error_code, request_id, created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		matchingUUID(event.ID), optionalMatchingAuthUUID(event.ActorUserID), optionalMatchingUUID(event.AnalysisID), optionalMatchingUUID(event.CaseID),
		event.EventType, event.Outcome, optionalScoreBand(event.ScoreBand), optionalInteger(event.AffectedCount), optionalText(event.ErrorCode), event.RequestID, event.CreatedAt)
	if err != nil {
		return fmt.Errorf("record matching audit: %w", err)
	}
	return nil
}

func normalizePostgresError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		switch databaseError.Code {
		case "57014":
			return ErrTimeout
		case "23505", "40001", "40P01":
			return ErrConflict
		case "23503", "23514", "23P01":
			return ErrDependencyConflict
		}
	}
	return err
}

func matchingUUID(value Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: !value.IsZero()}
}

func matchingAuthUUID(value auth.Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: value != (auth.Identifier{})}
}

func optionalMatchingUUID(value *Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return matchingUUID(*value)
}

func optionalMatchingAuthUUID(value *auth.Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return matchingAuthUUID(*value)
}

func matchingIdentifier(value pgtype.UUID) Identifier { return Identifier(value.Bytes) }

func authIdentifier(value pgtype.UUID) auth.Identifier { return auth.Identifier(value.Bytes) }

func optionalText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func optionalScoreBand(value *ScoreBand) any {
	if value == nil {
		return nil
	}
	return *value
}

func optionalInteger(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

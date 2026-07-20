package taskengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
FROM app_users WHERE id=$1`, taskAuthUUID(id)).Scan(&databaseID, &subject, &value.Email, &value.DisplayName, &avatar, &value.Role, &value.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, ErrForbidden
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("load current task user: %w", err)
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

const draftColumns = `id,owner_user_id,catalog_version,state,spec_json,spec_fingerprint,version,expires_at,created_at,updated_at`
const jobColumns = `id,owner_user_id,draft_id,retry_of_job_id,idempotency_key,request_fingerprint,catalog_version,
state,river_job_id,attempt_count,progress_current,progress_total,candidate_count,composition_count,error_code,
cancel_requested_at,started_at,completed_at,expires_at,version,created_at,updated_at`

type taskRowScanner interface{ Scan(...any) error }

func scanDraft(row taskRowScanner) (Draft, error) {
	var value Draft
	var id, owner pgtype.UUID
	var state string
	var raw []byte
	var fingerprint []byte
	if err := row.Scan(&id, &owner, &value.CatalogVersion, &state, &raw, &fingerprint, &value.Version, &value.ExpiresAt, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return Draft{}, err
	}
	if !id.Valid || !owner.Valid || len(fingerprint) != len(value.SpecFingerprint) || len(raw) == 0 {
		return Draft{}, ErrUnsafeResult
	}
	if err := json.Unmarshal(raw, &value.Spec); err != nil {
		return Draft{}, ErrUnsafeResult
	}
	value.ID = Identifier(id.Bytes)
	value.OwnerUserID = auth.Identifier(owner.Bytes)
	value.State = DraftState(state)
	copy(value.SpecFingerprint[:], fingerprint)
	return value, nil
}

func scanJob(row taskRowScanner) (Job, error) {
	var value Job
	var id, owner, draft, retry pgtype.UUID
	var fingerprint []byte
	var state string
	var riverID pgtype.Int8
	var errorCode pgtype.Text
	var cancelAt, startedAt, completedAt pgtype.Timestamptz
	if err := row.Scan(&id, &owner, &draft, &retry, &value.IdempotencyKey, &fingerprint, &value.CatalogVersion,
		&state, &riverID, &value.AttemptCount, &value.ProgressCurrent, &value.ProgressTotal,
		&value.CandidateCount, &value.CompositionCount, &errorCode, &cancelAt, &startedAt, &completedAt,
		&value.ExpiresAt, &value.Version, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return Job{}, err
	}
	if !id.Valid || !owner.Valid || !draft.Valid || len(fingerprint) != len(value.RequestFingerprint) {
		return Job{}, ErrUnsafeResult
	}
	value.ID = Identifier(id.Bytes)
	value.OwnerUserID = auth.Identifier(owner.Bytes)
	value.DraftID = Identifier(draft.Bytes)
	if retry.Valid {
		parsed := Identifier(retry.Bytes)
		value.RetryOfJobID = &parsed
	}
	copy(value.RequestFingerprint[:], fingerprint)
	value.State = JobState(state)
	if riverID.Valid {
		value.RiverJobID = riverID.Int64
	}
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	if cancelAt.Valid {
		parsed := cancelAt.Time
		value.CancelRequestedAt = &parsed
	}
	if startedAt.Valid {
		parsed := startedAt.Time
		value.StartedAt = &parsed
	}
	if completedAt.Valid {
		parsed := completedAt.Time
		value.CompletedAt = &parsed
	}
	return value, nil
}

func taskUUID(value Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: !value.IsZero()}
}
func taskAuthUUID(value auth.Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: value != (auth.Identifier{})}
}
func optionalTaskUUID(value *Identifier) pgtype.UUID {
	if value == nil || value.IsZero() {
		return pgtype.UUID{}
	}
	return taskUUID(*value)
}

func taskPostgresError(operation string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505", "23503", "40001", "40P01":
			return fmt.Errorf("%s: %w", operation, ErrConflict)
		case "23514", "22P02":
			return fmt.Errorf("%s: %w", operation, ErrInvalidInput)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func taskJSON(value any, maximum int) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) == 0 || len(encoded) > maximum {
		return nil, ErrInvalidInput
	}
	return encoded, nil
}

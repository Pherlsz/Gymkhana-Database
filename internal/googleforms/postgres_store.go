package googleforms

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

type rowScanner interface {
	Scan(...any) error
}

const connectionSelect = `SELECT id, owner_user_id, state, refresh_token_ciphertext,
       refresh_token_nonce, token_key_version, granted_scopes, error_code,
       connected_at, disconnected_at, version, created_at, updated_at
  FROM google_forms_connections`

const sourceSelect = `SELECT id, connection_id, owner_user_id, provider_form_id, title,
       module, state, schema_revision, schema_fingerprint, sync_mode,
	       poll_interval_seconds, cursor_submitted_at, response_page_token,
	       page_token_cursor_started_at, last_synced_at, next_sync_at,
       error_code, version, created_at, updated_at
  FROM google_forms_sources`

const syncRunSelect = `SELECT id, source_id, owner_user_id, actor_user_id, trigger_kind,
       state, idempotency_key, operation_import_id, cursor_started_at,
       cursor_completed_at, received_count, staged_count, duplicate_count,
       river_job_id, error_code, started_at, completed_at, version, created_at, updated_at
  FROM google_forms_sync_runs`

func scanConnection(row rowScanner) (Connection, error) {
	var value Connection
	var id, ownerID pgtype.UUID
	var ciphertext, nonce []byte
	var keyVersion pgtype.Int4
	var errorCode pgtype.Text
	var connectedAt, disconnectedAt pgtype.Timestamptz
	if err := row.Scan(&id, &ownerID, &value.State, &ciphertext, &nonce, &keyVersion,
		&value.GrantedScopes, &errorCode, &connectedAt, &disconnectedAt, &value.Version,
		&value.CreatedAt, &value.UpdatedAt); err != nil {
		return Connection{}, err
	}
	value.ID = identifierFromUUID(id)
	value.OwnerUserID = authIdentifierFromUUID(ownerID)
	value.RefreshTokenCiphertext = append([]byte(nil), ciphertext...)
	value.RefreshTokenNonce = append([]byte(nil), nonce...)
	if keyVersion.Valid {
		value.TokenKeyVersion = uint16(keyVersion.Int32)
	}
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	if connectedAt.Valid {
		value.ConnectedAt = &connectedAt.Time
	}
	if disconnectedAt.Valid {
		value.DisconnectedAt = &disconnectedAt.Time
	}
	return value, nil
}

func scanSource(row rowScanner) (Source, error) {
	var value Source
	var id, connectionID, ownerID pgtype.UUID
	var fingerprint []byte
	var cursor, pageTokenCursor, lastSynced, nextSync pgtype.Timestamptz
	var pageToken, errorCode pgtype.Text
	var pollSeconds int
	if err := row.Scan(&id, &connectionID, &ownerID, &value.ProviderFormID, &value.Title,
		&value.Module, &value.State, &value.SchemaRevision, &fingerprint, &value.SyncMode,
		&pollSeconds, &cursor, &pageToken, &pageTokenCursor, &lastSynced, &nextSync, &errorCode, &value.Version,
		&value.CreatedAt, &value.UpdatedAt); err != nil {
		return Source{}, err
	}
	value.ID = identifierFromUUID(id)
	value.ConnectionID = identifierFromUUID(connectionID)
	value.OwnerUserID = authIdentifierFromUUID(ownerID)
	copy(value.SchemaFingerprint[:], fingerprint)
	value.PollInterval = time.Duration(pollSeconds) * time.Second
	if cursor.Valid {
		value.CursorSubmittedAt = &cursor.Time
	}
	if pageToken.Valid {
		value.ResponsePageToken = pageToken.String
	}
	if pageTokenCursor.Valid {
		value.PageTokenCursor = &pageTokenCursor.Time
	}
	if lastSynced.Valid {
		value.LastSyncedAt = &lastSynced.Time
	}
	if nextSync.Valid {
		value.NextSyncAt = &nextSync.Time
	}
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	return value, nil
}

func scanSyncRun(row rowScanner) (SyncRun, error) {
	var value SyncRun
	var id, sourceID, ownerID, actorID, operationID pgtype.UUID
	var cursorStarted, cursorCompleted, startedAt, completedAt pgtype.Timestamptz
	var riverJobID pgtype.Int8
	var errorCode pgtype.Text
	if err := row.Scan(&id, &sourceID, &ownerID, &actorID, &value.TriggerKind,
		&value.State, &value.IdempotencyKey, &operationID, &cursorStarted,
		&cursorCompleted, &value.ReceivedCount, &value.StagedCount, &value.DuplicateCount,
		&riverJobID, &errorCode, &startedAt, &completedAt, &value.Version,
		&value.CreatedAt, &value.UpdatedAt); err != nil {
		return SyncRun{}, err
	}
	value.ID = identifierFromUUID(id)
	value.SourceID = identifierFromUUID(sourceID)
	value.OwnerUserID = authIdentifierFromUUID(ownerID)
	if actorID.Valid {
		identifier := authIdentifierFromUUID(actorID)
		value.ActorUserID = &identifier
	}
	if operationID.Valid {
		identifier := operations.Identifier(operationID.Bytes)
		value.OperationImportID = &identifier
	}
	if cursorStarted.Valid {
		value.CursorStartedAt = &cursorStarted.Time
	}
	if cursorCompleted.Valid {
		value.CursorCompletedAt = &cursorCompleted.Time
	}
	if riverJobID.Valid {
		value.RiverJobID = riverJobID.Int64
	}
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	if startedAt.Valid {
		value.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		value.CompletedAt = &completedAt.Time
	}
	return value, nil
}

func (store *PostgresStore) SaveOAuthState(ctx context.Context, value OAuthState) error {
	if store == nil || store.pool == nil || value.OwnerUserID == (auth.Identifier{}) ||
		value.SessionID == (auth.Identifier{}) || value.StateHash == ([32]byte{}) ||
		len(value.VerifierCiphertext) == 0 || len(value.VerifierNonce) == 0 {
		return ErrInvalidInput
	}
	_, err := store.pool.Exec(ctx, `INSERT INTO google_forms_oauth_states (
  state_hash, owner_user_id, session_id, verifier_ciphertext, verifier_nonce,
  token_key_version, return_path, created_at, expires_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, value.StateHash[:], authDatabaseUUID(value.OwnerUserID),
		authDatabaseUUID(value.SessionID), value.VerifierCiphertext, value.VerifierNonce,
		int(value.TokenKeyVersion), value.ReturnPath, value.CreatedAt, value.ExpiresAt)
	if err != nil {
		return mapPostgresError("save oauth state", err)
	}
	return nil
}

func (store *PostgresStore) ConsumeOAuthState(ctx context.Context, stateHash [32]byte, ownerID, sessionID auth.Identifier, now time.Time) (OAuthState, error) {
	var value OAuthSta×½=¶‰žËkºwµç}version+1, updated_at=$2
 WHERE owner_user_id=$1 AND state<>'NEEDS_REAUTH'`, authDatabaseUUID(ownerID), now); err != nil {
		return Connection{}, fmt.Errorf("pause disconnected sources: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Connection{}, fmt.Errorf("commit connection disconnect: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) MarkConnectionNeedsReauth(ctx context.Context, ownerID auth.Identifier, code string, now time.Time) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin connection invalidation: %w", err)
	}
	defer tx.Rollback(ctx)
	command, err := tx.Exec(ctx, `UPDATE google_forms_connections
   SET state='NEEDS_REAUTH', refresh_token_ciphertext=NULL, refresh_token_nonce=NULL,
       token_key_version=NULL, granted_scopes='{}', error_code=$2,
       version=version+1, updated_at=$3
 WHERE owner_user_id=$1 AND state<>'DISCONNECTED'`, authDatabaseUUID(ownerID), code, now)
	if err != nil {
		return fmt.Errorf("invalidate connection: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE google_forms_sources
   SET state='NEEDS_REAUTH', next_sync_at=NULL, error_code=$2,
       version=version+1, updated_at=$3
 WHERE owner_user_id=$1 AND state<>'NEEDS_REAUTH'`, authDatabaseUUID(ownerID), code, now); err != nil {
		return fmt.Errorf("invalidate connection sources: %w", err)
	}
	return tx.Commit(ctx)
}

func (store *PostgresStore) connectionWriteError(ctx context.Context, tx pgx.Tx, ownerID auth.Identifier) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM google_forms_connections WHERE owner_user_id=$1)`, authDatabaseUUID(ownerID)).Scan(&exists); err != nil {
		return fmt.Errorf("classify connection write: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	return ErrConflict
}

func databaseUUID(value Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: true}
}

func authDatabaseUUID(value auth.Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: true}
}

func optionalAuthDatabaseUUID(value *auth.Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return authDatabaseUUID(*value)
}

func identifierFromUUID(value pgtype.UUID) Identifier {
	return Identifier(value.Bytes)
}

func authIdentifierFromUUID(value pgtype.UUID) auth.Identifier {
	return auth.Identifier(value.Bytes)
}

func mapPostgresError(operation string, err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		switch databaseError.Code {
		case "23505":
			return fmt.Errorf("%s: %w", operation, ErrConflict)
		case "23503":
			return fmt.Errorf("%s: %w", operation, ErrNotFound)
		case "23514", "22001", "22P02":
			return fmt.Errorf("%s: %w", operation, ErrInvalidInput)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func optionalText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func optionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func sortedResponses(values []Response) []Response {
	result := append([]Response(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		if result[left].SubmittedAt.Equal(result[right].SubmittedAt) {
			return result[left].ID < result[right].ID
		}
		return result[left].SubmittedAt.Before(result[right].SubmittedAt)
	})
	return result
}

func responseFingerprint(value Response) [sha256.Size]byte {
	questions := make([]string, 0, len(value.Answers))
	for question := range value.Answers {
		questions = append(questions, question)
	}
	sort.Strings(questions)
	var builder strings.Builder
	builder.WriteString(value.ID)
	builder.WriteByte(0)
	builder.WriteString(value.SubmittedAt.UTC().Format(time.RFC3339Nano))
	for _, question := range questions {
		builder.WriteByte(0)
		builder.WriteString(question)
		for _, answer := range value.Answers[question] {
			builder.WriteByte(0)
			builder.WriteString(answer)
		}
	}
	return sha256.Sum256([]byte(builder.String()))
}

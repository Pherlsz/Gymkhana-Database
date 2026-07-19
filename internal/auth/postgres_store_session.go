package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) CreateSession(ctx context.Context, params CreateSessionParams) error {
	_, err := store.pool.Exec(ctx, `
INSERT INTO app_sessions (id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)`, databaseUUID(params.ID), databaseUUID(params.UserID), params.TokenHash, params.ExpiresAt)
	return err
}

func (store *PostgresStore) FindAuthenticatedSession(ctx context.Context, tokenHash []byte, now time.Time) (Session, error) {
	var sessionID pgtype.UUID
	var userID pgtype.UUID
	var subject *string
	var email, displayName, role string
	var avatar *string
	var active bool
	err := store.pool.QueryRow(ctx, `
SELECT s.id, u.id, u.google_subject, u.email, u.display_name, u.avatar_url, u.role, u.active
FROM app_sessions s
JOIN app_users u ON u.id = s.user_id
WHERE s.token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > $2
  AND u.active`, tokenHash, now).Scan(&sessionID, &userID, &subject, &email, &displayName, &avatar, &role, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, err
	}
	return Session{ID: identifierFromDatabase(sessionID), User: userFromValues(userID, subject, email, displayName, avatar, role, active)}, nil
}

func (store *PostgresStore) TouchSession(ctx context.Context, sessionID Identifier) error {
	_, err := store.pool.Exec(ctx, `UPDATE app_sessions SET last_seen_at = now() WHERE id = $1 AND revoked_at IS NULL AND expires_at > now()`, databaseUUID(sessionID))
	return err
}

func (store *PostgresStore) RevokeSessionByTokenHash(ctx context.Context, tokenHash []byte) error {
	_, err := store.pool.Exec(ctx, `UPDATE app_sessions SET revoked_at = COALESCE(revoked_at, now()) WHERE token_hash = $1`, tokenHash)
	return err
}

func (store *PostgresStore) RevokeAllSessionsForUser(ctx context.Context, userID Identifier) error {
	_, err := store.pool.Exec(ctx, `UPDATE app_sessions SET revoked_at = COALESCE(revoked_at, now()) WHERE user_id = $1`, databaseUUID(userID))
	return err
}

func (store *PostgresStore) RecordAuditEvent(ctx context.Context, event AuditEvent) error {
	_, err := store.pool.Exec(ctx, `
INSERT INTO auth_audit_events (id, actor_user_id, subject_user_id, event_type, outcome, request_id, provider_email)
VALUES ($1, $2, $3, $4, $5, $6, $7)`, databaseUUID(event.ID), optionalDatabaseUUID(event.ActorUserID), optionalDatabaseUUID(event.SubjectUserID), string(event.EventType), string(event.Outcome), event.RequestID, optionalString(event.ProviderLogin))
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (User, error) {
	var id pgtype.UUID
	var subject *string
	var email, displayName, role string
	var avatar *string
	var active bool
	if err := row.Scan(&id, &subject, &email, &displayName, &avatar, &role, &active); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	return userFromValues(id, subject, email, displayName, avatar, role, active), nil
}

func scanManagedUser(row rowScanner) (User, int64, error) {
	var id pgtype.UUID
	var subject *string
	var email, displayName, role string
	var avatar *string
	var active bool
	var version int64
	if err := row.Scan(&id, &subject, &email, &displayName, &avatar, &role, &active, &version); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, 0, ErrUserNotFound
		}
		return User{}, 0, err
	}
	return userFromValues(id, subject, email, displayName, avatar, role, active), version, nil
}

func userFromValues(id pgtype.UUID, subject *string, email, displayName string, avatar *string, role string, active bool) User {
	return User{
		ID:            identifierFromDatabase(id),
		GoogleSubject: stringValue(subject),
		Email:         email,
		Login:         email,
		DisplayName:   displayName,
		AvatarURL:     stringValue(avatar),
		Role:          Role(role),
		Active:        active,
	}
}

func databaseUUID(value Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: true}
}

func optionalDatabaseUUID(value *Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return databaseUUID(*value)
}

func identifierFromDatabase(value pgtype.UUID) Identifier {
	return Identifier(value.Bytes)
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

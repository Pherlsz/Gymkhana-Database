package auth

import (
	"context"
	"errors"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	queries *dbgen.Queries
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{queries: dbgen.New(pool)}
}

func (store *PostgresStore) FindUserByGitHubID(ctx context.Context, githubUserID int64) (User, error) {
	value, err := store.queries.GetAppUserByGitHubID(ctx, githubUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	return userFromDatabase(value), nil
}

func (store *PostgresStore) FindUserByID(ctx context.Context, userID Identifier) (ManagedUser, error) {
	value, err := store.queries.GetAppUserByID(ctx, databaseUUID(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ManagedUser{}, ErrUserNotFound
	}
	if err != nil {
		return ManagedUser{}, err
	}
	return managedUserFromDatabase(value), nil
}

func (store *PostgresStore) ListUsers(ctx context.Context, limit, offset int32) ([]ManagedUser, error) {
	values, err := store.queries.ListAppUsers(ctx, dbgen.ListAppUsersParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	users := make([]ManagedUser, 0, len(values))
	for _, value := range values {
		users = append(users, managedUserFromDatabase(value))
	}
	return users, nil
}

func (store *PostgresStore) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	avatarURL := optionalString(params.Identity.AvatarURL)
	value, err := store.queries.CreateAppUser(ctx, dbgen.CreateAppUserParams{
		ID:           databaseUUID(params.ID),
		GithubUserID: params.Identity.UserID,
		GithubLogin:  params.Identity.Login,
		DisplayName:  params.Identity.DisplayName,
		AvatarUrl:    avatarURL,
		Role:         string(params.Role),
		Active:       true,
	})
	if err != nil {
		return User{}, err
	}
	return userFromDatabase(value), nil
}

func (store *PostgresStore) UpdateUserIdentity(ctx context.Context, userID Identifier, identity GitHubIdentity) (User, error) {
	value, err := store.queries.UpdateAppUserIdentity(ctx, dbgen.UpdateAppUserIdentityParams{
		ID:          databaseUUID(userID),
		GithubLogin: identity.Login,
		DisplayName: identity.DisplayName,
		AvatarUrl:   optionalString(identity.AvatarURL),
	})
	if err != nil {
		return User{}, err
	}
	return userFromDatabase(value), nil
}

func (store *PostgresStore) UpdateUserAccess(ctx context.Context, params UpdateUserAccessParams) (ManagedUser, error) {
	value, err := store.queries.UpdateAppUserAccess(ctx, dbgen.UpdateAppUserAccessParams{
		ID:      databaseUUID(params.UserID),
		Role:    string(params.Role),
		Active:  params.Active,
		Version: params.Version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ManagedUser{}, ErrUserAccessConflict
	}
	if err != nil {
		return ManagedUser{}, err
	}
	return managedUserFromDatabase(value), nil
}

func (store *PostgresStore) CountActiveSuperadmins(ctx context.Context) (int64, error) {
	return store.queries.CountActiveSuperadmins(ctx)
}

func (store *PostgresStore) CreateSession(ctx context.Context, params CreateSessionParams) error {
	_, err := store.queries.CreateAppSession(ctx, dbgen.CreateAppSessionParams{
		ID:        databaseUUID(params.ID),
		UserID:    databaseUUID(params.UserID),
		TokenHash: params.TokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: params.ExpiresAt, Valid: true},
	})
	return err
}

func (store *PostgresStore) FindAuthenticatedSession(ctx context.Context, tokenHash []byte, now time.Time) (Session, error) {
	value, err := store.queries.GetAuthenticatedAppSession(ctx, dbgen.GetAuthenticatedAppSessionParams{
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, err
	}
	return Session{
		ID: identifierFromDatabase(value.SessionID),
		User: User{
			ID:           identifierFromDatabase(value.AppUserID),
			GitHubUserID: value.GithubUserID,
			Login:        value.GithubLogin,
			DisplayName:  value.DisplayName,
			AvatarURL:    stringValue(value.AvatarUrl),
			Role:         Role(value.Role),
			Active:       value.Active,
		},
	}, nil
}

func (store *PostgresStore) TouchSession(ctx context.Context, sessionID Identifier) error {
	return store.queries.TouchAppSession(ctx, databaseUUID(sessionID))
}

func (store *PostgresStore) RevokeSessionByTokenHash(ctx context.Context, tokenHash []byte) error {
	return store.queries.RevokeAppSessionByTokenHash(ctx, tokenHash)
}

func (store *PostgresStore) RevokeAllSessionsForUser(ctx context.Context, userID Identifier) error {
	return store.queries.RevokeAllAppSessionsForUser(ctx, databaseUUID(userID))
}

func (store *PostgresStore) RecordAuditEvent(ctx context.Context, event AuditEvent) error {
	_, err := store.queries.CreateAuthAuditEvent(ctx, dbgen.CreateAuthAuditEventParams{
		ID:            databaseUUID(event.ID),
		ActorUserID:   optionalDatabaseUUID(event.ActorUserID),
		SubjectUserID: optionalDatabaseUUID(event.SubjectUserID),
		EventType:     string(event.EventType),
		Outcome:       string(event.Outcome),
		RequestID:     event.RequestID,
		ProviderLogin: optionalString(event.ProviderLogin),
	})
	return err
}

func userFromDatabase(value dbgen.AppUser) User {
	return User{
		ID:           identifierFromDatabase(value.ID),
		GitHubUserID: value.GithubUserID,
		Login:        value.GithubLogin,
		DisplayName:  value.DisplayName,
		AvatarURL:    stringValue(value.AvatarUrl),
		Role:         Role(value.Role),
		Active:       value.Active,
	}
}

func managedUserFromDatabase(value dbgen.AppUser) ManagedUser {
	return ManagedUser{User: userFromDatabase(value), Version: value.Version}
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

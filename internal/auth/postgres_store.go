package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (store *PostgresStore) FindUserByGitHubID(context.Context, int64) (User, error) {
	return User{}, ErrUserNotFound
}

func (store *PostgresStore) FindUserByGoogleSubject(ctx context.Context, subject string) (User, error) {
	return scanUser(store.pool.QueryRow(ctx, `
SELECT id, google_subject, email, display_name, avatar_url, role, active
FROM app_users
WHERE google_subject = $1`, subject))
}

func (store *PostgresStore) FindUserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(store.pool.QueryRow(ctx, `
SELECT id, google_subject, email, display_name, avatar_url, role, active
FROM app_users
WHERE lower(email) = lower($1)`, email))
}

func (store *PostgresStore) FindUserByID(ctx context.Context, userID Identifier) (ManagedUser, error) {
	user, version, err := scanManagedUser(store.pool.QueryRow(ctx, `
SELECT id, google_subject, email, display_name, avatar_url, role, active, version
FROM app_users
WHERE id = $1`, databaseUUID(userID)))
	if err != nil {
		return ManagedUser{}, err
	}
	return ManagedUser{User: user, Version: version}, nil
}

func (store *PostgresStore) ListUsers(ctx context.Context, limit, offset int32) ([]ManagedUser, error) {
	rows, err := store.pool.Query(ctx, `
SELECT id, google_subject, email, display_name, avatar_url, role, active, version
FROM app_users
ORDER BY lower(email), id
LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]ManagedUser, 0)
	for rows.Next() {
		user, version, scanErr := scanManagedUser(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		users = append(users, ManagedUser{User: user, Version: version})
	}
	return users, rows.Err()
}

func (store *PostgresStore) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	email := params.Identity.Email
	if email == "" {
		email = params.Identity.Login
	}
	return scanUser(store.pool.QueryRow(ctx, `
INSERT INTO app_users (id, google_subject, email, display_name, avatar_url, role, active)
VALUES ($1, $2, $3, $4, $5, $6, true)
RETURNING id, google_subject, email, display_name, avatar_url, role, active`,
		databaseUUID(params.ID), optionalString(params.Identity.Subject), email,
		params.Identity.DisplayName, optionalString(params.Identity.AvatarURL), string(params.Role)))
}

func (store *PostgresStore) UpdateUserIdentity(ctx context.Context, userID Identifier, identity GitHubIdentity) (User, error) {
	return store.UpdateGoogleIdentity(ctx, userID, identity)
}

func (store *PostgresStore) UpdateGoogleIdentity(ctx context.Context, userID Identifier, identity GoogleIdentity) (User, error) {
	email := identity.Email
	if email == "" {
		email = identity.Login
	}
	user, err := scanUser(store.pool.QueryRow(ctx, `
UPDATE app_users
SET google_subject = COALESCE(google_subject, $2),
    email = $3,
    display_name = $4,
    avatar_url = $5,
    updated_at = now(),
    version = version + 1
WHERE id = $1
  AND (google_subject IS NULL OR google_subject = $2)
RETURNING id, google_subject, email, display_name, avatar_url, role, active`,
		databaseUUID(userID), optionalString(identity.Subject), email,
		identity.DisplayName, optionalString(identity.AvatarURL)))
	if errors.Is(err, ErrUserNotFound) {
		return User{}, ErrAccessDenied
	}
	return user, err
}

func (store *PostgresStore) UpdateUserAccess(ctx context.Context, params UpdateUserAccessParams) (ManagedUser, error) {
	user, version, err := scanManagedUser(store.pool.QueryRow(ctx, `
UPDATE app_users
SET role = $2, active = $3, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $4
RETURNING id, google_subject, email, display_name, avatar_url, role, active, version`,
		databaseUUID(params.UserID), string(params.Role), params.Active, params.Version))
	if errors.Is(err, ErrUserNotFound) {
		return ManagedUser{}, ErrUserAccessConflict
	}
	if err != nil {
		return ManagedUser{}, err
	}
	return ManagedUser{User: user, Version: version}, nil
}

func (store *PostgresStore) CountActiveSuperadmins(ctx context.Context) (int64, error) {
	var count int64
	err := store.pool.QueryRow(ctx, `SELECT count(*) FROM app_users WHERE active AND role = 'SUPERADMIN'`).Scan(&count)
	return count, err
}

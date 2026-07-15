package auth

import (
	"context"
	"errors"

	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres/dbgen"
	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) FindManagedUser(ctx context.Context, userID Identifier) (ManagedUser, error) {
	value, err := store.queries.GetAppUserByID(ctx, databaseUUID(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ManagedUser{}, ErrUserNotFound
	}
	if err != nil {
		return ManagedUser{}, err
	}
	return managedUserFromDatabase(value), nil
}

func (store *PostgresStore) ListManagedUsers(ctx context.Context, limit, offset int32) ([]ManagedUser, error) {
	values, err := store.queries.ListAppUsers(ctx, dbgen.ListAppUsersParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}

	users := make([]ManagedUser, 0, len(values))
	for _, value := range values {
		users = append(users, managedUserFromDatabase(value))
	}
	return users, nil
}

func (store *PostgresStore) UpdateManagedUserAccess(ctx context.Context, update UserAccessUpdate) (ManagedUser, error) {
	value, err := store.queries.UpdateAppUserAccess(ctx, dbgen.UpdateAppUserAccessParams{
		ID:      databaseUUID(update.UserID),
		Role:    string(update.Role),
		Active:  update.Active,
		Version: update.Version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ManagedUser{}, ErrUserAccessConflict
	}
	if err != nil {
		return ManagedUser{}, err
	}
	return managedUserFromDatabase(value), nil
}

func (store *PostgresStore) RevokeAllSessionsForUser(ctx context.Context, userID Identifier) error {
	return store.queries.RevokeAllAppSessionsForUser(ctx, databaseUUID(userID))
}

func managedUserFromDatabase(value dbgen.AppUser) ManagedUser {
	return ManagedUser{
		ID:          identifierFromDatabase(value.ID),
		Login:       value.GithubLogin,
		DisplayName: value.DisplayName,
		AvatarURL:   stringValue(value.AvatarUrl),
		Role:        Role(value.Role),
		Active:      value.Active,
		Version:     value.Version,
	}
}

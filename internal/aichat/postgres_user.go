package aichat

import (
	"context"
	"errors"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) CurrentUser(ctx context.Context, id auth.Identifier) (auth.User, error) {
	if store == nil || store.pool == nil || id == (auth.Identifier{}) {
		return auth.User{}, ErrForbidden
	}
	var value auth.User
	var databaseID pgtype.UUID
	var subject, avatar pgtype.Text
	err := store.pool.QueryRow(ctx, `SELECT id,google_subject,email,display_name,avatar_url,role,active
FROM app_users WHERE id=$1`, chatAuthUUID(id)).Scan(
		&databaseID, &subject, &value.Email, &value.DisplayName, &avatar, &value.Role, &value.Active,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, ErrForbidden
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("load current AI Chat user: %w", err)
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

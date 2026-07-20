package googleforms

import (
	"context"
	"errors"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) GetActor(ctx context.Context, id auth.Identifier) (auth.Session, error) {
	value, err := scanActor(store.pool.QueryRow(ctx, `SELECT id, google_subject, email, display_name,
       avatar_url, role, active FROM app_users WHERE id=$1`, authDatabaseUUID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Session{}, ErrNotFound
	}
	if err != nil {
		return auth.Session{}, fmt.Errorf("get sync actor: %w", err)
	}
	return value, nil
}

package featureflags

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	KeyAIChat      = "ai_chat"
	KeyGoogleForms = "google_forms"
	KeyOCR         = "ocr"
	KeyAttachments = "attachments"
)

var (
	ErrUnknownFlag = errors.New("unknown feature flag")
	ErrForbidden   = errors.New("only superadmin can manage feature flags")
)

// Known lists every product flag SUPERADMIN can toggle.
var Known = []string{KeyAIChat, KeyGoogleForms, KeyOCR, KeyAttachments}

type Flag struct {
	Key       string
	Enabled   bool
	UpdatedAt time.Time
}

type Store interface {
	List(context.Context) ([]Flag, error)
	IsEnabled(context.Context, string) (bool, error)
	SetEnabled(context.Context, string, bool, *auth.Identifier) (Flag, error)
}

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (store *PostgresStore) List(ctx context.Context) ([]Flag, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT key, enabled, updated_at
		FROM app_feature_flags
		ORDER BY key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	flags := make([]Flag, 0, len(Known))
	for rows.Next() {
		var flag Flag
		if err := rows.Scan(&flag.Key, &flag.Enabled, &flag.UpdatedAt); err != nil {
			return nil, err
		}
		flags = append(flags, flag)
	}
	return flags, rows.Err()
}

func (store *PostgresStore) IsEnabled(ctx context.Context, key string) (bool, error) {
	key = normalizeKey(key)
	if !IsKnown(key) {
		return false, ErrUnknownFlag
	}
	var enabled bool
	err := store.pool.QueryRow(ctx, `SELECT enabled FROM app_feature_flags WHERE key = $1`, key).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return enabled, err
}

func (store *PostgresStore) SetEnabled(ctx context.Context, key string, enabled bool, actor *auth.Identifier) (Flag, error) {
	key = normalizeKey(key)
	if !IsKnown(key) {
		return Flag{}, ErrUnknownFlag
	}
	var flag Flag
	var actorUUID pgtype.UUID
	if actor != nil {
		actorUUID = pgtype.UUID{Bytes: [16]byte(*actor), Valid: true}
	}
	err := store.pool.QueryRow(ctx, `
		INSERT INTO app_feature_flags (key, enabled, updated_at, updated_by)
		VALUES ($1, $2, NOW(), $3)
		ON CONFLICT (key) DO UPDATE
		SET enabled = EXCLUDED.enabled,
		    updated_at = NOW(),
		    updated_by = EXCLUDED.updated_by
		RETURNING key, enabled, updated_at
	`, key, enabled, actorUUID).Scan(&flag.Key, &flag.Enabled, &flag.UpdatedAt)
	return flag, err
}

type Service struct {
	store Store
}

func NewService(store Store) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("feature flag store is required")
	}
	return &Service{store: store}, nil
}

func (service *Service) List(ctx context.Context, actor auth.Session) ([]Flag, error) {
	if actor.User.Role != auth.RoleSuperadmin || !actor.User.Active {
		return nil, ErrForbidden
	}
	return service.store.List(ctx)
}

func (service *Service) SetEnabled(ctx context.Context, actor auth.Session, key string, enabled bool) (Flag, error) {
	if actor.User.Role != auth.RoleSuperadmin || !actor.User.Active {
		return Flag{}, ErrForbidden
	}
	return service.store.SetEnabled(ctx, key, enabled, &actor.User.ID)
}

func (service *Service) IsEnabled(ctx context.Context, key string) (bool, error) {
	return service.store.IsEnabled(ctx, key)
}

func IsKnown(key string) bool {
	key = normalizeKey(key)
	for _, known := range Known {
		if key == known {
			return true
		}
	}
	return false
}

func normalizeKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

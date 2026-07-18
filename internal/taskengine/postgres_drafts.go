package taskengine

import (
	"context"
	"errors"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) CreateDraft(ctx context.Context, input CreateDraftInput) (Draft, error) {
	if store == nil || store.pool == nil || input.ID.IsZero() || input.OwnerUserID == (auth.Identifier{}) ||
		input.CatalogVersion == "" || input.Now.IsZero() || !input.ExpiresAt.After(input.Now) ||
		(input.State != DraftProposed && input.State != DraftReviewed) {
		return Draft{}, ErrInvalidInput
	}
	raw, err := taskJSON(input.Spec, 262144)
	if err != nil {
		return Draft{}, err
	}
	value, err := scanDraft(store.pool.QueryRow(ctx, `INSERT INTO task_drafts
(id,owner_user_id,catalog_version,state,spec_json,spec_fingerprint,version,expires_at,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,1,$7,$8,$8)
RETURNING `+draftColumns, taskUUID(input.ID), taskAuthUUID(input.OwnerUserID), input.CatalogVersion,
		string(input.State), raw, input.SpecFingerprint[:], input.ExpiresAt, input.Now))
	if err != nil {
		return Draft{}, taskPostgresError("create task draft", err)
	}
	return value, nil
}

func (store *PostgresStore) UpdateDraft(ctx context.Context, input UpdateDraftInput) (Draft, error) {
	if store == nil || store.pool == nil || input.ID.IsZero() || input.OwnerUserID == (auth.Identifier{}) ||
		input.CatalogVersion == "" || input.Version < 1 || input.Now.IsZero() {
		return Draft{}, ErrInvalidInput
	}
	raw, err := taskJSON(input.Spec, 262144)
	if err != nil {
		return Draft{}, err
	}
	value, err := scanDraft(store.pool.QueryRow(ctx, `UPDATE task_drafts SET
catalog_version=$3,state='REVIEWED',spec_json=$4,spec_fingerprint=$5,version=version+1,updated_at=$6
WHERE id=$1 AND owner_user_id=$2 AND version=$7 AND expires_at>$6
RETURNING `+draftColumns, taskUUID(input.ID), taskAuthUUID(input.OwnerUserID), input.CatalogVersion,
		raw, input.SpecFingerprint[:], input.Now, input.Version))
	if errors.Is(err, pgx.ErrNoRows) {
		if _, loadErr := store.GetDraft(ctx, input.ID, input.OwnerUserID); errors.Is(loadErr, ErrNotFound) {
			return Draft{}, ErrNotFound
		}
		return Draft{}, ErrConflict
	}
	if err != nil {
		return Draft{}, taskPostgresError("review task draft", err)
	}
	return value, nil
}

func (store *PostgresStore) GetDraft(ctx context.Context, id Identifier, owner auth.Identifier) (Draft, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) {
		return Draft{}, ErrNotFound
	}
	value, err := scanDraft(store.pool.QueryRow(ctx, `SELECT `+draftColumns+` FROM task_drafts WHERE id=$1 AND owner_user_id=$2`, taskUUID(id), taskAuthUUID(owner)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Draft{}, ErrNotFound
	}
	if err != nil {
		return Draft{}, fmt.Errorf("load task draft: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) GetDraftForWorker(ctx context.Context, id Identifier) (Draft, error) {
	if store == nil || store.pool == nil || id.IsZero() {
		return Draft{}, ErrNotFound
	}
	value, err := scanDraft(store.pool.QueryRow(ctx, `SELECT `+draftColumns+` FROM task_drafts WHERE id=$1`, taskUUID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Draft{}, ErrNotFound
	}
	if err != nil {
		return Draft{}, fmt.Errorf("load task worker draft: %w", err)
	}
	return value, nil
}

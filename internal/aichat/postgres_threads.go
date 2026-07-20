package aichat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) CreateThread(ctx context.Context, input CreateThreadInput) (Thread, error) {
	if store == nil || store.pool == nil {
		return Thread{}, ErrInvalidSetup
	}
	return scanThread(store.pool.QueryRow(ctx, `INSERT INTO ai_chat_threads
(id,owner_user_id,title,retention_expires_at,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$5)
RETURNING id,owner_user_id,title,active_result_reference_id,retention_expires_at,version,created_at,updated_at`,
		chatUUID(input.ID), chatAuthUUID(input.OwnerUserID), input.Title, input.RetentionExpiresAt, input.Now))
}

func (store *PostgresStore) ListThreads(ctx context.Context, owner auth.Identifier, now time.Time, limit, offset int) (ThreadPage, error) {
	if store == nil || store.pool == nil {
		return ThreadPage{}, ErrInvalidSetup
	}
	rows, err := store.pool.Query(ctx, threadSelect+`
WHERE owner_user_id=$1 AND retention_expires_at>$2 ORDER BY updated_at DESC,id DESC LIMIT $3 OFFSET $4`, chatAuthUUID(owner), now, limit, offset)
	if err != nil {
		return ThreadPage{}, fmt.Errorf("list AI Chat threads: %w", err)
	}
	defer rows.Close()
	page := ThreadPage{Threads: make([]Thread, 0), Limit: limit, Offset: offset}
	for rows.Next() {
		value, err := scanThread(rows)
		if err != nil {
			return ThreadPage{}, fmt.Errorf("scan AI Chat thread: %w", err)
		}
		page.Threads = append(page.Threads, value)
	}
	if err := rows.Err(); err != nil {
		return ThreadPage{}, fmt.Errorf("iterate AI Chat threads: %w", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM ai_chat_threads WHERE owner_user_id=$1 AND retention_expires_at>$2`, chatAuthUUID(owner), now).Scan(&page.Total); err != nil {
		return ThreadPage{}, fmt.Errorf("count AI Chat threads: %w", err)
	}
	return page, nil
}

func (store *PostgresStore) GetThread(ctx context.Context, id Identifier, owner auth.Identifier) (Thread, error) {
	if store == nil || store.pool == nil {
		return Thread{}, ErrInvalidSetup
	}
	value, err := scanThread(store.pool.QueryRow(ctx, threadSelect+` WHERE id=$1 AND owner_user_id=$2`, chatUUID(id), chatAuthUUID(owner)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Thread{}, ErrNotFound
	}
	if err != nil {
		return Thread{}, fmt.Errorf("load AI Chat thread: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) RenameThread(ctx context.Context, id Identifier, owner auth.Identifier, title string, version int64, now time.Time) (Thread, error) {
	value, err := scanThread(store.pool.QueryRow(ctx, `UPDATE ai_chat_threads SET title=$3,version=version+1,updated_at=$4
WHERE id=$1 AND owner_user_id=$2 AND version=$5
RETURNING id,owner_user_id,title,active_result_reference_id,retention_expires_at,version,created_at,updated_at`,
		chatUUID(id), chatAuthUUID(owner), title, now, version))
	if errors.Is(err, pgx.ErrNoRows) {
		_, loadErr := store.GetThread(ctx, id, owner)
		if loadErr == nil {
			return Thread{}, ErrConflict
		}
		if errors.Is(loadErr, ErrNotFound) {
			return Thread{}, ErrNotFound
		}
		return Thread{}, loadErr
	}
	if err != nil {
		return Thread{}, fmt.Errorf("rename AI Chat thread: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) DeleteThread(ctx context.Context, id Identifier, owner auth.Identifier) error {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin AI Chat thread deletion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var locked pgtype.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM ai_chat_threads WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, chatUUID(id), chatAuthUUID(owner)).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock AI Chat thread for deletion: %w", err)
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ai_chat_runs WHERE thread_id=$1 AND state IN ('QUEUED','RUNNING','TOOL_RUNNING'))`, chatUUID(id)).Scan(&active); err != nil {
		return fmt.Errorf("check active AI Chat run: %w", err)
	}
	if active {
		return ErrConflict
	}
	if _, err := tx.Exec(ctx, `DELETE FROM ai_chat_threads WHERE id=$1`, chatUUID(id)); err != nil {
		return fmt.Errorf("delete AI Chat thread: %w", err)
	}
	return normalizePostgresError(tx.Commit(ctx))
}

func scanThread(row rowScanner) (Thread, error) {
	var value Thread
	var id, owner, active pgtype.UUID
	if err := row.Scan(&id, &owner, &value.Title, &active, &value.RetentionExpiresAt, &value.Version, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return Thread{}, err
	}
	value.ID, value.OwnerUserID = chatIdentifier(id), auth.Identifier(owner.Bytes)
	value.ActiveResultReferenceID = optionalIdentifier(active)
	return value, nil
}

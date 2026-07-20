package operations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) CreateExport(ctx context.Context, value Export, limits Limits) (Export, error) {
	if store == nil || store.pool == nil || value.ID.IsZero() || !value.Module.Valid() || value.ActorUserID == (auth.Identifier{}) {
		return Export{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Export{}, fmt.Errorf("begin export creation: %w", err)
	}
	defer tx.Rollback(ctx)
	actorID := authDatabaseUUID(value.ActorUserID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 8091))`, actorID); err != nil {
		return Export{}, fmt.Errorf("lock export actor: %w", err)
	}
	existing, err := scanExport(tx.QueryRow(ctx, exportSelect+` WHERE actor_user_id=$1 AND idempotency_key=$2`, actorID, value.IdempotencyKey))
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Export{}, fmt.Errorf("find idempotent export: %w", err)
	}
	if err := reserveOperationLimit(ctx, tx, actorID, limits); err != nil {
		return Export{}, err
	}
	created, err := scanExport(tx.QueryRow(ctx, `INSERT INTO operation_exports (
  id, actor_user_id, module, idempotency_key, state, object_key, filename,
  river_job_id, expires_at, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)
RETURNING id, actor_user_id, module, idempotency_key, state, object_key,
          filename, row_count, byte_size, content_sha256, river_job_id, error_code,
          expires_at, completed_at, version, created_at, updated_at`, databaseUUID(value.ID), actorID,
		value.Module, value.IdempotencyKey, value.State, value.ObjectKey, value.Filename,
		nullablePositiveInt64(value.RiverJobID), value.ExpiresAt, value.CreatedAt))
	if err != nil {
		return Export{}, mapPostgresError("create export", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Export{}, fmt.Errorf("commit export creation: %w", err)
	}
	return created, nil
}

func (store *PostgresStore) GetExport(ctx context.Context, id Identifier, actorID auth.Identifier) (Export, error) {
	value, err := scanExport(store.pool.QueryRow(ctx, exportSelect+` WHERE id=$1 AND actor_user_id=$2`, databaseUUID(id), authDatabaseUUID(actorID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Export{}, ErrNotFound
	}
	if err != nil {
		return Export{}, fmt.Errorf("get export: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) getExportTrusted(ctx context.Context, id Identifier) (Export, error) {
	value, err := scanExport(store.pool.QueryRow(ctx, exportSelect+` WHERE id=$1`, databaseUUID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Export{}, ErrNotFound
	}
	if err != nil {
		return Export{}, fmt.Errorf("get trusted export: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) GetExportForWorker(ctx context.Context, id Identifier) (Export, error) {
	return store.getExportTrusted(ctx, id)
}

func (store *PostgresStore) ListExports(ctx context.Context, actorID auth.Identifier, options ListOptions) (ExportPage, error) {
	options = normalizeListOptions(options)
	if options.Limit == 0 {
		return ExportPage{}, ErrInvalidInput
	}
	var total int64
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM operation_exports WHERE actor_user_id=$1`, authDatabaseUUID(actorID)).Scan(&total); err != nil {
		return ExportPage{}, fmt.Errorf("count exports: %w", err)
	}
	rows, err := store.pool.Query(ctx, exportSelect+` WHERE actor_user_id=$1 ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`, authDatabaseUUID(actorID), options.Limit, options.Offset)
	if err != nil {
		return ExportPage{}, fmt.Errorf("list exports: %w", err)
	}
	defer rows.Close()
	page := ExportPage{Total: total, Limit: options.Limit, Offset: options.Offset}
	for rows.Next() {
		value, scanErr := scanExport(rows)
		if scanErr != nil {
			return ExportPage{}, fmt.Errorf("scan export: %w", scanErr)
		}
		page.Exports = append(page.Exports, value)
	}
	if err := rows.Err(); err != nil {
		return ExportPage{}, fmt.Errorf("iterate exports: %w", err)
	}
	return page, nil
}

func (store *PostgresStore) BeginExport(ctx context.Context, id Identifier, now time.Time) (Export, error) {
	value, err := scanExport(store.pool.QueryRow(ctx, `UPDATE operation_exports
   SET state='RUNNING', version=version+1, updated_at=$2
 WHERE id=$1 AND state IN ('QUEUED','RUNNING') AND expires_at>$2
RETURNING id, actor_user_id, module, idempotency_key, state, object_key,
          filename, row_count, byte_size, content_sha256, river_job_id, error_code,
          expires_at, completed_at, version, created_at, updated_at`, databaseUUID(id), now))
	if errors.Is(err, pgx.ErrNoRows) {
		value, getErr := store.getExportTrusted(ctx, id)
		if getErr == nil && value.State == ExportCompleted {
			return value, nil
		}
		if getErr == nil && !value.ExpiresAt.After(now) {
			return Export{}, ErrExpired
		}
		return Export{}, ErrInvalidState
	}
	if err != nil {
		return Export{}, fmt.Errorf("begin export: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) CompleteExport(ctx context.Context, id Identifier, rowCount int, byteSize int64, sha [32]byte, now time.Time) (Export, error) {
	value, err := scanExport(store.pool.QueryRow(ctx, `UPDATE operation_exports
   SET state='COMPLETED', row_count=$2, byte_size=$3, content_sha256=$4,
       completed_at=$5, version=version+1, updated_at=$5
 WHERE id=$1 AND state='RUNNING'
RETURNING id, actor_user_id, module, idempotency_key, state, object_key,
          filename, row_count, byte_size, content_sha256, river_job_id, error_code,
          expires_at, completed_at, version, created_at, updated_at`, databaseUUID(id), rowCount, byteSize, sha[:], now))
	if errors.Is(err, pgx.ErrNoRows) {
		value, getErr := store.getExportTrusted(ctx, id)
		if getErr == nil && value.State == ExportCompleted {
			return value, nil
		}
		return Export{}, ErrInvalidState
	}
	if err != nil {
		return Export{}, fmt.Errorf("complete export: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) FailExport(ctx context.Context, id Identifier, expected ExportState, code string, now time.Time) (bool, error) {
	command, err := store.pool.Exec(ctx, `UPDATE operation_exports
   SET state='FAILED', error_code=$2, version=version+1, updated_at=$3
 WHERE id=$1 AND state=$4`, databaseUUID(id), code, now, expected)
	if err != nil {
		return false, fmt.Errorf("fail export: %w", err)
	}
	return command.RowsAffected() == 1, nil
}

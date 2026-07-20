package googleforms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) CreateSyncRun(ctx context.Context, value SyncRun) (SyncRun, error) {
	if store == nil || store.pool == nil || value.ID.IsZero() || value.SourceID.IsZero() ||
		value.OwnerUserID == (auth.Identifier{}) ||
		(value.TriggerKind != TriggerManual && value.TriggerKind != TriggerScheduled) ||
		len(value.IdempotencyKey) < 8 || len(value.IdempotencyKey) > 128 {
		return SyncRun{}, ErrInvalidInput
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return SyncRun{}, fmt.Errorf("begin sync run creation: %w", err)
	}
	defer tx.Rollback(ctx)
	ownerID := authDatabaseUUID(value.OwnerUserID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 9110))`, ownerID); err != nil {
		return SyncRun{}, fmt.Errorf("lock sync owner: %w", err)
	}
	existing, err := scanSyncRun(tx.QueryRow(ctx, syncRunSelect+` WHERE source_id=$1 AND idempotency_key=$2`,
		databaseUUID(value.SourceID), value.IdempotencyKey))
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return SyncRun{}, fmt.Errorf("commit replayed sync run: %w", err)
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SyncRun{}, fmt.Errorf("load replayed sync run: %w", err)
	}
	var active int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM google_forms_sync_runs
 WHERE owner_user_id=$1 AND state IN ('QUEUED','RUNNING')`, ownerID).Scan(&active); err != nil {
		return SyncRun{}, fmt.Errorf("count active owner syncs: %w", err)
	}
	if active >= MaximumActiveSyncs {
		return SyncRun{}, ErrRateLimited
	}
	created, err := scanSyncRun(tx.QueryRow(ctx, `INSERT INTO google_forms_sync_runs (
  id, source_id, owner_user_id, actor_user_id, trigger_kind, state,
  idempotency_key, cursor_started_at, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,'QUEUED',$6,$7,$8,$8)
RETURNING id, source_id, owner_user_id, actor_user_id, trigger_kind,
          state, idempotency_key, operation_import_id, cursor_started_at,
          cursor_completed_at, received_count, staged_count, duplicate_count,
          river_job_id, error_code, started_at, completed_at, version, created_at, updated_at`,
		databaseUUID(value.ID), databaseUUID(value.SourceID), authDatabaseUUID(value.OwnerUserID),
		optionalAuthDatabaseUUID(value.ActorUserID), value.TriggerKind, value.IdempotencyKey,
		optionalTime(value.CursorStartedAt), value.CreatedAt))
	if err != nil {
		return SyncRun{}, mapPostgresError("create sync run", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return SyncRun{}, fmt.Errorf("commit sync run creation: %w", err)
	}
	return created, nil
}

func (store *PostgresStore) SetSyncJob(ctx context.Context, id Identifier, jobID int64, now time.Time) error {
	if jobID <= 0 {
		return ErrInvalidInput
	}
	command, err := store.pool.Exec(ctx, `UPDATE google_forms_sync_runs
   SET river_job_id=$2, version=version+1, updated_at=$3
 WHERE id=$1 AND state='QUEUED' AND (river_job_id IS NULL OR river_job_id=$2)`, databaseUUID(id), jobID, now)
	if err != nil {
		return fmt.Errorf("set sync job: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

package googleforms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) CompleteSync(ctx context.Context, id Identifier, cursor *time.Time, nextPageToken string, received, staged, duplicates int, now time.Time) (SyncRun, error) {
	if len(nextPageToken) > 2048 {
		return SyncRun{}, ErrInvalidInput
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return SyncRun{}, fmt.Errorf("begin sync completion: %w", err)
	}
	defer tx.Rollback(ctx)
	run, err := scanSyncRun(tx.QueryRow(ctx, `UPDATE google_forms_sync_runs
   SET state='COMPLETED', cursor_completed_at=$2, received_count=$3,
       staged_count=$4, duplicate_count=$5, completed_at=$6,
       error_code=NULL, version=version+1, updated_at=$6
 WHERE id=$1 AND state='RUNNING'
RETURNING id, source_id, owner_user_id, actor_user_id, trigger_kind,
          state, idempotency_key, operation_import_id, cursor_started_at,
          cursor_completed_at, received_count, staged_count, duplicate_count,
          river_job_id, error_code, started_at, completed_at, version, created_at, updated_at`,
		databaseUUID(id), optionalTime(cursor), received, staged, duplicates, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return SyncRun{}, ErrInvalidState
	}
	if err != nil {
		return SyncRun{}, fmt.Errorf("complete sync run: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE google_forms_sources
   SET cursor_submitted_at=CASE
         WHEN $2::timestamptz IS NULL THEN cursor_submitted_at
         WHEN cursor_submitted_at IS NULL OR cursor_submitted_at<$2 THEN $2
         ELSE cursor_submitted_at
       END,
       response_page_token=NULLIF($3,''),
       page_token_cursor_started_at=CASE
         WHEN $3='' THEN NULL
         ELSE COALESCE(page_token_cursor_started_at,$4)
       END,
       last_synced_at=$5,
       next_sync_at=CASE WHEN state='ACTIVE' AND sync_mode='POLL'
         THEN $5::timestamptz+make_interval(secs=>poll_interval_seconds) ELSE NULL END,
       error_code=NULL, version=version+1, updated_at=$5
 WHERE id=$1`, databaseUUID(run.SourceID), optionalTime(cursor), nextPageToken,
		optionalTime(run.CursorStartedAt), now); err != nil {
		return SyncRun{}, fmt.Errorf("advance source cursor: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return SyncRun{}, fmt.Errorf("commit sync completion: %w", err)
	}
	return run, nil
}

func (store *PostgresStore) FailSync(ctx context.Context, id Identifier, code string, now time.Time) (bool, error) {
	command, err := store.pool.Exec(ctx, `UPDATE google_forms_sync_runs
   SET state='FAILED', error_code=$2, completed_at=$3,
       version=version+1, updated_at=$3
 WHERE id=$1 AND state IN ('QUEUED','RUNNING')`, databaseUUID(id), code, now)
	if err != nil {
		return false, fmt.Errorf("fail sync run: %w", err)
	}
	return command.RowsAffected() == 1, nil
}

func (store *PostgresStore) CancelSync(ctx context.Context, id Identifier, ownerID auth.Identifier, version int64, now time.Time) (SyncRun, error) {
	value, err := scanSyncRun(store.pool.QueryRow(ctx, `UPDATE google_forms_sync_runs
   SET state='CANCELLED', completed_at=$4, error_code=NULL,
       version=version+1, updated_at=$4
 WHERE id=$1 AND owner_user_id=$2 AND version=$3 AND state IN ('QUEUED','RUNNING')
RETURNING id, source_id, owner_user_id, actor_user_id, trigger_kind,
          state, idempotency_key, operation_import_id, cursor_started_at,
          cursor_completed_at, received_count, staged_count, duplicate_count,
          river_job_id, error_code, started_at, completed_at, version, created_at, updated_at`,
		databaseUUID(id), authDatabaseUUID(ownerID), version, now))
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if queryErr := store.pool.QueryRow(ctx, `SELECT EXISTS (
  SELECT 1 FROM google_forms_sync_runs WHERE id=$1 AND owner_user_id=$2
)`, databaseUUID(id), authDatabaseUUID(ownerID)).Scan(&exists); queryErr != nil {
			return SyncRun{}, fmt.Errorf("classify sync cancellation: %w", queryErr)
		}
		if !exists {
			return SyncRun{}, ErrNotFound
		}
		return SyncRun{}, ErrConflict
	}
	if err != nil {
		return SyncRun{}, fmt.Errorf("cancel sync: %w", err)
	}
	return value, nil
}

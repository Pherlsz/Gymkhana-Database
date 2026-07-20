package googleforms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) BeginSync(ctx context.Context, id Identifier, now time.Time) (SyncRun, Source, Connection, auth.Session, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("begin sync: %w", err)
	}
	defer tx.Rollback(ctx)
	run, err := scanSyncRun(tx.QueryRow(ctx, syncRunSelect+` WHERE id=$1 FOR UPDATE`, databaseUUID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrNotFound
	}
	if err != nil {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("lock sync run: %w", err)
	}
	switch run.State {
	case SyncCancelled:
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrCancelled
	case SyncCompleted, SyncFailed:
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrInvalidState
	case SyncQueued:
		run, err = scanSyncRun(tx.QueryRow(ctx, `UPDATE google_forms_sync_runs
   SET state='RUNNING', started_at=COALESCE(started_at,$2), version=version+1, updated_at=$2
 WHERE id=$1
RETURNING id, source_id, owner_user_id, actor_user_id, trigger_kind,
          state, idempotency_key, operation_import_id, cursor_started_at,
          cursor_completed_at, received_count, staged_count, duplicate_count,
          river_job_id, error_code, started_at, completed_at, version, created_at, updated_at`, databaseUUID(id), now))
		if err != nil {
			return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("start sync run: %w", err)
		}
	case SyncRunning:
	default:
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrInvalidState
	}
	source, err := scanSource(tx.QueryRow(ctx, sourceSelect+` WHERE id=$1 AND owner_user_id=$2 FOR SHARE`, databaseUUID(run.SourceID), authDatabaseUUID(run.OwnerUserID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrNotFound
	}
	if err != nil {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("load sync source: %w", err)
	}
	if source.State != SourceActive {
		if source.State == SourceNeedsReauth {
			return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrNeedsReauth
		}
		if source.State == SourceSchemaDrift {
			return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrSchemaDrift
		}
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrInvalidState
	}
	connection, err := scanConnection(tx.QueryRow(ctx, connectionSelect+` WHERE id=$1 AND owner_user_id=$2 FOR SHARE`, databaseUUID(source.ConnectionID), authDatabaseUUID(source.OwnerUserID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrNotFound
	}
	if err != nil {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("load sync connection: %w", err)
	}
	if connection.State != ConnectionActive {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrNeedsReauth
	}
	actorID := run.OwnerUserID
	if run.ActorUserID != nil {
		actorID = *run.ActorUserID
	}
	actor, err := scanActor(tx.QueryRow(ctx, `SELECT id, google_subject, email, display_name,
       avatar_url, role, active FROM app_users WHERE id=$1`, authDatabaseUUID(actorID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrForbidden
	}
	if err != nil {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("load sync actor: %w", err)
	}
	if !actor.User.Active || !actor.User.Role.CanManageGoogleForms() || actor.User.ID != source.OwnerUserID {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrForbidden
	}
	if err := tx.Commit(ctx); err != nil {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("commit sync start: %w", err)
	}
	return run, source, connection, actor, nil
}

package googleforms

import (
	"context"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) ListSyncRuns(ctx context.Context, ownerID auth.Identifier, sourceID *Identifier, limit, offset int) (SyncRunPage, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return SyncRunPage{}, ErrInvalidInput
	}
	filter := ` WHERE owner_user_id=$1`
	arguments := []any{authDatabaseUUID(ownerID)}
	if sourceID != nil {
		filter += ` AND source_id=$2`
		arguments = append(arguments, databaseUUID(*sourceID))
	}
	var total int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM google_forms_sync_runs`+filter, arguments...).Scan(&total); err != nil {
		return SyncRunPage{}, fmt.Errorf("count sync runs: %w", err)
	}
	limitPosition := len(arguments) + 1
	offsetPosition := len(arguments) + 2
	query := syncRunSelect + filter + fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`, limitPosition, offsetPosition)
	arguments = append(arguments, limit, offset)
	rows, err := store.pool.Query(ctx, query, arguments...)
	if err != nil {
		return SyncRunPage{}, fmt.Errorf("list sync runs: %w", err)
	}
	defer rows.Close()
	page := SyncRunPage{Total: total}
	for rows.Next() {
		value, scanErr := scanSyncRun(rows)
		if scanErr != nil {
			return SyncRunPage{}, fmt.Errorf("scan sync run: %w", scanErr)
		}
		page.Runs = append(page.Runs, value)
	}
	if err := rows.Err(); err != nil {
		return SyncRunPage{}, fmt.Errorf("iterate sync runs: %w", err)
	}
	return page, nil
}

func (store *PostgresStore) RecordAuditEvent(ctx context.Context, value AuditEvent) error {
	if value.ID.IsZero() || value.EventType == "" || value.Outcome == "" {
		return ErrInvalidInput
	}
	_, err := store.pool.Exec(ctx, `INSERT INTO google_forms_audit_events (
  id, actor_user_id, connection_id, source_id, sync_run_id, event_type,
  outcome, affected_count, request_id, created_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, databaseUUID(value.ID),
		optionalAuthDatabaseUUID(value.ActorUserID), optionalIdentifierUUID(value.ConnectionID),
		optionalIdentifierUUID(value.SourceID), optionalIdentifierUUID(value.SyncRunID),
		value.EventType, value.Outcome, value.AffectedCount, value.RequestID, value.CreatedAt)
	if err != nil {
		return mapPostgresError("record google forms audit", err)
	}
	return nil
}

func optionalIdentifierUUID(value *Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return databaseUUID(*value)
}

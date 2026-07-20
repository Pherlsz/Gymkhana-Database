package operations

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) BulkDelete(ctx context.Context, actorID auth.Identifier, module Module, items []BulkItem, requestID string, now time.Time) (int, error) {
	if !module.Valid() || len(items) == 0 || len(items) > MaximumBulkSelection || requestID == "" || len(requestID) > 128 {
		return 0, ErrInvalidInput
	}
	items = append([]BulkItem(nil), items...)
	sort.Slice(items, func(left, right int) bool { return items[left].ID.String() < items[right].ID.String() })
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin bulk delete: %w", err)
	}
	defer tx.Rollback(ctx)
	var actorActive bool
	var actorRole auth.Role
	if err := tx.QueryRow(ctx, `SELECT active, role FROM app_users WHERE id=$1 FOR SHARE`, authDatabaseUUID(actorID)).Scan(&actorActive, &actorRole); errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrForbidden
	} else if err != nil {
		return 0, fmt.Errorf("authorize operation bulk delete: %w", err)
	}
	catalog, ok := moduleCatalog(actorRole, module)
	if !actorActive || !ok || !catalog.CanBulkDelete {
		return 0, ErrForbidden
	}
	seen := make(map[Identifier]struct{}, len(items))
	for _, item := range items {
		if item.ID.IsZero() || item.Version <= 0 {
			return 0, ErrInvalidInput
		}
		if _, duplicate := seen[item.ID]; duplicate {
			return 0, ErrInvalidInput
		}
		seen[item.ID] = struct{}{}
		if err := deleteCanonicalItem(ctx, tx, actorID, module, item, requestID, now); err != nil {
			return 0, err
		}
	}
	auditID, err := NewIdentifier()
	if err != nil {
		return 0, err
	}
	count := len(items)
	if _, err := tx.Exec(ctx, `INSERT INTO operation_audit_events (
  id, actor_user_id, module, event_type, outcome, affected_count, request_id, created_at
) VALUES ($1,$2,$3,'BULK_DELETE','SUCCESS',$4,$5,$6)`, databaseUUID(auditID), authDatabaseUUID(actorID), module, count, requestID, now); err != nil {
		return 0, fmt.Errorf("audit bulk delete: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit bulk delete: %w", err)
	}
	return count, nil
}

func deleteCanonicalItem(ctx context.Context, tx pgx.Tx, actorID auth.Identifier, module Module, item BulkItem, requestID string, now time.Time) error {
	var table, auditTable, idColumn, eventType string
	switch module {
	case ModuleProfiles:
		table, auditTable, idColumn, eventType = "profiles", "profile_audit_events", "profile_id", "PROFILE_DELETED"
	case ModuleDocuments:
		table, auditTable, idColumn, eventType = "documents", "document_audit_events", "document_id", "DOCUMENT_DELETED"
	case ModuleBills:
		table, auditTable, idColumn, eventType = "bills", "bill_audit_events", "bill_id", "BILL_DELETED"
	default:
		return ErrInvalidInput
	}
	command, err := tx.Exec(ctx, `DELETE FROM `+table+` WHERE id=$1 AND version=$2`, databaseUUID(item.ID), item.Version)
	if err != nil {
		return mapPostgresError("bulk delete canonical record", err)
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	auditID, err := NewIdentifier()
	if err != nil {
		return err
	}
	query := `INSERT INTO ` + auditTable + ` (id, actor_user_id, ` + idColumn + `, event_type, outcome, request_id, occurred_at)
VALUES ($1,$2,$3,$4,'SUCCESS',$5,$6)`
	if _, err := tx.Exec(ctx, query, databaseUUID(auditID), authDatabaseUUID(actorID), databaseUUID(item.ID), eventType, requestID, now); err != nil {
		return fmt.Errorf("audit canonical bulk delete: %w", err)
	}
	return nil
}

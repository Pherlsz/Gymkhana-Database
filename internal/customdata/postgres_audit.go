package customdata

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) RecordAuditEvent(ctx context.Context, event AuditEvent) error {
	var resourceID, targetKind, targetID any
	if event.ResourceID != nil {
		resourceID = databaseUUID(*event.ResourceID)
	}
	if event.Target != nil {
		targetKind = string(event.Target.Kind)
		targetID = databaseUUID(event.Target.ID)
	}
	_, err := store.db.Exec(ctx, `INSERT INTO custom_data_audit_events
(id, actor_user_id, resource_kind, resource_id, target_kind, target_id, event_type, outcome, request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, databaseUUID(event.ID), pgtype.UUID{Bytes: [16]byte(event.ActorUserID), Valid: true}, string(event.ResourceKind), resourceID,
		targetKind, targetID, string(event.EventType), string(event.Outcome), normalizeRequestID(event.RequestID))
	if err != nil {
		return fmt.Errorf("record custom data audit event: %w", err)
	}
	return nil
}

package bill

import (
	"context"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres/dbgen"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	"github.com/jackc/pgx/v5/pgtype"
)

type billAuditQueries interface {
	RecordBillAuditEvent(context.Context, dbgen.RecordBillAuditEventParams) error
}

func (store *PostgresStore) RecordAuditEvent(ctx context.Context, event AuditEvent) error {
	queries, ok := store.queries.(billAuditQueries)
	if !ok {
		return fmt.Errorf("record bill audit event: generated query is unavailable")
	}
	params := dbgen.RecordBillAuditEventParams{
		ID:              databaseUUID(event.ID),
		ActorUserID:     pgtype.UUID{Bytes: event.ActorUserID, Valid: true},
		BillID:          optionalDatabaseUUID(event.BillID),
		SourceBillID:    optionalDatabaseUUID(event.SourceBillID),
		BillTypeID:      optionalDatabaseUUID(event.BillTypeID),
		HolderProfileID: optionalAuditProfileUUID(event.HolderProfileID),
		EventType:       string(event.EventType),
		Outcome:         string(event.Outcome),
		RequestID:       event.RequestID,
	}
	if err := queries.RecordBillAuditEvent(ctx, params); err != nil {
		return fmt.Errorf("record bill audit event: %w", err)
	}
	return nil
}

func optionalAuditProfileUUID(identifier *profile.Identifier) pgtype.UUID {
	if identifier == nil {
		return pgtype.UUID{}
	}
	return profileUUID(*identifier)
}

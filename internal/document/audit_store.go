package document

import (
	"context"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres/dbgen"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	"github.com/jackc/pgx/v5/pgtype"
)

type documentAuditQueries interface {
	RecordDocumentAuditEvent(context.Context, dbgen.RecordDocumentAuditEventParams) error
}

func (store *PostgresStore) RecordAuditEvent(ctx context.Context, event AuditEvent) error {
	queries, ok := store.queries.(documentAuditQueries)
	if !ok {
		return fmt.Errorf("record document audit event: generated query is unavailable")
	}
	params := dbgen.RecordDocumentAuditEventParams{
		ID:               databaseUUID(event.ID),
		ActorUserID:      pgtype.UUID{Bytes: event.ActorUserID, Valid: true},
		DocumentID:       optionalDatabaseUUID(event.DocumentID),
		SourceDocumentID: optionalDatabaseUUID(event.SourceDocumentID),
		DocumentTypeID:   optionalDatabaseUUID(event.DocumentTypeID),
		HolderProfileID:  optionalAuditProfileUUID(event.HolderProfileID),
		EventType:        string(event.EventType),
		Outcome:          string(event.Outcome),
		RequestID:        event.RequestID,
	}
	if err := queries.RecordDocumentAuditEvent(ctx, params); err != nil {
		return fmt.Errorf("record document audit event: %w", err)
	}
	return nil
}

func optionalAuditProfileUUID(identifier *profile.Identifier) pgtype.UUID {
	if identifier == nil {
		return pgtype.UUID{}
	}
	return profileUUID(*identifier)
}

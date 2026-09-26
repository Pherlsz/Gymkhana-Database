package operations

import (
	"context"
	"errors"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/importcatalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func syncImportedSidecarDocuments(ctx context.Context, tx pgx.Tx, profileID Identifier, mapped map[string]string, now time.Time) error {
	for _, sidecar := range importcatalog.CollectDocumentSidecars(mapped) {
		typeKey := importcatalog.ResolveDocumentType(sidecar.TypeKey, sidecar.Identifier)
		if typeKey == "" {
			continue
		}
		identifier := importcatalog.CanonicalSidecarIdentifier(typeKey, sidecar.Identifier)
		if identifier == "" {
			continue
		}
		var typeID pgtype.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM document_types WHERE technical_key=$1 AND active=true`, typeKey).Scan(&typeID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		var identifierArg any = identifier
		presenceID, err := upsertImportedDocumentPresence(ctx, tx, profileID, identifierFromUUID(typeID), identifierArg, now)
		if err != nil {
			return err
		}
		docID, err := NewIdentifier()
		if err != nil {
			return err
		}
		date := sidecar.Date
		if date != "" {
			date = coerceCivilDate(date)
		}
		_, err = tx.Exec(ctx, `INSERT INTO documents (
  id, presence_id, document_date, notes, medium, idle_custody, valid_until, created_at, updated_at
) VALUES ($1,$2,$3,NULL,'DIGITAL',NULL,NULL,$4,$4)
ON CONFLICT (presence_id, medium) DO UPDATE
SET document_date = COALESCE(EXCLUDED.document_date, documents.document_date),
    version = documents.version + 1, updated_at = EXCLUDED.updated_at`,
			databaseUUID(docID), databaseUUID(presenceID), nullableDate(date), now)
		if err != nil {
			return err
		}
	}
	return nil
}

package customdata

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type transactionDB interface {
	dbgen.DBTX
	Begin(context.Context) (pgx.Tx, error)
}

type PostgresStore struct {
	db transactionDB
}

func NewPostgresStore(database transactionDB) *PostgresStore {
	return &PostgresStore{db: database}
}

func (store *PostgresStore) withTx(ctx context.Context, operation func(pgx.Tx) error) error {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin custom data transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := operation(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit custom data transaction: %w", err)
	}
	return nil
}

func databaseUUID(value Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: !value.IsZero()}
}

func identifierFromUUID(value pgtype.UUID) (Identifier, error) {
	if !value.Valid {
		return Identifier{}, ErrInvalidIdentifier
	}
	return Identifier(value.Bytes), nil
}

func optionalUUID(value *Identifier) pgtype.UUID {
	if value == nil || value.IsZero() {
		return pgtype.UUID{}
	}
	return databaseUUID(*value)
}

func optionalIdentifier(value pgtype.UUID) (*Identifier, error) {
	if !value.Valid {
		return nil, nil
	}
	mapped, err := identifierFromUUID(value)
	if err != nil {
		return nil, err
	}
	return &mapped, nil
}

func boolFilter(value *bool) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func targetColumns(values FieldDefinitionValues) (documentTypeID, billTypeID, entityTypeID any) {
	switch values.TargetKind {
	case TargetDocumentType:
		documentTypeID = databaseUUID(values.TargetID)
	case TargetBillType:
		billTypeID = databaseUUID(values.TargetID)
	case TargetCustomEntityType:
		entityTypeID = databaseUUID(values.TargetID)
	}
	return
}

func targetIDFromColumns(kind TargetKind, documentTypeID, billTypeID, entityTypeID pgtype.UUID) (Identifier, error) {
	switch kind {
	case TargetProfile:
		return Identifier{}, nil
	case TargetDocumentType:
		return identifierFromUUID(documentTypeID)
	case TargetBillType:
		return identifierFromUUID(billTypeID)
	case TargetCustomEntityType:
		return identifierFromUUID(entityTypeID)
	default:
		return Identifier{}, ErrInvalidTarget
	}
}

func mapPersistenceError(operation string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505":
			if strings.Contains(postgresError.ConstraintName, "custom_entities_one_per_profile") {
				return ErrCardinalityConflict
			}
			return ErrTechnicalKeyConflict
		case "23503":
			return ErrReferenceNotFound
		case "23514":
			if strings.Contains(strings.ToLower(postgresError.Message), "inactive") {
				return ErrEntityTypeInactive
			}
			return ErrInvalidTarget
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

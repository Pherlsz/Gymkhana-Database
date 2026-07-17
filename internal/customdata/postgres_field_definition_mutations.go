package customdata

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) CreateFieldDefinition(ctx context.Context, id Identifier, values FieldDefinitionValues) (FieldDefinition, error) {
	normalized, err := NormalizeFieldDefinition(values)
	if err != nil {
		return FieldDefinition{}, err
	}
	documentTypeID, billTypeID, entityTypeID := targetColumns(normalized)
	value, err := scanFieldDefinition(store.db.QueryRow(ctx, `INSERT INTO custom_field_definitions
(id, target_kind, document_type_id, bill_type_id, custom_entity_type_id, technical_key, label, field_kind,
 required, active, minimum_length, maximum_length, validation_regex, minimum_decimal, maximum_decimal)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
RETURNING `+fieldDefinitionColumns,
		databaseUUID(id), string(normalized.TargetKind), documentTypeID, billTypeID, entityTypeID,
		normalized.TechnicalKey, normalized.Label, string(normalized.Kind), normalized.Required, normalized.Active,
		nullableInt(normalized.MinimumLength), nullableInt(normalized.MaximumLength), nullableString(normalized.ValidationRegex),
		nullableString(normalized.MinimumDecimal), nullableString(normalized.MaximumDecimal)))
	if err != nil {
		return FieldDefinition{}, mapPersistenceError("create custom field definition", err)
	}
	return value, nil
}

func (store *PostgresStore) UpdateFieldDefinition(ctx context.Context, id Identifier, version int64, values FieldDefinitionValues) (FieldDefinition, error) {
	if version <= 0 {
		return FieldDefinition{}, ErrConflict
	}
	normalized, err := NormalizeFieldDefinition(values)
	if err != nil {
		return FieldDefinition{}, err
	}
	var updated FieldDefinition
	err = store.withTx(ctx, func(tx pgx.Tx) error {
		existing, getErr := scanFieldDefinition(tx.QueryRow(ctx, `SELECT `+fieldDefinitionColumns+` FROM custom_field_definitions WHERE id = $1 FOR UPDATE`, databaseUUID(id)))
		if getErr != nil {
			return getErr
		}
		if existing.Version != version {
			return ErrConflict
		}
		if existing.Values.TechnicalKey != normalized.TechnicalKey || existing.Values.TargetKind != normalized.TargetKind || existing.Values.TargetID != normalized.TargetID {
			return ErrTechnicalKeyImmutable
		}
		if !sameFieldRules(existing.Values, normalized) {
			var hasValues bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM custom_field_values WHERE field_definition_id = $1)`, databaseUUID(id)).Scan(&hasValues); err != nil {
				return fmt.Errorf("check custom field values: %w", err)
			}
			if hasValues {
				return ErrDefinitionChangeUnsafe
			}
		}
		updated, getErr = scanFieldDefinition(tx.QueryRow(ctx, `UPDATE custom_field_definitions SET
label=$2, field_kind=$3, required=$4, active=$5, minimum_length=$6, maximum_length=$7,
validation_regex=$8, minimum_decimal=$9, maximum_decimal=$10, version=version+1, updated_at=now()
WHERE id=$1 AND version=$11 RETURNING `+fieldDefinitionColumns,
			databaseUUID(id), normalized.Label, string(normalized.Kind), normalized.Required, normalized.Active,
			nullableInt(normalized.MinimumLength), nullableInt(normalized.MaximumLength), nullableString(normalized.ValidationRegex),
			nullableString(normalized.MinimumDecimal), nullableString(normalized.MaximumDecimal), version))
		return getErr
	})
	if err != nil {
		return FieldDefinition{}, mapKnownOrPersistence("update custom field definition", err)
	}
	return updated, nil
}

func (store *PostgresStore) DeleteFieldDefinition(ctx context.Context, id Identifier, version int64) error {
	if version <= 0 {
		return ErrConflict
	}
	return store.withTx(ctx, func(tx pgx.Tx) error {
		existing, err := scanFieldDefinition(tx.QueryRow(ctx, `SELECT `+fieldDefinitionColumns+` FROM custom_field_definitions WHERE id=$1 FOR UPDATE`, databaseUUID(id)))
		if err != nil {
			return err
		}
		if existing.Version != version {
			return ErrConflict
		}
		var used bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM custom_field_values WHERE field_definition_id=$1)`, databaseUUID(id)).Scan(&used); err != nil {
			return fmt.Errorf("check custom field definition usage: %w", err)
		}
		if used {
			return ErrDefinitionInUse
		}
		command, err := tx.Exec(ctx, `DELETE FROM custom_field_definitions WHERE id=$1 AND version=$2`, databaseUUID(id), version)
		if err != nil {
			return mapPersistenceError("delete custom field definition", err)
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		return nil
	})
}

func sameFieldRules(left, right FieldDefinitionValues) bool {
	return left.Kind == right.Kind && left.Required == right.Required &&
		left.MinimumLength == right.MinimumLength && left.MaximumLength == right.MaximumLength &&
		left.ValidationRegex == right.ValidationRegex && left.MinimumDecimal == right.MinimumDecimal &&
		left.MaximumDecimal == right.MaximumDecimal
}

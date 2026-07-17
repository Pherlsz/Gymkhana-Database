package customdata

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) ListOptions(ctx context.Context, fieldID Identifier) ([]Option, error) {
	rows, err := store.db.Query(ctx, `SELECT id, field_definition_id, technical_key, label, active, sort_order, version, created_at, updated_at
FROM custom_field_options WHERE field_definition_id=$1 ORDER BY active DESC, sort_order, lower(label), id`, databaseUUID(fieldID))
	if err != nil {
		return nil, fmt.Errorf("list custom field options: %w", err)
	}
	defer rows.Close()
	result := make([]Option, 0)
	for rows.Next() {
		value, scanErr := scanOption(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (store *PostgresStore) CreateOption(ctx context.Context, id, fieldID Identifier, values OptionValues) (Option, error) {
	normalized, err := NormalizeOption(values)
	if err != nil {
		return Option{}, err
	}
	definition, err := store.GetFieldDefinition(ctx, fieldID)
	if err != nil {
		return Option{}, err
	}
	if definition.Values.Kind != FieldSingleSelect && definition.Values.Kind != FieldMultiSelect {
		return Option{}, ErrInvalidTarget
	}
	value, err := scanOption(store.db.QueryRow(ctx, `INSERT INTO custom_field_options
(id, field_definition_id, technical_key, label, active, sort_order)
VALUES($1,$2,$3,$4,$5,$6)
RETURNING id, field_definition_id, technical_key, label, active, sort_order, version, created_at, updated_at`,
		databaseUUID(id), databaseUUID(fieldID), normalized.TechnicalKey, normalized.Label, normalized.Active, normalized.SortOrder))
	if err != nil {
		return Option{}, mapPersistenceError("create custom field option", err)
	}
	return value, nil
}

func (store *PostgresStore) UpdateOption(ctx context.Context, optionID, fieldID Identifier, version int64, values OptionValues) (Option, error) {
	if version <= 0 {
		return Option{}, ErrConflict
	}
	normalized, err := NormalizeOption(values)
	if err != nil {
		return Option{}, err
	}
	var updated Option
	err = store.withTx(ctx, func(tx pgx.Tx) error {
		existing, getErr := scanOption(tx.QueryRow(ctx, `SELECT id, field_definition_id, technical_key, label, active, sort_order, version, created_at, updated_at
FROM custom_field_options WHERE id=$1 AND field_definition_id=$2 FOR UPDATE`, databaseUUID(optionID), databaseUUID(fieldID)))
		if getErr != nil {
			return getErr
		}
		if existing.Version != version {
			return ErrConflict
		}
		if existing.Values.TechnicalKey != normalized.TechnicalKey {
			return ErrTechnicalKeyImmutable
		}
		updated, getErr = scanOption(tx.QueryRow(ctx, `UPDATE custom_field_options SET label=$3, active=$4, sort_order=$5,
version=version+1, updated_at=now() WHERE id=$1 AND field_definition_id=$2 AND version=$6
RETURNING id, field_definition_id, technical_key, label, active, sort_order, version, created_at, updated_at`,
			databaseUUID(optionID), databaseUUID(fieldID), normalized.Label, normalized.Active, normalized.SortOrder, version))
		return getErr
	})
	if err != nil {
		return Option{}, mapKnownOrPersistence("update custom field option", err)
	}
	return updated, nil
}

func (store *PostgresStore) DeleteOption(ctx context.Context, optionID, fieldID Identifier, version int64) error {
	if version <= 0 {
		return ErrConflict
	}
	return store.withTx(ctx, func(tx pgx.Tx) error {
		existing, err := scanOption(tx.QueryRow(ctx, `SELECT id, field_definition_id, technical_key, label, active, sort_order, version, created_at, updated_at
FROM custom_field_options WHERE id=$1 AND field_definition_id=$2 FOR UPDATE`, databaseUUID(optionID), databaseUUID(fieldID)))
		if err != nil {
			return err
		}
		if existing.Version != version {
			return ErrConflict
		}
		var used bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM custom_field_value_options WHERE option_id=$1)`, databaseUUID(optionID)).Scan(&used); err != nil {
			return fmt.Errorf("check custom field option usage: %w", err)
		}
		if used {
			return ErrOptionInUse
		}
		command, err := tx.Exec(ctx, `DELETE FROM custom_field_options WHERE id=$1 AND field_definition_id=$2 AND version=$3`, databaseUUID(optionID), databaseUUID(fieldID), version)
		if err != nil {
			return mapPersistenceError("delete custom field option", err)
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		return nil
	})
}

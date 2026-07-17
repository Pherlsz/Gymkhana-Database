package customdata

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) CountEntityTypes(ctx context.Context, filters EntityTypeFilters) (int64, error) {
	var count int64
	err := store.db.QueryRow(ctx, `SELECT count(*) FROM custom_entity_types
WHERE ($1::text = '' OR lower(label) LIKE '%' || lower($1) || '%')
  AND ($2::boolean IS NULL OR active = $2)`, filters.Label, boolFilter(filters.Active)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count custom entity types: %w", err)
	}
	return count, nil
}

func (store *PostgresStore) ListEntityTypes(ctx context.Context, options EntityTypeListOptions) ([]EntityType, error) {
	sortColumn := map[EntityTypeSortField]string{
		EntityTypeSortLabel: "lower(label)", EntityTypeSortCreatedAt: "created_at", EntityTypeSortUpdatedAt: "updated_at",
	}[options.SortField]
	order := strings.ToUpper(string(options.SortOrder))
	query := fmt.Sprintf(`SELECT id, technical_key, label, active, profile_cardinality, version, created_at, updated_at
FROM custom_entity_types
WHERE ($1::text = '' OR lower(label) LIKE '%%' || lower($1) || '%%')
  AND ($2::boolean IS NULL OR active = $2)
ORDER BY %s %s, id %s OFFSET $3 LIMIT $4`, sortColumn, order, order)
	rows, err := store.db.Query(ctx, query, options.Filters.Label, boolFilter(options.Filters.Active), options.Offset, options.Limit)
	if err != nil {
		return nil, fmt.Errorf("list custom entity types: %w", err)
	}
	defer rows.Close()
	result := make([]EntityType, 0)
	for rows.Next() {
		value, scanErr := scanEntityType(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan custom entity type: %w", scanErr)
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (store *PostgresStore) GetEntityType(ctx context.Context, id Identifier) (EntityType, error) {
	value, err := scanEntityType(store.db.QueryRow(ctx, `SELECT id, technical_key, label, active, profile_cardinality, version, created_at, updated_at
FROM custom_entity_types WHERE id = $1`, databaseUUID(id)))
	if err != nil {
		return EntityType{}, fmt.Errorf("get custom entity type: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) CreateEntityType(ctx context.Context, id Identifier, values EntityTypeValues) (EntityType, error) {
	normalized, err := NormalizeEntityType(values)
	if err != nil {
		return EntityType{}, err
	}
	var cardinality any
	if normalized.ProfileCardinality != "" {
		cardinality = string(normalized.ProfileCardinality)
	}
	value, err := scanEntityType(store.db.QueryRow(ctx, `INSERT INTO custom_entity_types
(id, technical_key, label, active, profile_cardinality)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, technical_key, label, active, profile_cardinality, version, created_at, updated_at`,
		databaseUUID(id), normalized.TechnicalKey, normalized.Label, normalized.Active, cardinality))
	if err != nil {
		return EntityType{}, mapPersistenceError("create custom entity type", err)
	}
	return value, nil
}

func (store *PostgresStore) UpdateEntityType(ctx context.Context, id Identifier, version int64, values EntityTypeValues) (EntityType, error) {
	if version <= 0 {
		return EntityType{}, ErrConflict
	}
	normalized, err := NormalizeEntityType(values)
	if err != nil {
		return EntityType{}, err
	}
	var updated EntityType
	err = store.withTx(ctx, func(tx pgx.Tx) error {
		existing, getErr := scanEntityType(tx.QueryRow(ctx, `SELECT id, technical_key, label, active, profile_cardinality, version, created_at, updated_at
FROM custom_entity_types WHERE id = $1 FOR UPDATE`, databaseUUID(id)))
		if getErr != nil {
			return getErr
		}
		if existing.Version != version {
			return ErrConflict
		}
		if existing.Values.TechnicalKey != normalized.TechnicalKey {
			return ErrTechnicalKeyImmutable
		}
		if existing.Values.ProfileCardinality != normalized.ProfileCardinality {
			var used bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM custom_entities WHERE custom_entity_type_id = $1)`, databaseUUID(id)).Scan(&used); err != nil {
				return fmt.Errorf("check custom entity type usage: %w", err)
			}
			if used {
				return ErrEntityTypeInUse
			}
		}
		var cardinality any
		if normalized.ProfileCardinality != "" {
			cardinality = string(normalized.ProfileCardinality)
		}
		updated, getErr = scanEntityType(tx.QueryRow(ctx, `UPDATE custom_entity_types
SET label = $2, active = $3, profile_cardinality = $4, version = version + 1, updated_at = now()
WHERE id = $1 AND version = $5
RETURNING id, technical_key, label, active, profile_cardinality, version, created_at, updated_at`,
			databaseUUID(id), normalized.Label, normalized.Active, cardinality, version))
		return getErr
	})
	if err != nil {
		return EntityType{}, mapKnownOrPersistence("update custom entity type", err)
	}
	return updated, nil
}

func (store *PostgresStore) DeleteEntityType(ctx context.Context, id Identifier, version int64) error {
	if version <= 0 {
		return ErrConflict
	}
	return store.withTx(ctx, func(tx pgx.Tx) error {
		existing, err := scanEntityType(tx.QueryRow(ctx, `SELECT id, technical_key, label, active, profile_cardinality, version, created_at, updated_at
FROM custom_entity_types WHERE id = $1 FOR UPDATE`, databaseUUID(id)))
		if err != nil {
			return err
		}
		if existing.Version != version {
			return ErrConflict
		}
		var used bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM custom_entities WHERE custom_entity_type_id = $1
UNION ALL SELECT 1 FROM custom_field_definitions WHERE custom_entity_type_id = $1)`, databaseUUID(id)).Scan(&used); err != nil {
			return fmt.Errorf("check custom entity type references: %w", err)
		}
		if used {
			return ErrEntityTypeInUse
		}
		command, err := tx.Exec(ctx, `DELETE FROM custom_entity_types WHERE id = $1 AND version = $2`, databaseUUID(id), version)
		if err != nil {
			return mapPersistenceError("delete custom entity type", err)
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		return nil
	})
}

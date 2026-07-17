package customdata

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) CountEntities(ctx context.Context, options EntityListOptions) (int64, error) {
	var count int64
	if err := store.db.QueryRow(ctx, `SELECT count(*) FROM custom_entities
WHERE custom_entity_type_id=$1 AND ($2::uuid IS NULL OR owner_profile_id=$2)`, databaseUUID(options.TypeID), optionalUUID(options.OwnerProfileID)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count custom entities: %w", err)
	}
	return count, nil
}

func (store *PostgresStore) ListEntities(ctx context.Context, options EntityListOptions) ([]Entity, error) {
	rows, err := store.db.Query(ctx, `SELECT id, custom_entity_type_id, owner_profile_id, profile_cardinality, version, created_at, updated_at
FROM custom_entities WHERE custom_entity_type_id=$1 AND ($2::uuid IS NULL OR owner_profile_id=$2)
ORDER BY updated_at DESC, id OFFSET $3 LIMIT $4`, databaseUUID(options.TypeID), optionalUUID(options.OwnerProfileID), options.Offset, options.Limit)
	if err != nil {
		return nil, fmt.Errorf("list custom entities: %w", err)
	}
	defer rows.Close()
	result := make([]Entity, 0)
	for rows.Next() {
		entity, scanErr := scanEntity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values, readErr := store.readValues(ctx, store.db, TargetReference{Kind: ValueTargetCustomEntity, ID: entity.ID})
		if readErr != nil {
			return nil, readErr
		}
		entity.Values = values
		result = append(result, entity)
	}
	return result, rows.Err()
}

func (store *PostgresStore) GetEntity(ctx context.Context, id Identifier) (Entity, error) {
	entity, err := scanEntity(store.db.QueryRow(ctx, `SELECT id, custom_entity_type_id, owner_profile_id, profile_cardinality, version, created_at, updated_at
FROM custom_entities WHERE id=$1`, databaseUUID(id)))
	if err != nil {
		return Entity{}, fmt.Errorf("get custom entity: %w", err)
	}
	entity.Values, err = store.readValues(ctx, store.db, TargetReference{Kind: ValueTargetCustomEntity, ID: entity.ID})
	if err != nil {
		return Entity{}, err
	}
	return entity, nil
}

func scanEntity(row pgx.Row) (Entity, error) {
	var id, typeID, ownerID pgtype.UUID
	var cardinality *string
	var version int64
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &typeID, &ownerID, &cardinality, &version, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entity{}, ErrNotFound
		}
		return Entity{}, err
	}
	mappedID, err := identifierFromUUID(id)
	if err != nil {
		return Entity{}, err
	}
	mappedTypeID, err := identifierFromUUID(typeID)
	if err != nil {
		return Entity{}, err
	}
	owner, err := optionalIdentifier(ownerID)
	if err != nil {
		return Entity{}, err
	}
	entity := Entity{ID: mappedID, TypeID: mappedTypeID, OwnerProfileID: owner, Version: version, CreatedAt: createdAt, UpdatedAt: updatedAt}
	if cardinality != nil {
		entity.ProfileCardinality = ProfileCardinality(*cardinality)
	}
	return entity, nil
}

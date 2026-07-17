package customdata

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) CreateEntity(ctx context.Context, id, typeID Identifier, ownerProfileID *Identifier, inputs []ValueInput) (Entity, error) {
	var created Entity
	err := store.withTx(ctx, func(tx pgx.Tx) error {
		entityType, err := scanEntityType(tx.QueryRow(ctx, `SELECT id, technical_key, label, active, profile_cardinality, version, created_at, updated_at
FROM custom_entity_types WHERE id=$1 FOR UPDATE`, databaseUUID(typeID)))
		if err != nil {
			return err
		}
		if !entityType.Values.Active {
			return ErrEntityTypeInactive
		}
		if entityType.Values.ProfileCardinality == "" {
			if ownerProfileID != nil {
				return ErrInvalidTarget
			}
		} else {
			if ownerProfileID == nil || ownerProfileID.IsZero() {
				return ErrInvalidTarget
			}
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM profiles WHERE id=$1)`, databaseUUID(*ownerProfileID)).Scan(&exists); err != nil {
				return fmt.Errorf("check custom entity owner: %w", err)
			}
			if !exists {
				return ErrReferenceNotFound
			}
		}
		var cardinality any
		if entityType.Values.ProfileCardinality != "" {
			cardinality = string(entityType.Values.ProfileCardinality)
		}
		created, err = scanEntity(tx.QueryRow(ctx, `INSERT INTO custom_entities
(id, custom_entity_type_id, owner_profile_id, profile_cardinality)
VALUES($1,$2,$3,$4)
RETURNING id, custom_entity_type_id, owner_profile_id, profile_cardinality, version, created_at, updated_at`,
			databaseUUID(id), databaseUUID(typeID), optionalUUID(ownerProfileID), cardinality))
		if err != nil {
			return mapPersistenceError("create custom entity", err)
		}
		metadata := targetMetadata{Version: created.Version, DefinitionKind: TargetCustomEntityType, DefinitionID: typeID}
		definitions, err := store.readApplicableDefinitions(ctx, tx, metadata, true)
		if err != nil {
			return err
		}
		target := TargetReference{Kind: ValueTargetCustomEntity, ID: id}
		normalized, err := store.normalizeReplacementValues(ctx, tx, target, definitions, inputs)
		if err != nil {
			return err
		}
		for _, value := range normalized {
			if valuePresent(value) {
				if err := insertStoredValue(ctx, tx, target, value); err != nil {
					return err
				}
			}
		}
		created.Values, err = store.readValues(ctx, tx, target)
		return err
	})
	if err != nil {
		return Entity{}, mapKnownOrPersistence("create custom entity", err)
	}
	return created, nil
}

func (store *PostgresStore) UpdateEntity(ctx context.Context, id Identifier, version int64, inputs []ValueInput) (Entity, error) {
	if version <= 0 {
		return Entity{}, ErrConflict
	}
	var updated Entity
	err := store.withTx(ctx, func(tx pgx.Tx) error {
		existing, err := scanEntity(tx.QueryRow(ctx, `SELECT id, custom_entity_type_id, owner_profile_id, profile_cardinality, version, created_at, updated_at
FROM custom_entities WHERE id=$1 FOR UPDATE`, databaseUUID(id)))
		if err != nil {
			return err
		}
		if existing.Version != version {
			return ErrConflict
		}
		metadata := targetMetadata{Version: existing.Version, DefinitionKind: TargetCustomEntityType, DefinitionID: existing.TypeID}
		definitions, err := store.readApplicableDefinitions(ctx, tx, metadata, true)
		if err != nil {
			return err
		}
		target := TargetReference{Kind: ValueTargetCustomEntity, ID: id}
		normalized, err := store.normalizeReplacementValues(ctx, tx, target, definitions, inputs)
		if err != nil {
			return err
		}
		activeIDs := make([]Identifier, 0, len(definitions))
		for _, definition := range definitions {
			if definition.Values.Active {
				activeIDs = append(activeIDs, definition.ID)
			}
		}
		if err := deleteTargetValues(ctx, tx, target, activeIDs); err != nil {
			return err
		}
		for _, value := range normalized {
			if valuePresent(value) {
				if err := insertStoredValue(ctx, tx, target, value); err != nil {
					return err
				}
			}
		}
		newVersion, err := incrementTargetVersion(ctx, tx, target, version)
		if err != nil {
			return err
		}
		existing.Version = newVersion
		existing.UpdatedAt = time.Now().UTC()
		existing.Values, err = store.readValues(ctx, tx, target)
		updated = existing
		return err
	})
	if err != nil {
		return Entity{}, mapKnownOrPersistence("update custom entity", err)
	}
	return updated, nil
}

func (store *PostgresStore) DeleteEntity(ctx context.Context, id Identifier, version int64) error {
	if version <= 0 {
		return ErrConflict
	}
	return store.withTx(ctx, func(tx pgx.Tx) error {
		existing, err := scanEntity(tx.QueryRow(ctx, `SELECT id, custom_entity_type_id, owner_profile_id, profile_cardinality, version, created_at, updated_at
FROM custom_entities WHERE id=$1 FOR UPDATE`, databaseUUID(id)))
		if err != nil {
			return err
		}
		if existing.Version != version {
			return ErrConflict
		}
		command, err := tx.Exec(ctx, `DELETE FROM custom_entities WHERE id=$1 AND version=$2`, databaseUUID(id), version)
		if err != nil {
			return mapPersistenceError("delete custom entity", err)
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		return nil
	})
}

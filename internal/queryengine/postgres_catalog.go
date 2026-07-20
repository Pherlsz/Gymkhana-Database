package queryengine

import (
	"context"
	"errors"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (store *PostgresStore) CurrentUser(ctx context.Context, id auth.Identifier) (auth.User, error) {
	if store == nil || store.pool == nil || id == (auth.Identifier{}) {
		return auth.User{}, ErrForbidden
	}
	var value auth.User
	var databaseID pgtype.UUID
	var subject, avatar pgtype.Text
	err := store.pool.QueryRow(ctx, `SELECT id, google_subject, email, display_name, avatar_url, role, active
FROM app_users WHERE id=$1`, queryAuthUUID(id)).Scan(
		&databaseID, &subject, &value.Email, &value.DisplayName, &avatar, &value.Role, &value.Active,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, ErrForbidden
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("load current query user: %w", err)
	}
	value.ID = auth.Identifier(databaseID.Bytes)
	if subject.Valid {
		value.GoogleSubject = subject.String
	}
	if avatar.Valid {
		value.AvatarURL = avatar.String
	}
	return value, nil
}

func (store *PostgresStore) CatalogDefinitions(ctx context.Context) (CatalogDefinitions, error) {
	if store == nil || store.pool == nil {
		return CatalogDefinitions{}, ErrInvalidSetup
	}
	rows, err := store.pool.Query(ctx, `SELECT id::text, technical_key, label, COALESCE(profile_cardinality, '')
FROM custom_entity_types
WHERE active=true
ORDER BY technical_key, id`)
	if err != nil {
		return CatalogDefinitions{}, fmt.Errorf("list query entity definitions: %w", err)
	}
	definitions := CatalogDefinitions{Entities: make([]DynamicEntityDefinition, 0), Fields: make([]DynamicFieldDefinition, 0)}
	for rows.Next() {
		var value DynamicEntityDefinition
		if err := rows.Scan(&value.ID, &value.TechnicalKey, &value.Label, &value.ProfileCardinality); err != nil {
			rows.Close()
			return CatalogDefinitions{}, fmt.Errorf("scan query entity definition: %w", err)
		}
		definitions.Entities = append(definitions.Entities, value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return CatalogDefinitions{}, fmt.Errorf("iterate query entity definitions: %w", err)
	}
	rows.Close()

	rows, err = store.pool.Query(ctx, `SELECT definition.id::text,
       definition.target_kind,
       COALESCE(definition.document_type_id::text, ''),
       COALESCE(definition.bill_type_id::text, ''),
       COALESCE(definition.custom_entity_type_id::text, ''),
       definition.technical_key,
       definition.label,
       definition.field_kind
FROM custom_field_definitions AS definition
LEFT JOIN document_types AS document_type ON document_type.id=definition.document_type_id
LEFT JOIN bill_types AS bill_type ON bill_type.id=definition.bill_type_id
LEFT JOIN custom_entity_types AS entity_type ON entity_type.id=definition.custom_entity_type_id
WHERE definition.active=true
  AND definition.field_kind <> 'ATTACHMENT'
  AND (
    definition.target_kind='PROFILE' OR
    (definition.target_kind='DOCUMENT_TYPE' AND document_type.active=true) OR
    (definition.target_kind='BILL_TYPE' AND bill_type.active=true) OR
    (definition.target_kind='CUSTOM_ENTITY_TYPE' AND entity_type.active=true)
  )
ORDER BY definition.target_kind, definition.technical_key, definition.id`)
	if err != nil {
		return CatalogDefinitions{}, fmt.Errorf("list query field definitions: %w", err)
	}
	fieldIndex := make(map[string]int)
	for rows.Next() {
		var value DynamicFieldDefinition
		if err := rows.Scan(&value.ID, &value.TargetKind, &value.DocumentTypeID, &value.BillTypeID,
			&value.CustomEntityTypeID, &value.TechnicalKey, &value.Label, &value.FieldKind); err != nil {
			rows.Close()
			return CatalogDefinitions{}, fmt.Errorf("scan query field definition: %w", err)
		}
		fieldIndex[value.ID] = len(definitions.Fields)
		definitions.Fields = append(definitions.Fields, value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return CatalogDefinitions{}, fmt.Errorf("iterate query field definitions: %w", err)
	}
	rows.Close()

	rows, err = store.pool.Query(ctx, `SELECT option_value.field_definition_id::text, option_value.technical_key, option_value.label
FROM custom_field_options AS option_value
JOIN custom_field_definitions AS definition ON definition.id=option_value.field_definition_id
WHERE option_value.active=true AND definition.active=true
ORDER BY option_value.field_definition_id, option_value.sort_order, lower(option_value.label), option_value.id`)
	if err != nil {
		return CatalogDefinitions{}, fmt.Errorf("list query field options: %w", err)
	}
	for rows.Next() {
		var fieldID string
		var option OptionDefinition
		if err := rows.Scan(&fieldID, &option.Key, &option.Label); err != nil {
			rows.Close()
			return CatalogDefinitions{}, fmt.Errorf("scan query field option: %w", err)
		}
		if index, ok := fieldIndex[fieldID]; ok {
			definitions.Fields[index].Options = append(definitions.Fields[index].Options, option)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return CatalogDefinitions{}, fmt.Errorf("iterate query field options: %w", err)
	}
	rows.Close()
	return definitions, nil
}

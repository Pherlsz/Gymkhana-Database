package customdata

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type targetMetadata struct {
	Version        int64
	DefinitionKind TargetKind
	DefinitionID   Identifier
}

func (store *PostgresStore) GetValues(ctx context.Context, target TargetReference) (ValueSet, error) {
	if !target.Valid() {
		return ValueSet{}, ErrInvalidTarget
	}
	metadata, err := store.readTargetMetadata(ctx, store.db, target, false)
	if err != nil {
		return ValueSet{}, err
	}
	values, err := store.readValues(ctx, store.db, target)
	if err != nil {
		return ValueSet{}, err
	}
	return ValueSet{Target: target, Values: values, Version: metadata.Version}, nil
}

func (store *PostgresStore) readTargetMetadata(ctx context.Context, database interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, target TargetReference, lock bool) (targetMetadata, error) {
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE"
	}
	var metadata targetMetadata
	var typeUUID pgtype.UUID
	var query string
	switch target.Kind {
	case ValueTargetProfile:
		query = `SELECT version, NULL::uuid FROM profiles WHERE id=$1` + lockClause
		metadata.DefinitionKind = TargetProfile
	case ValueTargetDocument:
		query = `SELECT version, document_type_id FROM documents WHERE id=$1` + lockClause
		metadata.DefinitionKind = TargetDocumentType
	case ValueTargetBill:
		query = `SELECT version, bill_type_id FROM bills WHERE id=$1` + lockClause
		metadata.DefinitionKind = TargetBillType
	case ValueTargetCustomEntity:
		query = `SELECT version, custom_entity_type_id FROM custom_entities WHERE id=$1` + lockClause
		metadata.DefinitionKind = TargetCustomEntityType
	default:
		return targetMetadata{}, ErrInvalidTarget
	}
	if err := database.QueryRow(ctx, query, databaseUUID(target.ID)).Scan(&metadata.Version, &typeUUID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return targetMetadata{}, ErrNotFound
		}
		return targetMetadata{}, fmt.Errorf("read custom data target: %w", err)
	}
	if typeUUID.Valid {
		mapped, err := identifierFromUUID(typeUUID)
		if err != nil {
			return targetMetadata{}, err
		}
		metadata.DefinitionID = mapped
	}
	return metadata, nil
}

func (store *PostgresStore) readApplicableDefinitions(ctx context.Context, database interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, metadata targetMetadata, includeInactive bool) ([]FieldDefinition, error) {
	condition, args := definitionTargetCondition(metadata.DefinitionKind, metadata.DefinitionID, 1)
	if !includeInactive {
		condition += " AND active"
	}
	rows, err := database.Query(ctx, `SELECT `+fieldDefinitionColumns+` FROM custom_field_definitions WHERE `+condition+` ORDER BY technical_key, id`, args...)
	if err != nil {
		return nil, fmt.Errorf("read applicable custom field definitions: %w", err)
	}
	defer rows.Close()
	result := make([]FieldDefinition, 0)
	for rows.Next() {
		definition, scanErr := scanFieldDefinition(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, definition)
	}
	return result, rows.Err()
}

func targetValueCondition(target TargetReference, firstPosition int) (string, []any) {
	column := map[ValueTargetKind]string{
		ValueTargetProfile: "profile_id", ValueTargetDocument: "document_id",
		ValueTargetBill: "bill_id", ValueTargetCustomEntity: "custom_entity_id",
	}[target.Kind]
	if column == "" {
		return "false", nil
	}
	return fmt.Sprintf(`%s = $%d`, column, firstPosition), []any{databaseUUID(target.ID)}
}

func (store *PostgresStore) readValues(ctx context.Context, database interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, target TargetReference) ([]StoredValue, error) {
	condition, args := targetValueCondition(target, 1)
	rows, err := database.Query(ctx, `SELECT id, field_definition_id, field_kind, text_value, integer_value,
 decimal_value::text, boolean_value, civil_date_value, civil_month_value, version, created_at, updated_at
FROM custom_field_values WHERE `+condition+` ORDER BY field_definition_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("read custom field values: %w", err)
	}
	defer rows.Close()
	result := make([]StoredValue, 0)
	for rows.Next() {
		var id, fieldID pgtype.UUID
		var kind string
		var textValue, decimalValue, civilMonth *string
		var integerValue *int64
		var booleanValue *bool
		var civilDate *time.Time
		var version int64
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &fieldID, &kind, &textValue, &integerValue, &decimalValue, &booleanValue, &civilDate, &civilMonth, &version, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan custom field value: %w", err)
		}
		mappedID, err := identifierFromUUID(id)
		if err != nil {
			return nil, err
		}
		mappedFieldID, err := identifierFromUUID(fieldID)
		if err != nil {
			return nil, err
		}
		input := ValueInput{FieldDefinitionID: mappedFieldID, Kind: FieldKind(kind), Integer: integerValue, Boolean: booleanValue}
		if textValue != nil {
			input.Text = *textValue
		}
		if decimalValue != nil {
			input.Decimal = *decimalValue
		}
		if civilDate != nil {
			input.CivilDate = civilDate.Format("2006-01-02")
		}
		if civilMonth != nil {
			input.CivilMonth = *civilMonth
		}
		if input.Kind == FieldSingleSelect || input.Kind == FieldMultiSelect {
			options, err := readValueOptions(ctx, database, mappedID)
			if err != nil {
				return nil, err
			}
			input.OptionIDs = options
		}
		result = append(result, StoredValue{ID: mappedID, Target: target, Input: input, Version: version, CreatedAt: createdAt, UpdatedAt: updatedAt})
	}
	return result, rows.Err()
}

func readValueOptions(ctx context.Context, database interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, valueID Identifier) ([]Identifier, error) {
	rows, err := database.Query(ctx, `SELECT option_id FROM custom_field_value_options WHERE custom_field_value_id=$1 ORDER BY option_id`, databaseUUID(valueID))
	if err != nil {
		return nil, fmt.Errorf("read selected custom field options: %w", err)
	}
	defer rows.Close()
	result := make([]Identifier, 0)
	for rows.Next() {
		var id pgtype.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		mapped, err := identifierFromUUID(id)
		if err != nil {
			return nil, err
		}
		result = append(result, mapped)
	}
	return result, rows.Err()
}

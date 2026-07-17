package customdata

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanEntityType(row pgx.Row) (EntityType, error) {
	var id pgtype.UUID
	var values EntityTypeValues
	var cardinality *string
	var version int64
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &values.TechnicalKey, &values.Label, &values.Active, &cardinality, &version, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EntityType{}, ErrNotFound
		}
		return EntityType{}, err
	}
	mappedID, err := identifierFromUUID(id)
	if err != nil {
		return EntityType{}, err
	}
	if cardinality != nil {
		values.ProfileCardinality = ProfileCardinality(*cardinality)
	}
	return EntityType{ID: mappedID, Values: values, Version: version, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

func scanFieldDefinition(row pgx.Row) (FieldDefinition, error) {
	var id, documentTypeID, billTypeID, entityTypeID pgtype.UUID
	var targetKind string
	var values FieldDefinitionValues
	var minimumLength, maximumLength *int
	var regex, minimumDecimal, maximumDecimal *string
	var version int64
	var createdAt, updatedAt time.Time
	if err := row.Scan(
		&id, &targetKind, &documentTypeID, &billTypeID, &entityTypeID,
		&values.TechnicalKey, &values.Label, &values.Kind, &values.Required, &values.Active,
		&minimumLength, &maximumLength, &regex, &minimumDecimal, &maximumDecimal,
		&version, &createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FieldDefinition{}, ErrNotFound
		}
		return FieldDefinition{}, err
	}
	mappedID, err := identifierFromUUID(id)
	if err != nil {
		return FieldDefinition{}, err
	}
	values.TargetKind = TargetKind(targetKind)
	values.TargetID, err = targetIDFromColumns(values.TargetKind, documentTypeID, billTypeID, entityTypeID)
	if err != nil {
		return FieldDefinition{}, err
	}
	if minimumLength != nil {
		values.MinimumLength = *minimumLength
	}
	if maximumLength != nil {
		values.MaximumLength = *maximumLength
	}
	if regex != nil {
		values.ValidationRegex = *regex
	}
	if minimumDecimal != nil {
		values.MinimumDecimal = *minimumDecimal
	}
	if maximumDecimal != nil {
		values.MaximumDecimal = *maximumDecimal
	}
	return FieldDefinition{ID: mappedID, Values: values, Version: version, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

func scanOption(row pgx.Row) (Option, error) {
	var id, fieldID pgtype.UUID
	var values OptionValues
	var version int64
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &fieldID, &values.TechnicalKey, &values.Label, &values.Active, &values.SortOrder, &version, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Option{}, ErrNotFound
		}
		return Option{}, err
	}
	mappedID, err := identifierFromUUID(id)
	if err != nil {
		return Option{}, err
	}
	mappedFieldID, err := identifierFromUUID(fieldID)
	if err != nil {
		return Option{}, err
	}
	return Option{ID: mappedID, FieldDefinitionID: mappedFieldID, Values: values, Version: version, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

const fieldDefinitionColumns = `id, target_kind, document_type_id, bill_type_id, custom_entity_type_id,
 technical_key, label, field_kind, required, active, minimum_length, maximum_length,
 validation_regex, minimum_decimal::text, maximum_decimal::text, version, created_at, updated_at`

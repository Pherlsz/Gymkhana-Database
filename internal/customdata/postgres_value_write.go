package customdata

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) ReplaceValues(ctx context.Context, target TargetReference, version int64, inputs []ValueInput) (ValueSet, error) {
	if !target.Valid() {
		return ValueSet{}, ErrInvalidTarget
	}
	if version <= 0 {
		return ValueSet{}, ErrConflict
	}
	var result ValueSet
	err := store.withTx(ctx, func(tx pgx.Tx) error {
		metadata, err := store.readTargetMetadata(ctx, tx, target, true)
		if err != nil {
			return err
		}
		if metadata.Version != version {
			return ErrConflict
		}
		definitions, err := store.readApplicableDefinitions(ctx, tx, metadata, true)
		if err != nil {
			return err
		}
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
			if !valuePresent(value) {
				continue
			}
			if err := insertStoredValue(ctx, tx, target, value); err != nil {
				return err
			}
		}
		newVersion, err := incrementTargetVersion(ctx, tx, target, version)
		if err != nil {
			return err
		}
		stored, err := store.readValues(ctx, tx, target)
		if err != nil {
			return err
		}
		result = ValueSet{Target: target, Values: stored, Version: newVersion}
		return nil
	})
	if err != nil {
		return ValueSet{}, mapKnownOrPersistence("replace custom field values", err)
	}
	return result, nil
}

func (store *PostgresStore) normalizeReplacementValues(ctx context.Context, tx pgx.Tx, target TargetReference, definitions []FieldDefinition, inputs []ValueInput) ([]ValueInput, error) {
	definitionByID := make(map[Identifier]FieldDefinition, len(definitions))
	for _, definition := range definitions {
		definitionByID[definition.ID] = definition
	}
	seen := make(map[Identifier]struct{}, len(inputs))
	result := make([]ValueInput, 0, len(inputs))
	validation := &ValidationError{}
	for _, input := range inputs {
		if _, duplicate := seen[input.FieldDefinitionID]; duplicate {
			validation.add("field_definition_id", "duplicate")
			continue
		}
		seen[input.FieldDefinitionID] = struct{}{}
		definition, ok := definitionByID[input.FieldDefinitionID]
		if !ok || !definition.Values.Active {
			validation.add("field_definition_id", "invalid_value")
			continue
		}
		normalized, err := NormalizeValue(input, definition)
		if err != nil {
			var fieldValidation *ValidationError
			if errors.As(err, &fieldValidation) {
				validation.Fields = append(validation.Fields, fieldValidation.Fields...)
				continue
			}
			return nil, err
		}
		if normalized.Kind == FieldSingleSelect || normalized.Kind == FieldMultiSelect {
			if err := validateSelectedOptions(ctx, tx, definition.ID, normalized.OptionIDs); err != nil {
				if errors.Is(err, ErrReferenceNotFound) {
					validation.add("option_ids", "invalid_value")
					continue
				}
				return nil, err
			}
		}
		result = append(result, normalized)
	}
	for _, definition := range definitions {
		if definition.Values.Active && definition.Values.Required {
			if _, provided := seen[definition.ID]; !provided {
				validation.add(definition.Values.TechnicalKey, "required")
			}
		}
	}
	if len(validation.Fields) > 0 {
		return nil, validation
	}
	return result, nil
}

func validateSelectedOptions(ctx context.Context, tx pgx.Tx, fieldID Identifier, optionIDs []Identifier) error {
	for _, optionID := range optionIDs {
		var active bool
		err := tx.QueryRow(ctx, `SELECT active FROM custom_field_options WHERE id=$1 AND field_definition_id=$2`, databaseUUID(optionID), databaseUUID(fieldID)).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) || !active {
			return ErrReferenceNotFound
		}
		if err != nil {
			return fmt.Errorf("validate custom field option: %w", err)
		}
	}
	return nil
}

func valuePresent(value ValueInput) bool {
	switch value.Kind {
	case FieldText, FieldLongText, FieldEmail, FieldPhone:
		return value.Text != ""
	case FieldInteger:
		return value.Integer != nil
	case FieldDecimal:
		return value.Decimal != ""
	case FieldBoolean:
		return value.Boolean != nil
	case FieldCivilDate:
		return value.CivilDate != ""
	case FieldCivilMonth:
		return value.CivilMonth != ""
	case FieldSingleSelect, FieldMultiSelect:
		return len(value.OptionIDs) > 0
	default:
		return false
	}
}

func deleteTargetValues(ctx context.Context, tx pgx.Tx, target TargetReference, fieldIDs []Identifier) error {
	if len(fieldIDs) == 0 {
		return nil
	}
	condition, args := targetValueCondition(target, 1)
	args = append(args, identifiersToUUIDs(fieldIDs))
	if _, err := tx.Exec(ctx, `DELETE FROM custom_field_values WHERE `+condition+` AND field_definition_id = ANY($2::uuid[])`, args...); err != nil {
		return fmt.Errorf("delete replaced custom field values: %w", err)
	}
	return nil
}

func identifiersToUUIDs(values []Identifier) []pgtype.UUID {
	result := make([]pgtype.UUID, 0, len(values))
	for _, value := range values {
		result = append(result, databaseUUID(value))
	}
	return result
}

func insertStoredValue(ctx context.Context, tx pgx.Tx, target TargetReference, input ValueInput) error {
	id, err := NewIdentifier()
	if err != nil {
		return fmt.Errorf("generate custom field value identifier: %w", err)
	}
	var profileID, documentID, billID, entityID any
	switch target.Kind {
	case ValueTargetProfile:
		profileID = databaseUUID(target.ID)
	case ValueTargetDocument:
		documentID = databaseUUID(target.ID)
	case ValueTargetBill:
		billID = databaseUUID(target.ID)
	case ValueTargetCustomEntity:
		entityID = databaseUUID(target.ID)
	}
	var textValue, integerValue, decimalValue, booleanValue, civilDate, civilMonth any
	switch input.Kind {
	case FieldText, FieldLongText, FieldEmail, FieldPhone:
		textValue = input.Text
	case FieldInteger:
		integerValue = *input.Integer
	case FieldDecimal:
		decimalValue = input.Decimal
	case FieldBoolean:
		booleanValue = *input.Boolean
	case FieldCivilDate:
		parsed, _ := time.Parse("2006-01-02", input.CivilDate)
		civilDate = parsed
	case FieldCivilMonth:
		civilMonth = input.CivilMonth
	}
	_, err = tx.Exec(ctx, `INSERT INTO custom_field_values
(id, field_definition_id, profile_id, document_id, bill_id, custom_entity_id, field_kind,
 text_value, integer_value, decimal_value, boolean_value, civil_date_value, civil_month_value)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, databaseUUID(id), databaseUUID(input.FieldDefinitionID),
		profileID, documentID, billID, entityID, string(input.Kind), textValue, integerValue, decimalValue, booleanValue, civilDate, civilMonth)
	if err != nil {
		return mapPersistenceError("insert custom field value", err)
	}
	for _, optionID := range input.OptionIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO custom_field_value_options(custom_field_value_id, field_definition_id, option_id)
VALUES($1,$2,$3)`, databaseUUID(id), databaseUUID(input.FieldDefinitionID), databaseUUID(optionID)); err != nil {
			return mapPersistenceError("insert custom field selected option", err)
		}
	}
	return nil
}

func incrementTargetVersion(ctx context.Context, tx pgx.Tx, target TargetReference, version int64) (int64, error) {
	table := map[ValueTargetKind]string{
		ValueTargetProfile: "profiles", ValueTargetDocument: "documents", ValueTargetBill: "bills", ValueTargetCustomEntity: "custom_entities",
	}[target.Kind]
	if table == "" {
		return 0, ErrInvalidTarget
	}
	var newVersion int64
	query := fmt.Sprintf(`UPDATE %s SET version=version+1, updated_at=now() WHERE id=$1 AND version=$2 RETURNING version`, table)
	if err := tx.QueryRow(ctx, query, databaseUUID(target.ID), version).Scan(&newVersion); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrConflict
		}
		return 0, fmt.Errorf("increment custom data target version: %w", err)
	}
	return newVersion, nil
}

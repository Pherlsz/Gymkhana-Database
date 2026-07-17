package operations

import (
	"context"
	"errors"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) CustomFields(ctx context.Context, module Module) ([]CustomFieldDefinition, error) {
	if module != ModuleProfiles {
		if !module.Valid() {
			return nil, ErrInvalidInput
		}
		// Profile definitions are global and therefore have an unambiguous
		// logical key. Document and bill definitions are scoped to a record type;
		// exposing only `custom:<technical_key>` for them could silently select a
		// definition from the wrong type. Their canonical fields remain available.
		return nil, nil
	}
	rows, err := store.pool.Query(ctx, `SELECT id, technical_key, label, field_kind, required,
       minimum_length, maximum_length, validation_regex,
       minimum_decimal::text, maximum_decimal::text, version, created_at, updated_at
  FROM custom_field_definitions
 WHERE target_kind='PROFILE' AND active=true
   AND field_kind IN ('TEXT','LONG_TEXT','INTEGER','DECIMAL','BOOLEAN','CIVIL_DATE','CIVIL_MONTH','EMAIL','PHONE')
 ORDER BY technical_key, id`)
	if err != nil {
		return nil, fmt.Errorf("list operation custom fields: %w", err)
	}
	defer rows.Close()
	result := make([]CustomFieldDefinition, 0)
	for rows.Next() {
		var id pgtype.UUID
		var technicalKey, label string
		var kind customdata.FieldKind
		var required bool
		var minimumLength, maximumLength pgtype.Int4
		var validationRegex, minimumDecimal, maximumDecimal pgtype.Text
		var version int64
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&id, &technicalKey, &label, &kind, &required,
			&minimumLength, &maximumLength, &validationRegex, &minimumDecimal, &maximumDecimal,
			&version, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan operation custom field: %w", err)
		}
		fieldKind, ok := operationCustomFieldKind(kind)
		if !ok {
			continue
		}
		definition := customdata.FieldDefinition{
			ID: customdata.Identifier(id.Bytes),
			Values: customdata.FieldDefinitionValues{
				TargetKind: customdata.TargetProfile, TechnicalKey: technicalKey, Label: label,
				Kind: kind, Required: required, Active: true,
			},
			Version: version, CreatedAt: createdAt.Time, UpdatedAt: updatedAt.Time,
		}
		if minimumLength.Valid {
			definition.Values.MinimumLength = int(minimumLength.Int32)
		}
		if maximumLength.Valid {
			definition.Values.MaximumLength = int(maximumLength.Int32)
		}
		if validationRegex.Valid {
			definition.Values.ValidationRegex = validationRegex.String
		}
		if minimumDecimal.Valid {
			definition.Values.MinimumDecimal = minimumDecimal.String
		}
		if maximumDecimal.Valid {
			definition.Values.MaximumDecimal = maximumDecimal.String
		}
		result = append(result, CustomFieldDefinition{
			Field:      Field{ID: CustomFieldPrefix + technicalKey, Label: "Personalizado · " + label, Kind: fieldKind, Required: required, Importable: true, Exportable: true},
			Definition: definition,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operation custom fields: %w", err)
	}
	return result, nil
}

func operationCustomFieldKind(kind customdata.FieldKind) (FieldKind, bool) {
	switch kind {
	case customdata.FieldText, customdata.FieldLongText, customdata.FieldEmail, customdata.FieldPhone:
		return FieldText, true
	case customdata.FieldInteger:
		return FieldInteger, true
	case customdata.FieldDecimal:
		return FieldDecimal, true
	case customdata.FieldBoolean:
		return FieldBoolean, true
	case customdata.FieldCivilDate:
		return FieldCivilDate, true
	case customdata.FieldCivilMonth:
		return FieldCivilMonth, true
	default:
		return "", false
	}
}

func (store *PostgresStore) CanonicalValues(ctx context.Context, module Module, id Identifier) (map[string]string, error) {
	var values map[string]string
	var err error
	switch module {
	case ModuleProfiles:
		values, err = scanCanonicalValues(store.pool.QueryRow(ctx, `SELECT version::text, full_name, COALESCE(social_name,''), COALESCE(cpf,''),
       COALESCE(email,''), COALESCE(mobile_phone,''), COALESCE(landline_phone,''),
       COALESCE(address_street,''), COALESCE(address_number,''), COALESCE(address_complement,''),
       COALESCE(address_neighborhood,''), COALESCE(address_city,''), COALESCE(address_state,''),
       COALESCE(address_postal_code,''), COALESCE(notes,'')
  FROM profiles WHERE id=$1`, databaseUUID(id)),
			"version", "full_name", "social_name", "cpf", "email", "mobile_phone", "landline_phone",
			"address_street", "address_number", "address_complement", "address_neighborhood",
			"address_city", "address_state", "address_postal_code", "notes")
	case ModuleDocuments:
		values, err = scanCanonicalValues(store.pool.QueryRow(ctx, `SELECT version::text, owner_profile_id::text, document_type_id::text, identifier_value,
       COALESCE(document_date::text,''), COALESCE(notes,''), record_state
  FROM documents WHERE id=$1`, databaseUUID(id)),
			"version", "owner_profile_id", "document_type_id", "identifier_value", "document_date", "notes", "record_state")
	case ModuleBills:
		values, err = scanCanonicalValues(store.pool.QueryRow(ctx, `SELECT version::text, owner_profile_id::text, bill_type_id::text,
       COALESCE(printed_holder_name,''), COALESCE(printed_address,''), COALESCE(reference_value,''),
       COALESCE(competence,''), COALESCE(amount::text,''), COALESCE(currency,''),
       COALESCE(notes,''), record_state
  FROM bills WHERE id=$1`, databaseUUID(id)),
			"version", "owner_profile_id", "bill_type_id", "printed_holder_name", "printed_address", "reference_value",
			"competence", "amount", "currency", "notes", "record_state")
	default:
		return nil, ErrInvalidInput
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load canonical operation values: %w", err)
	}
	return values, nil
}

func scanCanonicalValues(row pgx.Row, keys ...string) (map[string]string, error) {
	values := make([]string, len(keys))
	targets := make([]any, len(keys))
	for index := range values {
		targets[index] = &values[index]
	}
	if err := row.Scan(targets...); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(keys))
	for index, key := range keys {
		result[key] = values[index]
	}
	return result, nil
}

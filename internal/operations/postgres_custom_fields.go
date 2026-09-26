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
		values, err = scanCanonicalValues(store.pool.QueryRow(ctx, `SELECT version::text, full_name, COALESCE(social_name,''), COALESCE((
         SELECT presence.identifier_value FROM document_presences presence
         JOIN document_types document_type ON document_type.id = presence.document_type_id
         WHERE presence.profile_id = profiles.id AND document_type.technical_key = 'cpf' AND presence.claim = 'informed_number'
         LIMIT 1
       ),''),
       COALESCE(email,''), COALESCE(mobile_phone,''), COALESCE(landline_phone,''),
       COALESCE(address_street,''), COALESCE(address_number,''), COALESCE(address_complement,''),
       COALESCE(address_neighborhood,''), COALESCE(address_city,''), COALESCE(address_state,''),
       COALESCE(address_postal_code,''), COALESCE(notes,''),
       COALESCE(birth_date::text,''), COALESCE(gender,''), COALESCE(blood_type,''), COALESCE(nationality,''),
       COALESCE(birth_city,''), COALESCE(marital_status,''), COALESCE(wedding_date::text,''),
       COALESCE(father_name,''), COALESCE(father_birth_date::text,''), COALESCE(mother_name,''),
       COALESCE(mother_birth_date::text,''), COALESCE(health_plan,''), COALESCE(blood_donor::text,''),
       COALESCE(organ_donor::text,''), COALESCE(team,''), COALESCE(sector,''), COALESCE(collections,''),
       COALESCE(vehicle_model,''), COALESCE(vehicle_color,''), COALESCE(vehicle_plate,''),
       COALESCE(vehicle_year::text,''), COALESCE(club_membership,''), COALESCE(membership_type,''),
       COALESCE(place_of_origin,''), COALESCE(birth_country,''), COALESCE(parents_wedding_date::text,''),
       COALESCE(supermarket_club,''), COALESCE(pet,''), COALESCE(travel_countries,''),
       COALESCE(card_brand,''), COALESCE(card_bank,'')
  FROM profiles WHERE id=$1`, databaseUUID(id)),
			"version", "full_name", "social_name", "cpf", "email", "mobile_phone", "landline_phone",
			"address_street", "address_number", "address_complement", "address_neighborhood",
			"address_city", "address_state", "address_postal_code", "notes",
			"birth_date", "gender", "blood_type", "nationality", "birth_city", "marital_status", "wedding_date",
			"father_name", "father_birth_date", "mother_name", "mother_birth_date", "health_plan", "blood_donor",
			"organ_donor", "team", "sector", "collections", "vehicle_model", "vehicle_color", "vehicle_plate",
			"vehicle_year", "club_membership", "membership_type", "place_of_origin", "birth_country",
			"parents_wedding_date", "supermarket_club", "pet", "travel_countries", "card_brand", "card_bank")
	case ModuleDocuments:
		values, err = scanCanonicalValues(store.pool.QueryRow(ctx, `SELECT document.version::text, presence.profile_id::text, presence.document_type_id::text, COALESCE(presence.identifier_value,''),
       COALESCE(document.document_date::text,''), COALESCE(document.notes,''), document.medium
  FROM documents document
  JOIN document_presences presence ON presence.id = document.presence_id
 WHERE document.id=$1`, databaseUUID(id)),
			"version", "owner_profile_id", "document_type_id", "identifier_value", "document_date", "notes", "medium")
	case ModuleBills:
		values, err = scanCanonicalValues(store.pool.QueryRow(ctx, `SELECT version::text, owner_profile_id::text, bill_type_id::text,
       COALESCE(printed_holder_name,''), COALESCE(printed_address,''), COALESCE(reference_value,''),
       COALESCE(competence,''), COALESCE(amount::text,''), COALESCE(currency,''),
       COALESCE(notes,''), medium
  FROM bills WHERE id=$1`, databaseUUID(id)),
			"version", "owner_profile_id", "bill_type_id", "printed_holder_name", "printed_address", "reference_value",
			"competence", "amount", "currency", "notes", "medium")
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

func (store *PostgresStore) FindProfileIDsByCPF(ctx context.Context, digits string) ([]Identifier, error) {
	if digits == "" {
		return nil, nil
	}
	rows, err := store.pool.Query(ctx, `
SELECT DISTINCT profiles.id
  FROM profiles
  JOIN document_presences presence ON presence.profile_id = profiles.id
  JOIN document_types document_type ON document_type.id = presence.document_type_id
 WHERE document_type.technical_key = 'cpf'
   AND presence.claim = 'informed_number'
   AND presence.identifier_digits = $1
 LIMIT 3`, digits)
	if err != nil {
		return nil, fmt.Errorf("find profiles by cpf: %w", err)
	}
	defer rows.Close()
	var ids []Identifier
	for rows.Next() {
		var id pgtype.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id.Valid {
			ids = append(ids, identifierFromUUID(id))
		}
	}
	return ids, rows.Err()
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

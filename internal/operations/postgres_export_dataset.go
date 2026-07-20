package operations

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) ExportDataset(ctx context.Context, module Module) (ExportDataset, error) {
	switch module {
	case ModuleProfiles:
		rows, err := store.pool.Query(ctx, `SELECT id::text, version::text, full_name,
  COALESCE(social_name,''), COALESCE(cpf,''), COALESCE(email,''),
  COALESCE(mobile_phone,''), COALESCE(landline_phone,''), COALESCE(address_street,''),
  COALESCE(address_number,''), COALESCE(address_complement,''), COALESCE(address_neighborhood,''),
  COALESCE(address_city,''), COALESCE(address_state,''), COALESCE(address_postal_code,''), COALESCE(notes,'')
 FROM profiles ORDER BY id`)
		if err != nil {
			return ExportDataset{}, fmt.Errorf("query profile export: %w", err)
		}
		dataset, err := collectDataset(rows, []string{"record_id", "version", "full_name", "social_name", "cpf", "email", "mobile_phone", "landline_phone", "address_street", "address_number", "address_complement", "address_neighborhood", "address_city", "address_state", "address_postal_code", "notes"})
		if err != nil {
			return ExportDataset{}, err
		}
		return store.appendProfileCustomFieldExport(ctx, dataset)
	case ModuleDocuments:
		rows, err := store.pool.Query(ctx, `SELECT id::text, version::text, owner_profile_id::text,
  document_type_id::text, identifier_value, COALESCE(document_date::text,''),
  COALESCE(notes,''), record_state FROM documents ORDER BY id`)
		if err != nil {
			return ExportDataset{}, fmt.Errorf("query document export: %w", err)
		}
		return collectDataset(rows, []string{"record_id", "version", "owner_profile_id", "document_type_id", "identifier_value", "document_date", "notes", "record_state"})
	case ModuleBills:
		rows, err := store.pool.Query(ctx, `SELECT id::text, version::text, owner_profile_id::text,
  bill_type_id::text, COALESCE(printed_holder_name,''), COALESCE(printed_address,''),
  COALESCE(reference_value,''), COALESCE(competence,''), COALESCE(amount::text,''),
  COALESCE(currency,''), COALESCE(notes,''), record_state FROM bills ORDER BY id`)
		if err != nil {
			return ExportDataset{}, fmt.Errorf("query bill export: %w", err)
		}
		return collectDataset(rows, []string{"record_id", "version", "owner_profile_id", "bill_type_id", "printed_holder_name", "printed_address", "reference_value", "competence", "amount", "currency", "notes", "record_state"})
	default:
		return ExportDataset{}, ErrInvalidInput
	}
}

func (store *PostgresStore) appendProfileCustomFieldExport(ctx context.Context, dataset ExportDataset) (ExportDataset, error) {
	fields, err := store.CustomFields(ctx, ModuleProfiles)
	if err != nil {
		return ExportDataset{}, err
	}
	if len(fields) == 0 {
		return dataset, nil
	}
	positions := make(map[string]int, len(fields))
	for _, field := range fields {
		positions[field.Definition.ID.String()] = len(dataset.Headers)
		dataset.Headers = append(dataset.Headers, field.Field.ID)
	}
	values := make(map[string]map[int]string)
	rows, err := store.pool.Query(ctx, `SELECT value.profile_id::text, value.field_definition_id::text,
       value.field_kind, value.text_value, value.integer_value, value.decimal_value::text,
       value.boolean_value, value.civil_date_value, value.civil_month_value
  FROM custom_field_values value
  JOIN custom_field_definitions definition ON definition.id=value.field_definition_id
 WHERE value.profile_id IS NOT NULL AND definition.target_kind='PROFILE' AND definition.active=true
   AND definition.field_kind IN ('TEXT','LONG_TEXT','INTEGER','DECIMAL','BOOLEAN','CIVIL_DATE','CIVIL_MONTH','EMAIL','PHONE')
 ORDER BY value.profile_id, value.field_definition_id`)
	if err != nil {
		return ExportDataset{}, fmt.Errorf("query profile custom field export: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var profileID, fieldID string
		var kind string
		var textValue, decimalValue, civilMonth pgtype.Text
		var integerValue pgtype.Int8
		var booleanValue pgtype.Bool
		var civilDate pgtype.Date
		if err := rows.Scan(&profileID, &fieldID, &kind, &textValue, &integerValue, &decimalValue, &booleanValue, &civilDate, &civilMonth); err != nil {
			return ExportDataset{}, fmt.Errorf("scan profile custom field export: %w", err)
		}
		position, ok := positions[fieldID]
		if !ok {
			continue
		}
		if values[profileID] == nil {
			values[profileID] = make(map[int]string)
		}
		switch kind {
		case "TEXT", "LONG_TEXT", "EMAIL", "PHONE":
			values[profileID][position] = textValue.String
		case "INTEGER":
			values[profileID][position] = fmt.Sprint(integerValue.Int64)
		case "DECIMAL":
			values[profileID][position] = decimalValue.String
		case "BOOLEAN":
			values[profileID][position] = fmt.Sprint(booleanValue.Bool)
		case "CIVIL_DATE":
			values[profileID][position] = civilDate.Time.Format("2006-01-02")
		case "CIVIL_MONTH":
			values[profileID][position] = civilMonth.String
		}
	}
	if err := rows.Err(); err != nil {
		return ExportDataset{}, fmt.Errorf("iterate profile custom field export: %w", err)
	}
	for index := range dataset.Rows {
		row := make([]string, len(dataset.Headers))
		copy(row, dataset.Rows[index])
		if len(row) > 0 {
			for position, value := range values[row[0]] {
				row[position] = value
			}
		}
		dataset.Rows[index] = row
	}
	return dataset, nil
}

func collectDataset(rows pgx.Rows, headers []string) (ExportDataset, error) {
	defer rows.Close()
	dataset := ExportDataset{Headers: headers}
	for rows.Next() {
		if len(dataset.Rows) >= MaximumExportRows {
			return ExportDataset{}, ErrWorkbookLimit
		}
		values, err := rows.Values()
		if err != nil {
			return ExportDataset{}, fmt.Errorf("read export row: %w", err)
		}
		row := make([]string, len(values))
		for index, value := range values {
			switch typed := value.(type) {
			case string:
				row[index] = typed
			case []byte:
				row[index] = string(typed)
			default:
				row[index] = fmt.Sprint(typed)
			}
		}
		dataset.Rows = append(dataset.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return ExportDataset{}, fmt.Errorf("iterate export rows: %w", err)
	}
	return dataset, nil
}

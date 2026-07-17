package customdata

import (
	"context"
	"fmt"
	"strings"
)

func (store *PostgresStore) CountFieldDefinitions(ctx context.Context, filters FieldDefinitionFilters) (int64, error) {
	condition, args := definitionTargetCondition(filters.TargetKind, filters.TargetID, 1)
	args = append(args, filters.Label, boolFilter(filters.Active))
	query := fmt.Sprintf(`SELECT count(*) FROM custom_field_definitions WHERE %s
AND ($%d::text = '' OR lower(label) LIKE '%%' || lower($%d) || '%%')
AND ($%d::boolean IS NULL OR active = $%d)`, condition, len(args)-1, len(args)-1, len(args), len(args))
	var count int64
	if err := store.db.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count custom field definitions: %w", err)
	}
	return count, nil
}

func (store *PostgresStore) ListFieldDefinitions(ctx context.Context, options FieldDefinitionListOptions) ([]FieldDefinition, error) {
	condition, args := definitionTargetCondition(options.Filters.TargetKind, options.Filters.TargetID, 1)
	args = append(args, options.Filters.Label, boolFilter(options.Filters.Active), options.Offset, options.Limit)
	sortColumn := map[FieldDefinitionSortField]string{
		FieldDefinitionSortLabel: "lower(label)", FieldDefinitionSortKey: "technical_key",
		FieldDefinitionSortCreatedAt: "created_at", FieldDefinitionSortUpdatedAt: "updated_at",
	}[options.SortField]
	order := strings.ToUpper(string(options.SortOrder))
	labelPosition := len(args) - 3
	activePosition := len(args) - 2
	offsetPosition := len(args) - 1
	limitPosition := len(args)
	query := fmt.Sprintf(`SELECT %s FROM custom_field_definitions WHERE %s
AND ($%d::text = '' OR lower(label) LIKE '%%' || lower($%d) || '%%')
AND ($%d::boolean IS NULL OR active = $%d)
ORDER BY %s %s, id %s OFFSET $%d LIMIT $%d`, fieldDefinitionColumns, condition,
		labelPosition, labelPosition, activePosition, activePosition, sortColumn, order, order, offsetPosition, limitPosition)
	rows, err := store.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list custom field definitions: %w", err)
	}
	defer rows.Close()
	result := make([]FieldDefinition, 0)
	for rows.Next() {
		value, scanErr := scanFieldDefinition(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan custom field definition: %w", scanErr)
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func definitionTargetCondition(kind TargetKind, id Identifier, firstPosition int) (string, []any) {
	switch kind {
	case TargetProfile:
		return `target_kind = 'PROFILE'`, nil
	case TargetDocumentType:
		return fmt.Sprintf(`target_kind = 'DOCUMENT_TYPE' AND document_type_id = $%d`, firstPosition), []any{databaseUUID(id)}
	case TargetBillType:
		return fmt.Sprintf(`target_kind = 'BILL_TYPE' AND bill_type_id = $%d`, firstPosition), []any{databaseUUID(id)}
	case TargetCustomEntityType:
		return fmt.Sprintf(`target_kind = 'CUSTOM_ENTITY_TYPE' AND custom_entity_type_id = $%d`, firstPosition), []any{databaseUUID(id)}
	default:
		return `false`, nil
	}
}

func (store *PostgresStore) GetFieldDefinition(ctx context.Context, id Identifier) (FieldDefinition, error) {
	value, err := scanFieldDefinition(store.db.QueryRow(ctx, `SELECT `+fieldDefinitionColumns+` FROM custom_field_definitions WHERE id = $1`, databaseUUID(id)))
	if err != nil {
		return FieldDefinition{}, fmt.Errorf("get custom field definition: %w", err)
	}
	return value, nil
}

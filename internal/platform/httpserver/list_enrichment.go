package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	"github.com/jackc/pgx/v5"
)

type listEnrichmentQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func isNilQuerier(querier listEnrichmentQuerier) bool {
	if querier == nil {
		return true
	}
	v := reflect.ValueOf(querier)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

func enrichProfileList(ctx context.Context, querier listEnrichmentQuerier, logger *slog.Logger, profiles []profileResponse, reveal bool) {
	for i := range profiles {
		if profiles[i].CustomValues == nil {
			profiles[i].CustomValues = map[string]string{}
		}
		if profiles[i].DocumentIdentifiers == nil {
			profiles[i].DocumentIdentifiers = map[string]string{}
		}
		if profiles[i].DocumentBadges == nil {
			profiles[i].DocumentBadges = []profileDocumentBadge{}
		}
		if profiles[i].DocumentPresences == nil {
			profiles[i].DocumentPresences = []profileDocumentPresence{}
		}
	}
	if isNilQuerier(querier) || len(profiles) == 0 {
		return
	}
	ids, index := listRecordIDs(profiles, func(value profileResponse) string { return value.ID })
	values, err := queryCustomValueMaps(ctx, querier, "profile_id", ids)
	if err != nil {
		logger.Error("enrich profile custom values", "error", err)
	} else {
		for id, mapped := range values {
			if i, ok := index[id]; ok {
				profiles[i].CustomValues = mapped
			}
		}
	}
	identifiers, err := queryDocumentIdentifiers(ctx, querier, ids)
	if err != nil {
		logger.Error("enrich profile document identifiers", "error", err)
	} else {
		for id, mapped := range identifiers {
			if i, ok := index[id]; ok {
				profiles[i].DocumentIdentifiers = mapped
				if cpf := mapped["cpf"]; cpf != "" {
					profiles[i].CPF = profile.DisplayCPF(cpf, reveal)
					profiles[i].CPFDigitSum = profile.DigitSumCPF(cpf)
				}
			}
		}
	}
	badges, presences, err := queryDocumentBadges(ctx, querier, ids)
	if err != nil {
		logger.Error("enrich profile document badges", "error", err)
		return
	}
	for id, mapped := range badges {
		if i, ok := index[id]; ok {
			profiles[i].DocumentBadges = mapped
		}
	}
	for id, mapped := range presences {
		if i, ok := index[id]; ok {
			profiles[i].DocumentPresences = mapped
		}
	}
}

func enrichDocumentList(ctx context.Context, querier listEnrichmentQuerier, logger *slog.Logger, documents []documentResponse) {
	for i := range documents {
		if documents[i].CustomValues == nil {
			documents[i].CustomValues = map[string]string{}
		}
	}
	if isNilQuerier(querier) || len(documents) == 0 {
		return
	}
	ids, index := listRecordIDs(documents, func(value documentResponse) string { return value.ID })
	values, err := queryCustomValueMaps(ctx, querier, "document_id", ids)
	if err != nil {
		logger.Error("enrich document custom values", "error", err)
		return
	}
	for id, mapped := range values {
		if i, ok := index[id]; ok {
			documents[i].CustomValues = mapped
		}
	}
}

func enrichBillList(ctx context.Context, querier listEnrichmentQuerier, logger *slog.Logger, bills []billResponse) {
	for i := range bills {
		if bills[i].CustomValues == nil {
			bills[i].CustomValues = map[string]string{}
		}
	}
	if isNilQuerier(querier) || len(bills) == 0 {
		return
	}
	ids, index := listRecordIDs(bills, func(value billResponse) string { return value.ID })
	values, err := queryCustomValueMaps(ctx, querier, "bill_id", ids)
	if err != nil {
		logger.Error("enrich bill custom values", "error", err)
		return
	}
	for id, mapped := range values {
		if i, ok := index[id]; ok {
			bills[i].CustomValues = mapped
		}
	}
}

func listRecordIDs[T any](values []T, id func(T) string) ([]string, map[string]int) {
	ids := make([]string, 0, len(values))
	index := make(map[string]int, len(values))
	for i, value := range values {
		recordID := id(value)
		ids = append(ids, recordID)
		index[recordID] = i
	}
	return ids, index
}

func queryCustomValueMaps(ctx context.Context, querier listEnrichmentQuerier, column string, ids []string) (map[string]map[string]string, error) {
	if column != "profile_id" && column != "document_id" && column != "bill_id" {
		return nil, fmt.Errorf("unsupported custom value column %q", column)
	}
	query := fmt.Sprintf(`SELECT v.%s::text, d.technical_key, COALESCE(
  v.text_value,
  v.integer_value::text,
  NULLIF(trim(trailing '.' FROM trim(trailing '0' FROM v.decimal_value::text)), ''),
  CASE v.boolean_value WHEN true THEN 'true' WHEN false THEN 'false' END,
  to_char(v.civil_date_value, 'YYYY-MM-DD'),
  v.civil_month_value,
  (
    SELECT string_agg(option_row.label, ', ' ORDER BY option_row.sort_order, option_row.label)
    FROM custom_field_value_options selected
    JOIN custom_field_options option_row ON option_row.id = selected.option_id
    WHERE selected.custom_field_value_id = v.id
  )
)
FROM custom_field_values v
JOIN custom_field_definitions d ON d.id = v.field_definition_id
WHERE v.%s = ANY($1::uuid[])`, column, column)
	rows, err := querier.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("list custom values: %w", err)
	}
	defer rows.Close()
	result := make(map[string]map[string]string)
	for rows.Next() {
		var recordID, key string
		var value *string
		if err := rows.Scan(&recordID, &key, &value); err != nil {
			return nil, fmt.Errorf("scan custom value: %w", err)
		}
		if value == nil || *value == "" {
			continue
		}
		if result[recordID] == nil {
			result[recordID] = map[string]string{}
		}
		result[recordID][key] = *value
	}
	return result, rows.Err()
}

func queryDocumentIdentifiers(ctx context.Context, querier listEnrichmentQuerier, ownerIDs []string) (map[string]map[string]string, error) {
	rows, err := querier.Query(ctx, `SELECT DISTINCT ON (presence.profile_id, document_type.technical_key)
  presence.profile_id::text,
  document_type.technical_key,
  presence.identifier_value
FROM document_presences presence
JOIN document_types document_type ON document_type.id = presence.document_type_id
WHERE presence.profile_id = ANY($1::uuid[])
  AND presence.claim = 'informed_number'
  AND document_type.technical_key NOT IN ('identidade', 'identity', 'crea_oab', 'crea-oab')
ORDER BY presence.profile_id, document_type.technical_key, presence.updated_at DESC`, ownerIDs)
	if err != nil {
		return nil, fmt.Errorf("list document identifiers: %w", err)
	}
	defer rows.Close()
	result := make(map[string]map[string]string)
	for rows.Next() {
		var ownerID, key string
		var identifier *string
		if err := rows.Scan(&ownerID, &key, &identifier); err != nil {
			return nil, fmt.Errorf("scan document identifier: %w", err)
		}
		if identifier == nil || *identifier == "" {
			continue
		}
		if result[ownerID] == nil {
			result[ownerID] = map[string]string{}
		}
		result[ownerID][key] = *identifier
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	seriesRows, err := querier.Query(ctx, `SELECT DISTINCT ON (presence.profile_id)
  presence.profile_id::text,
  COALESCE(value.text_value, '')
FROM documents document
JOIN document_presences presence ON presence.id = document.presence_id
JOIN document_types document_type ON document_type.id = presence.document_type_id
JOIN custom_field_values value ON value.document_id = document.id
JOIN custom_field_definitions field ON field.id = value.field_definition_id
WHERE presence.profile_id = ANY($1::uuid[])
  AND document_type.technical_key = 'ctps'
  AND field.technical_key = 'series'
  AND COALESCE(value.text_value, '') <> ''
ORDER BY presence.profile_id, document.updated_at DESC`, ownerIDs)
	if err != nil {
		return result, fmt.Errorf("list ctps series: %w", err)
	}
	defer seriesRows.Close()
	for seriesRows.Next() {
		var ownerID, series string
		if err := seriesRows.Scan(&ownerID, &series); err != nil {
			return result, fmt.Errorf("scan ctps series: %w", err)
		}
		if result[ownerID] == nil {
			result[ownerID] = map[string]string{}
		}
		result[ownerID]["ctps_series"] = series
	}
	return result, seriesRows.Err()
}

func queryDocumentBadges(ctx context.Context, querier listEnrichmentQuerier, ownerIDs []string) (map[string][]profileDocumentBadge, map[string][]profileDocumentPresence, error) {
	rows, err := querier.Query(ctx, `SELECT
  presence.profile_id::text,
  presence.document_type_id::text,
  document_type.technical_key,
  document_type.label,
  presence.claim,
  COALESCE(presence.identifier_value, ''),
  physical.id IS NOT NULL,
  digital.id IS NOT NULL,
  COALESCE(physical.idle_custody, ''),
  current_use.document_id IS NOT NULL
FROM document_presences presence
JOIN document_types document_type ON document_type.id = presence.document_type_id
LEFT JOIN documents physical ON physical.presence_id = presence.id AND physical.medium = 'PHYSICAL'
LEFT JOIN documents digital ON digital.presence_id = presence.id AND digital.medium = 'DIGITAL'
LEFT JOIN document_current_uses current_use ON current_use.document_id = physical.id
WHERE presence.profile_id = ANY($1::uuid[])
ORDER BY presence.profile_id, document_type.label, document_type.technical_key`, ownerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("list document badges: %w", err)
	}
	defer rows.Close()
	badges := make(map[string][]profileDocumentBadge)
	presences := make(map[string][]profileDocumentPresence)
	for rows.Next() {
		var ownerID, typeID, technicalKey, label, claim, identifier, idleCustody string
		var hasPhysical, hasDigital, inUse bool
		if err := rows.Scan(&ownerID, &typeID, &technicalKey, &label, &claim, &identifier, &hasPhysical, &hasDigital, &idleCustody, &inUse); err != nil {
			return nil, nil, fmt.Errorf("scan document badge: %w", err)
		}
		state := document.PresenceState{
			Claim: document.Claim(claim), Identifier: identifier, HasPhysical: hasPhysical, HasDigital: hasDigital,
			IdleCustody: document.IdleCustody(idleCustody), InUse: inUse,
		}
		presence := profileDocumentPresence{
			DocumentTypeID: typeID, TechnicalKey: technicalKey, Label: label, Claim: claim,
			HasPhysical: hasPhysical, HasDigital: hasDigital,
		}
		if claim == string(document.ClaimInformedNumber) && identifier != "" {
			presence.IdentifierValue = identifier
		}
		presences[ownerID] = append(presences[ownerID], presence)
		badge := state.Badge()
		if badge == "" {
			continue
		}
		item := profileDocumentBadge{
			DocumentTypeID: typeID, TechnicalKey: technicalKey, Label: label, Claim: claim, Badge: string(badge),
			HasPhysical: hasPhysical, HasDigital: hasDigital, InHands: state.InHands(),
		}
		if claim == string(document.ClaimInformedNumber) && identifier != "" {
			item.IdentifierValue = identifier
		}
		if hasPhysical && idleCustody != "" {
			item.IdleCustody = idleCustody
		}
		badges[ownerID] = append(badges[ownerID], item)
	}
	return badges, presences, rows.Err()
}

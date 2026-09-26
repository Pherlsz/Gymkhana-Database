package search

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (store *PostgresStore) ListDynamicFields(ctx context.Context) ([]FieldDefinition, error) {
	rows, err := store.pool.Query(ctx, `
SELECT 'custom.' || definition.id::text AS field_key,
       CASE definition.target_kind
         WHEN 'PROFILE' THEN 'profiles'
         WHEN 'DOCUMENT_TYPE' THEN 'documents'
         WHEN 'BILL_TYPE' THEN 'bills'
         WHEN 'CUSTOM_ENTITY_TYPE' THEN 'custom_data'
       END AS field_group,
       definition.label || CASE definition.target_kind
         WHEN 'PROFILE' THEN ' · Pessoa'
         WHEN 'DOCUMENT_TYPE' THEN ' · ' || document_type.label
         WHEN 'BILL_TYPE' THEN ' · ' || bill_type.label
         WHEN 'CUSTOM_ENTITY_TYPE' THEN ' · ' || entity_type.label
       END AS field_label,
       lower(definition.field_kind) AS field_kind
FROM custom_field_definitions AS definition
LEFT JOIN document_types AS document_type ON document_type.id = definition.document_type_id
LEFT JOIN bill_types AS bill_type ON bill_type.id = definition.bill_type_id
LEFT JOIN custom_entity_types AS entity_type ON entity_type.id = definition.custom_entity_type_id
WHERE definition.active = true
  AND definition.field_kind <> 'ATTACHMENT'
ORDER BY definition.label, definition.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fields := make([]FieldDefinition, 0)
	for rows.Next() {
		var field FieldDefinition
		var group string
		field.Module = ModuleCustomData
		if err := rows.Scan(&field.Key, &group, &field.Label, &field.Kind); err != nil {
			return nil, err
		}
		field.Group = Module(group)
		fields = append(fields, field)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return fields, nil
}

func (store *PostgresStore) ReserveRateLimit(ctx context.Context, actorID auth.Identifier, window time.Time, maximum int) error {
	var reserved bool
	err := store.pool.QueryRow(ctx, `
WITH reservation AS (
  INSERT INTO search_rate_limits(actor_user_id, window_started_at, request_count, updated_at)
  VALUES($1, $2, 1, now())
  ON CONFLICT (actor_user_id) DO UPDATE SET
    window_started_at = CASE
      WHEN search_rate_limits.window_started_at < EXCLUDED.window_started_at
        THEN EXCLUDED.window_started_at
      ELSE search_rate_limits.window_started_at
    END,
    request_count = CASE
      WHEN search_rate_limits.window_started_at < EXCLUDED.window_started_at THEN 1
      ELSE search_rate_limits.request_count + 1
    END,
    updated_at = now()
  WHERE search_rate_limits.window_started_at < EXCLUDED.window_started_at
     OR search_rate_limits.request_count < $3
  RETURNING 1
)
SELECT EXISTS(SELECT 1 FROM reservation)`, actorID.String(), window, maximum).Scan(&reserved)
	if err != nil {
		return err
	}
	if !reserved {
		return ErrRateLimited
	}
	return nil
}

func (store *PostgresStore) Execute(ctx context.Context, plan Plan) ([]Result, int64, error) {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	timeoutMilliseconds := plan.StatementTimeout.Milliseconds()
	if timeoutMilliseconds < 1 {
		timeoutMilliseconds = 1
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('statement_timeout', $1::text, true)`, strconv.FormatInt(timeoutMilliseconds, 10)); err != nil {
		return nil, 0, err
	}
	includes, excludes, err := encodeTermSpecs(plan)
	if err != nil {
		return nil, 0, err
	}
	rows, err := tx.Query(ctx, searchSQL, searchArgs(plan, includes, excludes)...)
	if err != nil {
		return nil, 0, normalizeExecutionError(err)
	}
	defer rows.Close()
	results := make([]Result, 0, plan.Limit)
	var total int64
	for rows.Next() {
		var result Result
		var module string
		if err := rows.Scan(
			&module,
			&result.EntityKind,
			&result.EntityID,
			&result.ProfileID,
			&result.TargetKind,
			&result.TargetID,
			&result.EntityLabel,
			&result.FieldKey,
			&result.FieldLabel,
			&result.Preview,
			&result.Score,
			&result.UpdatedAt,
			&total,
		); err != nil {
			return nil, 0, err
		}
		result.Module = Module(module)
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, normalizeExecutionError(err)
	}
	if err := tx.Commit(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return nil, 0, err
	}
	return results, total, nil
}

func normalizeExecutionError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "57014" {
		return ErrQueryTimeout
	}
	return err
}

func encodeTermSpecs(plan Plan) ([]byte, []byte, error) {
	includes := plan.Includes
	if len(includes) == 0 {
		includes = make([]TermSpec, 0, len(plan.Terms))
		for _, term := range plan.Terms {
			patterns := textSearchPatterns(term)
			includes = append(includes, TermSpec{Term: term, Pattern: patterns[0], Patterns: patterns, FieldKeys: []string{}})
		}
	}
	includeJSON, err := json.Marshal(includes)
	if err != nil {
		return nil, nil, err
	}
	excludes := plan.Excludes
	if excludes == nil {
		excludes = []TermSpec{}
	}
	excludeJSON, err := json.Marshal(excludes)
	if err != nil {
		return nil, nil, err
	}
	return includeJSON, excludeJSON, nil
}

func searchArgs(plan Plan, includes, excludes []byte) []any {
	modules := make([]string, 0, len(plan.Modules))
	for _, module := range plan.Modules {
		modules = append(modules, string(module))
	}
	fields := plan.Fields
	if fields == nil {
		fields = []string{}
	}
	return []any{
		modules,
		fields,
		plan.Limit,
		plan.Offset,
		string(plan.Sort),
		string(plan.Order),
		plan.CandidateLimit,
		includes,
		excludes,
	}
}

func (store *PostgresStore) MatchIDs(ctx context.Context, plan Plan, grain Module) ([]string, error) {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	timeoutMilliseconds := plan.StatementTimeout.Milliseconds()
	if timeoutMilliseconds < 1 {
		timeoutMilliseconds = 1
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('statement_timeout', $1::text, true)`, strconv.FormatInt(timeoutMilliseconds, 10)); err != nil {
		return nil, err
	}
	includes, excludes, err := encodeTermSpecs(plan)
	if err != nil {
		return nil, err
	}
	sql := searchProfileIDSQL
	args := searchArgs(plan, includes, excludes)
	if grain != ModuleProfiles {
		sql = searchIDSQL
		args = append(args, string(grain))
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, normalizeExecutionError(err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id != "" {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, normalizeExecutionError(err)
	}
	if err := tx.Commit(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return nil, err
	}
	return ids, nil
}

func (store *PostgresStore) Suggest(ctx context.Context, query SuggestQuery) ([]SuggestHit, error) {
	limit := query.Limit
	if limit < 1 || limit > MaxSuggest {
		limit = MaxSuggest
	}
	fragment := strings.TrimSpace(query.Q)
	sql, args := suggestSQL(query.Field, fragment, limit)
	if sql == "" {
		return []SuggestHit{}, nil
	}
	rows, err := store.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := make([]SuggestHit, 0, limit)
	for rows.Next() {
		var hit SuggestHit
		if err := rows.Scan(&hit.Value, &hit.Label); err != nil {
			return nil, err
		}
		hits = append(hits, hit)
	}
	return hits, rows.Err()
}

func suggestSQL(field, fragment string, limit int32) (string, []any) {
	folded := foldToken(field)
	like := "%" + escapeLike(fragment) + "%"
	switch folded {
	case "cidade", "city", "profile.address_city":
		return `SELECT value, value FROM (
  SELECT DISTINCT address_city AS value
  FROM profiles
  WHERE address_city IS NOT NULL AND address_city <> ''
    AND ($1 = '' OR lower(address_city) LIKE $1 ESCAPE '\' OR similarity(address_city, $2) > 0.15)
  ORDER BY CASE WHEN $2 = '' THEN 0 ELSE similarity(address_city, $2) END DESC, address_city
  LIMIT $3
) AS ranked`, []any{like, fragment, limit}
	case "tipo", "type":
		return `SELECT value, label FROM (
  SELECT technical_key AS value, label
  FROM document_types WHERE active = true AND ($1 = '' OR lower(label) LIKE $1 ESCAPE '\' OR lower(technical_key) LIKE $1 ESCAPE '\')
  UNION ALL
  SELECT technical_key, label
  FROM bill_types WHERE active = true AND ($1 = '' OR lower(label) LIKE $1 ESCAPE '\' OR lower(technical_key) LIKE $1 ESCAPE '\')
) AS types
ORDER BY similarity(label, $2) DESC NULLS LAST, label
LIMIT $3`, []any{like, fragment, limit}
	case "nome", "name", "em_uso", "holder":
		return `SELECT id::text, full_name FROM profiles
WHERE full_name <> ''
  AND ($1 = '' OR lower(full_name) LIKE $1 ESCAPE '\' OR similarity(full_name, $2) > 0.12)
ORDER BY CASE WHEN $2 = '' THEN 0 ELSE similarity(full_name, $2) END DESC, full_name
LIMIT $3`, []any{like, fragment, limit}
	default:
		typeKey := ""
		if folded == "identificador" || folded == "identifier" {
			typeKey = ""
		} else if kind, ok := lookupTypeSugar(folded); ok {
			typeKey = kind
		} else {
			return "", nil
		}
		return `SELECT presence.identifier_value, document_type.label || ' · ' || presence.identifier_value
FROM document_presences AS presence
JOIN document_types AS document_type ON document_type.id = presence.document_type_id
WHERE presence.claim = 'informed_number'
  AND presence.identifier_value IS NOT NULL AND presence.identifier_value <> ''
  AND ($1 = '' OR document_type.technical_key = $1)
  AND ($2 = '' OR coalesce(presence.identifier_digits, '') LIKE $2 || '%' OR lower(presence.identifier_value) LIKE $3 ESCAPE '\')
ORDER BY presence.identifier_value
LIMIT $4`, []any{typeKey, digitValue(fragment), like, limit}
	}
}

const searchBody = `
WITH include_specs AS MATERIALIZED (
  SELECT spec->>'term' AS term,
         COALESCE(
           CASE WHEN jsonb_typeof(spec->'patterns') = 'array'
                THEN NULLIF(ARRAY(SELECT jsonb_array_elements_text(spec->'patterns')), ARRAY[]::text[])
           END,
           CASE WHEN spec->>'pattern' IS NULL OR spec->>'pattern' = '' THEN ARRAY[]::text[]
                ELSE ARRAY[spec->>'pattern'] END
         ) AS patterns,
         CASE WHEN jsonb_typeof(spec->'field_keys') = 'array'
              THEN COALESCE(ARRAY(SELECT jsonb_array_elements_text(spec->'field_keys')), ARRAY[]::text[])
              ELSE ARRAY[]::text[]
         END AS field_keys
  FROM jsonb_array_elements($8::jsonb) AS spec
),
exclude_specs AS MATERIALIZED (
  SELECT spec->>'term' AS term,
         COALESCE(
           CASE WHEN jsonb_typeof(spec->'patterns') = 'array'
                THEN NULLIF(ARRAY(SELECT jsonb_array_elements_text(spec->'patterns')), ARRAY[]::text[])
           END,
           CASE WHEN spec->>'pattern' IS NULL OR spec->>'pattern' = '' THEN ARRAY[]::text[]
                ELSE ARRAY[spec->>'pattern'] END
         ) AS patterns,
         CASE WHEN jsonb_typeof(spec->'field_keys') = 'array'
              THEN COALESCE(ARRAY(SELECT jsonb_array_elements_text(spec->'field_keys')), ARRAY[]::text[])
              ELSE ARRAY[]::text[]
         END AS field_keys
  FROM jsonb_array_elements(COALESCE($9::jsonb, '[]'::jsonb)) AS spec
),
search_values AS NOT MATERIALIZED (
  SELECT 'profiles'::text AS module,
         'profile'::text AS entity_kind,
         profile.id::text AS entity_id,
         profile.id::text AS profile_id,
         'profile'::text AS target_kind,
         profile.id::text AS target_id,
         'profile:' || profile.id::text AS scope_id,
         profile.full_name AS entity_label,
         field.field_key,
         field.field_label,
         field.search_value,
         field.display_value,
         field.weight,
         profile.updated_at
	FROM profiles AS profile
  CROSS JOIN LATERAL (VALUES
    ('profile.full_name', 'Nome completo', profile.full_name, profile.full_name, 80),
    ('profile.social_name', 'Nome social', profile.social_name, profile.social_name, 70),
    ('profile.email', 'E-mail', profile.email, profile.email, 60),
    ('profile.mobile_phone', 'Celular', profile.mobile_phone, profile.mobile_phone, 55),
    ('profile.landline_phone', 'Telefone', profile.landline_phone, profile.landline_phone, 50),
    ('profile.address_street', 'Logradouro', profile.address_street, profile.address_street, 35),
    ('profile.address_number', 'Número', profile.address_number, profile.address_number, 25),
    ('profile.address_complement', 'Complemento', profile.address_complement, profile.address_complement, 20),
    ('profile.address_neighborhood', 'Bairro', profile.address_neighborhood, profile.address_neighborhood, 30),
    ('profile.address_city', 'Cidade', profile.address_city, profile.address_city, 40),
    ('profile.address_state', 'UF', profile.address_state, profile.address_state, 20),
    ('profile.address_postal_code', 'CEP', profile.address_postal_code, profile.address_postal_code, 45),
    ('profile.notes', 'Observações', profile.notes, profile.notes, 10),
    ('profile.team', 'Equipe', profile.team, profile.team, 40),
    ('profile.club_membership', 'Sócio clube', profile.club_membership, profile.club_membership, 35),
    ('profile.place_of_origin', 'Naturalidade', profile.place_of_origin, profile.place_of_origin, 30),
    ('profile.birth_country', 'País de nascimento', profile.birth_country, profile.birth_country, 25),
    ('profile.supermarket_club', 'Clube de supermercado', profile.supermarket_club, profile.supermarket_club, 25),
    ('profile.pet', 'Animal', profile.pet, profile.pet, 20),
    ('profile.travel_countries', 'Viagem', profile.travel_countries, profile.travel_countries, 20),
    ('profile.card_brand', 'Bandeira do cartão', profile.card_brand, profile.card_brand, 15),
    ('profile.card_bank', 'Banco do cartão', profile.card_bank, profile.card_bank, 15)
  ) AS field(field_key, field_label, search_value, display_value, weight)
  WHERE
    field.search_value IS NOT NULL AND field.search_value <> ''
    AND EXISTS (
      SELECT 1 FROM include_specs AS spec
      WHERE EXISTS (
        SELECT 1 FROM unnest(spec.patterns) AS pattern
        WHERE profile.full_name ILIKE pattern ESCAPE '\'
           OR COALESCE(profile.social_name, '') ILIKE pattern ESCAPE '\'
           OR COALESCE(profile.email, '') ILIKE pattern ESCAPE '\'
           OR COALESCE(profile.team, '') ILIKE pattern ESCAPE '\'
           OR COALESCE(profile.address_city, '') ILIKE pattern ESCAPE '\'
           OR COALESCE(profile.address_street, '') ILIKE pattern ESCAPE '\'
           OR COALESCE(profile.place_of_origin, '') ILIKE pattern ESCAPE '\'
           OR COALESCE(profile.birth_country, '') ILIKE pattern ESCAPE '\'
           OR COALESCE(profile.mobile_phone, '') ILIKE pattern ESCAPE '\'
           OR COALESCE(profile.notes, '') ILIKE pattern ESCAPE '\'
      )
    )

  UNION ALL

  SELECT 'profiles'::text,
         'profile'::text,
         profile.id::text,
         profile.id::text,
         'profile'::text,
         profile.id::text,
         'profile:' || profile.id::text,
         profile.full_name,
         'profile.document_identifier',
         document_type.label || ' · número informado',
         document_type.label || ' ' || document_type.technical_key || ' ' || presence.identifier_value,
         presence.identifier_value,
         75,
         presence.updated_at
  FROM document_presences AS presence
  JOIN document_types AS document_type ON document_type.id = presence.document_type_id
  JOIN profiles AS profile ON profile.id = presence.profile_id
  WHERE presence.claim = 'informed_number'
    AND presence.identifier_value IS NOT NULL
    AND presence.identifier_value <> ''
    AND EXISTS (
      SELECT 1 FROM include_specs AS spec
      WHERE EXISTS (
        SELECT 1 FROM unnest(spec.patterns) AS pattern
        WHERE presence.identifier_value ILIKE pattern ESCAPE '\'
           OR COALESCE(presence.identifier_digits, '') LIKE pattern ESCAPE '\'
           OR document_type.technical_key ILIKE pattern ESCAPE '\'
           OR translate(lower(document_type.label || ' ' || document_type.technical_key || ' ' || presence.identifier_value),
             'áàâãäéèêëíìîïóòôõöúùûüçñÁÀÂÃÄÉÈÊËÍÌÎÏÓÒÔÕÖÚÙÛÜÇÑ',
             'aaaaaeeeeiiiiooooouuuucnaaaaaeeeeiiiiooooouuuucn') LIKE pattern ESCAPE '\'
      )
    )

  UNION ALL

  SELECT 'documents',
         'document',
         document.id::text,
         presence.profile_id::text,
         'document',
         document.id::text,
         'profile:' || presence.profile_id::text,
         document_type.label || ' · ' || COALESCE(presence.identifier_value, ''),
         field.field_key,
         field.field_label,
         field.search_value,
         field.display_value,
         field.weight,
         document.updated_at
  FROM documents AS document
  JOIN document_presences AS presence ON presence.id = document.presence_id
  JOIN document_types AS document_type ON document_type.id = presence.document_type_id
  LEFT JOIN document_current_uses AS current_use ON current_use.document_id = document.id
  LEFT JOIN profiles AS holder ON holder.id = current_use.holder_profile_id
  CROSS JOIN LATERAL (VALUES
    ('document.type', 'Tipo de documento', document_type.label || ' ' || document_type.technical_key, document_type.label, 55),
    ('document.identifier', 'Identificador', presence.identifier_value, presence.identifier_value, 75),
    ('document.date', 'Data', document.document_date::text, document.document_date::text, 35),
    ('document.notes', 'Observações', document.notes, document.notes, 10),
    ('document.medium', 'Meio', document.medium, document.medium, 20),
    ('document.current_holder', 'Pessoa em uso', holder.full_name, holder.full_name, 45)
  ) AS field(field_key, field_label, search_value, display_value, weight)
  WHERE field.search_value IS NOT NULL AND field.search_value <> ''
    AND EXISTS (
      SELECT 1 FROM include_specs AS spec
      WHERE (cardinality(spec.field_keys) = 0 OR field.field_key = ANY(spec.field_keys))
        AND EXISTS (
          SELECT 1 FROM unnest(spec.patterns) AS pattern
          WHERE translate(lower(field.search_value),
            'áàâãäéèêëíìîïóòôõöúùûüçñÁÀÂÃÄÉÈÊËÍÌÎÏÓÒÔÕÖÚÙÛÜÇÑ',
            'aaaaaeeeeiiiiooooouuuucnaaaaaeeeeiiiiooooouuuucn') LIKE pattern ESCAPE '\'
            OR (
              regexp_replace(field.search_value, '[^0-9]', '', 'g') <> ''
              AND regexp_replace(field.search_value, '[^0-9]', '', 'g') LIKE pattern ESCAPE '\'
            )
        )
    )

  UNION ALL

  SELECT 'bills',
         'bill',
         bill.id::text,
         bill.owner_profile_id::text,
         'bill',
         bill.id::text,
         'profile:' || bill.owner_profile_id::text,
         bill_type.label || COALESCE(' · ' || bill.reference_value, ''),
         field.field_key,
         field.field_label,
         field.search_value,
         field.display_value,
         field.weight,
         bill.updated_at
  FROM bills AS bill
  JOIN bill_types AS bill_type ON bill_type.id = bill.bill_type_id
  LEFT JOIN bill_current_uses AS current_use ON current_use.bill_id = bill.id
  LEFT JOIN profiles AS holder ON holder.id = current_use.holder_profile_id
  CROSS JOIN LATERAL (VALUES
    ('bill.type', 'Tipo de conta/comprovante', bill_type.label || ' ' || bill_type.technical_key, bill_type.label, 55),
    ('bill.printed_holder_name', 'Titular impresso', bill.printed_holder_name, bill.printed_holder_name, 50),
    ('bill.printed_address', 'Endereço impresso', bill.printed_address, bill.printed_address, 40),
    ('bill.reference', 'Referência', bill.reference_value, bill.reference_value, 70),
    ('bill.competence', 'Competência', bill.competence, bill.competence, 45),
    ('bill.amount', 'Valor', bill.amount::text, bill.amount::text, 35),
    ('bill.currency', 'Moeda', bill.currency, bill.currency, 20),
    ('bill.notes', 'Observações', bill.notes, bill.notes, 10),
    ('bill.medium', 'Meio', bill.medium, bill.medium, 20),
    ('bill.current_holder', 'Pessoa em uso', holder.full_name, holder.full_name, 45)
  ) AS field(field_key, field_label, search_value, display_value, weight)
  WHERE field.search_value IS NOT NULL AND field.search_value <> ''
    AND EXISTS (
      SELECT 1 FROM include_specs AS spec
      WHERE (cardinality(spec.field_keys) = 0 OR field.field_key = ANY(spec.field_keys))
        AND EXISTS (
          SELECT 1 FROM unnest(spec.patterns) AS pattern
          WHERE translate(lower(field.search_value),
            'áàâãäéèêëíìîïóòôõöúùûüçñÁÀÂÃÄÉÈÊËÍÌÎÏÓÒÔÕÖÚÙÛÜÇÑ',
            'aaaaaeeeeiiiiooooouuuucnaaaaaeeeeiiiiooooouuuucn') LIKE pattern ESCAPE '\'
            OR (
              regexp_replace(field.search_value, '[^0-9]', '', 'g') <> ''
              AND regexp_replace(field.search_value, '[^0-9]', '', 'g') LIKE pattern ESCAPE '\'
            )
        )
    )

  UNION ALL

  SELECT 'custom_data',
         CASE
           WHEN value.profile_id IS NOT NULL THEN 'profile'
           WHEN value.document_id IS NOT NULL THEN 'document'
           WHEN value.bill_id IS NOT NULL THEN 'bill'
           ELSE 'custom_entity'
         END,
         COALESCE(value.profile_id, value.document_id, value.bill_id, value.custom_entity_id)::text,
         COALESCE(value.profile_id, document_presence.profile_id, bill.owner_profile_id, entity.owner_profile_id)::text,
         CASE
           WHEN value.profile_id IS NOT NULL THEN 'profile'
           WHEN value.document_id IS NOT NULL THEN 'document'
           WHEN value.bill_id IS NOT NULL THEN 'bill'
           ELSE 'custom_entity'
         END,
         COALESCE(value.profile_id, value.document_id, value.bill_id, value.custom_entity_id)::text,
         COALESCE(
           'profile:' || COALESCE(value.profile_id, document_presence.profile_id, bill.owner_profile_id, entity.owner_profile_id)::text,
           'custom_entity:' || value.custom_entity_id::text
         ),
         COALESCE(
           profile.full_name,
           document_type.label || ' · ' || COALESCE(document_presence.identifier_value, ''),
           bill_type.label || COALESCE(' · ' || bill.reference_value, ''),
           entity_type.label
         ),
         'custom.' || definition.id::text,
         definition.label,
         rendered.search_value,
         rendered.display_value,
         25,
         value.updated_at
  FROM custom_field_values AS value
  JOIN custom_field_definitions AS definition ON definition.id = value.field_definition_id AND definition.active = true
  LEFT JOIN profiles AS profile ON profile.id = value.profile_id
  LEFT JOIN documents AS document ON document.id = value.document_id
  LEFT JOIN document_presences AS document_presence ON document_presence.id = document.presence_id
  LEFT JOIN document_types AS document_type ON document_type.id = document_presence.document_type_id
  LEFT JOIN bills AS bill ON bill.id = value.bill_id
  LEFT JOIN bill_types AS bill_type ON bill_type.id = bill.bill_type_id
  LEFT JOIN custom_entities AS entity ON entity.id = value.custom_entity_id
  LEFT JOIN custom_entity_types AS entity_type ON entity_type.id = entity.custom_entity_type_id
  LEFT JOIN LATERAL (
    SELECT string_agg(option.label, ', ' ORDER BY selected.option_id) AS labels
    FROM custom_field_value_options AS selected
    JOIN custom_field_options AS option
      ON option.id = selected.option_id AND option.field_definition_id = selected.field_definition_id
    WHERE selected.custom_field_value_id = value.id
  ) AS choices ON true
  CROSS JOIN LATERAL (
    SELECT CASE value.field_kind
             WHEN 'TEXT' THEN value.text_value
             WHEN 'LONG_TEXT' THEN value.text_value
             WHEN 'EMAIL' THEN value.text_value
             WHEN 'PHONE' THEN value.text_value
             WHEN 'INTEGER' THEN value.integer_value::text
             WHEN 'DECIMAL' THEN value.decimal_value::text
             WHEN 'BOOLEAN' THEN CASE WHEN value.boolean_value THEN 'Sim' ELSE 'Não' END
             WHEN 'CIVIL_DATE' THEN value.civil_date_value::text
             WHEN 'CIVIL_MONTH' THEN value.civil_month_value
             WHEN 'SINGLE_SELECT' THEN choices.labels
             WHEN 'MULTI_SELECT' THEN choices.labels
           END AS display_value,
           CASE value.field_kind
             WHEN 'BOOLEAN' THEN CASE WHEN value.boolean_value THEN 'sim true' ELSE 'não nao false' END
             ELSE CASE value.field_kind
               WHEN 'TEXT' THEN value.text_value
               WHEN 'LONG_TEXT' THEN value.text_value
               WHEN 'EMAIL' THEN value.text_value
               WHEN 'PHONE' THEN value.text_value
               WHEN 'INTEGER' THEN value.integer_value::text
               WHEN 'DECIMAL' THEN value.decimal_value::text
               WHEN 'CIVIL_DATE' THEN value.civil_date_value::text
               WHEN 'CIVIL_MONTH' THEN value.civil_month_value
               WHEN 'SINGLE_SELECT' THEN choices.labels
               WHEN 'MULTI_SELECT' THEN choices.labels
             END
           END AS search_value
  ) AS rendered
  WHERE definition.field_kind <> 'ATTACHMENT'
    AND rendered.search_value IS NOT NULL
    AND rendered.search_value <> ''
    AND EXISTS (
      SELECT 1 FROM include_specs AS spec
      WHERE (cardinality(spec.field_keys) = 0 OR ('custom.' || definition.id::text) = ANY(spec.field_keys))
        AND EXISTS (
          SELECT 1 FROM unnest(spec.patterns) AS pattern
          WHERE translate(lower(rendered.search_value),
            'áàâãäéèêëíìîïóòôõöúùûüçñÁÀÂÃÄÉÈÊËÍÌÎÏÓÒÔÕÖÚÙÛÜÇÑ',
            'aaaaaeeeeiiiiooooouuuucnaaaaaeeeeiiiiooooouuuucn') LIKE pattern ESCAPE '\'
            OR (
              regexp_replace(rendered.search_value, '[^0-9]', '', 'g') <> ''
              AND regexp_replace(rendered.search_value, '[^0-9]', '', 'g') LIKE pattern ESCAPE '\'
            )
        )
    )

  UNION ALL

  SELECT 'attachments',
         'attachment',
         attachment.id::text,
         COALESCE(document_presence.profile_id, bill.owner_profile_id, attachment.custom_profile_id,
                  custom_document_presence.profile_id, custom_bill.owner_profile_id, custom_entity.owner_profile_id)::text,
         CASE attachment.owner_kind
           WHEN 'DOCUMENT' THEN 'document'
           WHEN 'BILL' THEN 'bill'
           ELSE lower(attachment.custom_target_kind)
         END,
         COALESCE(attachment.document_id, attachment.bill_id, attachment.custom_profile_id,
                  attachment.custom_document_id, attachment.custom_bill_id, attachment.custom_entity_id)::text,
         COALESCE(
           'profile:' || COALESCE(document_presence.profile_id, bill.owner_profile_id, attachment.custom_profile_id,
                                  custom_document_presence.profile_id, custom_bill.owner_profile_id, custom_entity.owner_profile_id)::text,
           'custom_entity:' || attachment.custom_entity_id::text
         ),
         attachment.original_filename,
         field.field_key,
         field.field_label,
         field.search_value,
         field.display_value,
         field.weight,
         attachment.updated_at
  FROM attachments AS attachment
  LEFT JOIN documents AS document ON document.id = attachment.document_id
  LEFT JOIN document_presences AS document_presence ON document_presence.id = document.presence_id
  LEFT JOIN bills AS bill ON bill.id = attachment.bill_id
  LEFT JOIN documents AS custom_document ON custom_document.id = attachment.custom_document_id
  LEFT JOIN document_presences AS custom_document_presence ON custom_document_presence.id = custom_document.presence_id
  LEFT JOIN bills AS custom_bill ON custom_bill.id = attachment.custom_bill_id
  LEFT JOIN custom_entities AS custom_entity ON custom_entity.id = attachment.custom_entity_id
  LEFT JOIN custom_field_definitions AS definition ON definition.id = attachment.field_definition_id
  CROSS JOIN LATERAL (VALUES
    ('attachment.filename', 'Nome do arquivo', attachment.original_filename, attachment.original_filename, 65),
    ('attachment.declared_mime', 'Tipo declarado', attachment.declared_mime, attachment.declared_mime, 25),
    ('attachment.detected_mime', 'Tipo detectado', attachment.detected_mime, attachment.detected_mime, 25),
    ('attachment.byte_size', 'Tamanho em bytes', attachment.byte_size::text, attachment.byte_size::text, 15),
    ('attachment.owner_field', 'Campo de anexo', definition.label, definition.label, 35)
  ) AS field(field_key, field_label, search_value, display_value, weight)
  WHERE attachment.lifecycle_state = 'ACTIVE'
    AND field.search_value IS NOT NULL
    AND field.search_value <> ''
    AND EXISTS (
      SELECT 1 FROM include_specs AS spec
      WHERE (cardinality(spec.field_keys) = 0 OR field.field_key = ANY(spec.field_keys))
        AND EXISTS (
          SELECT 1 FROM unnest(spec.patterns) AS pattern
          WHERE translate(lower(field.search_value),
            'áàâãäéèêëíìîïóòôõöúùûüçñÁÀÂÃÄÉÈÊËÍÌÎÏÓÒÔÕÖÚÙÛÜÇÑ',
            'aaaaaeeeeiiiiooooouuuucnaaaaaeeeeiiiiooooouuuucn') LIKE pattern ESCAPE '\'
            OR (
              regexp_replace(field.search_value, '[^0-9]', '', 'g') <> ''
              AND regexp_replace(field.search_value, '[^0-9]', '', 'g') LIKE pattern ESCAPE '\'
            )
        )
    )
),
filtered_values AS NOT MATERIALIZED (
  SELECT value.*
  FROM search_values AS value
  WHERE value.module = ANY($1::text[])
    AND (cardinality($2::text[]) = 0 OR value.field_key = ANY($2::text[]))
),
matched_values AS MATERIALIZED (
  SELECT value.*, include_spec.term
  FROM filtered_values AS value
  JOIN include_specs AS include_spec
    ON (cardinality(include_spec.field_keys) = 0 OR value.field_key = ANY(include_spec.field_keys))
   AND EXISTS (
     SELECT 1 FROM unnest(include_spec.patterns) AS pattern
     WHERE translate(lower(value.search_value),
       'áàâãäéèêëíìîïóòôõöúùûüçñÁÀÂÃÄÉÈÊËÍÌÎÏÓÒÔÕÖÚÙÛÜÇÑ',
       'aaaaaeeeeiiiiooooouuuucnaaaaaeeeeiiiiooooouuuucn')
       LIKE pattern ESCAPE '\'
     OR (
       regexp_replace(value.search_value, '[^0-9]', '', 'g') <> ''
       AND regexp_replace(value.search_value, '[^0-9]', '', 'g') LIKE pattern ESCAPE '\'
     )
   )
),
excluded_scopes AS NOT MATERIALIZED (
  SELECT DISTINCT value.scope_id
  FROM filtered_values AS value
  JOIN exclude_specs AS exclude_spec
    ON (cardinality(exclude_spec.field_keys) = 0 OR value.field_key = ANY(exclude_spec.field_keys))
   AND EXISTS (
     SELECT 1 FROM unnest(exclude_spec.patterns) AS pattern
     WHERE translate(lower(value.search_value),
       'áàâãäéèêëíìîïóòôõöúùûüçñÁÀÂÃÄÉÈÊËÍÌÎÏÓÒÔÕÖÚÙÛÜÇÑ',
       'aaaaaeeeeiiiiooooouuuucnaaaaaeeeeiiiiooooouuuucn')
       LIKE pattern ESCAPE '\'
   )
),
eligible_scopes AS NOT MATERIALIZED (
  SELECT matched.scope_id
  FROM matched_values AS matched
  WHERE NOT EXISTS (SELECT 1 FROM excluded_scopes AS excluded WHERE excluded.scope_id = matched.scope_id)
  GROUP BY matched.scope_id
  HAVING count(DISTINCT matched.term) = (SELECT count(DISTINCT term) FROM include_specs)
),
matches AS (
  SELECT value.*,
         ranking.score + value.weight AS score
  FROM matched_values AS value
  JOIN eligible_scopes AS eligible ON eligible.scope_id = value.scope_id
  CROSS JOIN LATERAL (
    SELECT COALESCE(sum(CASE
             WHEN lower(value.display_value) = lower(matched.term) THEN 1000
             WHEN strpos(lower(value.search_value), lower(matched.term)) = 1 THEN 700
             ELSE 400
           END), 0)::integer AS score,
           count(*) AS matched_terms
    FROM include_specs AS matched
    WHERE EXISTS (
      SELECT 1 FROM unnest(matched.patterns) AS pattern
      WHERE translate(lower(value.search_value),
            'áàâãäéèêëíìîïóòôõöúùûüçñÁÀÂÃÄÉÈÊËÍÌÎÏÓÒÔÕÖÚÙÛÜÇÑ',
            'aaaaaeeeeiiiiooooouuuucnaaaaaeeeeiiiiooooouuuucn')
          LIKE pattern ESCAPE '\'
       OR (
         regexp_replace(value.search_value, '[^0-9]', '', 'g') <> ''
         AND regexp_replace(value.search_value, '[^0-9]', '', 'g') LIKE pattern ESCAPE '\'
       )
    )
  ) AS ranking
  WHERE ranking.matched_terms > 0
),
bounded_matches AS MATERIALIZED (
  SELECT * FROM matches LIMIT $7
)`

const searchSQL = searchBody + `
SELECT module,
       entity_kind,
       entity_id,
       COALESCE(profile_id, ''),
       target_kind,
       target_id,
       entity_label,
       field_key,
       field_label,
       left(display_value, 200),
       score,
       updated_at,
       count(*) OVER() AS total
FROM bounded_matches
ORDER BY
  CASE WHEN $5 = 'relevance' AND $6 = 'desc' THEN score END DESC,
  CASE WHEN $5 = 'relevance' AND $6 = 'asc' THEN score END ASC,
  CASE WHEN $5 = 'updated_at' AND $6 = 'desc' THEN updated_at END DESC,
  CASE WHEN $5 = 'updated_at' AND $6 = 'asc' THEN updated_at END ASC,
  score DESC,
  updated_at DESC,
  module,
  entity_kind,
  entity_id,
  field_key
LIMIT $3 OFFSET $4`

const searchIDSQL = searchBody + `
SELECT DISTINCT entity_id
FROM bounded_matches
WHERE module = $10
  AND entity_id <> ''`

const searchProfileIDSQL = searchBody + `
SELECT DISTINCT split_part(scope_id, ':', 2)
FROM bounded_matches
WHERE scope_id LIKE 'profile:%'
  AND split_part(scope_id, ':', 2) <> ''`

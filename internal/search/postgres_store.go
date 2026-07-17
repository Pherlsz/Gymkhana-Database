package search

import (
	"context"
	"errors"
	"strconv"
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
		field.Module = ModuleCustomData
		if err := rows.Scan(&field.Key, &field.Label, &field.Kind); err != nil {
			return nil, err
		}
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
	if _, err := tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true)`, strconv.FormatInt(timeoutMilliseconds, 10)); err != nil {
		return nil, 0, err
	}
	modules := make([]string, 0, len(plan.Modules))
	for _, module := range plan.Modules {
		modules = append(modules, string(module))
	}
	rows, err := tx.Query(ctx, searchSQL,
		plan.Terms,
		plan.LiteralPatterns,
		modules,
		plan.Fields,
		plan.Limit,
		plan.Offset,
		string(plan.Sort),
		string(plan.Order),
		plan.CandidateLimit,
	)
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

const searchSQL = `
WITH search_values AS NOT MATERIALIZED (
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
    ('profile.cpf', 'CPF', profile.cpf, profile.cpf, 75),
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
    ('profile.notes', 'Observações', profile.notes, profile.notes, 10)
  ) AS field(field_key, field_label, search_value, display_value, weight)
  WHERE field.search_value IS NOT NULL AND field.search_value <> ''

  UNION ALL

  SELECT 'documents',
         'document',
         document.id::text,
         document.owner_profile_id::text,
         'document',
         document.id::text,
         'profile:' || document.owner_profile_id::text,
         document_type.label || ' · ' || document.identifier_value,
         field.field_key,
         field.field_label,
         field.search_value,
         field.display_value,
         field.weight,
         document.updated_at
  FROM documents AS document
  JOIN document_types AS document_type ON document_type.id = document.document_type_id
  LEFT JOIN document_current_uses AS current_use ON current_use.document_id = document.id
  LEFT JOIN profiles AS holder ON holder.id = current_use.holder_profile_id
  CROSS JOIN LATERAL (VALUES
    ('document.type', 'Tipo de documento', document_type.label, document_type.label, 55),
    ('document.identifier', 'Identificador', document.identifier_value, document.identifier_value, 75),
    ('document.date', 'Data', document.document_date::text, document.document_date::text, 35),
    ('document.notes', 'Observações', document.notes, document.notes, 10),
    ('document.record_state', 'Estado do registro', document.record_state, document.record_state, 20),
    ('document.current_holder', 'Pessoa em uso', holder.full_name, holder.full_name, 45)
  ) AS field(field_key, field_label, search_value, display_value, weight)
  WHERE field.search_value IS NOT NULL AND field.search_value <> ''

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
    ('bill.type', 'Tipo de conta/comprovante', bill_type.label, bill_type.label, 55),
    ('bill.printed_holder_name', 'Titular impresso', bill.printed_holder_name, bill.printed_holder_name, 50),
    ('bill.printed_address', 'Endereço impresso', bill.printed_address, bill.printed_address, 40),
    ('bill.reference', 'Referência', bill.reference_value, bill.reference_value, 70),
    ('bill.competence', 'Competência', bill.competence, bill.competence, 45),
    ('bill.amount', 'Valor', bill.amount::text, bill.amount::text, 35),
    ('bill.currency', 'Moeda', bill.currency, bill.currency, 20),
    ('bill.notes', 'Observações', bill.notes, bill.notes, 10),
    ('bill.record_state', 'Estado do registro', bill.record_state, bill.record_state, 20),
    ('bill.current_holder', 'Pessoa em uso', holder.full_name, holder.full_name, 45)
  ) AS field(field_key, field_label, search_value, display_value, weight)
  WHERE field.search_value IS NOT NULL AND field.search_value <> ''

  UNION ALL

  SELECT 'custom_data',
         CASE
           WHEN value.profile_id IS NOT NULL THEN 'profile'
           WHEN value.document_id IS NOT NULL THEN 'document'
           WHEN value.bill_id IS NOT NULL THEN 'bill'
           ELSE 'custom_entity'
         END,
         COALESCE(value.profile_id, value.document_id, value.bill_id, value.custom_entity_id)::text,
         COALESCE(value.profile_id, document.owner_profile_id, bill.owner_profile_id, entity.owner_profile_id)::text,
         CASE
           WHEN value.profile_id IS NOT NULL THEN 'profile'
           WHEN value.document_id IS NOT NULL THEN 'document'
           WHEN value.bill_id IS NOT NULL THEN 'bill'
           ELSE 'custom_entity'
         END,
         COALESCE(value.profile_id, value.document_id, value.bill_id, value.custom_entity_id)::text,
         COALESCE(
           'profile:' || COALESCE(value.profile_id, document.owner_profile_id, bill.owner_profile_id, entity.owner_profile_id)::text,
           'custom_entity:' || value.custom_entity_id::text
         ),
         COALESCE(
           profile.full_name,
           document_type.label || ' · ' || document.identifier_value,
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
  LEFT JOIN document_types AS document_type ON document_type.id = document.document_type_id
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

  UNION ALL

  SELECT 'attachments',
         'attachment',
         attachment.id::text,
         COALESCE(document.owner_profile_id, bill.owner_profile_id, attachment.custom_profile_id,
                  custom_document.owner_profile_id, custom_bill.owner_profile_id, custom_entity.owner_profile_id)::text,
         CASE attachment.owner_kind
           WHEN 'DOCUMENT' THEN 'document'
           WHEN 'BILL' THEN 'bill'
           ELSE lower(attachment.custom_target_kind)
         END,
         COALESCE(attachment.document_id, attachment.bill_id, attachment.custom_profile_id,
                  attachment.custom_document_id, attachment.custom_bill_id, attachment.custom_entity_id)::text,
         COALESCE(
           'profile:' || COALESCE(document.owner_profile_id, bill.owner_profile_id, attachment.custom_profile_id,
                                  custom_document.owner_profile_id, custom_bill.owner_profile_id, custom_entity.owner_profile_id)::text,
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
  LEFT JOIN bills AS bill ON bill.id = attachment.bill_id
  LEFT JOIN documents AS custom_document ON custom_document.id = attachment.custom_document_id
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
),
filtered_values AS NOT MATERIALIZED (
  SELECT value.*
  FROM search_values AS value
  WHERE value.module = ANY($3::text[])
    AND (cardinality($4::text[]) = 0 OR value.field_key = ANY($4::text[]))
),
eligible_scopes AS (
  SELECT DISTINCT candidate.scope_id
  FROM filtered_values AS candidate
  WHERE NOT EXISTS (
    SELECT 1
    FROM unnest($1::text[], $2::text[]) AS required(term, pattern)
    WHERE NOT EXISTS (
      SELECT 1
      FROM filtered_values AS related
      WHERE related.scope_id = candidate.scope_id
        AND lower(related.search_value) LIKE required.pattern ESCAPE '\'
    )
  )
),
matches AS (
  SELECT value.*,
         ranking.score + value.weight AS score
  FROM filtered_values AS value
  JOIN eligible_scopes AS eligible ON eligible.scope_id = value.scope_id
  CROSS JOIN LATERAL (
    SELECT COALESCE(sum(CASE
             WHEN lower(value.display_value) = lower(matched.term) THEN 1000
             WHEN strpos(lower(value.search_value), lower(matched.term)) = 1 THEN 700
             ELSE 400
           END), 0)::integer AS score,
           count(*) AS matched_terms
    FROM unnest($1::text[], $2::text[]) AS matched(term, pattern)
    WHERE lower(value.search_value) LIKE matched.pattern ESCAPE '\'
  ) AS ranking
  WHERE ranking.matched_terms > 0
),
bounded_matches AS MATERIALIZED (
  SELECT * FROM matches LIMIT $9
)
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
  CASE WHEN $7 = 'relevance' AND $8 = 'desc' THEN score END DESC,
  CASE WHEN $7 = 'relevance' AND $8 = 'asc' THEN score END ASC,
  CASE WHEN $7 = 'updated_at' AND $8 = 'desc' THEN updated_at END DESC,
  CASE WHEN $7 = 'updated_at' AND $8 = 'asc' THEN updated_at END ASC,
  score DESC,
  updated_at DESC,
  module,
  entity_kind,
  entity_id,
  field_key
LIMIT $5 OFFSET $6`

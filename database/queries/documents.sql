-- name: CreateDocumentType :one
INSERT INTO document_types (
  id, technical_key, label, active, uniqueness_policy, validation_regex, date_required
) VALUES (
  sqlc.arg(id), sqlc.arg(technical_key), sqlc.arg(label), sqlc.arg(active),
  sqlc.arg(uniqueness_policy), sqlc.narg(validation_regex), sqlc.arg(date_required)
)
RETURNING *;

-- name: GetDocumentTypeByID :one
SELECT * FROM document_types WHERE id = sqlc.arg(id);

-- name: CountDocumentTypes :one
SELECT count(*)
FROM document_types
WHERE (sqlc.arg(label_filter)::text = '' OR lower(label) LIKE '%' || lower(sqlc.arg(label_filter)::text) || '%')
  AND (sqlc.arg(active_filter)::text = '' OR active = sqlc.arg(active_filter)::boolean);

-- name: ListDocumentTypes :many
SELECT *
FROM document_types
WHERE (sqlc.arg(label_filter)::text = '' OR lower(label) LIKE '%' || lower(sqlc.arg(label_filter)::text) || '%')
  AND (sqlc.arg(active_filter)::text = '' OR active = sqlc.arg(active_filter)::boolean)
ORDER BY
  CASE WHEN sqlc.arg(sort_field)::text = 'label' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(label) END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'label' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(label) END DESC,
  CASE WHEN sqlc.arg(sort_field)::text = 'technical_key' AND sqlc.arg(sort_order)::text = 'asc' THEN technical_key END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'technical_key' AND sqlc.arg(sort_order)::text = 'desc' THEN technical_key END DESC,
  CASE WHEN sqlc.arg(sort_field)::text = 'created_at' AND sqlc.arg(sort_order)::text = 'asc' THEN created_at END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'created_at' AND sqlc.arg(sort_order)::text = 'desc' THEN created_at END DESC,
  CASE WHEN sqlc.arg(sort_field)::text = 'updated_at' AND sqlc.arg(sort_order)::text = 'asc' THEN updated_at END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'updated_at' AND sqlc.arg(sort_order)::text = 'desc' THEN updated_at END DESC,
  id ASC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: DocumentTypeHasDocuments :one
SELECT EXISTS(SELECT 1 FROM documents WHERE document_type_id = sqlc.arg(document_type_id));

-- name: UpdateDocumentType :one
UPDATE document_types
SET label = sqlc.arg(label), active = sqlc.arg(active), uniqueness_policy = sqlc.arg(uniqueness_policy),
  validation_regex = sqlc.narg(validation_regex), date_required = sqlc.arg(date_required),
  version = version + 1, updated_at = now()
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version)
RETURNING *;

-- name: DeleteDocumentType :one
DELETE FROM document_types WHERE id = sqlc.arg(id) AND version = sqlc.arg(version) RETURNING id;

-- name: CreateDocument :one
INSERT INTO documents (
  id, owner_profile_id, document_type_id, identifier_value, uniqueness_policy,
  document_date, notes, record_state
)
SELECT sqlc.arg(id), sqlc.arg(owner_profile_id), document_type.id, sqlc.arg(identifier_value),
  document_type.uniqueness_policy, sqlc.narg(document_date), sqlc.narg(notes), sqlc.arg(record_state)
FROM document_types AS document_type
WHERE document_type.id = sqlc.arg(document_type_id) AND document_type.active
RETURNING *;

-- name: GetDocumentByID :one
SELECT
  document.*,
  document_type.technical_key AS type_technical_key,
  document_type.label AS type_label,
  document_type.active AS type_active,
  document_type.validation_regex AS type_validation_regex,
  document_type.date_required AS type_date_required,
  document_type.version AS type_version,
  document_type.created_at AS type_created_at,
  document_type.updated_at AS type_updated_at,
  document_current_use.holder_profile_id AS current_holder_profile_id,
  document_current_use.assigned_at AS current_assigned_at
FROM documents AS document
JOIN document_types AS document_type ON document_type.id = document.document_type_id
LEFT JOIN document_current_uses AS document_current_use ON document_current_use.document_id = document.id
WHERE document.id = sqlc.arg(id);

-- name: CountDocuments :one
SELECT count(*)
FROM documents AS document
LEFT JOIN document_current_uses AS document_current_use ON document_current_use.document_id = document.id
WHERE (sqlc.narg(owner_profile_id_filter)::uuid IS NULL OR document.owner_profile_id = sqlc.narg(owner_profile_id_filter)::uuid)
  AND (sqlc.narg(document_type_id_filter)::uuid IS NULL OR document.document_type_id = sqlc.narg(document_type_id_filter)::uuid)
  AND (sqlc.arg(identifier_filter)::text = '' OR lower(document.identifier_value) LIKE '%' || lower(sqlc.arg(identifier_filter)::text) || '%')
  AND (sqlc.arg(record_state_filter)::text = '' OR document.record_state = sqlc.arg(record_state_filter)::text)
  AND (sqlc.arg(status_filter)::text = '' OR
    (sqlc.arg(status_filter)::text = 'IN_USE' AND document_current_use.document_id IS NOT NULL) OR
    (sqlc.arg(status_filter)::text = 'AVAILABLE' AND document_current_use.document_id IS NULL))
  AND (sqlc.narg(holder_profile_id_filter)::uuid IS NULL OR document_current_use.holder_profile_id = sqlc.narg(holder_profile_id_filter)::uuid);

-- name: ListDocuments :many
SELECT
  document.*,
  document_type.technical_key AS type_technical_key,
  document_type.label AS type_label,
  document_type.active AS type_active,
  document_type.validation_regex AS type_validation_regex,
  document_type.date_required AS type_date_required,
  document_type.version AS type_version,
  document_type.created_at AS type_created_at,
  document_type.updated_at AS type_updated_at,
  document_current_use.holder_profile_id AS current_holder_profile_id,
  document_current_use.assigned_at AS current_assigned_at
FROM documents AS document
JOIN document_types AS document_type ON document_type.id = document.document_type_id
LEFT JOIN document_current_uses AS document_current_use ON document_current_use.document_id = document.id
WHERE (sqlc.narg(owner_profile_id_filter)::uuid IS NULL OR document.owner_profile_id = sqlc.narg(owner_profile_id_filter)::uuid)
  AND (sqlc.narg(document_type_id_filter)::uuid IS NULL OR document.document_type_id = sqlc.narg(document_type_id_filter)::uuid)
  AND (sqlc.arg(identifier_filter)::text = '' OR lower(document.identifier_value) LIKE '%' || lower(sqlc.arg(identifier_filter)::text) || '%')
  AND (sqlc.arg(record_state_filter)::text = '' OR document.record_state = sqlc.arg(record_state_filter)::text)
  AND (sqlc.arg(status_filter)::text = '' OR
    (sqlc.arg(status_filter)::text = 'IN_USE' AND document_current_use.document_id IS NOT NULL) OR
    (sqlc.arg(status_filter)::text = 'AVAILABLE' AND document_current_use.document_id IS NULL))
  AND (sqlc.narg(holder_profile_id_filter)::uuid IS NULL OR document_current_use.holder_profile_id = sqlc.narg(holder_profile_id_filter)::uuid)
ORDER BY
  CASE WHEN sqlc.arg(sort_field)::text = 'identifier_value' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(document.identifier_value) END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'identifier_value' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(document.identifier_value) END DESC,
  CASE WHEN sqlc.arg(sort_field)::text = 'type_label' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(document_type.label) END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'type_label' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(document_type.label) END DESC,
  CASE WHEN sqlc.arg(sort_field)::text = 'document_date' AND sqlc.arg(sort_order)::text = 'asc' THEN document.document_date END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'document_date' AND sqlc.arg(sort_order)::text = 'desc' THEN document.document_date END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'created_at' AND sqlc.arg(sort_order)::text = 'asc' THEN document.created_at END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'created_at' AND sqlc.arg(sort_order)::text = 'desc' THEN document.created_at END DESC,
  CASE WHEN sqlc.arg(sort_field)::text = 'updated_at' AND sqlc.arg(sort_order)::text = 'asc' THEN document.updated_at END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'updated_at' AND sqlc.arg(sort_order)::text = 'desc' THEN document.updated_at END DESC,
  document.id ASC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: UpdateDocument :one
UPDATE documents AS document
SET owner_profile_id = sqlc.arg(owner_profile_id), document_type_id = document_type.id,
  identifier_value = sqlc.arg(identifier_value), uniqueness_policy = document_type.uniqueness_policy,
  document_date = sqlc.narg(document_date), notes = sqlc.narg(notes), record_state = sqlc.arg(record_state),
  version = document.version + 1, updated_at = now()
FROM document_types AS document_type
WHERE document.id = sqlc.arg(id) AND document.version = sqlc.arg(version)
  AND document_type.id = sqlc.arg(document_type_id) AND document_type.active
RETURNING document.*;

-- name: DuplicateDocument :one
INSERT INTO documents (
  id, owner_profile_id, document_type_id, identifier_value, uniqueness_policy,
  document_date, notes, record_state
)
SELECT sqlc.arg(new_id), source.owner_profile_id, source.document_type_id, source.identifier_value,
  source.uniqueness_policy, source.document_date, source.notes, source.record_state
FROM documents AS source
WHERE source.id = sqlc.arg(source_id)
RETURNING *;

-- name: DeleteDocument :one
DELETE FROM documents WHERE id = sqlc.arg(id) AND version = sqlc.arg(version) RETURNING id;

-- name: AssignDocumentCurrentUse :one
INSERT INTO document_current_uses (document_id, holder_profile_id)
VALUES (sqlc.arg(document_id), sqlc.arg(holder_profile_id))
ON CONFLICT (document_id) DO UPDATE
SET holder_profile_id = EXCLUDED.holder_profile_id, assigned_at = now()
RETURNING *;

-- name: ReturnDocumentCurrentUse :one
DELETE FROM document_current_uses WHERE document_id = sqlc.arg(document_id) RETURNING document_id;

-- name: GetDocumentCurrentUse :one
SELECT * FROM document_current_uses WHERE document_id = sqlc.arg(document_id);

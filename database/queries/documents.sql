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

-- name: CountDocumentsByType :many
SELECT presence.document_type_id, count(*)::bigint AS document_count
FROM documents AS document
JOIN document_presences AS presence ON presence.id = document.presence_id
LEFT JOIN document_current_uses AS current_use ON current_use.document_id = document.id
WHERE document.medium = 'PHYSICAL'
  AND (document.idle_custody = 'ORGANIZATION' OR current_use.document_id IS NOT NULL)
GROUP BY presence.document_type_id;

-- name: DocumentTypeHasDocuments :one
SELECT EXISTS(
  SELECT 1 FROM document_presences WHERE document_type_id = sqlc.arg(document_type_id)
);

-- name: UpdateDocumentType :one
UPDATE document_types
SET label = sqlc.arg(label), active = sqlc.arg(active), uniqueness_policy = sqlc.arg(uniqueness_policy),
  validation_regex = sqlc.narg(validation_regex), date_required = sqlc.arg(date_required),
  version = version + 1, updated_at = now()
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version)
RETURNING *;

-- name: DeleteDocumentType :one
DELETE FROM document_types WHERE id = sqlc.arg(id) AND version = sqlc.arg(version) RETURNING id;

-- name: GetDocumentPresence :one
SELECT * FROM document_presences
WHERE profile_id = sqlc.arg(profile_id) AND document_type_id = sqlc.arg(document_type_id);

-- name: UpsertDocumentPresence :one
INSERT INTO document_presences (
  id, profile_id, document_type_id, uniqueness_policy, claim, identifier_value
)
SELECT sqlc.arg(id), sqlc.arg(profile_id), document_type.id, document_type.uniqueness_policy,
  sqlc.arg(claim), sqlc.narg(identifier_value)
FROM document_types AS document_type
WHERE document_type.id = sqlc.arg(document_type_id)
ON CONFLICT (profile_id, document_type_id) DO UPDATE
SET claim = EXCLUDED.claim,
  identifier_value = EXCLUDED.identifier_value,
  uniqueness_policy = EXCLUDED.uniqueness_policy,
  version = document_presences.version + 1,
  updated_at = now()
RETURNING *;

-- name: UpdateDocumentPresence :one
UPDATE document_presences
SET claim = sqlc.arg(claim), identifier_value = sqlc.narg(identifier_value),
  uniqueness_policy = sqlc.arg(uniqueness_policy),
  version = version + 1, updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: CountExemplarsByPresence :one
SELECT count(*) FROM documents WHERE presence_id = sqlc.arg(presence_id);

-- name: CreateDocument :one
INSERT INTO documents (
  id, presence_id, medium, idle_custody, document_date, valid_until, notes
) VALUES (
  sqlc.arg(id), sqlc.arg(presence_id), sqlc.arg(medium), sqlc.narg(idle_custody),
  sqlc.narg(document_date), sqlc.narg(valid_until), sqlc.narg(notes)
)
RETURNING *;

-- name: GetDocumentByID :one
SELECT
  document.*,
  presence.profile_id AS owner_profile_id,
  owner.full_name AS owner_full_name,
  presence.document_type_id,
  presence.identifier_value,
  presence.claim AS presence_claim,
  presence.uniqueness_policy,
  document_type.technical_key AS type_technical_key,
  document_type.label AS type_label,
  document_type.active AS type_active,
  document_type.validation_regex AS type_validation_regex,
  document_type.date_required AS type_date_required,
  document_type.version AS type_version,
  document_type.created_at AS type_created_at,
  document_type.updated_at AS type_updated_at,
  document_current_use.holder_profile_id AS current_holder_profile_id,
  holder.full_name AS current_holder_full_name,
  document_current_use.assigned_at AS current_assigned_at
FROM documents AS document
JOIN document_presences AS presence ON presence.id = document.presence_id
JOIN profiles AS owner ON owner.id = presence.profile_id
JOIN document_types AS document_type ON document_type.id = presence.document_type_id
LEFT JOIN document_current_uses AS document_current_use ON document_current_use.document_id = document.id
LEFT JOIN profiles AS holder ON holder.id = document_current_use.holder_profile_id
WHERE document.id = sqlc.arg(id);

-- name: CountDocuments :one
SELECT count(*)
FROM documents AS document
JOIN document_presences AS presence ON presence.id = document.presence_id
LEFT JOIN document_current_uses AS document_current_use ON document_current_use.document_id = document.id
WHERE (sqlc.narg(owner_profile_id_filter)::uuid IS NULL OR presence.profile_id = sqlc.narg(owner_profile_id_filter)::uuid)
  AND (sqlc.narg(document_type_id_filter)::uuid IS NULL OR presence.document_type_id = sqlc.narg(document_type_id_filter)::uuid)
  AND (sqlc.arg(identifier_filter)::text = '' OR lower(COALESCE(presence.identifier_value, '')) LIKE '%' || lower(sqlc.arg(identifier_filter)::text) || '%')
  AND (sqlc.arg(medium_filter)::text = '' OR document.medium = sqlc.arg(medium_filter)::text)
  AND (sqlc.arg(status_filter)::text = '' OR
    (sqlc.arg(status_filter)::text = 'IN_USE' AND document.medium = 'PHYSICAL' AND document_current_use.document_id IS NOT NULL) OR
    (sqlc.arg(status_filter)::text = 'AVAILABLE' AND document.medium = 'PHYSICAL' AND document.idle_custody = 'ORGANIZATION' AND document_current_use.document_id IS NULL))
  AND (sqlc.narg(holder_profile_id_filter)::uuid IS NULL OR document_current_use.holder_profile_id = sqlc.narg(holder_profile_id_filter)::uuid);

-- name: ListDocuments :many
SELECT
  document.*,
  presence.profile_id AS owner_profile_id,
  owner.full_name AS owner_full_name,
  presence.document_type_id,
  presence.identifier_value,
  presence.claim AS presence_claim,
  presence.uniqueness_policy,
  document_type.technical_key AS type_technical_key,
  document_type.label AS type_label,
  document_type.active AS type_active,
  document_type.validation_regex AS type_validation_regex,
  document_type.date_required AS type_date_required,
  document_type.version AS type_version,
  document_type.created_at AS type_created_at,
  document_type.updated_at AS type_updated_at,
  document_current_use.holder_profile_id AS current_holder_profile_id,
  holder.full_name AS current_holder_full_name,
  document_current_use.assigned_at AS current_assigned_at
FROM documents AS document
JOIN document_presences AS presence ON presence.id = document.presence_id
JOIN profiles AS owner ON owner.id = presence.profile_id
JOIN document_types AS document_type ON document_type.id = presence.document_type_id
LEFT JOIN document_current_uses AS document_current_use ON document_current_use.document_id = document.id
LEFT JOIN profiles AS holder ON holder.id = document_current_use.holder_profile_id
WHERE (sqlc.narg(owner_profile_id_filter)::uuid IS NULL OR presence.profile_id = sqlc.narg(owner_profile_id_filter)::uuid)
  AND (sqlc.narg(document_type_id_filter)::uuid IS NULL OR presence.document_type_id = sqlc.narg(document_type_id_filter)::uuid)
  AND (sqlc.arg(identifier_filter)::text = '' OR lower(COALESCE(presence.identifier_value, '')) LIKE '%' || lower(sqlc.arg(identifier_filter)::text) || '%')
  AND (sqlc.arg(medium_filter)::text = '' OR document.medium = sqlc.arg(medium_filter)::text)
  AND (sqlc.arg(status_filter)::text = '' OR
    (sqlc.arg(status_filter)::text = 'IN_USE' AND document.medium = 'PHYSICAL' AND document_current_use.document_id IS NOT NULL) OR
    (sqlc.arg(status_filter)::text = 'AVAILABLE' AND document.medium = 'PHYSICAL' AND document.idle_custody = 'ORGANIZATION' AND document_current_use.document_id IS NULL))
  AND (sqlc.narg(holder_profile_id_filter)::uuid IS NULL OR document_current_use.holder_profile_id = sqlc.narg(holder_profile_id_filter)::uuid)
ORDER BY
  CASE WHEN sqlc.arg(sort_field)::text = 'identifier_value' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(presence.identifier_value) END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'identifier_value' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(presence.identifier_value) END DESC,
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
UPDATE documents
SET presence_id = sqlc.arg(presence_id), idle_custody = sqlc.narg(idle_custody),
  document_date = sqlc.narg(document_date), valid_until = sqlc.narg(valid_until),
  notes = sqlc.narg(notes), medium = sqlc.arg(medium),
  version = version + 1, updated_at = now()
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version)
RETURNING *;

-- name: DuplicateDocument :one
INSERT INTO documents (
  id, presence_id, medium, idle_custody, document_date, valid_until, notes
)
SELECT sqlc.arg(new_id), source.presence_id, source.medium, source.idle_custody,
  source.document_date, source.valid_until, source.notes
FROM documents AS source
WHERE source.id = sqlc.arg(source_id)
RETURNING *;

-- name: DeleteDocument :one
DELETE FROM documents WHERE id = sqlc.arg(id) AND version = sqlc.arg(version) RETURNING id;

-- name: DeleteDocumentCurrentUseForDelete :exec
DELETE FROM document_current_uses WHERE document_id = sqlc.arg(document_id);

-- name: AssignDocumentCurrentUse :one
INSERT INTO document_current_uses (document_id, holder_profile_id)
SELECT document.id, sqlc.arg(holder_profile_id)
FROM documents AS document
WHERE document.id = sqlc.arg(document_id) AND document.medium = 'PHYSICAL'
ON CONFLICT (document_id) DO UPDATE
SET holder_profile_id = EXCLUDED.holder_profile_id, assigned_at = now()
RETURNING *;

-- name: ReturnDocumentCurrentUse :one
DELETE FROM document_current_uses WHERE document_id = sqlc.arg(document_id) RETURNING document_id;

-- name: GetDocumentCurrentUse :one
SELECT * FROM document_current_uses WHERE document_id = sqlc.arg(document_id);

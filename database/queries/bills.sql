-- name: CreateBillType :one
INSERT INTO bill_types (
  id, technical_key, label, active
) VALUES (
  sqlc.arg(id), sqlc.arg(technical_key), sqlc.arg(label), sqlc.arg(active)
)
RETURNING *;

-- name: GetBillTypeByID :one
SELECT * FROM bill_types WHERE id = sqlc.arg(id);

-- name: CountBillTypes :one
SELECT count(*)
FROM bill_types
WHERE (sqlc.arg(label_filter)::text = '' OR lower(label) LIKE '%' || lower(sqlc.arg(label_filter)::text) || '%')
  AND (sqlc.arg(active_filter)::text = '' OR active = sqlc.arg(active_filter)::boolean);

-- name: ListBillTypes :many
SELECT *
FROM bill_types
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

-- name: CountBillsByType :many
SELECT bill.bill_type_id, count(*)::bigint AS bill_count
FROM bills AS bill
LEFT JOIN bill_current_uses AS current_use ON current_use.bill_id = bill.id
WHERE bill.medium = 'PHYSICAL'
  AND (bill.idle_custody = 'ORGANIZATION' OR current_use.bill_id IS NOT NULL)
GROUP BY bill.bill_type_id;

-- name: BillTypeHasBills :one
SELECT EXISTS(SELECT 1 FROM bills WHERE bill_type_id = sqlc.arg(bill_type_id));

-- name: UpdateBillType :one
UPDATE bill_types
SET label = sqlc.arg(label), active = sqlc.arg(active),
  version = version + 1, updated_at = now()
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version)
RETURNING *;

-- name: DeleteBillType :one
DELETE FROM bill_types WHERE id = sqlc.arg(id) AND version = sqlc.arg(version) RETURNING id;

-- name: CreateBill :one
INSERT INTO bills (
  id, owner_profile_id, bill_type_id, printed_holder_name, printed_address,
  reference_value, competence, amount, currency, notes, medium, idle_custody
)
SELECT sqlc.arg(id), sqlc.arg(owner_profile_id), bill_type.id, sqlc.narg(printed_holder_name),
  sqlc.narg(printed_address), sqlc.narg(reference_value), sqlc.narg(competence),
  sqlc.narg(amount), sqlc.narg(currency), sqlc.narg(notes), sqlc.arg(medium), sqlc.narg(idle_custody)
FROM bill_types AS bill_type
WHERE bill_type.id = sqlc.arg(bill_type_id) AND bill_type.active
RETURNING *;

-- name: GetBillByID :one
SELECT
  bill.*,
  owner.full_name AS owner_full_name,
  bill_type.technical_key AS type_technical_key,
  bill_type.label AS type_label,
  bill_type.active AS type_active,
  bill_type.version AS type_version,
  bill_type.created_at AS type_created_at,
  bill_type.updated_at AS type_updated_at,
  bill_current_use.holder_profile_id AS current_holder_profile_id,
  holder.full_name AS current_holder_full_name,
  bill_current_use.assigned_at AS current_assigned_at
FROM bills AS bill
JOIN profiles AS owner ON owner.id = bill.owner_profile_id
JOIN bill_types AS bill_type ON bill_type.id = bill.bill_type_id
LEFT JOIN bill_current_uses AS bill_current_use ON bill_current_use.bill_id = bill.id
LEFT JOIN profiles AS holder ON holder.id = bill_current_use.holder_profile_id
WHERE bill.id = sqlc.arg(id);

-- name: CountBills :one
SELECT count(*)
FROM bills AS bill
LEFT JOIN bill_current_uses AS bill_current_use ON bill_current_use.bill_id = bill.id
WHERE (sqlc.narg(owner_profile_id_filter)::uuid IS NULL OR bill.owner_profile_id = sqlc.narg(owner_profile_id_filter)::uuid)
  AND (sqlc.narg(bill_type_id_filter)::uuid IS NULL OR bill.bill_type_id = sqlc.narg(bill_type_id_filter)::uuid)
  AND (sqlc.arg(reference_filter)::text = '' OR lower(coalesce(bill.reference_value, '')) LIKE '%' || lower(sqlc.arg(reference_filter)::text) || '%')
  AND (sqlc.arg(competence_filter)::text = '' OR coalesce(bill.competence, '') = sqlc.arg(competence_filter)::text)
  AND (sqlc.arg(medium_filter)::text = '' OR bill.medium = sqlc.arg(medium_filter)::text)
  AND (sqlc.arg(status_filter)::text = '' OR
    (sqlc.arg(status_filter)::text = 'IN_USE' AND bill.medium = 'PHYSICAL' AND bill_current_use.bill_id IS NOT NULL) OR
    (sqlc.arg(status_filter)::text = 'AVAILABLE' AND bill.medium = 'PHYSICAL' AND bill.idle_custody = 'ORGANIZATION' AND bill_current_use.bill_id IS NULL))
  AND (sqlc.narg(holder_profile_id_filter)::uuid IS NULL OR bill_current_use.holder_profile_id = sqlc.narg(holder_profile_id_filter)::uuid);

-- name: ListBills :many
SELECT
  bill.*,
  owner.full_name AS owner_full_name,
  bill_type.technical_key AS type_technical_key,
  bill_type.label AS type_label,
  bill_type.active AS type_active,
  bill_type.version AS type_version,
  bill_type.created_at AS type_created_at,
  bill_type.updated_at AS type_updated_at,
  bill_current_use.holder_profile_id AS current_holder_profile_id,
  holder.full_name AS current_holder_full_name,
  bill_current_use.assigned_at AS current_assigned_at
FROM bills AS bill
JOIN profiles AS owner ON owner.id = bill.owner_profile_id
JOIN bill_types AS bill_type ON bill_type.id = bill.bill_type_id
LEFT JOIN bill_current_uses AS bill_current_use ON bill_current_use.bill_id = bill.id
LEFT JOIN profiles AS holder ON holder.id = bill_current_use.holder_profile_id
WHERE (sqlc.narg(owner_profile_id_filter)::uuid IS NULL OR bill.owner_profile_id = sqlc.narg(owner_profile_id_filter)::uuid)
  AND (sqlc.narg(bill_type_id_filter)::uuid IS NULL OR bill.bill_type_id = sqlc.narg(bill_type_id_filter)::uuid)
  AND (sqlc.arg(reference_filter)::text = '' OR lower(coalesce(bill.reference_value, '')) LIKE '%' || lower(sqlc.arg(reference_filter)::text) || '%')
  AND (sqlc.arg(competence_filter)::text = '' OR coalesce(bill.competence, '') = sqlc.arg(competence_filter)::text)
  AND (sqlc.arg(medium_filter)::text = '' OR bill.medium = sqlc.arg(medium_filter)::text)
  AND (sqlc.arg(status_filter)::text = '' OR
    (sqlc.arg(status_filter)::text = 'IN_USE' AND bill.medium = 'PHYSICAL' AND bill_current_use.bill_id IS NOT NULL) OR
    (sqlc.arg(status_filter)::text = 'AVAILABLE' AND bill.medium = 'PHYSICAL' AND bill.idle_custody = 'ORGANIZATION' AND bill_current_use.bill_id IS NULL))
  AND (sqlc.narg(holder_profile_id_filter)::uuid IS NULL OR bill_current_use.holder_profile_id = sqlc.narg(holder_profile_id_filter)::uuid)
ORDER BY
  CASE WHEN sqlc.arg(sort_field)::text = 'reference_value' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(bill.reference_value) END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'reference_value' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(bill.reference_value) END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'type_label' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(bill_type.label) END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'type_label' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(bill_type.label) END DESC,
  CASE WHEN sqlc.arg(sort_field)::text = 'competence' AND sqlc.arg(sort_order)::text = 'asc' THEN bill.competence END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'competence' AND sqlc.arg(sort_order)::text = 'desc' THEN bill.competence END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'amount' AND sqlc.arg(sort_order)::text = 'asc' THEN bill.amount END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'amount' AND sqlc.arg(sort_order)::text = 'desc' THEN bill.amount END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'created_at' AND sqlc.arg(sort_order)::text = 'asc' THEN bill.created_at END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'created_at' AND sqlc.arg(sort_order)::text = 'desc' THEN bill.created_at END DESC,
  CASE WHEN sqlc.arg(sort_field)::text = 'updated_at' AND sqlc.arg(sort_order)::text = 'asc' THEN bill.updated_at END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'updated_at' AND sqlc.arg(sort_order)::text = 'desc' THEN bill.updated_at END DESC,
  bill.id ASC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: UpdateBill :one
UPDATE bills AS bill
SET owner_profile_id = sqlc.arg(owner_profile_id), bill_type_id = bill_type.id,
  printed_holder_name = sqlc.narg(printed_holder_name), printed_address = sqlc.narg(printed_address),
  reference_value = sqlc.narg(reference_value), competence = sqlc.narg(competence),
  amount = sqlc.narg(amount), currency = sqlc.narg(currency), notes = sqlc.narg(notes),
  medium = sqlc.arg(medium), idle_custody = sqlc.narg(idle_custody),
  version = bill.version + 1, updated_at = now()
FROM bill_types AS bill_type
WHERE bill.id = sqlc.arg(id) AND bill.version = sqlc.arg(version)
  AND bill_type.id = sqlc.arg(bill_type_id) AND bill_type.active
RETURNING bill.*;

-- name: DuplicateBill :one
INSERT INTO bills (
  id, owner_profile_id, bill_type_id, printed_holder_name, printed_address,
  reference_value, competence, amount, currency, notes, medium, idle_custody
)
SELECT sqlc.arg(new_id), source.owner_profile_id, source.bill_type_id, source.printed_holder_name,
  source.printed_address, source.reference_value, source.competence, source.amount,
  source.currency, source.notes, source.medium, source.idle_custody
FROM bills AS source
WHERE source.id = sqlc.arg(source_id)
RETURNING *;

-- name: DeleteBill :one
DELETE FROM bills WHERE id = sqlc.arg(id) AND version = sqlc.arg(version) RETURNING id;

-- name: AssignBillCurrentUse :one
INSERT INTO bill_current_uses (bill_id, holder_profile_id)
SELECT bill.id, sqlc.arg(holder_profile_id)
FROM bills AS bill
WHERE bill.id = sqlc.arg(bill_id) AND bill.medium = 'PHYSICAL'
ON CONFLICT (bill_id) DO UPDATE
SET holder_profile_id = EXCLUDED.holder_profile_id, assigned_at = now()
RETURNING *;

-- name: ReturnBillCurrentUse :one
DELETE FROM bill_current_uses WHERE bill_id = sqlc.arg(bill_id) RETURNING bill_id;

-- name: GetBillCurrentUse :one
SELECT * FROM bill_current_uses WHERE bill_id = sqlc.arg(bill_id);

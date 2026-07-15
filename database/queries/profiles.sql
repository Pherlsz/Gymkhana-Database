-- name: CreateProfile :one
INSERT INTO profiles (
  id, full_name, social_name, cpf, email, mobile_phone, landline_phone,
  address_street, address_number, address_complement, address_neighborhood,
  address_city, address_state, address_postal_code, notes
) VALUES (
  sqlc.arg(id), sqlc.arg(full_name), sqlc.narg(social_name), sqlc.narg(cpf), sqlc.narg(email),
  sqlc.narg(mobile_phone), sqlc.narg(landline_phone), sqlc.narg(address_street),
  sqlc.narg(address_number), sqlc.narg(address_complement), sqlc.narg(address_neighborhood),
  sqlc.narg(address_city), sqlc.narg(address_state), sqlc.narg(address_postal_code), sqlc.narg(notes)
)
RETURNING *;

-- name: GetProfileByID :one
SELECT * FROM profiles WHERE id = sqlc.arg(id);

-- name: CountProfiles :one
SELECT count(*)
FROM profiles
WHERE (sqlc.arg(full_name_filter)::text = '' OR lower(full_name) LIKE '%' || lower(sqlc.arg(full_name_filter)::text) || '%')
  AND (sqlc.arg(cpf_filter)::text = '' OR coalesce(cpf, '') LIKE '%' || sqlc.arg(cpf_filter)::text || '%')
  AND (sqlc.arg(email_filter)::text = '' OR lower(coalesce(email, '')) LIKE '%' || lower(sqlc.arg(email_filter)::text) || '%')
  AND (sqlc.arg(city_filter)::text = '' OR lower(coalesce(address_city, '')) LIKE '%' || lower(sqlc.arg(city_filter)::text) || '%')
  AND (sqlc.arg(state_filter)::text = '' OR coalesce(address_state, '') = sqlc.arg(state_filter)::text);

-- name: ListProfiles :many
SELECT *
FROM profiles
WHERE (sqlc.arg(full_name_filter)::text = '' OR lower(full_name) LIKE '%' || lower(sqlc.arg(full_name_filter)::text) || '%')
  AND (sqlc.arg(cpf_filter)::text = '' OR coalesce(cpf, '') LIKE '%' || sqlc.arg(cpf_filter)::text || '%')
  AND (sqlc.arg(email_filter)::text = '' OR lower(coalesce(email, '')) LIKE '%' || lower(sqlc.arg(email_filter)::text) || '%')
  AND (sqlc.arg(city_filter)::text = '' OR lower(coalesce(address_city, '')) LIKE '%' || lower(sqlc.arg(city_filter)::text) || '%')
  AND (sqlc.arg(state_filter)::text = '' OR coalesce(address_state, '') = sqlc.arg(state_filter)::text)
ORDER BY
  CASE WHEN sqlc.arg(sort_field)::text = 'full_name' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(full_name) END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'full_name' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(full_name) END DESC,
  CASE WHEN sqlc.arg(sort_field)::text = 'cpf' AND sqlc.arg(sort_order)::text = 'asc' THEN cpf END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'cpf' AND sqlc.arg(sort_order)::text = 'desc' THEN cpf END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'email' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(email) END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'email' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(email) END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'address_city' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(address_city) END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'address_city' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(address_city) END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'created_at' AND sqlc.arg(sort_order)::text = 'asc' THEN created_at END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'created_at' AND sqlc.arg(sort_order)::text = 'desc' THEN created_at END DESC,
  CASE WHEN sqlc.arg(sort_field)::text = 'updated_at' AND sqlc.arg(sort_order)::text = 'asc' THEN updated_at END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'updated_at' AND sqlc.arg(sort_order)::text = 'desc' THEN updated_at END DESC,
  id ASC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: UpdateProfile :one
UPDATE profiles
SET full_name = sqlc.arg(full_name), social_name = sqlc.narg(social_name), cpf = sqlc.narg(cpf),
  email = sqlc.narg(email), mobile_phone = sqlc.narg(mobile_phone), landline_phone = sqlc.narg(landline_phone),
  address_street = sqlc.narg(address_street), address_number = sqlc.narg(address_number),
  address_complement = sqlc.narg(address_complement), address_neighborhood = sqlc.narg(address_neighborhood),
  address_city = sqlc.narg(address_city), address_state = sqlc.narg(address_state),
  address_postal_code = sqlc.narg(address_postal_code), notes = sqlc.narg(notes),
  version = version + 1, updated_at = now()
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version)
RETURNING *;

-- name: DuplicateProfile :one
INSERT INTO profiles (
  id, full_name, social_name, cpf, email, mobile_phone, landline_phone,
  address_street, address_number, address_complement, address_neighborhood,
  address_city, address_state, address_postal_code, notes
)
SELECT sqlc.arg(new_id), source.full_name, source.social_name, source.cpf, source.email,
  source.mobile_phone, source.landline_phone, source.address_street, source.address_number,
  source.address_complement, source.address_neighborhood, source.address_city, source.address_state,
  source.address_postal_code, source.notes
FROM profiles AS source
WHERE source.id = sqlc.arg(source_id)
RETURNING *;

-- name: DeleteProfile :one
DELETE FROM profiles WHERE id = sqlc.arg(id) AND version = sqlc.arg(version) RETURNING id;

-- name: RecordProfileAuditEvent :exec
INSERT INTO profile_audit_events (id, actor_user_id, profile_id, source_profile_id, event_type, outcome, request_id)
VALUES (sqlc.arg(id), sqlc.arg(actor_user_id), sqlc.arg(profile_id), sqlc.narg(source_profile_id), sqlc.arg(event_type), sqlc.arg(outcome), sqlc.arg(request_id));

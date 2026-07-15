-- name: CreateProfile :one
INSERT INTO profiles (
  id,
  full_name,
  social_name,
  cpf,
  email,
  mobile_phone,
  landline_phone,
  address_street,
  address_number,
  address_complement,
  address_neighborhood,
  address_city,
  address_state,
  address_postal_code,
  notes
) VALUES (
  sqlc.arg(id),
  sqlc.arg(full_name),
  sqlc.narg(social_name),
  sqlc.narg(cpf),
  sqlc.narg(email),
  sqlc.narg(mobile_phone),
  sqlc.narg(landline_phone),
  sqlc.narg(address_street),
  sqlc.narg(address_number),
  sqlc.narg(address_complement),
  sqlc.narg(address_neighborhood),
  sqlc.narg(address_city),
  sqlc.narg(address_state),
  sqlc.narg(address_postal_code),
  sqlc.narg(notes)
)
RETURNING *;

-- name: GetProfileByID :one
SELECT *
FROM profiles
WHERE id = sqlc.arg(id);

-- name: CountProfiles :one
SELECT count(*)
FROM profiles;

-- name: ListProfiles :many
SELECT *
FROM profiles
ORDER BY lower(full_name), id
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: UpdateProfile :one
UPDATE profiles
SET
  full_name = sqlc.arg(full_name),
  social_name = sqlc.narg(social_name),
  cpf = sqlc.narg(cpf),
  email = sqlc.narg(email),
  mobile_phone = sqlc.narg(mobile_phone),
  landline_phone = sqlc.narg(landline_phone),
  address_street = sqlc.narg(address_street),
  address_number = sqlc.narg(address_number),
  address_complement = sqlc.narg(address_complement),
  address_neighborhood = sqlc.narg(address_neighborhood),
  address_city = sqlc.narg(address_city),
  address_state = sqlc.narg(address_state),
  address_postal_code = sqlc.narg(address_postal_code),
  notes = sqlc.narg(notes),
  version = version + 1,
  updated_at = now()
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(version)
RETURNING *;

-- name: DuplicateProfile :one
INSERT INTO profiles (
  id,
  full_name,
  social_name,
  cpf,
  email,
  mobile_phone,
  landline_phone,
  address_street,
  address_number,
  address_complement,
  address_neighborhood,
  address_city,
  address_state,
  address_postal_code,
  notes
)
SELECT
  sqlc.arg(new_id),
  source.full_name,
  source.social_name,
  source.cpf,
  source.email,
  source.mobile_phone,
  source.landline_phone,
  source.address_street,
  source.address_number,
  source.address_complement,
  source.address_neighborhood,
  source.address_city,
  source.address_state,
  source.address_postal_code,
  source.notes
FROM profiles AS source
WHERE source.id = sqlc.arg(source_id)
RETURNING *;

-- name: DeleteProfile :one
DELETE FROM profiles
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(version)
RETURNING id;

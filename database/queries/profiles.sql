-- name: CreateProfile :one
INSERT INTO profiles (
  id, full_name, social_name, email, mobile_phone, landline_phone,
  address_street, address_number, address_complement, address_neighborhood,
  address_city, address_state, address_postal_code, notes,
  birth_date, gender, blood_type, nationality, birth_city, marital_status, wedding_date,
  father_name, father_birth_date, mother_name, mother_birth_date, health_plan, blood_donor, organ_donor,
  team, sector, collections, vehicle_model, vehicle_color, vehicle_plate, vehicle_year, club_membership,
  membership_type, place_of_origin, birth_country, parents_wedding_date, supermarket_club, pet,
  travel_countries, card_brand, card_bank
) VALUES (
  sqlc.arg(id), sqlc.arg(full_name), sqlc.narg(social_name), sqlc.narg(email),
  sqlc.narg(mobile_phone), sqlc.narg(landline_phone), sqlc.narg(address_street),
  sqlc.narg(address_number), sqlc.narg(address_complement), sqlc.narg(address_neighborhood),
  sqlc.narg(address_city), sqlc.narg(address_state), sqlc.narg(address_postal_code), sqlc.narg(notes),
  sqlc.narg(birth_date), sqlc.narg(gender), sqlc.narg(blood_type), sqlc.narg(nationality),
  sqlc.narg(birth_city), sqlc.narg(marital_status), sqlc.narg(wedding_date),
  sqlc.narg(father_name), sqlc.narg(father_birth_date), sqlc.narg(mother_name), sqlc.narg(mother_birth_date),
  sqlc.narg(health_plan), sqlc.narg(blood_donor), sqlc.narg(organ_donor),
  sqlc.narg(team), sqlc.narg(sector), sqlc.narg(collections), sqlc.narg(vehicle_model),
  sqlc.narg(vehicle_color), sqlc.narg(vehicle_plate), sqlc.narg(vehicle_year), sqlc.narg(club_membership),
  sqlc.narg(membership_type), sqlc.narg(place_of_origin), sqlc.narg(birth_country),
  sqlc.narg(parents_wedding_date), sqlc.narg(supermarket_club), sqlc.narg(pet),
  sqlc.narg(travel_countries), sqlc.narg(card_brand), sqlc.narg(card_bank)
)
RETURNING *;

-- name: GetProfileByID :one
SELECT * FROM profiles WHERE id = sqlc.arg(id);

-- name: CountProfiles :one
SELECT count(*)
FROM profiles
WHERE (sqlc.arg(full_name_filter)::text = '' OR lower(full_name) LIKE '%' || lower(sqlc.arg(full_name_filter)::text) || '%')
  AND (sqlc.arg(cpf_filter)::text = '' OR EXISTS (
    SELECT 1
    FROM document_presences AS presence
    JOIN document_types AS document_type ON document_type.id = presence.document_type_id
    WHERE presence.profile_id = profiles.id
      AND document_type.technical_key = 'cpf'
      AND presence.claim = 'informed_number'
      AND coalesce(presence.identifier_digits, presence.identifier_value, '') LIKE '%' || sqlc.arg(cpf_filter)::text || '%'
  ))
  AND (sqlc.arg(email_filter)::text = '' OR lower(coalesce(email, '')) LIKE '%' || lower(sqlc.arg(email_filter)::text) || '%')
  AND (sqlc.arg(city_filter)::text = '' OR lower(coalesce(address_city, '')) LIKE '%' || lower(sqlc.arg(city_filter)::text) || '%')
  AND (sqlc.arg(state_filter)::text = '' OR coalesce(address_state, '') = sqlc.arg(state_filter)::text)
  AND (NOT sqlc.arg(restrict_ids)::bool OR id = ANY(sqlc.arg(id_filter)::uuid[]));

-- name: ListProfiles :many
SELECT *
FROM profiles
WHERE (sqlc.arg(full_name_filter)::text = '' OR lower(full_name) LIKE '%' || lower(sqlc.arg(full_name_filter)::text) || '%')
  AND (sqlc.arg(cpf_filter)::text = '' OR EXISTS (
    SELECT 1
    FROM document_presences AS presence
    JOIN document_types AS document_type ON document_type.id = presence.document_type_id
    WHERE presence.profile_id = profiles.id
      AND document_type.technical_key = 'cpf'
      AND presence.claim = 'informed_number'
      AND coalesce(presence.identifier_digits, presence.identifier_value, '') LIKE '%' || sqlc.arg(cpf_filter)::text || '%'
  ))
  AND (sqlc.arg(email_filter)::text = '' OR lower(coalesce(email, '')) LIKE '%' || lower(sqlc.arg(email_filter)::text) || '%')
  AND (sqlc.arg(city_filter)::text = '' OR lower(coalesce(address_city, '')) LIKE '%' || lower(sqlc.arg(city_filter)::text) || '%')
  AND (sqlc.arg(state_filter)::text = '' OR coalesce(address_state, '') = sqlc.arg(state_filter)::text)
  AND (NOT sqlc.arg(restrict_ids)::bool OR id = ANY(sqlc.arg(id_filter)::uuid[]))
ORDER BY
  CASE
    WHEN sqlc.arg(full_name_filter)::text <> '' THEN
      CASE
        WHEN lower(full_name) = lower(sqlc.arg(full_name_filter)::text) THEN 0
        WHEN lower(full_name) LIKE lower(sqlc.arg(full_name_filter)::text) || ' %' THEN 1
        WHEN lower(full_name) LIKE lower(sqlc.arg(full_name_filter)::text) || '%' THEN 2
        WHEN lower(full_name) LIKE '% ' || lower(sqlc.arg(full_name_filter)::text) || ' %' THEN 3
        WHEN lower(full_name) LIKE '% ' || lower(sqlc.arg(full_name_filter)::text) || '%' THEN 4
        ELSE 5
      END
    ELSE 0
  END ASC,
  (CASE WHEN sqlc.arg(sort_field)::text = 'full_name' AND sqlc.arg(sort_order)::text = 'asc' THEN full_name END) COLLATE gymkhana_pt_br ASC,
  (CASE WHEN sqlc.arg(sort_field)::text = 'full_name' AND sqlc.arg(sort_order)::text = 'desc' THEN full_name END) COLLATE gymkhana_pt_br DESC,
  CASE WHEN sqlc.arg(sort_field)::text = 'cpf' AND sqlc.arg(sort_order)::text = 'asc' THEN (
    SELECT presence.identifier_digits
    FROM document_presences AS presence
    JOIN document_types AS document_type ON document_type.id = presence.document_type_id
    WHERE presence.profile_id = profiles.id AND document_type.technical_key = 'cpf'
    LIMIT 1
  ) END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'cpf' AND sqlc.arg(sort_order)::text = 'desc' THEN (
    SELECT presence.identifier_digits
    FROM document_presences AS presence
    JOIN document_types AS document_type ON document_type.id = presence.document_type_id
    WHERE presence.profile_id = profiles.id AND document_type.technical_key = 'cpf'
    LIMIT 1
  ) END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'email' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(email) END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'email' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(email) END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'address_city' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(address_city) END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'address_city' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(address_city) END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'address_street' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(coalesce(address_street, '')) END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'address_street' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(coalesce(address_street, '')) END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'address_neighborhood' AND sqlc.arg(sort_order)::text = 'asc' THEN lower(coalesce(address_neighborhood, '')) END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'address_neighborhood' AND sqlc.arg(sort_order)::text = 'desc' THEN lower(coalesce(address_neighborhood, '')) END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'mobile_phone' AND sqlc.arg(sort_order)::text = 'asc' THEN mobile_phone END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'mobile_phone' AND sqlc.arg(sort_order)::text = 'desc' THEN mobile_phone END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'birth_date' AND sqlc.arg(sort_order)::text = 'asc' THEN birth_date END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'birth_date' AND sqlc.arg(sort_order)::text = 'desc' THEN birth_date END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_field)::text = 'created_at' AND sqlc.arg(sort_order)::text = 'asc' THEN created_at END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'created_at' AND sqlc.arg(sort_order)::text = 'desc' THEN created_at END DESC,
  CASE WHEN sqlc.arg(sort_field)::text = 'updated_at' AND sqlc.arg(sort_order)::text = 'asc' THEN updated_at END ASC,
  CASE WHEN sqlc.arg(sort_field)::text = 'updated_at' AND sqlc.arg(sort_order)::text = 'desc' THEN updated_at END DESC,
  id ASC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: UpdateProfile :one
UPDATE profiles
SET full_name = sqlc.arg(full_name), social_name = sqlc.narg(social_name),
  email = sqlc.narg(email), mobile_phone = sqlc.narg(mobile_phone), landline_phone = sqlc.narg(landline_phone),
  address_street = sqlc.narg(address_street), address_number = sqlc.narg(address_number),
  address_complement = sqlc.narg(address_complement), address_neighborhood = sqlc.narg(address_neighborhood),
  address_city = sqlc.narg(address_city), address_state = sqlc.narg(address_state),
  address_postal_code = sqlc.narg(address_postal_code), notes = sqlc.narg(notes),
  birth_date = sqlc.narg(birth_date), gender = sqlc.narg(gender), blood_type = sqlc.narg(blood_type),
  nationality = sqlc.narg(nationality), birth_city = sqlc.narg(birth_city),
  marital_status = sqlc.narg(marital_status), wedding_date = sqlc.narg(wedding_date),
  father_name = sqlc.narg(father_name), father_birth_date = sqlc.narg(father_birth_date),
  mother_name = sqlc.narg(mother_name), mother_birth_date = sqlc.narg(mother_birth_date),
  health_plan = sqlc.narg(health_plan), blood_donor = sqlc.narg(blood_donor), organ_donor = sqlc.narg(organ_donor),
  team = sqlc.narg(team), sector = sqlc.narg(sector), collections = sqlc.narg(collections),
  vehicle_model = sqlc.narg(vehicle_model), vehicle_color = sqlc.narg(vehicle_color),
  vehicle_plate = sqlc.narg(vehicle_plate), vehicle_year = sqlc.narg(vehicle_year),
  club_membership = sqlc.narg(club_membership), membership_type = sqlc.narg(membership_type),
  place_of_origin = sqlc.narg(place_of_origin), birth_country = sqlc.narg(birth_country),
  parents_wedding_date = sqlc.narg(parents_wedding_date), supermarket_club = sqlc.narg(supermarket_club),
  pet = sqlc.narg(pet), travel_countries = sqlc.narg(travel_countries),
  card_brand = sqlc.narg(card_brand), card_bank = sqlc.narg(card_bank),
  version = version + 1, updated_at = now()
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version)
RETURNING *;

-- name: DuplicateProfile :one
INSERT INTO profiles (
  id, full_name, social_name, email, mobile_phone, landline_phone,
  address_street, address_number, address_complement, address_neighborhood,
  address_city, address_state, address_postal_code, notes,
  birth_date, gender, blood_type, nationality, birth_city, marital_status, wedding_date,
  father_name, father_birth_date, mother_name, mother_birth_date, health_plan, blood_donor, organ_donor,
  team, sector, collections, vehicle_model, vehicle_color, vehicle_plate, vehicle_year, club_membership,
  membership_type, place_of_origin, birth_country, parents_wedding_date, supermarket_club, pet,
  travel_countries, card_brand, card_bank
)
SELECT sqlc.arg(new_id), source.full_name, source.social_name, source.email,
  source.mobile_phone, source.landline_phone, source.address_street, source.address_number,
  source.address_complement, source.address_neighborhood, source.address_city, source.address_state,
  source.address_postal_code, source.notes,
  source.birth_date, source.gender, source.blood_type, source.nationality, source.birth_city, source.marital_status, source.wedding_date,
  source.father_name, source.father_birth_date, source.mother_name, source.mother_birth_date, source.health_plan, source.blood_donor, source.organ_donor,
  source.team, source.sector, source.collections, source.vehicle_model, source.vehicle_color, source.vehicle_plate, source.vehicle_year, source.club_membership,
  source.membership_type, source.place_of_origin, source.birth_country, source.parents_wedding_date,
  source.supermarket_club, source.pet, source.travel_countries, source.card_brand, source.card_bank
FROM profiles AS source
WHERE source.id = sqlc.arg(source_id)
RETURNING *;

-- name: DeleteProfile :one
DELETE FROM profiles WHERE id = sqlc.arg(id) AND version = sqlc.arg(version) RETURNING id;

-- name: RecordProfileAuditEvent :exec
INSERT INTO profile_audit_events (id, actor_user_id, profile_id, source_profile_id, event_type, outcome, request_id)
VALUES (sqlc.arg(id), sqlc.arg(actor_user_id), sqlc.arg(profile_id), sqlc.narg(source_profile_id), sqlc.arg(event_type), sqlc.arg(outcome), sqlc.arg(request_id));

-- name: ListProfilesByExactFullName :many
SELECT *
FROM profiles
WHERE lower(full_name) = lower(btrim(sqlc.arg(full_name)))
ORDER BY id;

-- name: ListDistinctCities :many
SELECT DISTINCT address_city
FROM profiles
WHERE address_city IS NOT NULL
  AND address_city <> ''
  AND (sqlc.arg(full_name_filter)::text = '' OR lower(full_name) LIKE '%' || lower(sqlc.arg(full_name_filter)::text) || '%')
  AND (sqlc.arg(cpf_filter)::text = '' OR EXISTS (
    SELECT 1
    FROM document_presences AS presence
    JOIN document_types AS document_type ON document_type.id = presence.document_type_id
    WHERE presence.profile_id = profiles.id
      AND document_type.technical_key = 'cpf'
      AND presence.claim = 'informed_number'
      AND coalesce(presence.identifier_digits, presence.identifier_value, '') LIKE '%' || sqlc.arg(cpf_filter)::text || '%'
  ))
  AND (sqlc.arg(email_filter)::text = '' OR lower(coalesce(email, '')) LIKE '%' || lower(sqlc.arg(email_filter)::text) || '%')
  AND (sqlc.arg(state_filter)::text = '' OR coalesce(address_state, '') = sqlc.arg(state_filter)::text)
ORDER BY address_city COLLATE gymkhana_pt_br
LIMIT sqlc.arg(value_limit);

-- name: GetDocumentTypeByTechnicalKey :one
SELECT * FROM document_types WHERE technical_key = sqlc.arg(technical_key);

-- name: UpsertCPFPresence :one
INSERT INTO document_presences (
  id, profile_id, document_type_id, uniqueness_policy, claim, identifier_value
)
SELECT sqlc.arg(id), sqlc.arg(profile_id), document_type.id, document_type.uniqueness_policy,
  'informed_number', sqlc.arg(identifier_value)
FROM document_types AS document_type
WHERE document_type.technical_key = 'cpf'
ON CONFLICT (profile_id, document_type_id) DO UPDATE
SET claim = 'informed_number',
  identifier_value = EXCLUDED.identifier_value,
  uniqueness_policy = EXCLUDED.uniqueness_policy,
  version = document_presences.version + 1,
  updated_at = now()
RETURNING *;

-- name: ClearCPFPresenceNumber :exec
UPDATE document_presences AS presence
SET claim = 'indication', identifier_value = NULL, version = presence.version + 1, updated_at = now()
FROM document_types AS document_type
WHERE document_type.id = presence.document_type_id
  AND document_type.technical_key = 'cpf'
  AND presence.profile_id = sqlc.arg(profile_id)
  AND presence.claim = 'informed_number'
  AND NOT EXISTS (SELECT 1 FROM documents WHERE presence_id = presence.id);

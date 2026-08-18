-- Canonical Profile columns kept in the unified archive sheet.
-- Naturalidade is distinct from birth_city. Card brand/bank only — never PAN.

ALTER TABLE profiles
  ADD COLUMN IF NOT EXISTS place_of_origin TEXT,
  ADD COLUMN IF NOT EXISTS birth_country TEXT,
  ADD COLUMN IF NOT EXISTS parents_wedding_date DATE,
  ADD COLUMN IF NOT EXISTS supermarket_club TEXT,
  ADD COLUMN IF NOT EXISTS pet TEXT,
  ADD COLUMN IF NOT EXISTS travel_countries TEXT,
  ADD COLUMN IF NOT EXISTS card_brand TEXT,
  ADD COLUMN IF NOT EXISTS card_bank TEXT;

CREATE INDEX IF NOT EXISTS profiles_place_of_origin_trgm_index
  ON profiles USING gin (lower(place_of_origin) gin_trgm_ops)
  WHERE place_of_origin IS NOT NULL;
CREATE INDEX IF NOT EXISTS profiles_birth_country_trgm_index
  ON profiles USING gin (lower(birth_country) gin_trgm_ops)
  WHERE birth_country IS NOT NULL;
CREATE INDEX IF NOT EXISTS profiles_supermarket_club_index
  ON profiles (lower(supermarket_club), id)
  WHERE supermarket_club IS NOT NULL;

WITH profile_fields(id, technical_key, label, field_kind, maximum_length) AS (
  VALUES
    ('024f0024-91e7-57eb-954b-000000000024'::uuid, 'place_of_origin', 'Naturalidade', 'TEXT', 100),
    ('024f0025-91e7-57eb-954b-000000000025'::uuid, 'birth_country', 'País de nascimento', 'TEXT', 80),
    ('024f0026-91e7-57eb-954b-000000000026'::uuid, 'parents_wedding_date', 'Casamento dos pais', 'CIVIL_DATE', NULL::integer),
    ('024f0027-91e7-57eb-954b-000000000027'::uuid, 'supermarket_club', 'Clube de supermercado', 'TEXT', 80),
    ('024f0028-91e7-57eb-954b-000000000028'::uuid, 'pet', 'Animal', 'TEXT', 120),
    ('024f0029-91e7-57eb-954b-000000000029'::uuid, 'travel_countries', 'Viagem', 'TEXT', 500),
    ('024f0030-91e7-57eb-954b-000000000030'::uuid, 'card_brand', 'Bandeira do cartão', 'TEXT', 120),
    ('024f0031-91e7-57eb-954b-000000000031'::uuid, 'card_bank', 'Banco do cartão', 'TEXT', 120)
)
INSERT INTO custom_field_definitions (
  id, target_kind, document_type_id, bill_type_id, custom_entity_type_id,
  technical_key, label, field_kind, required, active,
  minimum_length, maximum_length, validation_regex, minimum_decimal, maximum_decimal
)
SELECT
  profile_fields.id, 'PROFILE', NULL, NULL, NULL,
  profile_fields.technical_key, profile_fields.label, profile_fields.field_kind,
  false, true, NULL, profile_fields.maximum_length, NULL, NULL, NULL
FROM profile_fields
ON CONFLICT (technical_key) WHERE target_kind = 'PROFILE'
DO UPDATE SET
  label = EXCLUDED.label, field_kind = EXCLUDED.field_kind, active = true,
  maximum_length = EXCLUDED.maximum_length, updated_at = now();

INSERT INTO app_metadata (key, value)
VALUES ('schema.archive_profile_fields', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.archive_profile_fields';
DELETE FROM custom_field_definitions WHERE id IN (
  '024f0024-91e7-57eb-954b-000000000024'::uuid,
  '024f0025-91e7-57eb-954b-000000000025'::uuid,
  '024f0026-91e7-57eb-954b-000000000026'::uuid,
  '024f0027-91e7-57eb-954b-000000000027'::uuid,
  '024f0028-91e7-57eb-954b-000000000028'::uuid,
  '024f0029-91e7-57eb-954b-000000000029'::uuid,
  '024f0030-91e7-57eb-954b-000000000030'::uuid,
  '024f0031-91e7-57eb-954b-000000000031'::uuid
);
DROP INDEX IF EXISTS profiles_supermarket_club_index;
DROP INDEX IF EXISTS profiles_birth_country_trgm_index;
DROP INDEX IF EXISTS profiles_place_of_origin_trgm_index;
ALTER TABLE profiles
  DROP COLUMN IF EXISTS card_bank,
  DROP COLUMN IF EXISTS card_brand,
  DROP COLUMN IF EXISTS travel_countries,
  DROP COLUMN IF EXISTS pet,
  DROP COLUMN IF EXISTS supermarket_club,
  DROP COLUMN IF EXISTS parents_wedding_date,
  DROP COLUMN IF EXISTS birth_country,
  DROP COLUMN IF EXISTS place_of_origin;

-- Persist document presence separately from exemplars, move CPF off profiles,
-- absorb profile_details into profiles, and add idle_custody on physical rows.

ALTER TABLE profiles
  ADD COLUMN IF NOT EXISTS birth_date DATE,
  ADD COLUMN IF NOT EXISTS gender TEXT,
  ADD COLUMN IF NOT EXISTS blood_type TEXT,
  ADD COLUMN IF NOT EXISTS nationality TEXT,
  ADD COLUMN IF NOT EXISTS birth_city TEXT,
  ADD COLUMN IF NOT EXISTS marital_status TEXT,
  ADD COLUMN IF NOT EXISTS wedding_date DATE,
  ADD COLUMN IF NOT EXISTS father_name TEXT,
  ADD COLUMN IF NOT EXISTS father_birth_date DATE,
  ADD COLUMN IF NOT EXISTS mother_name TEXT,
  ADD COLUMN IF NOT EXISTS mother_birth_date DATE,
  ADD COLUMN IF NOT EXISTS health_plan TEXT,
  ADD COLUMN IF NOT EXISTS blood_donor BOOLEAN,
  ADD COLUMN IF NOT EXISTS organ_donor BOOLEAN,
  ADD COLUMN IF NOT EXISTS team TEXT,
  ADD COLUMN IF NOT EXISTS sector TEXT,
  ADD COLUMN IF NOT EXISTS collections TEXT,
  ADD COLUMN IF NOT EXISTS vehicle_model TEXT,
  ADD COLUMN IF NOT EXISTS vehicle_color TEXT,
  ADD COLUMN IF NOT EXISTS vehicle_plate TEXT,
  ADD COLUMN IF NOT EXISTS vehicle_year INTEGER,
  ADD COLUMN IF NOT EXISTS club_membership TEXT,
  ADD COLUMN IF NOT EXISTS membership_type TEXT;

ALTER TABLE profiles DROP CONSTRAINT IF EXISTS profiles_vehicle_year_check;
ALTER TABLE profiles ADD CONSTRAINT profiles_vehicle_year_check
  CHECK (vehicle_year IS NULL OR vehicle_year BETWEEN 1886 AND 9999);

UPDATE profiles AS profile
SET
  birth_date = COALESCE(profile.birth_date, details.birth_date),
  gender = COALESCE(profile.gender, details.gender),
  blood_type = COALESCE(profile.blood_type, details.blood_type),
  nationality = COALESCE(profile.nationality, details.nationality),
  birth_city = COALESCE(profile.birth_city, details.birth_city),
  marital_status = COALESCE(profile.marital_status, details.marital_status),
  wedding_date = COALESCE(profile.wedding_date, details.wedding_date),
  father_name = COALESCE(profile.father_name, details.father_name),
  father_birth_date = COALESCE(profile.father_birth_date, details.father_birth_date),
  mother_name = COALESCE(profile.mother_name, details.mother_name),
  mother_birth_date = COALESCE(profile.mother_birth_date, details.mother_birth_date),
  health_plan = COALESCE(profile.health_plan, details.health_plan),
  blood_donor = COALESCE(profile.blood_donor, details.blood_donor),
  organ_donor = COALESCE(profile.organ_donor, details.organ_donor),
  team = COALESCE(profile.team, details.team),
  sector = COALESCE(profile.sector, details.sector),
  collections = COALESCE(profile.collections, details.collections),
  vehicle_model = COALESCE(profile.vehicle_model, details.vehicle_model),
  vehicle_color = COALESCE(profile.vehicle_color, details.vehicle_color),
  vehicle_plate = COALESCE(profile.vehicle_plate, details.vehicle_plate),
  vehicle_year = COALESCE(profile.vehicle_year, details.vehicle_year),
  club_membership = COALESCE(profile.club_membership, details.club_membership),
  membership_type = COALESCE(profile.membership_type, details.membership_type)
FROM profile_details AS details
WHERE details.profile_id = profile.id;

DROP TABLE IF EXISTS profile_details;

CREATE INDEX IF NOT EXISTS profiles_team_index
  ON profiles (lower(team), id) WHERE team IS NOT NULL;
CREATE INDEX IF NOT EXISTS profiles_vehicle_plate_index
  ON profiles (upper(vehicle_plate), id) WHERE vehicle_plate IS NOT NULL;
CREATE INDEX IF NOT EXISTS profiles_mother_name_trgm_index
  ON profiles USING gin (lower(mother_name) gin_trgm_ops)
  WHERE mother_name IS NOT NULL;
CREATE INDEX IF NOT EXISTS profiles_father_name_trgm_index
  ON profiles USING gin (lower(father_name) gin_trgm_ops)
  WHERE father_name IS NOT NULL;
CREATE INDEX IF NOT EXISTS profiles_birth_city_trgm_index
  ON profiles USING gin (lower(birth_city) gin_trgm_ops)
  WHERE birth_city IS NOT NULL;
CREATE INDEX IF NOT EXISTS profiles_team_trgm_index
  ON profiles USING gin (lower(team) gin_trgm_ops)
  WHERE team IS NOT NULL;

INSERT INTO document_types (id, technical_key, label, active, uniqueness_policy, validation_regex, date_required)
VALUES (
  'b7de3989-9d10-5071-b913-778836542d3d'::uuid,
  'cpf', 'CPF', true, 'PER_PROFILE', '^[0-9]{11}$', false
)
ON CONFLICT (technical_key) DO UPDATE
SET label = EXCLUDED.label, active = true, uniqueness_policy = EXCLUDED.uniqueness_policy,
  validation_regex = EXCLUDED.validation_regex, date_required = EXCLUDED.date_required, updated_at = now();

CREATE TABLE document_presences (
  id UUID PRIMARY KEY,
  profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE RESTRICT,
  document_type_id UUID NOT NULL REFERENCES document_types(id) ON DELETE RESTRICT,
  uniqueness_policy TEXT NOT NULL CHECK (uniqueness_policy IN ('NONE', 'PER_PROFILE', 'GLOBAL_BY_TYPE')),
  claim TEXT NOT NULL CHECK (claim IN ('absence', 'indication', 'informed_number')),
  identifier_value TEXT CHECK (
    identifier_value IS NULL
    OR (identifier_value = btrim(identifier_value) AND identifier_value <> '' AND char_length(identifier_value) <= 500)
  ),
  identifier_digits TEXT GENERATED ALWAYS AS (
    NULLIF(regexp_replace(COALESCE(identifier_value, ''), '[^0-9]', '', 'g'), '')
  ) STORED,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (profile_id, document_type_id),
  CHECK ((claim = 'informed_number') = (identifier_value IS NOT NULL))
);

INSERT INTO document_presences (
  id, profile_id, document_type_id, uniqueness_policy, claim, identifier_value, created_at, updated_at
)
SELECT gen_random_uuid(), source.owner_profile_id, source.document_type_id, source.uniqueness_policy,
  CASE WHEN source.identifier_value IS NOT NULL THEN 'informed_number' ELSE 'indication' END,
  source.identifier_value, source.created_at, source.updated_at
FROM (
  SELECT DISTINCT ON (owner_profile_id, document_type_id)
    owner_profile_id, document_type_id, uniqueness_policy, identifier_value, created_at, updated_at
  FROM documents
  ORDER BY owner_profile_id, document_type_id,
    (identifier_value IS NOT NULL) DESC, updated_at DESC, id DESC
) AS source;

INSERT INTO document_presences (
  id, profile_id, document_type_id, uniqueness_policy, claim, identifier_value
)
SELECT gen_random_uuid(), profile.id, document_type.id, document_type.uniqueness_policy,
  'informed_number', profile.cpf
FROM profiles AS profile
JOIN document_types AS document_type ON document_type.technical_key = 'cpf'
WHERE profile.cpf IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM document_presences AS presence
    WHERE presence.profile_id = profile.id
      AND presence.document_type_id = document_type.id
  );

ALTER TABLE documents
  ADD COLUMN IF NOT EXISTS presence_id UUID REFERENCES document_presences(id) ON DELETE RESTRICT,
  ADD COLUMN IF NOT EXISTS idle_custody TEXT,
  ADD COLUMN IF NOT EXISTS valid_until DATE;

UPDATE documents AS document
SET presence_id = presence.id
FROM document_presences AS presence
WHERE presence.profile_id = document.owner_profile_id
  AND presence.document_type_id = document.document_type_id
  AND document.presence_id IS NULL;

-- If an older 026 nulled every medium, restore the 025 default so exemplars
-- are not deleted. Digital vs physical cannot be recovered after that 026.
UPDATE documents SET medium = 'PHYSICAL'
WHERE medium IS NULL
  AND (
    identifier_value IS NOT NULL
    OR document_date IS NOT NULL
    OR notes IS NOT NULL
    OR id IN (SELECT document_id FROM document_current_uses)
  );

DELETE FROM document_current_uses
WHERE document_id IN (SELECT id FROM documents WHERE medium IS NULL OR presence_id IS NULL);

WITH doomed AS (
  SELECT document.id
  FROM documents AS document
  WHERE document.medium IS NULL OR document.presence_id IS NULL
), keeper AS (
  SELECT DISTINCT ON (presence_id)
    id, presence_id
  FROM documents
  WHERE medium IS NOT NULL AND presence_id IS NOT NULL
  ORDER BY presence_id, updated_at DESC, id DESC
)
UPDATE custom_field_values AS value
SET document_id = keeper.id
FROM doomed
JOIN keeper ON keeper.presence_id = (SELECT presence_id FROM documents WHERE id = doomed.id)
WHERE value.document_id = doomed.id
  AND NOT EXISTS (
    SELECT 1 FROM custom_field_values AS existing
    WHERE existing.document_id = keeper.id AND existing.field_definition_id = value.field_definition_id
  );

DELETE FROM documents WHERE medium IS NULL OR presence_id IS NULL;

DELETE FROM document_current_uses
WHERE document_id IN (
  SELECT extra.id
  FROM (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY presence_id, medium ORDER BY updated_at DESC, id DESC) AS row_number
    FROM documents
  ) AS extra
  WHERE extra.row_number > 1
);

DELETE FROM documents
WHERE id IN (
  SELECT extra.id
  FROM (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY presence_id, medium ORDER BY updated_at DESC, id DESC) AS row_number
    FROM documents
  ) AS extra
  WHERE extra.row_number > 1
);

UPDATE documents SET idle_custody = 'ORGANIZATION' WHERE medium = 'PHYSICAL' AND idle_custody IS NULL;
UPDATE documents SET idle_custody = NULL WHERE medium = 'DIGITAL';

ALTER TABLE documents ALTER COLUMN presence_id SET NOT NULL;
ALTER TABLE documents ALTER COLUMN medium SET NOT NULL;

ALTER TABLE documents DROP CONSTRAINT IF EXISTS documents_medium_idle_custody_check;
ALTER TABLE documents ADD CONSTRAINT documents_medium_idle_custody_check CHECK (
  (medium = 'PHYSICAL' AND idle_custody IN ('ORGANIZATION', 'OWNER'))
  OR (medium = 'DIGITAL' AND idle_custody IS NULL)
);

DROP INDEX IF EXISTS documents_per_profile_unique;
DROP INDEX IF EXISTS documents_global_by_type_unique;
DROP INDEX IF EXISTS documents_known_number_per_profile;
DROP INDEX IF EXISTS documents_known_number_global;
DROP INDEX IF EXISTS documents_missing_number_physical;
DROP INDEX IF EXISTS documents_missing_number_digital;
DROP INDEX IF EXISTS documents_missing_number_unspecified;
DROP INDEX IF EXISTS documents_identifier_search_index;
DROP INDEX IF EXISTS documents_identifier_trgm_index;
DROP INDEX IF EXISTS documents_owner_index;
DROP INDEX IF EXISTS documents_type_index;

ALTER TABLE documents
  DROP COLUMN IF EXISTS owner_profile_id,
  DROP COLUMN IF EXISTS document_type_id,
  DROP COLUMN IF EXISTS identifier_value,
  DROP COLUMN IF EXISTS uniqueness_policy;

CREATE UNIQUE INDEX documents_presence_medium_unique ON documents (presence_id, medium);
CREATE INDEX documents_presence_index ON documents (presence_id, updated_at DESC, id);
CREATE INDEX documents_idle_custody_index ON documents (idle_custody, id) WHERE medium = 'PHYSICAL';

CREATE UNIQUE INDEX document_presences_global_identifier_unique
  ON document_presences (document_type_id, identifier_digits)
  WHERE uniqueness_policy = 'GLOBAL_BY_TYPE' AND identifier_digits IS NOT NULL;
CREATE INDEX document_presences_profile_index ON document_presences (profile_id, updated_at DESC, id);
CREATE INDEX document_presences_type_index ON document_presences (document_type_id, id);
CREATE INDEX document_presences_identifier_digits_index
  ON document_presences (document_type_id, identifier_digits)
  WHERE identifier_digits IS NOT NULL;
CREATE INDEX document_presences_identifier_search_index
  ON document_presences (lower(identifier_value) text_pattern_ops, id)
  WHERE identifier_value IS NOT NULL;
CREATE INDEX document_presences_identifier_trgm_index
  ON document_presences USING gin (lower(identifier_value) gin_trgm_ops)
  WHERE identifier_value IS NOT NULL;

DROP INDEX IF EXISTS profiles_cpf_index;
ALTER TABLE profiles DROP COLUMN IF EXISTS cpf;

UPDATE bills SET medium = 'PHYSICAL' WHERE medium IS NULL;
ALTER TABLE bills ALTER COLUMN medium SET NOT NULL;
ALTER TABLE bills ADD COLUMN IF NOT EXISTS idle_custody TEXT;
UPDATE bills SET idle_custody = 'ORGANIZATION' WHERE medium = 'PHYSICAL' AND idle_custody IS NULL;
UPDATE bills SET idle_custody = NULL WHERE medium = 'DIGITAL';
ALTER TABLE bills DROP CONSTRAINT IF EXISTS bills_medium_idle_custody_check;
ALTER TABLE bills ADD CONSTRAINT bills_medium_idle_custody_check CHECK (
  (medium = 'PHYSICAL' AND idle_custody IN ('ORGANIZATION', 'OWNER'))
  OR (medium = 'DIGITAL' AND idle_custody IS NULL)
);
CREATE INDEX IF NOT EXISTS bills_idle_custody_index ON bills (idle_custody, id) WHERE medium = 'PHYSICAL';

INSERT INTO app_metadata (key, value)
VALUES ('schema.document_presences', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.document_presences';

DROP INDEX IF EXISTS bills_idle_custody_index;
ALTER TABLE bills DROP CONSTRAINT IF EXISTS bills_medium_idle_custody_check;
ALTER TABLE bills DROP COLUMN IF EXISTS idle_custody;
ALTER TABLE bills ALTER COLUMN medium DROP NOT NULL;

ALTER TABLE profiles ADD COLUMN IF NOT EXISTS cpf TEXT;
ALTER TABLE profiles DROP CONSTRAINT IF EXISTS profiles_cpf_check;
ALTER TABLE profiles ADD CONSTRAINT profiles_cpf_check CHECK (cpf IS NULL OR cpf ~ '^[0-9]{11}$');
CREATE INDEX IF NOT EXISTS profiles_cpf_index ON profiles (cpf) WHERE cpf IS NOT NULL;

UPDATE profiles AS profile
SET cpf = presence.identifier_digits
FROM document_presences AS presence
JOIN document_types AS document_type ON document_type.id = presence.document_type_id
WHERE presence.profile_id = profile.id
  AND document_type.technical_key = 'cpf'
  AND presence.claim = 'informed_number'
  AND presence.identifier_digits ~ '^[0-9]{11}$';

ALTER TABLE documents ADD COLUMN IF NOT EXISTS owner_profile_id UUID REFERENCES profiles(id) ON DELETE RESTRICT;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS document_type_id UUID REFERENCES document_types(id) ON DELETE RESTRICT;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS identifier_value TEXT;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS uniqueness_policy TEXT;

UPDATE documents AS document
SET owner_profile_id = presence.profile_id,
  document_type_id = presence.document_type_id,
  identifier_value = presence.identifier_value,
  uniqueness_policy = presence.uniqueness_policy
FROM document_presences AS presence
WHERE presence.id = document.presence_id;

DROP INDEX IF EXISTS documents_presence_medium_unique;
DROP INDEX IF EXISTS documents_presence_index;
DROP INDEX IF EXISTS documents_idle_custody_index;
ALTER TABLE documents DROP CONSTRAINT IF EXISTS documents_medium_idle_custody_check;
ALTER TABLE documents ALTER COLUMN medium DROP NOT NULL;
ALTER TABLE documents DROP COLUMN IF EXISTS valid_until;
ALTER TABLE documents DROP COLUMN IF EXISTS idle_custody;
ALTER TABLE documents DROP COLUMN IF EXISTS presence_id;

DROP INDEX IF EXISTS document_presences_identifier_trgm_index;
DROP INDEX IF EXISTS document_presences_identifier_search_index;
DROP INDEX IF EXISTS document_presences_identifier_digits_index;
DROP INDEX IF EXISTS document_presences_type_index;
DROP INDEX IF EXISTS document_presences_profile_index;
DROP INDEX IF EXISTS document_presences_global_identifier_unique;
DROP TABLE IF EXISTS document_presences;

CREATE UNIQUE INDEX IF NOT EXISTS documents_per_profile_unique
  ON documents (owner_profile_id, document_type_id, identifier_value, medium)
  WHERE uniqueness_policy = 'PER_PROFILE';
CREATE UNIQUE INDEX IF NOT EXISTS documents_global_by_type_unique
  ON documents (document_type_id, identifier_value, medium)
  WHERE uniqueness_policy = 'GLOBAL_BY_TYPE';
CREATE UNIQUE INDEX IF NOT EXISTS documents_known_number_per_profile
  ON documents (owner_profile_id, document_type_id, identifier_value)
  WHERE uniqueness_policy = 'PER_PROFILE' AND medium IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS documents_known_number_global
  ON documents (document_type_id, identifier_value)
  WHERE uniqueness_policy = 'GLOBAL_BY_TYPE' AND medium IS NULL;
CREATE INDEX IF NOT EXISTS documents_owner_index ON documents (owner_profile_id, updated_at DESC, id);
CREATE INDEX IF NOT EXISTS documents_type_index ON documents (document_type_id, identifier_value, id);
CREATE INDEX IF NOT EXISTS documents_identifier_search_index
  ON documents (lower(identifier_value) text_pattern_ops, id);
CREATE INDEX IF NOT EXISTS documents_identifier_trgm_index
  ON documents USING gin (lower(identifier_value) gin_trgm_ops);

CREATE TABLE IF NOT EXISTS profile_details (
  profile_id UUID PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
  birth_date DATE,
  gender TEXT,
  nationality TEXT,
  birth_city TEXT,
  marital_status TEXT,
  wedding_date DATE,
  father_name TEXT,
  father_birth_date DATE,
  mother_name TEXT,
  mother_birth_date DATE,
  health_plan TEXT,
  blood_donor BOOLEAN,
  organ_donor BOOLEAN,
  team TEXT,
  sector TEXT,
  collections TEXT,
  vehicle_model TEXT,
  vehicle_color TEXT,
  vehicle_plate TEXT,
  vehicle_year INTEGER CHECK (vehicle_year IS NULL OR vehicle_year BETWEEN 1886 AND 9999),
  club_membership TEXT,
  blood_type TEXT,
  membership_type TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO profile_details (
  profile_id, birth_date, gender, blood_type, nationality, birth_city, marital_status, wedding_date,
  father_name, father_birth_date, mother_name, mother_birth_date, health_plan, blood_donor, organ_donor,
  team, sector, collections, vehicle_model, vehicle_color, vehicle_plate, vehicle_year, club_membership,
  membership_type
)
SELECT id, birth_date, gender, blood_type, nationality, birth_city, marital_status, wedding_date,
  father_name, father_birth_date, mother_name, mother_birth_date, health_plan, blood_donor, organ_donor,
  team, sector, collections, vehicle_model, vehicle_color, vehicle_plate, vehicle_year, club_membership,
  membership_type
FROM profiles
ON CONFLICT (profile_id) DO NOTHING;

DROP INDEX IF EXISTS profiles_team_trgm_index;
DROP INDEX IF EXISTS profiles_birth_city_trgm_index;
DROP INDEX IF EXISTS profiles_father_name_trgm_index;
DROP INDEX IF EXISTS profiles_mother_name_trgm_index;
DROP INDEX IF EXISTS profiles_vehicle_plate_index;
DROP INDEX IF EXISTS profiles_team_index;
ALTER TABLE profiles DROP CONSTRAINT IF EXISTS profiles_vehicle_year_check;
ALTER TABLE profiles
  DROP COLUMN IF EXISTS membership_type,
  DROP COLUMN IF EXISTS club_membership,
  DROP COLUMN IF EXISTS vehicle_year,
  DROP COLUMN IF EXISTS vehicle_plate,
  DROP COLUMN IF EXISTS vehicle_color,
  DROP COLUMN IF EXISTS vehicle_model,
  DROP COLUMN IF EXISTS collections,
  DROP COLUMN IF EXISTS sector,
  DROP COLUMN IF EXISTS team,
  DROP COLUMN IF EXISTS organ_donor,
  DROP COLUMN IF EXISTS blood_donor,
  DROP COLUMN IF EXISTS health_plan,
  DROP COLUMN IF EXISTS mother_birth_date,
  DROP COLUMN IF EXISTS mother_name,
  DROP COLUMN IF EXISTS father_birth_date,
  DROP COLUMN IF EXISTS father_name,
  DROP COLUMN IF EXISTS wedding_date,
  DROP COLUMN IF EXISTS marital_status,
  DROP COLUMN IF EXISTS birth_city,
  DROP COLUMN IF EXISTS nationality,
  DROP COLUMN IF EXISTS blood_type,
  DROP COLUMN IF EXISTS gender,
  DROP COLUMN IF EXISTS birth_date;

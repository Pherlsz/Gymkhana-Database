-- Canonical one-to-one Profile extension for mapped legacy personal data.
-- These fields are not custom data and remain owned by the Profile aggregate.

ALTER TABLE profiles DROP CONSTRAINT profiles_mobile_phone_check;
ALTER TABLE profiles ADD CONSTRAINT profiles_mobile_phone_check
  CHECK (mobile_phone IS NULL OR mobile_phone ~ '^\+55[0-9]{10,11}$');

CREATE TABLE profile_details (
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
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX profile_details_team_index ON profile_details (lower(team), profile_id) WHERE team IS NOT NULL;
CREATE INDEX profile_details_vehicle_plate_index ON profile_details (upper(vehicle_plate), profile_id) WHERE vehicle_plate IS NOT NULL;

INSERT INTO app_metadata (key, value)
VALUES ('schema.profile_details', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.profile_details';
DROP TABLE profile_details;
ALTER TABLE profiles DROP CONSTRAINT profiles_mobile_phone_check;
ALTER TABLE profiles ADD CONSTRAINT profiles_mobile_phone_check
  CHECK (mobile_phone IS NULL OR mobile_phone ~ '^\+55[0-9]{11}$');

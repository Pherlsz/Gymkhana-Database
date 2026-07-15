-- M3 canonical physical-person Profile persistence.

CREATE TABLE profiles (
  id UUID PRIMARY KEY,
  full_name TEXT NOT NULL CHECK (
    full_name = btrim(full_name)
    AND full_name <> ''
    AND char_length(full_name) <= 200
  ),
  social_name TEXT CHECK (
    social_name IS NULL
    OR (social_name = btrim(social_name) AND social_name <> '' AND char_length(social_name) <= 200)
  ),
  cpf TEXT CHECK (cpf IS NULL OR cpf ~ '^[0-9]{11}$'),
  email TEXT CHECK (
    email IS NULL
    OR (email = btrim(email) AND email = lower(email) AND email <> '' AND char_length(email) <= 320)
  ),
  mobile_phone TEXT CHECK (mobile_phone IS NULL OR mobile_phone ~ '^\+55[0-9]{11}$'),
  landline_phone TEXT CHECK (landline_phone IS NULL OR landline_phone ~ '^\+55[0-9]{10}$'),
  address_street TEXT CHECK (
    address_street IS NULL
    OR (address_street = btrim(address_street) AND address_street <> '' AND char_length(address_street) <= 200)
  ),
  address_number TEXT CHECK (
    address_number IS NULL
    OR (address_number = btrim(address_number) AND address_number <> '' AND char_length(address_number) <= 30)
  ),
  address_complement TEXT CHECK (
    address_complement IS NULL
    OR (address_complement = btrim(address_complement) AND address_complement <> '' AND char_length(address_complement) <= 100)
  ),
  address_neighborhood TEXT CHECK (
    address_neighborhood IS NULL
    OR (address_neighborhood = btrim(address_neighborhood) AND address_neighborhood <> '' AND char_length(address_neighborhood) <= 100)
  ),
  address_city TEXT CHECK (
    address_city IS NULL
    OR (address_city = btrim(address_city) AND address_city <> '' AND char_length(address_city) <= 100)
  ),
  address_state TEXT CHECK (address_state IS NULL OR address_state ~ '^[A-Z]{2}$'),
  address_postal_code TEXT CHECK (address_postal_code IS NULL OR address_postal_code ~ '^[0-9]{8}$'),
  notes TEXT CHECK (notes IS NULL OR (notes <> '' AND char_length(notes) <= 5000)),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX profiles_full_name_search_index
  ON profiles (lower(full_name) text_pattern_ops, id);

CREATE INDEX profiles_cpf_index
  ON profiles (cpf)
  WHERE cpf IS NOT NULL;

CREATE INDEX profiles_email_index
  ON profiles (email)
  WHERE email IS NOT NULL;

CREATE INDEX profiles_updated_at_index
  ON profiles (updated_at DESC, id);

INSERT INTO app_metadata (key, value)
VALUES ('schema.profiles', 'm3.1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.profiles';
DROP TABLE profiles;

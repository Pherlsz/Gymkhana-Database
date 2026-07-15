CREATE TABLE app_metadata (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE app_users (
  id UUID PRIMARY KEY,
  github_user_id BIGINT NOT NULL UNIQUE CHECK (github_user_id > 0),
  github_login TEXT NOT NULL CHECK (github_login = btrim(github_login) AND github_login <> ''),
  display_name TEXT NOT NULL CHECK (display_name = btrim(display_name) AND display_name <> ''),
  avatar_url TEXT,
  role TEXT NOT NULL CHECK (role IN ('MEMBER', 'ADMIN', 'SUPERADMIN')),
  active BOOLEAN NOT NULL DEFAULT true,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX app_users_github_login_ci_unique
  ON app_users (lower(github_login));

CREATE UNIQUE INDEX app_users_one_active_superadmin
  ON app_users (role)
  WHERE active AND role = 'SUPERADMIN';

CREATE TABLE app_sessions (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE RESTRICT,
  token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at TIMESTAMPTZ,
  CHECK (expires_at > created_at),
  CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX app_sessions_user_id_index ON app_sessions (user_id);
CREATE INDEX app_sessions_expiration_index ON app_sessions (expires_at) WHERE revoked_at IS NULL;

CREATE TABLE auth_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  subject_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL CHECK (event_type <> '' AND char_length(event_type) <= 100),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  request_id TEXT NOT NULL CHECK (request_id <> '' AND char_length(request_id) <= 128),
  provider_login TEXT,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX auth_audit_events_occurred_at_index ON auth_audit_events (occurred_at DESC);
CREATE INDEX auth_audit_events_actor_index ON auth_audit_events (actor_user_id, occurred_at DESC);
CREATE INDEX auth_audit_events_subject_index ON auth_audit_events (subject_user_id, occurred_at DESC);

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

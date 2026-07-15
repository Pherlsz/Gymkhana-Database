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
CREATE UNIQUE INDEX app_users_github_login_ci_unique ON app_users (lower(github_login));
CREATE UNIQUE INDEX app_users_one_active_superadmin ON app_users (role) WHERE active AND role = 'SUPERADMIN';

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
  full_name TEXT NOT NULL CHECK (full_name = btrim(full_name) AND full_name <> '' AND char_length(full_name) <= 200),
  social_name TEXT CHECK (social_name IS NULL OR (social_name = btrim(social_name) AND social_name <> '' AND char_length(social_name) <= 200)),
  cpf TEXT CHECK (cpf IS NULL OR cpf ~ '^[0-9]{11}$'),
  email TEXT CHECK (email IS NULL OR (email = btrim(email) AND email = lower(email) AND email <> '' AND char_length(email) <= 320)),
  mobile_phone TEXT CHECK (mobile_phone IS NULL OR mobile_phone ~ '^\+55[0-9]{11}$'),
  landline_phone TEXT CHECK (landline_phone IS NULL OR landline_phone ~ '^\+55[0-9]{10}$'),
  address_street TEXT CHECK (address_street IS NULL OR (address_street = btrim(address_street) AND address_street <> '' AND char_length(address_street) <= 200)),
  address_number TEXT CHECK (address_number IS NULL OR (address_number = btrim(address_number) AND address_number <> '' AND char_length(address_number) <= 30)),
  address_complement TEXT CHECK (address_complement IS NULL OR (address_complement = btrim(address_complement) AND address_complement <> '' AND char_length(address_complement) <= 100)),
  address_neighborhood TEXT CHECK (address_neighborhood IS NULL OR (address_neighborhood = btrim(address_neighborhood) AND address_neighborhood <> '' AND char_length(address_neighborhood) <= 100)),
  address_city TEXT CHECK (address_city IS NULL OR (address_city = btrim(address_city) AND address_city <> '' AND char_length(address_city) <= 100)),
  address_state TEXT CHECK (address_state IS NULL OR address_state ~ '^[A-Z]{2}$'),
  address_postal_code TEXT CHECK (address_postal_code IS NULL OR address_postal_code ~ '^[0-9]{8}$'),
  notes TEXT CHECK (notes IS NULL OR (notes <> '' AND char_length(notes) <= 5000)),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX profiles_full_name_search_index ON profiles (lower(full_name) text_pattern_ops, id);
CREATE INDEX profiles_cpf_index ON profiles (cpf) WHERE cpf IS NOT NULL;
CREATE INDEX profiles_email_index ON profiles (email) WHERE email IS NOT NULL;
CREATE INDEX profiles_updated_at_index ON profiles (updated_at DESC, id);

CREATE TABLE profile_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  profile_id UUID NOT NULL,
  source_profile_id UUID,
  event_type TEXT NOT NULL CHECK (event_type IN ('PROFILE_CREATED', 'PROFILE_UPDATED', 'PROFILE_DUPLICATED', 'PROFILE_DELETED')),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  request_id TEXT NOT NULL CHECK (request_id <> '' AND char_length(request_id) <= 128),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX profile_audit_events_occurred_at_index ON profile_audit_events (occurred_at DESC);
CREATE INDEX profile_audit_events_actor_index ON profile_audit_events (actor_user_id, occurred_at DESC);
CREATE INDEX profile_audit_events_profile_index ON profile_audit_events (profile_id, occurred_at DESC);

CREATE TABLE document_types (
  id UUID PRIMARY KEY,
  technical_key TEXT NOT NULL UNIQUE CHECK (technical_key ~ '^[a-z][a-z0-9_]{1,63}$'),
  label TEXT NOT NULL CHECK (label = btrim(label) AND label <> '' AND char_length(label) <= 120),
  active BOOLEAN NOT NULL DEFAULT true,
  uniqueness_policy TEXT NOT NULL CHECK (uniqueness_policy IN ('NONE', 'PER_PROFILE', 'GLOBAL_BY_TYPE')),
  validation_regex TEXT CHECK (validation_regex IS NULL OR (validation_regex = btrim(validation_regex) AND validation_regex <> '' AND char_length(validation_regex) <= 500)),
  date_required BOOLEAN NOT NULL DEFAULT false,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX document_types_label_index ON document_types (lower(label), id);
CREATE INDEX document_types_active_index ON document_types (active, lower(label), id);

CREATE TABLE documents (
  id UUID PRIMARY KEY,
  owner_profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE RESTRICT,
  document_type_id UUID NOT NULL REFERENCES document_types(id) ON DELETE RESTRICT,
  identifier_value TEXT NOT NULL CHECK (identifier_value = btrim(identifier_value) AND identifier_value <> '' AND char_length(identifier_value) <= 500),
  uniqueness_policy TEXT NOT NULL CHECK (uniqueness_policy IN ('NONE', 'PER_PROFILE', 'GLOBAL_BY_TYPE')),
  document_date DATE,
  notes TEXT CHECK (notes IS NULL OR (notes <> '' AND char_length(notes) <= 5000)),
  record_state TEXT NOT NULL DEFAULT 'CURRENT' CHECK (record_state IN ('CURRENT', 'REPLACED', 'EXPIRED', 'ARCHIVED')),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX documents_per_profile_unique ON documents (owner_profile_id, document_type_id, identifier_value) WHERE uniqueness_policy = 'PER_PROFILE';
CREATE UNIQUE INDEX documents_global_by_type_unique ON documents (document_type_id, identifier_value) WHERE uniqueness_policy = 'GLOBAL_BY_TYPE';
CREATE INDEX documents_owner_index ON documents (owner_profile_id, updated_at DESC, id);
CREATE INDEX documents_type_index ON documents (document_type_id, identifier_value, id);
CREATE INDEX documents_identifier_search_index ON documents (lower(identifier_value) text_pattern_ops, id);

CREATE TABLE document_current_uses (
  document_id UUID PRIMARY KEY REFERENCES documents(id) ON DELETE RESTRICT,
  holder_profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE RESTRICT,
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX document_current_uses_holder_index ON document_current_uses (holder_profile_id, assigned_at DESC);

CREATE TABLE bill_types (
  id UUID PRIMARY KEY,
  technical_key TEXT NOT NULL UNIQUE CHECK (technical_key ~ '^[a-z][a-z0-9_]{1,63}$'),
  label TEXT NOT NULL CHECK (label = btrim(label) AND label <> '' AND char_length(label) <= 120),
  active BOOLEAN NOT NULL DEFAULT true,
  supports_current_use BOOLEAN NOT NULL DEFAULT false,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX bill_types_label_index ON bill_types (lower(label), id);
CREATE INDEX bill_types_active_index ON bill_types (active, lower(label), id);

CREATE TABLE bills (
  id UUID PRIMARY KEY,
  owner_profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE RESTRICT,
  bill_type_id UUID NOT NULL REFERENCES bill_types(id) ON DELETE RESTRICT,
  printed_holder_name TEXT CHECK (printed_holder_name IS NULL OR (printed_holder_name = btrim(printed_holder_name) AND printed_holder_name <> '' AND char_length(printed_holder_name) <= 200)),
  printed_address TEXT CHECK (printed_address IS NULL OR (printed_address = btrim(printed_address) AND printed_address <> '' AND char_length(printed_address) <= 500)),
  reference_value TEXT CHECK (reference_value IS NULL OR (reference_value = btrim(reference_value) AND reference_value <> '' AND char_length(reference_value) <= 500)),
  competence TEXT CHECK (competence IS NULL OR competence ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
  amount NUMERIC(18,2) CHECK (amount IS NULL OR amount >= 0),
  currency TEXT CHECK (currency IS NULL OR currency ~ '^[A-Z]{3}$'),
  notes TEXT CHECK (notes IS NULL OR (notes <> '' AND char_length(notes) <= 5000)),
  record_state TEXT NOT NULL DEFAULT 'CURRENT' CHECK (record_state IN ('CURRENT', 'REPLACED', 'EXPIRED', 'ARCHIVED')),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((amount IS NULL AND currency IS NULL) OR (amount IS NOT NULL AND currency IS NOT NULL))
);
CREATE INDEX bills_owner_index ON bills (owner_profile_id, updated_at DESC, id);
CREATE INDEX bills_type_index ON bills (bill_type_id, competence DESC, id);
CREATE INDEX bills_reference_search_index ON bills (lower(reference_value) text_pattern_ops, id) WHERE reference_value IS NOT NULL;

CREATE TABLE bill_current_uses (
  bill_id UUID PRIMARY KEY REFERENCES bills(id) ON DELETE RESTRICT,
  holder_profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE RESTRICT,
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX bill_current_uses_holder_index ON bill_current_uses (holder_profile_id, assigned_at DESC);

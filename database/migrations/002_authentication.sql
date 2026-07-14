-- M2 authentication persistence foundations.

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

INSERT INTO app_metadata (key, value)
VALUES ('schema.authentication', 'm2.1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.authentication';
DROP TABLE auth_audit_events;
DROP TABLE app_sessions;
DROP TABLE app_users;

-- M9 owner-scoped Google Forms connections and source-neutral response staging.

ALTER TABLE operation_imports
  ALTER COLUMN original_filename DROP NOT NULL,
  ALTER COLUMN declared_size DROP NOT NULL,
  ALTER COLUMN object_key DROP NOT NULL,
  ADD COLUMN source_reference_id UUID;

ALTER TABLE operation_import_cells
  ADD COLUMN source_validation_code TEXT CHECK (
    source_validation_code IS NULL OR char_length(source_validation_code) BETWEEN 1 AND 80
  );

CREATE TABLE google_forms_oauth_states (
  state_hash BYTEA PRIMARY KEY CHECK (octet_length(state_hash) = 32),
  owner_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  session_id UUID NOT NULL REFERENCES app_sessions(id) ON DELETE CASCADE,
  verifier_ciphertext BYTEA NOT NULL CHECK (octet_length(verifier_ciphertext) BETWEEN 32 AND 512),
  verifier_nonce BYTEA NOT NULL CHECK (octet_length(verifier_nonce) = 12),
	token_key_version INTEGER NOT NULL CHECK (token_key_version BETWEEN 1 AND 65535),
	return_path TEXT NOT NULL DEFAULT '/google-forms' CHECK (
		char_length(return_path) BETWEEN 1 AND 500
		AND return_path ~ '^/[A-Za-z0-9/_?=&.%-]*$'
		AND return_path !~ '^//'
	),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  CHECK (expires_at > created_at),
  CHECK (consumed_at IS NULL OR consumed_at >= created_at)
);

CREATE INDEX google_forms_oauth_states_expiry_index
  ON google_forms_oauth_states (expires_at)
  WHERE consumed_at IS NULL;

CREATE TABLE google_forms_connections (
  id UUID PRIMARY KEY,
  owner_user_id UUID NOT NULL UNIQUE REFERENCES app_users(id) ON DELETE RESTRICT,
  state TEXT NOT NULL CHECK (state IN ('ACTIVE', 'NEEDS_REAUTH', 'DISCONNECTED')),
  refresh_token_ciphertext BYTEA,
  refresh_token_nonce BYTEA,
  token_key_version INTEGER CHECK (token_key_version BETWEEN 1 AND 65535),
  granted_scopes TEXT[] NOT NULL DEFAULT '{}',
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  connected_at TIMESTAMPTZ,
  disconnected_at TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (id, owner_user_id),
  CHECK (
    (state = 'ACTIVE'
      AND refresh_token_ciphertext IS NOT NULL
      AND octet_length(refresh_token_ciphertext) BETWEEN 16 AND 4096
      AND refresh_token_nonce IS NOT NULL
      AND octet_length(refresh_token_nonce) = 12
      AND token_key_version IS NOT NULL
      AND connected_at IS NOT NULL
      AND disconnected_at IS NULL
      AND cardinality(granted_scopes) = 2)
    OR
    (state IN ('NEEDS_REAUTH', 'DISCONNECTED')
      AND refresh_token_ciphertext IS NULL
      AND refresh_token_nonce IS NULL
      AND token_key_version IS NULL)
  )
);

CREATE TABLE google_forms_sources (
  id UUID PRIMARY KEY,
  connection_id UUID NOT NULL,
  owner_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE RESTRICT,
  provider_form_id TEXT NOT NULL CHECK (provider_form_id ~ '^[A-Za-z0-9_-]{10,200}$'),
  title TEXT NOT NULL CHECK (title = btrim(title) AND char_length(title) BETWEEN 1 AND 500),
  module TEXT NOT NULL CHECK (module IN ('PROFILES', 'DOCUMENTS', 'BILLS')),
  state TEXT NOT NULL CHECK (state IN ('DRAFT', 'ACTIVE', 'PAUSED', 'SCHEMA_DRIFT', 'NEEDS_REAUTH', 'ERROR')),
  schema_revision TEXT NOT NULL CHECK (char_length(schema_revision) BETWEEN 1 AND 500),
  schema_fingerprint BYTEA NOT NULL CHECK (octet_length(schema_fingerprint) = 32),
  sync_mode TEXT NOT NULL DEFAULT 'MANUAL' CHECK (sync_mode IN ('MANUAL', 'POLL')),
  poll_interval_seconds INTEGER NOT NULL DEFAULT 900 CHECK (poll_interval_seconds BETWEEN 300 AND 86400),
  cursor_submitted_at TIMESTAMPTZ,
  response_page_token TEXT CHECK (
    response_page_token IS NULL OR char_length(response_page_token) BETWEEN 1 AND 2048
  ),
  page_token_cursor_started_at TIMESTAMPTZ,
  last_synced_at TIMESTAMPTZ,
  next_sync_at TIMESTAMPTZ,
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (owner_user_id, provider_form_id),
  UNIQUE (id, owner_user_id),
  FOREIGN KEY (connection_id, owner_user_id)
    REFERENCES google_forms_connections(id, owner_user_id) ON DELETE RESTRICT,
  CHECK (
    (sync_mode = 'MANUAL' AND next_sync_at IS NULL)
    OR sync_mode = 'POLL'
  ),
  CHECK ((response_page_token IS NULL) = (page_token_cursor_started_at IS NULL))
);

CREATE INDEX google_forms_sources_owner_created_index
  ON google_forms_sources (owner_user_id, created_at DESC, id DESC);
CREATE INDEX google_forms_sources_due_index
  ON google_forms_sources (next_sync_at, id)
  WHERE state = 'ACTIVE' AND sync_mode = 'POLL' AND next_sync_at IS NOT NULL;

CREATE TABLE google_forms_questions (
  source_id UUID NOT NULL REFERENCES google_forms_sources(id) ON DELETE CASCADE,
  question_id TEXT NOT NULL CHECK (char_length(question_id) BETWEEN 1 AND 300),
  position INTEGER NOT NULL CHECK (position BETWEEN 0 AND 255),
  title TEXT NOT NULL CHECK (title = btrim(title) AND char_length(title) BETWEEN 1 AND 500),
  answer_kind TEXT NOT NULL CHECK (answer_kind IN ('TEXT', 'DATE', 'TIME', 'SCALE', 'UNSUPPORTED')),
  required BOOLEAN NOT NULL DEFAULT false,
  supported BOOLEAN NOT NULL,
  unsupported_code TEXT CHECK (unsupported_code IS NULL OR char_length(unsupported_code) BETWEEN 1 AND 80),
  question_fingerprint BYTEA NOT NULL CHECK (octet_length(question_fingerprint) = 32),
  target_field TEXT CHECK (target_field IS NULL OR char_length(target_field) BETWEEN 1 AND 120),
  PRIMARY KEY (source_id, question_id),
  UNIQUE (source_id, position),
  CHECK (supported = (answer_kind <> 'UNSUPPORTED')),
  CHECK ((supported AND unsupported_code IS NULL) OR (NOT supported AND unsupported_code IS NOT NULL))
);

CREATE UNIQUE INDEX google_forms_questions_target_unique
  ON google_forms_questions (source_id, target_field)
  WHERE target_field IS NOT NULL;

CREATE TABLE google_forms_sync_runs (
  id UUID PRIMARY KEY,
  source_id UUID NOT NULL,
  owner_user_id UUID NOT NULL,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  trigger_kind TEXT NOT NULL CHECK (trigger_kind IN ('MANUAL', 'SCHEDULED')),
  state TEXT NOT NULL CHECK (state IN ('QUEUED', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED')),
  idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 128),
  operation_import_id UUID REFERENCES operation_imports(id) ON DELETE SET NULL,
  cursor_started_at TIMESTAMPTZ,
  cursor_completed_at TIMESTAMPTZ,
  received_count INTEGER NOT NULL DEFAULT 0 CHECK (received_count >= 0),
  staged_count INTEGER NOT NULL DEFAULT 0 CHECK (staged_count >= 0),
  duplicate_count INTEGER NOT NULL DEFAULT 0 CHECK (duplicate_count >= 0),
  river_job_id BIGINT,
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (source_id, idempotency_key),
  FOREIGN KEY (source_id, owner_user_id)
    REFERENCES google_forms_sources(id, owner_user_id) ON DELETE RESTRICT,
  CHECK (completed_at IS NULL OR state IN ('COMPLETED', 'FAILED', 'CANCELLED'))
);

CREATE UNIQUE INDEX google_forms_sync_runs_one_active_per_source
  ON google_forms_sync_runs (source_id)
  WHERE state IN ('QUEUED', 'RUNNING');
CREATE INDEX google_forms_sync_runs_owner_created_index
  ON google_forms_sync_runs (owner_user_id, created_at DESC, id DESC);

ALTER TABLE operation_imports
  ADD CONSTRAINT operation_imports_google_forms_source_fk
    FOREIGN KEY (source_reference_id) REFERENCES google_forms_sources(id) ON DELETE RESTRICT,
  ADD CONSTRAINT operation_imports_source_payload_check CHECK (
    (source_kind = 'XLSX'
      AND source_reference_id IS NULL
      AND original_filename IS NOT NULL
      AND declared_size IS NOT NULL
      AND object_key IS NOT NULL)
    OR
    (source_kind = 'GOOGLE_FORMS'
      AND source_reference_id IS NOT NULL
      AND original_filename IS NULL
      AND declared_size IS NULL
      AND actual_size IS NULL
      AND content_sha256 IS NULL
      AND object_key IS NULL)
  );

CREATE TABLE google_forms_response_receipts (
  source_id UUID NOT NULL REFERENCES google_forms_sources(id) ON DELETE RESTRICT,
  provider_response_id TEXT NOT NULL CHECK (char_length(provider_response_id) BETWEEN 1 AND 500),
  submitted_at TIMESTAMPTZ NOT NULL,
  response_fingerprint BYTEA NOT NULL CHECK (octet_length(response_fingerprint) = 32),
  import_id UUID NOT NULL,
  sheet_index INTEGER NOT NULL DEFAULT 0 CHECK (sheet_index = 0),
  row_number INTEGER NOT NULL CHECK (row_number BETWEEN 2 AND 10001),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (source_id, provider_response_id),
  UNIQUE (import_id, sheet_index, row_number),
  FOREIGN KEY (import_id, sheet_index, row_number)
    REFERENCES operation_import_rows(import_id, sheet_index, row_number) ON DELETE RESTRICT
);

CREATE INDEX google_forms_response_receipts_source_submitted_index
  ON google_forms_response_receipts (source_id, submitted_at, provider_response_id);

CREATE TABLE google_forms_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  connection_id UUID REFERENCES google_forms_connections(id) ON DELETE SET NULL,
  source_id UUID REFERENCES google_forms_sources(id) ON DELETE SET NULL,
  sync_run_id UUID REFERENCES google_forms_sync_runs(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL CHECK (event_type IN (
    'CONNECTION_STARTED', 'CONNECTION_COMPLETED', 'CONNECTION_FAILED', 'CONNECTION_DISCONNECTED',
    'SOURCE_CREATED', 'SOURCE_MAPPED', 'SOURCE_STATE_CHANGED',
    'SYNC_REQUESTED', 'SYNC_STARTED', 'SYNC_COMPLETED', 'SYNC_FAILED', 'SYNC_CANCELLED'
  )),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  affected_count INTEGER CHECK (affected_count IS NULL OR affected_count >= 0),
  request_id TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (num_nonnulls(connection_id, source_id, sync_run_id) <= 1)
);

CREATE INDEX google_forms_audit_events_actor_created_index
  ON google_forms_audit_events (actor_user_id, created_at DESC);

INSERT INTO app_metadata (key, value)
VALUES ('schema.google_forms', 'm9')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

INSERT INTO app_metadata (key, value)
VALUES ('schema.operations', 'm9')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.google_forms';
UPDATE app_metadata SET value = 'm8', updated_at = now() WHERE key = 'schema.operations';

DROP TABLE google_forms_audit_events;
DROP TABLE google_forms_response_receipts;
ALTER TABLE operation_imports DROP CONSTRAINT operation_imports_source_payload_check;
ALTER TABLE operation_imports DROP CONSTRAINT operation_imports_google_forms_source_fk;
DROP TABLE google_forms_sync_runs;
DROP TABLE google_forms_questions;
DROP TABLE google_forms_sources;
DROP TABLE google_forms_connections;
DROP TABLE google_forms_oauth_states;

DELETE FROM operation_imports WHERE source_kind = 'GOOGLE_FORMS';
ALTER TABLE operation_imports
  DROP COLUMN source_reference_id,
  ALTER COLUMN original_filename SET NOT NULL,
  ALTER COLUMN declared_size SET NOT NULL,
  ALTER COLUMN object_key SET NOT NULL;
ALTER TABLE operation_import_cells DROP COLUMN source_validation_code;

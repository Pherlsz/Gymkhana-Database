-- M8 durable operations, source-neutral import staging and private XLSX exports.

CREATE TABLE operation_imports (
  id UUID PRIMARY KEY,
  actor_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE RESTRICT,
  module TEXT NOT NULL CHECK (module IN ('PROFILES', 'DOCUMENTS', 'BILLS')),
  source_kind TEXT NOT NULL DEFAULT 'XLSX' CHECK (source_kind IN ('XLSX', 'GOOGLE_FORMS')),
  original_filename TEXT NOT NULL CHECK (char_length(original_filename) BETWEEN 1 AND 255),
  declared_size BIGINT NOT NULL CHECK (declared_size BETWEEN 1 AND 26214400),
  actual_size BIGINT CHECK (actual_size BETWEEN 1 AND 26214400),
  content_sha256 BYTEA CHECK (content_sha256 IS NULL OR octet_length(content_sha256) = 32),
  object_key TEXT NOT NULL UNIQUE CHECK (char_length(object_key) BETWEEN 1 AND 512),
  idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 128),
  state TEXT NOT NULL CHECK (state IN (
    'UPLOADING', 'UPLOADED', 'PARSING', 'MAPPING', 'PREVIEW_READY',
    'DECISIONS_REQUIRED', 'READY', 'QUEUED', 'RUNNING', 'COMPLETED',
    'FAILED', 'CANCELLED', 'EXPIRED'
  )),
  stage TEXT NOT NULL CHECK (stage IN ('UPLOAD', 'PARSE', 'MAP', 'PREVIEW', 'EXECUTE', 'REPORT', 'CLEANUP')),
  selected_sheet_index INTEGER CHECK (selected_sheet_index IS NULL OR selected_sheet_index >= 0),
  mapping_version BIGINT NOT NULL DEFAULT 0 CHECK (mapping_version >= 0),
  unresolved_count INTEGER NOT NULL DEFAULT 0 CHECK (unresolved_count >= 0),
  validation_error_count INTEGER NOT NULL DEFAULT 0 CHECK (validation_error_count >= 0),
  inserted_count INTEGER NOT NULL DEFAULT 0 CHECK (inserted_count >= 0),
  updated_count INTEGER NOT NULL DEFAULT 0 CHECK (updated_count >= 0),
  linked_count INTEGER NOT NULL DEFAULT 0 CHECK (linked_count >= 0),
  skipped_count INTEGER NOT NULL DEFAULT 0 CHECK (skipped_count >= 0),
  errored_count INTEGER NOT NULL DEFAULT 0 CHECK (errored_count >= 0),
  conflicted_count INTEGER NOT NULL DEFAULT 0 CHECK (conflicted_count >= 0),
  river_job_id BIGINT,
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  expires_at TIMESTAMPTZ NOT NULL,
  cancelled_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  cleanup_claimed_at TIMESTAMPTZ,
  object_deleted_at TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (actor_user_id, idempotency_key),
  CHECK (actual_size IS NULL OR actual_size = declared_size),
  CHECK (cancelled_at IS NULL OR state IN ('CANCELLED', 'EXPIRED')),
  CHECK (completed_at IS NULL OR state IN ('COMPLETED', 'EXPIRED'))
);

CREATE INDEX operation_imports_actor_created_index
  ON operation_imports (actor_user_id, created_at DESC, id DESC);
CREATE INDEX operation_imports_expiry_index
  ON operation_imports (expires_at, id)
  WHERE object_deleted_at IS NULL AND state NOT IN ('PARSING', 'RUNNING');

CREATE TABLE operation_import_sheets (
  import_id UUID NOT NULL REFERENCES operation_imports(id) ON DELETE CASCADE,
  sheet_index INTEGER NOT NULL CHECK (sheet_index >= 0),
  name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  row_count INTEGER NOT NULL CHECK (row_count BETWEEN 0 AND 10000),
  column_count INTEGER NOT NULL CHECK (column_count BETWEEN 0 AND 256),
  PRIMARY KEY (import_id, sheet_index),
  UNIQUE (import_id, name)
);

CREATE TABLE operation_import_columns (
  import_id UUID NOT NULL,
  sheet_index INTEGER NOT NULL,
  source_column INTEGER NOT NULL CHECK (source_column BETWEEN 0 AND 255),
  source_header TEXT NOT NULL CHECK (char_length(source_header) BETWEEN 1 AND 500),
  target_field TEXT CHECK (target_field IS NULL OR char_length(target_field) BETWEEN 1 AND 120),
  PRIMARY KEY (import_id, sheet_index, source_column),
  FOREIGN KEY (import_id, sheet_index)
    REFERENCES operation_import_sheets(import_id, sheet_index) ON DELETE CASCADE
);

CREATE TABLE operation_import_rows (
  import_id UUID NOT NULL,
  sheet_index INTEGER NOT NULL,
  row_number INTEGER NOT NULL CHECK (row_number BETWEEN 2 AND 10001),
  proposed_action TEXT CHECK (proposed_action IS NULL OR proposed_action IN ('CREATE', 'UPDATE', 'LINK', 'SKIP', 'ERROR')),
  decision TEXT CHECK (decision IS NULL OR decision IN ('CREATE', 'UPDATE', 'LINK', 'SKIP')),
  target_id UUID,
  target_version BIGINT CHECK (target_version IS NULL OR target_version > 0),
  validation_error_count INTEGER NOT NULL DEFAULT 0 CHECK (validation_error_count >= 0),
  decision_required BOOLEAN NOT NULL DEFAULT false,
  source_fingerprint BYTEA CHECK (source_fingerprint IS NULL OR octet_length(source_fingerprint) = 32),
  PRIMARY KEY (import_id, sheet_index, row_number),
  FOREIGN KEY (import_id, sheet_index)
    REFERENCES operation_import_sheets(import_id, sheet_index) ON DELETE CASCADE,
  CHECK ((target_id IS NULL) = (target_version IS NULL))
);

CREATE TABLE operation_import_cells (
  import_id UUID NOT NULL,
  sheet_index INTEGER NOT NULL,
  row_number INTEGER NOT NULL,
  source_column INTEGER NOT NULL CHECK (source_column BETWEEN 0 AND 255),
  raw_value TEXT NOT NULL CHECK (octet_length(raw_value) <= 32768),
  value_kind TEXT NOT NULL CHECK (value_kind IN ('EMPTY', 'TEXT', 'NUMBER', 'BOOLEAN', 'DATE')),
  formula_present BOOLEAN NOT NULL DEFAULT false,
  validation_code TEXT CHECK (validation_code IS NULL OR char_length(validation_code) BETWEEN 1 AND 80),
  PRIMARY KEY (import_id, sheet_index, row_number, source_column),
  FOREIGN KEY (import_id, sheet_index, row_number)
    REFERENCES operation_import_rows(import_id, sheet_index, row_number) ON DELETE CASCADE,
  FOREIGN KEY (import_id, sheet_index, source_column)
    REFERENCES operation_import_columns(import_id, sheet_index, source_column) ON DELETE CASCADE
);

CREATE TABLE operation_import_outcomes (
  import_id UUID NOT NULL,
  sheet_index INTEGER NOT NULL,
  row_number INTEGER NOT NULL,
  outcome TEXT NOT NULL CHECK (outcome IN ('INSERTED', 'UPDATED', 'LINKED', 'SKIPPED', 'ERRORED', 'CONFLICTED')),
  target_id UUID,
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  committed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (import_id, sheet_index, row_number),
  FOREIGN KEY (import_id, sheet_index, row_number)
    REFERENCES operation_import_rows(import_id, sheet_index, row_number) ON DELETE CASCADE
);

CREATE TABLE operation_exports (
  id UUID PRIMARY KEY,
  actor_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE RESTRICT,
  module TEXT NOT NULL CHECK (module IN ('PROFILES', 'DOCUMENTS', 'BILLS')),
  idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 128),
  state TEXT NOT NULL CHECK (state IN ('QUEUED', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED', 'EXPIRED')),
  object_key TEXT NOT NULL UNIQUE CHECK (char_length(object_key) BETWEEN 1 AND 512),
  filename TEXT NOT NULL CHECK (char_length(filename) BETWEEN 1 AND 255),
  row_count INTEGER NOT NULL DEFAULT 0 CHECK (row_count >= 0),
  byte_size BIGINT CHECK (byte_size IS NULL OR byte_size > 0),
  content_sha256 BYTEA CHECK (content_sha256 IS NULL OR octet_length(content_sha256) = 32),
  river_job_id BIGINT,
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  expires_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ,
  cleanup_claimed_at TIMESTAMPTZ,
  object_deleted_at TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (actor_user_id, idempotency_key),
  CHECK (completed_at IS NULL OR state IN ('COMPLETED', 'EXPIRED'))
);

CREATE INDEX operation_exports_actor_created_index
  ON operation_exports (actor_user_id, created_at DESC, id DESC);
CREATE INDEX operation_exports_expiry_index
  ON operation_exports (expires_at, id)
  WHERE object_deleted_at IS NULL AND state <> 'RUNNING';

CREATE TABLE operation_rate_limits (
  actor_user_id UUID PRIMARY KEY REFERENCES app_users(id) ON DELETE CASCADE,
  window_started_at TIMESTAMPTZ NOT NULL,
  request_count INTEGER NOT NULL CHECK (request_count > 0),
  active_imports INTEGER NOT NULL DEFAULT 0 CHECK (active_imports >= 0),
  active_exports INTEGER NOT NULL DEFAULT 0 CHECK (active_exports >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE operation_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  import_id UUID REFERENCES operation_imports(id) ON DELETE SET NULL,
  export_id UUID REFERENCES operation_exports(id) ON DELETE SET NULL,
  module TEXT CHECK (module IS NULL OR module IN ('PROFILES', 'DOCUMENTS', 'BILLS')),
  event_type TEXT NOT NULL CHECK (event_type IN (
    'IMPORT_CREATED', 'IMPORT_CONFIRMED', 'IMPORT_PARSED', 'IMPORT_MAPPED',
    'IMPORT_PREVIEWED', 'IMPORT_DECIDED', 'IMPORT_STARTED', 'IMPORT_CANCELLED',
    'IMPORT_COMPLETED', 'IMPORT_FAILED', 'IMPORT_EXPIRED', 'EXPORT_CREATED',
    'EXPORT_COMPLETED', 'EXPORT_DOWNLOADED', 'EXPORT_FAILED', 'EXPORT_EXPIRED',
    'BULK_DELETE'
  )),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  affected_count INTEGER CHECK (affected_count IS NULL OR affected_count >= 0),
  request_id TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (num_nonnulls(import_id, export_id) <= 1)
);

CREATE INDEX operation_audit_events_actor_created_index
  ON operation_audit_events (actor_user_id, created_at DESC);

INSERT INTO app_metadata (key, value)
VALUES ('schema.operations', 'm8')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.operations';
DROP TABLE operation_audit_events;
DROP TABLE operation_rate_limits;
DROP TABLE operation_exports;
DROP TABLE operation_import_outcomes;
DROP TABLE operation_import_cells;
DROP TABLE operation_import_rows;
DROP TABLE operation_import_columns;
DROP TABLE operation_import_sheets;
DROP TABLE operation_imports;

-- M10 typed Query Engine executions and owner-scoped result sets.

CREATE TABLE query_rate_limits (
  actor_user_id UUID PRIMARY KEY REFERENCES app_users(id) ON DELETE CASCADE,
  window_started_at TIMESTAMPTZ NOT NULL,
  request_count INTEGER NOT NULL CHECK (request_count > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE query_executions (
  id UUID PRIMARY KEY,
  owner_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE RESTRICT,
  state TEXT NOT NULL CHECK (state IN ('RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED')),
  idempotency_key TEXT NOT NULL CHECK (
    char_length(idempotency_key) BETWEEN 8 AND 128
    AND idempotency_key ~ '^[A-Za-z0-9._:-]+$'
  ),
  plan_fingerprint BYTEA NOT NULL CHECK (octet_length(plan_fingerprint) = 32),
  catalog_version TEXT NOT NULL CHECK (catalog_version ~ '^[a-f0-9]{64}$'),
  root_entity TEXT NOT NULL CHECK (root_entity ~ '^[a-z][a-z0-9_.-]{1,199}$'),
  maximum_rows INTEGER NOT NULL CHECK (maximum_rows BETWEEN 1 AND 500),
  row_count INTEGER NOT NULL DEFAULT 0 CHECK (row_count BETWEEN 0 AND 500),
  column_count INTEGER NOT NULL DEFAULT 0 CHECK (column_count BETWEEN 0 AND 20),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  started_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (owner_user_id, idempotency_key),
  CHECK (expires_at > created_at),
  CHECK (
    (state = 'RUNNING' AND completed_at IS NULL AND error_code IS NULL) OR
    (state = 'COMPLETED' AND completed_at IS NOT NULL AND error_code IS NULL) OR
    (state IN ('FAILED', 'CANCELLED') AND completed_at IS NOT NULL AND error_code IS NOT NULL)
  )
);
CREATE INDEX query_executions_owner_created_index
  ON query_executions (owner_user_id, created_at DESC, id DESC);
CREATE INDEX query_executions_expiry_index
  ON query_executions (expires_at, id);
CREATE UNIQUE INDEX query_executions_one_running_per_owner
  ON query_executions (owner_user_id)
  WHERE state = 'RUNNING';

CREATE TABLE query_result_columns (
  execution_id UUID NOT NULL REFERENCES query_executions(id) ON DELETE CASCADE,
  position INTEGER NOT NULL CHECK (position BETWEEN 0 AND 19),
  field_key TEXT NOT NULL CHECK (field_key ~ '^[a-z][A-Za-z0-9_.-]{1,199}$'),
  label TEXT NOT NULL CHECK (label = btrim(label) AND label <> '' AND char_length(label) <= 160),
  value_kind TEXT NOT NULL CHECK (value_kind IN (
    'text', 'long_text', 'identifier', 'integer', 'decimal', 'boolean',
    'civil_date', 'civil_month', 'timestamp', 'enum'
  )),
  PRIMARY KEY (execution_id, position),
  UNIQUE (execution_id, field_key)
);

CREATE TABLE query_result_rows (
  execution_id UUID NOT NULL REFERENCES query_executions(id) ON DELETE CASCADE,
  position INTEGER NOT NULL CHECK (position BETWEEN 0 AND 499),
  entity_kind TEXT NOT NULL CHECK (entity_kind ~ '^[a-z][a-z0-9_.-]{1,199}$'),
  entity_id TEXT NOT NULL CHECK (entity_id = btrim(entity_id) AND entity_id <> '' AND char_length(entity_id) <= 200),
  entity_label TEXT NOT NULL CHECK (entity_label = btrim(entity_label) AND entity_label <> '' AND char_length(entity_label) <= 300),
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (execution_id, position),
  UNIQUE (execution_id, entity_kind, entity_id, position)
);

CREATE TABLE query_result_cells (
  execution_id UUID NOT NULL,
  row_position INTEGER NOT NULL,
  column_position INTEGER NOT NULL,
  value_kind TEXT NOT NULL CHECK (value_kind IN (
    'text', 'long_text', 'identifier', 'integer', 'decimal', 'boolean',
    'civil_date', 'civil_month', 'timestamp', 'enum'
  )),
  is_null BOOLEAN NOT NULL DEFAULT false,
  text_value TEXT CHECK (text_value IS NULL OR char_length(text_value) <= 5000),
  integer_value BIGINT,
  decimal_value NUMERIC(38, 10),
  boolean_value BOOLEAN,
  civil_date_value DATE,
  timestamp_value TIMESTAMPTZ,
  PRIMARY KEY (execution_id, row_position, column_position),
  FOREIGN KEY (execution_id, row_position)
    REFERENCES query_result_rows(execution_id, position) ON DELETE CASCADE,
  FOREIGN KEY (execution_id, column_position)
    REFERENCES query_result_columns(execution_id, position) ON DELETE CASCADE,
  CHECK (
    (is_null AND num_nonnulls(text_value, integer_value, decimal_value, boolean_value, civil_date_value, timestamp_value) = 0) OR
    (NOT is_null AND num_nonnulls(text_value, integer_value, decimal_value, boolean_value, civil_date_value, timestamp_value) = 1)
  ),
  CHECK (value_kind = 'integer' OR integer_value IS NULL),
  CHECK (value_kind = 'decimal' OR decimal_value IS NULL),
  CHECK (value_kind = 'boolean' OR boolean_value IS NULL),
  CHECK (value_kind = 'civil_date' OR civil_date_value IS NULL),
  CHECK (value_kind = 'timestamp' OR timestamp_value IS NULL),
  CHECK (
    value_kind IN ('text', 'long_text', 'identifier', 'civil_month', 'enum') OR text_value IS NULL
  )
);

CREATE TABLE query_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  execution_id UUID REFERENCES query_executions(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL CHECK (event_type IN (
    'CATALOG_READ', 'PLAN_VALIDATED', 'EXECUTION_STARTED', 'EXECUTION_COMPLETED',
    'EXECUTION_FAILED', 'RESULT_READ', 'RESULT_EXPIRED'
  )),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  affected_count INTEGER CHECK (affected_count IS NULL OR affected_count >= 0),
  request_id TEXT NOT NULL DEFAULT '' CHECK (char_length(request_id) <= 128),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX query_audit_events_actor_created_index
  ON query_audit_events (actor_user_id, created_at DESC, id DESC);

INSERT INTO app_metadata (key, value)
VALUES ('schema.query_engine', 'm10')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.query_engine';
DROP TABLE query_audit_events;
DROP TABLE query_result_cells;
DROP TABLE query_result_rows;
DROP TABLE query_result_columns;
DROP TABLE query_executions;
DROP TABLE query_rate_limits;

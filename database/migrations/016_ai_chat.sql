-- M12 private, provider-neutral, read-only AI Chat lifecycle and result references.

ALTER TABLE query_executions
  ADD CONSTRAINT query_executions_id_owner_user_unique UNIQUE (id, owner_user_id);

CREATE TABLE ai_chat_threads (
  id UUID PRIMARY KEY,
  owner_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  title TEXT NOT NULL CHECK (
    title = btrim(title) AND char_length(title) BETWEEN 1 AND 120
  ),
  next_message_sequence BIGINT NOT NULL DEFAULT 1 CHECK (next_message_sequence > 0),
  retention_expires_at TIMESTAMPTZ NOT NULL,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (id, owner_user_id),
  CHECK (retention_expires_at > created_at)
);
CREATE INDEX ai_chat_threads_owner_updated_index
  ON ai_chat_threads (owner_user_id, updated_at DESC, id DESC);
CREATE INDEX ai_chat_threads_retention_index
  ON ai_chat_threads (retention_expires_at, id);

CREATE TABLE ai_chat_runs (
  id UUID PRIMARY KEY,
  thread_id UUID NOT NULL REFERENCES ai_chat_threads(id) ON DELETE CASCADE,
  owner_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  retry_of_run_id UUID REFERENCES ai_chat_runs(id) ON DELETE SET NULL,
  idempotency_key TEXT NOT NULL CHECK (
    char_length(idempotency_key) BETWEEN 8 AND 128
    AND idempotency_key ~ '^[A-Za-z0-9._:-]+$'
  ),
  request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint) = 32),
  state TEXT NOT NULL CHECK (state IN (
    'QUEUED', 'RUNNING', 'TOOL_RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED'
  )),
  tool_call_count INTEGER NOT NULL DEFAULT 0 CHECK (tool_call_count BETWEEN 0 AND 8),
  input_usage BIGINT NOT NULL DEFAULT 0 CHECK (input_usage >= 0),
  output_usage BIGINT NOT NULL DEFAULT 0 CHECK (output_usage >= 0),
  result_bytes BIGINT NOT NULL DEFAULT 0 CHECK (result_bytes BETWEEN 0 AND 262144),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  cancel_requested_at TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  next_event_sequence BIGINT NOT NULL DEFAULT 1 CHECK (next_event_sequence > 0),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (owner_user_id, idempotency_key),
  UNIQUE (id, thread_id),
  UNIQUE (id, thread_id, owner_user_id),
  FOREIGN KEY (thread_id, owner_user_id)
    REFERENCES ai_chat_threads(id, owner_user_id) ON DELETE CASCADE,
  CHECK (retry_of_run_id IS NULL OR retry_of_run_id <> id),
  CHECK (started_at IS NULL OR state <> 'QUEUED'),
  CHECK (
    (state IN ('QUEUED', 'RUNNING', 'TOOL_RUNNING') AND completed_at IS NULL AND error_code IS NULL) OR
    (state = 'COMPLETED' AND completed_at IS NOT NULL AND error_code IS NULL) OR
    (state IN ('FAILED', 'CANCELLED') AND completed_at IS NOT NULL AND error_code IS NOT NULL)
  ),
  CHECK (cancel_requested_at IS NULL OR state IN ('QUEUED', 'RUNNING', 'TOOL_RUNNING', 'CANCELLED'))
);
CREATE UNIQUE INDEX ai_chat_runs_one_active_thread_index
  ON ai_chat_runs (thread_id)
  WHERE state IN ('QUEUED', 'RUNNING', 'TOOL_RUNNING');
CREATE INDEX ai_chat_runs_owner_created_index
  ON ai_chat_runs (owner_user_id, created_at DESC, id DESC);
CREATE INDEX ai_chat_runs_thread_created_index
  ON ai_chat_runs (thread_id, created_at DESC, id DESC);

CREATE TABLE ai_chat_messages (
  id UUID PRIMARY KEY,
  thread_id UUID NOT NULL REFERENCES ai_chat_threads(id) ON DELETE CASCADE,
  run_id UUID NOT NULL REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
  sequence BIGINT NOT NULL CHECK (sequence > 0),
  role TEXT NOT NULL CHECK (role IN ('USER', 'ASSISTANT')),
  content TEXT NOT NULL CHECK (content <> '' AND char_length(content) <= 20000),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (thread_id, sequence),
  UNIQUE (run_id, role),
  FOREIGN KEY (run_id, thread_id)
    REFERENCES ai_chat_runs(id, thread_id) ON DELETE CASCADE
);
CREATE INDEX ai_chat_messages_thread_sequence_index
  ON ai_chat_messages (thread_id, sequence, id);

CREATE TABLE ai_chat_result_references (
  id UUID PRIMARY KEY,
  thread_id UUID NOT NULL REFERENCES ai_chat_threads(id) ON DELETE CASCADE,
  run_id UUID NOT NULL REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
  owner_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  reference_kind TEXT NOT NULL CHECK (reference_kind IN ('SEARCH', 'QUERY')),
  query_execution_id UUID,
  logical_request JSONB NOT NULL CHECK (
    jsonb_typeof(logical_request) = 'object' AND pg_column_size(logical_request) <= 32768
  ),
  context_fingerprint BYTEA NOT NULL CHECK (octet_length(context_fingerprint) = 32),
  label TEXT NOT NULL CHECK (label = btrim(label) AND char_length(label) BETWEEN 1 AND 160),
  row_count INTEGER NOT NULL CHECK (row_count BETWEEN 0 AND 100),
  column_count INTEGER NOT NULL CHECK (column_count BETWEEN 0 AND 20),
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (run_id, thread_id, owner_user_id)
    REFERENCES ai_chat_runs(id, thread_id, owner_user_id) ON DELETE CASCADE,
  FOREIGN KEY (query_execution_id, owner_user_id)
    REFERENCES query_executions(id, owner_user_id) ON DELETE CASCADE,
  CHECK (
    (reference_kind = 'QUERY' AND query_execution_id IS NOT NULL) OR
    (reference_kind = 'SEARCH' AND query_execution_id IS NULL)
  ),
  CHECK (expires_at > created_at)
);
CREATE INDEX ai_chat_result_references_owner_created_index
  ON ai_chat_result_references (owner_user_id, created_at DESC, id DESC);
CREATE INDEX ai_chat_result_references_thread_created_index
  ON ai_chat_result_references (thread_id, created_at DESC, id DESC);

ALTER TABLE ai_chat_threads
  ADD COLUMN active_result_reference_id UUID
  REFERENCES ai_chat_result_references(id) ON DELETE SET NULL;

CREATE FUNCTION enforce_ai_chat_active_reference()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.active_result_reference_id IS NOT NULL AND NOT EXISTS (
    SELECT 1
    FROM ai_chat_result_references reference
    WHERE reference.id = NEW.active_result_reference_id
      AND reference.thread_id = NEW.id
      AND reference.owner_user_id = NEW.owner_user_id
  ) THEN
    RAISE EXCEPTION 'AI Chat active result reference does not belong to the thread owner'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER ai_chat_threads_active_reference_guard
BEFORE INSERT OR UPDATE OF active_result_reference_id, owner_user_id
ON ai_chat_threads
FOR EACH ROW EXECUTE FUNCTION enforce_ai_chat_active_reference();

CREATE TABLE ai_chat_tool_steps (
  id UUID PRIMARY KEY,
  run_id UUID NOT NULL REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
  sequence INTEGER NOT NULL CHECK (sequence BETWEEN 1 AND 8),
  tool_kind TEXT NOT NULL CHECK (tool_kind IN ('CATALOG', 'SEARCH', 'QUERY', 'RESULT')),
  state TEXT NOT NULL CHECK (state IN ('RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED')),
  arguments_fingerprint BYTEA NOT NULL CHECK (octet_length(arguments_fingerprint) = 32),
  result_reference_id UUID REFERENCES ai_chat_result_references(id) ON DELETE SET NULL,
  row_count INTEGER NOT NULL DEFAULT 0 CHECK (row_count BETWEEN 0 AND 100),
  result_bytes INTEGER NOT NULL DEFAULT 0 CHECK (result_bytes BETWEEN 0 AND 262144),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  started_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ,
  UNIQUE (run_id, sequence),
  CHECK (
    (state = 'RUNNING' AND completed_at IS NULL AND error_code IS NULL) OR
    (state = 'COMPLETED' AND completed_at IS NOT NULL AND error_code IS NULL) OR
    (state IN ('FAILED', 'CANCELLED') AND completed_at IS NOT NULL AND error_code IS NOT NULL)
  )
);

CREATE TABLE ai_chat_run_events (
  run_id UUID NOT NULL REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
  sequence BIGINT NOT NULL CHECK (sequence > 0),
  event_kind TEXT NOT NULL CHECK (event_kind IN (
    'RUN_ACCEPTED', 'RUN_STARTED', 'TEXT_DELTA', 'TOOL_STARTED', 'TOOL_COMPLETED',
    'RESULT_REFERENCE', 'RUN_COMPLETED', 'RUN_FAILED', 'RUN_CANCELLED'
  )),
  text_delta TEXT CHECK (text_delta IS NULL OR (text_delta <> '' AND char_length(text_delta) <= 1000)),
  tool_step_id UUID,
  result_reference_id UUID,
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, sequence),
  CHECK (
    (event_kind = 'TEXT_DELTA' AND text_delta IS NOT NULL AND tool_step_id IS NULL AND result_reference_id IS NULL AND error_code IS NULL) OR
    (event_kind IN ('TOOL_STARTED', 'TOOL_COMPLETED') AND text_delta IS NULL AND tool_step_id IS NOT NULL AND result_reference_id IS NULL AND error_code IS NULL) OR
    (event_kind = 'RESULT_REFERENCE' AND text_delta IS NULL AND result_reference_id IS NOT NULL AND tool_step_id IS NULL AND error_code IS NULL) OR
    (event_kind IN ('RUN_FAILED', 'RUN_CANCELLED') AND text_delta IS NULL AND tool_step_id IS NULL AND result_reference_id IS NULL AND error_code IS NOT NULL) OR
    (event_kind IN ('RUN_ACCEPTED', 'RUN_STARTED', 'RUN_COMPLETED') AND text_delta IS NULL AND tool_step_id IS NULL AND result_reference_id IS NULL AND error_code IS NULL)
  )
);

CREATE TABLE ai_chat_usage_windows (
  owner_user_id UUID PRIMARY KEY REFERENCES app_users(id) ON DELETE CASCADE,
  window_started_at TIMESTAMPTZ NOT NULL,
  request_count INTEGER NOT NULL CHECK (request_count > 0),
  tool_call_count INTEGER NOT NULL DEFAULT 0 CHECK (tool_call_count >= 0),
  input_usage BIGINT NOT NULL DEFAULT 0 CHECK (input_usage >= 0),
  output_usage BIGINT NOT NULL DEFAULT 0 CHECK (output_usage >= 0),
  result_bytes BIGINT NOT NULL DEFAULT 0 CHECK (result_bytes >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ai_chat_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  thread_id UUID,
  run_id UUID,
  result_reference_id UUID,
  event_type TEXT NOT NULL CHECK (event_type IN (
    'THREAD_CREATED', 'THREAD_READ', 'THREAD_RENAMED', 'THREAD_DELETED',
    'MESSAGES_READ', 'CONTEXT_CHANGED',
    'RUN_CREATED', 'RUN_STARTED', 'RUN_CANCELLED', 'RUN_COMPLETED', 'RUN_FAILED', 'RUN_RECOVERED',
    'TOOL_EXECUTED', 'RESULT_READ', 'RETENTION_CLEANUP'
  )),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  tool_kind TEXT CHECK (tool_kind IS NULL OR tool_kind IN ('CATALOG', 'SEARCH', 'QUERY', 'RESULT')),
  affected_count INTEGER CHECK (affected_count IS NULL OR affected_count >= 0),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  request_id TEXT NOT NULL DEFAULT '' CHECK (char_length(request_id) <= 128),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (num_nonnulls(thread_id, run_id, result_reference_id) <= 1)
);
CREATE INDEX ai_chat_audit_events_actor_created_index
  ON ai_chat_audit_events (actor_user_id, created_at DESC, id DESC);

INSERT INTO app_metadata (key, value)
VALUES ('schema.ai_chat', 'm12')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.ai_chat';
DROP TABLE ai_chat_audit_events;
DROP TABLE ai_chat_usage_windows;
DROP TABLE ai_chat_run_events;
DROP TABLE ai_chat_tool_steps;
DROP TRIGGER ai_chat_threads_active_reference_guard ON ai_chat_threads;
DROP FUNCTION enforce_ai_chat_active_reference();
ALTER TABLE ai_chat_threads DROP COLUMN active_result_reference_id;
DROP TABLE ai_chat_result_references;
DROP TABLE ai_chat_messages;
DROP TABLE ai_chat_runs;
DROP TABLE ai_chat_threads;
ALTER TABLE query_executions DROP CONSTRAINT query_executions_id_owner_user_unique;

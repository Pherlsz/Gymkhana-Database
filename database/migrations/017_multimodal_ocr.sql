-- M13 private multimodal/OCR extraction, review evidence and explicit application lifecycle.

CREATE TABLE ocr_jobs (
  id UUID PRIMARY KEY,
  owner_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  attachment_id UUID NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
  retry_of_job_id UUID REFERENCES ocr_jobs(id) ON DELETE SET NULL,
  idempotency_key TEXT NOT NULL CHECK (
    char_length(idempotency_key) BETWEEN 8 AND 128
    AND idempotency_key ~ '^[A-Za-z0-9._:-]+$'
  ),
  request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint) = 32),
  source_sha256 BYTEA NOT NULL CHECK (octet_length(source_sha256) = 32),
  catalog_fingerprint BYTEA NOT NULL CHECK (octet_length(catalog_fingerprint) = 32),
  schema_version TEXT NOT NULL CHECK (schema_version = 'v1'),
  source_mime TEXT NOT NULL CHECK (source_mime IN ('application/pdf', 'image/jpeg', 'image/png')),
  source_bytes BIGINT NOT NULL CHECK (source_bytes BETWEEN 1 AND 20971520),
  state TEXT NOT NULL CHECK (state IN ('QUEUED', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED')),
  river_job_id BIGINT CHECK (river_job_id IS NULL OR river_job_id > 0),
  attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count BETWEEN 0 AND 3),
  page_count INTEGER NOT NULL DEFAULT 0 CHECK (page_count BETWEEN 0 AND 20),
  pixel_count BIGINT NOT NULL DEFAULT 0 CHECK (pixel_count BETWEEN 0 AND 40000000),
  suggestion_count INTEGER NOT NULL DEFAULT 0 CHECK (suggestion_count BETWEEN 0 AND 100),
  provider_usage BIGINT NOT NULL DEFAULT 0 CHECK (provider_usage BETWEEN 0 AND 100000000),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  cancel_requested_at TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  next_event_sequence BIGINT NOT NULL DEFAULT 1 CHECK (next_event_sequence > 0),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (owner_user_id, idempotency_key),
  CHECK (retry_of_job_id IS NULL OR retry_of_job_id <> id),
  CHECK (started_at IS NULL OR state <> 'QUEUED'),
  CHECK (
    (state IN ('QUEUED', 'RUNNING') AND completed_at IS NULL AND error_code IS NULL) OR
    (state = 'COMPLETED' AND completed_at IS NOT NULL AND error_code IS NULL) OR
    (state IN ('FAILED', 'CANCELLED') AND completed_at IS NOT NULL AND error_code IS NOT NULL)
  ),
  CHECK (cancel_requested_at IS NULL OR state IN ('QUEUED', 'RUNNING', 'CANCELLED')),
  CHECK (state <> 'COMPLETED' OR page_count > 0)
);
CREATE UNIQUE INDEX ocr_jobs_one_active_source_index
  ON ocr_jobs (owner_user_id, attachment_id)
  WHERE state IN ('QUEUED', 'RUNNING');
CREATE INDEX ocr_jobs_owner_created_index
  ON ocr_jobs (owner_user_id, created_at DESC, id DESC);
CREATE INDEX ocr_jobs_attachment_created_index
  ON ocr_jobs (attachment_id, created_at DESC, id DESC);
CREATE INDEX ocr_jobs_stale_index
  ON ocr_jobs (updated_at, id) WHERE state = 'RUNNING';

CREATE TABLE ocr_job_events (
  job_id UUID NOT NULL REFERENCES ocr_jobs(id) ON DELETE CASCADE,
  sequence BIGINT NOT NULL CHECK (sequence > 0),
  event_kind TEXT NOT NULL CHECK (event_kind IN (
    'JOB_ACCEPTED', 'JOB_STARTED', 'SOURCE_VALIDATED', 'SUGGESTIONS_READY',
    'JOB_COMPLETED', 'JOB_FAILED', 'JOB_CANCELLED'
  )),
  page_count INTEGER CHECK (page_count IS NULL OR page_count BETWEEN 1 AND 20),
  suggestion_count INTEGER CHECK (suggestion_count IS NULL OR suggestion_count BETWEEN 0 AND 100),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (job_id, sequence),
  CHECK (
    (event_kind = 'SOURCE_VALIDATED' AND page_count IS NOT NULL AND suggestion_count IS NULL AND error_code IS NULL) OR
    (event_kind = 'SUGGESTIONS_READY' AND page_count IS NULL AND suggestion_count IS NOT NULL AND error_code IS NULL) OR
    (event_kind IN ('JOB_FAILED', 'JOB_CANCELLED') AND page_count IS NULL AND suggestion_count IS NULL AND error_code IS NOT NULL) OR
    (event_kind IN ('JOB_ACCEPTED', 'JOB_STARTED', 'JOB_COMPLETED') AND page_count IS NULL AND suggestion_count IS NULL AND error_code IS NULL)
  )
);
CREATE UNIQUE INDEX ocr_job_events_one_terminal_index
  ON ocr_job_events (job_id)
  WHERE event_kind IN ('JOB_COMPLETED', 'JOB_FAILED', 'JOB_CANCELLED');

CREATE TABLE ocr_suggestions (
  id UUID PRIMARY KEY,
  job_id UUID NOT NULL REFERENCES ocr_jobs(id) ON DELETE CASCADE,
  ordinal INTEGER NOT NULL CHECK (ordinal BETWEEN 1 AND 100),
  target_kind TEXT NOT NULL CHECK (target_kind IN ('PROFILE', 'DOCUMENT', 'BILL', 'CUSTOM_FIELD')),
  target_id UUID NOT NULL,
  target_version BIGINT NOT NULL CHECK (target_version > 0),
  field_key TEXT NOT NULL CHECK (
    char_length(field_key) BETWEEN 2 AND 120
    AND field_key ~ '^[a-z][a-z0-9_.-]+$'
  ),
  field_label TEXT NOT NULL CHECK (
    field_label = btrim(field_label) AND char_length(field_label) BETWEEN 1 AND 160
  ),
  value_kind TEXT NOT NULL CHECK (value_kind IN (
    'TEXT', 'LONG_TEXT', 'INTEGER', 'DECIMAL', 'BOOLEAN',
    'CIVIL_DATE', 'CIVIL_MONTH', 'EMAIL', 'PHONE'
  )),
  proposed_value TEXT NOT NULL CHECK (
    char_length(proposed_value) BETWEEN 1 AND 5000
    AND proposed_value = btrim(proposed_value)
  ),
  confidence SMALLINT CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1000),
  evidence_page INTEGER NOT NULL CHECK (evidence_page BETWEEN 1 AND 20),
  region_x INTEGER,
  region_y INTEGER,
  region_width INTEGER,
  region_height INTEGER,
  evidence_excerpt TEXT CHECK (
    evidence_excerpt IS NULL OR (
      evidence_excerpt = btrim(evidence_excerpt) AND char_length(evidence_excerpt) BETWEEN 1 AND 500
    )
  ),
  review_state TEXT NOT NULL DEFAULT 'PENDING' CHECK (review_state IN (
    'PENDING', 'ACCEPTED', 'REJECTED', 'APPLIED', 'STALE'
  )),
  reviewed_value TEXT CHECK (
    reviewed_value IS NULL OR (
      char_length(reviewed_value) <= 5000 AND reviewed_value = btrim(reviewed_value)
    )
  ),
  reviewed_by_user_id UUID REFERENCES app_users(id) ON DELETE RESTRICT,
  reviewed_at TIMESTAMPTZ,
  applied_at TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (job_id, ordinal),
  UNIQUE (job_id, target_kind, target_id, field_key),
  CHECK (
    (value_kind = 'INTEGER' AND proposed_value ~ '^-?(0|[1-9][0-9]{0,17})$') OR
    (value_kind = 'DECIMAL' AND proposed_value ~ '^-?(0|[1-9][0-9]{0,15})(\.[0-9]{1,6})?$') OR
    (value_kind = 'BOOLEAN' AND proposed_value IN ('true', 'false')) OR
    (value_kind = 'CIVIL_DATE' AND proposed_value ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$') OR
    (value_kind = 'CIVIL_MONTH' AND proposed_value ~ '^[0-9]{4}-[0-9]{2}$') OR
    value_kind IN ('TEXT', 'LONG_TEXT', 'EMAIL', 'PHONE')
  ),
  CHECK (
    (region_x IS NULL AND region_y IS NULL AND region_width IS NULL AND region_height IS NULL) OR
    (region_x BETWEEN 0 AND 9999 AND region_y BETWEEN 0 AND 9999 AND
      region_width BETWEEN 1 AND 10000 AND region_height BETWEEN 1 AND 10000 AND
      region_x + region_width <= 10000 AND region_y + region_height <= 10000)
  ),
  CHECK (
    (review_state = 'PENDING' AND reviewed_value IS NULL AND reviewed_by_user_id IS NULL AND reviewed_at IS NULL AND applied_at IS NULL) OR
    (review_state = 'REJECTED' AND reviewed_value IS NULL AND reviewed_by_user_id IS NOT NULL AND reviewed_at IS NOT NULL AND applied_at IS NULL) OR
    (review_state IN ('ACCEPTED', 'STALE') AND reviewed_value IS NOT NULL AND reviewed_by_user_id IS NOT NULL AND reviewed_at IS NOT NULL AND applied_at IS NULL) OR
    (review_state = 'APPLIED' AND reviewed_value IS NOT NULL AND reviewed_by_user_id IS NOT NULL AND reviewed_at IS NOT NULL AND applied_at IS NOT NULL)
  )
);
CREATE INDEX ocr_suggestions_job_ordinal_index
  ON ocr_suggestions (job_id, ordinal, id);
CREATE INDEX ocr_suggestions_pending_index
  ON ocr_suggestions (job_id, review_state, ordinal, id);

CREATE TABLE ocr_apply_receipts (
  id UUID PRIMARY KEY,
  job_id UUID NOT NULL REFERENCES ocr_jobs(id) ON DELETE CASCADE,
  owner_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  idempotency_key TEXT NOT NULL CHECK (
    char_length(idempotency_key) BETWEEN 8 AND 128
    AND idempotency_key ~ '^[A-Za-z0-9._:-]+$'
  ),
  request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint) = 32),
  state TEXT NOT NULL CHECK (state IN ('RUNNING', 'COMPLETED')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  UNIQUE (owner_user_id, idempotency_key),
  CHECK (
    (state = 'RUNNING' AND completed_at IS NULL) OR
    (state = 'COMPLETED' AND completed_at IS NOT NULL)
  )
);
CREATE INDEX ocr_apply_receipts_job_created_index
  ON ocr_apply_receipts (job_id, created_at DESC, id DESC);

CREATE TABLE ocr_apply_results (
  receipt_id UUID NOT NULL REFERENCES ocr_apply_receipts(id) ON DELETE CASCADE,
  suggestion_id UUID NOT NULL REFERENCES ocr_suggestions(id) ON DELETE CASCADE,
  outcome TEXT NOT NULL CHECK (outcome IN ('APPLIED', 'STALE', 'FAILED')),
  target_version BIGINT CHECK (target_version IS NULL OR target_version > 0),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (receipt_id, suggestion_id),
  CHECK (
    (outcome = 'APPLIED' AND target_version IS NOT NULL AND error_code IS NULL) OR
    (outcome IN ('STALE', 'FAILED') AND error_code IS NOT NULL)
  )
);

CREATE TABLE ocr_usage_windows (
  owner_user_id UUID PRIMARY KEY REFERENCES app_users(id) ON DELETE CASCADE,
  window_started_at TIMESTAMPTZ NOT NULL,
  request_count INTEGER NOT NULL CHECK (request_count > 0),
  provider_usage BIGINT NOT NULL DEFAULT 0 CHECK (provider_usage >= 0),
  source_bytes BIGINT NOT NULL DEFAULT 0 CHECK (source_bytes >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ocr_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  job_id UUID,
  suggestion_id UUID,
  field_key TEXT CHECK (
    field_key IS NULL OR (char_length(field_key) BETWEEN 2 AND 120 AND field_key ~ '^[a-z][a-z0-9_.-]+$')
  ),
  review_action TEXT CHECK (review_action IS NULL OR review_action IN ('ACCEPT', 'REJECT')),
  event_type TEXT NOT NULL CHECK (event_type IN (
    'JOB_CREATED', 'JOB_READ', 'JOB_STARTED', 'JOB_CANCELLED', 'JOB_COMPLETED', 'JOB_FAILED', 'JOB_RECOVERED',
    'SUGGESTIONS_READ', 'SUGGESTION_REVIEWED', 'APPLICATION_STARTED', 'APPLICATION_COMPLETED'
  )),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  affected_count INTEGER CHECK (affected_count IS NULL OR affected_count >= 0),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  request_id TEXT NOT NULL DEFAULT '' CHECK (char_length(request_id) <= 128),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (num_nonnulls(job_id, suggestion_id) <= 1),
  CHECK (
    (event_type = 'SUGGESTION_REVIEWED' AND suggestion_id IS NOT NULL AND field_key IS NOT NULL AND review_action IS NOT NULL) OR
    (event_type <> 'SUGGESTION_REVIEWED' AND field_key IS NULL AND review_action IS NULL)
  )
);
CREATE INDEX ocr_audit_events_actor_created_index
  ON ocr_audit_events (actor_user_id, created_at DESC, id DESC);

INSERT INTO app_metadata (key, value)
VALUES ('schema.multimodal_ocr', 'm13')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.multimodal_ocr';
DROP TABLE ocr_audit_events;
DROP TABLE ocr_usage_windows;
DROP TABLE ocr_apply_results;
DROP TABLE ocr_apply_receipts;
DROP TABLE ocr_suggestions;
DROP TABLE ocr_job_events;
DROP TABLE ocr_jobs;

-- M14 advanced read-only query tasks, durable jobs, events and explainable results.

CREATE TABLE task_drafts (
  id UUID PRIMARY KEY,
  owner_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  catalog_version TEXT NOT NULL CHECK (catalog_version ~ '^[a-f0-9]{64}$'),
  state TEXT NOT NULL CHECK (state IN ('PROPOSED', 'REVIEWED')),
  spec_json JSONB NOT NULL CHECK (jsonb_typeof(spec_json) = 'object' AND octet_length(spec_json::text) <= 262144),
  spec_fingerprint BYTEA NOT NULL CHECK (octet_length(spec_fingerprint) = 32),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (expires_at > created_at)
);
CREATE INDEX task_drafts_owner_updated_index ON task_drafts(owner_user_id, updated_at DESC, id DESC);
CREATE INDEX task_drafts_expiry_index ON task_drafts(expires_at, id);

CREATE TABLE task_jobs (
  id UUID PRIMARY KEY,
  owner_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  draft_id UUID NOT NULL REFERENCES task_drafts(id) ON DELETE CASCADE,
  retry_of_job_id UUID REFERENCES task_jobs(id) ON DELETE SET NULL,
  idempotency_key TEXT NOT NULL CHECK (
    char_length(idempotency_key) BETWEEN 8 AND 128
    AND idempotency_key ~ '^[A-Za-z0-9._:-]+$'
  ),
  request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint) = 32),
  catalog_version TEXT NOT NULL CHECK (catalog_version ~ '^[a-f0-9]{64}$'),
  state TEXT NOT NULL CHECK (state IN ('QUEUED','RUNNING','COMPLETED','INCOMPLETE','FAILED','CANCELLED')),
  river_job_id BIGINT CHECK (river_job_id IS NULL OR river_job_id > 0),
  attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count BETWEEN 0 AND 3),
  progress_current INTEGER NOT NULL DEFAULT 0 CHECK (progress_current >= 0),
  progress_total INTEGER NOT NULL DEFAULT 0 CHECK (progress_total >= 0),
  candidate_count INTEGER NOT NULL DEFAULT 0 CHECK (candidate_count >= 0),
  composition_count INTEGER NOT NULL DEFAULT 0 CHECK (composition_count BETWEEN 0 AND 100),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  cancel_requested_at TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  next_event_sequence BIGINT NOT NULL DEFAULT 1 CHECK (next_event_sequence > 0),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(owner_user_id, idempotency_key),
  CHECK (retry_of_job_id IS NULL OR retry_of_job_id <> id),
  CHECK (expires_at > created_at),
  CHECK (
    (state IN ('QUEUED','RUNNING') AND completed_at IS NULL AND error_code IS NULL) OR
    (state = 'COMPLETED' AND completed_at IS NOT NULL AND error_code IS NULL) OR
    (state IN ('INCOMPLETE','FAILED','CANCELLED') AND completed_at IS NOT NULL AND error_code IS NOT NULL)
  )
);
CREATE UNIQUE INDEX task_jobs_one_active_draft_index
  ON task_jobs(owner_user_id, draft_id) WHERE state IN ('QUEUED','RUNNING');
CREATE INDEX task_jobs_owner_created_index ON task_jobs(owner_user_id, created_at DESC, id DESC);
CREATE INDEX task_jobs_stale_index ON task_jobs(updated_at, id) WHERE state='RUNNING';
CREATE INDEX task_jobs_expiry_index ON task_jobs(expires_at, id);

CREATE TABLE task_job_events (
  job_id UUID NOT NULL REFERENCES task_jobs(id) ON DELETE CASCADE,
  sequence BIGINT NOT NULL CHECK (sequence > 0),
  event_kind TEXT NOT NULL CHECK (event_kind IN (
    'JOB_ACCEPTED','JOB_STARTED','CATALOG_VALIDATED','CANDIDATES_STARTED',
    'CANDIDATES_READY','SOLVER_STARTED','JOB_COMPLETED','JOB_INCOMPLETE','JOB_FAILED','JOB_CANCELLED'
  )),
  progress_current INTEGER CHECK (progress_current IS NULL OR progress_current >= 0),
  progress_total INTEGER CHECK (progress_total IS NULL OR progress_total >= 0),
  candidate_count INTEGER CHECK (candidate_count IS NULL OR candidate_count >= 0),
  result_count INTEGER CHECK (result_count IS NULL OR result_count BETWEEN 0 AND 100),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(job_id, sequence)
);
CREATE UNIQUE INDEX task_job_events_one_terminal_index
  ON task_job_events(job_id)
  WHERE event_kind IN ('JOB_COMPLETED','JOB_INCOMPLETE','JOB_FAILED','JOB_CANCELLED');

CREATE TABLE task_candidate_references (
  job_id UUID NOT NULL REFERENCES task_jobs(id) ON DELETE CASCADE,
  role_key TEXT NOT NULL CHECK (role_key ~ '^[a-z][a-z0-9_]{0,63}$'),
  entity_kind TEXT NOT NULL CHECK (entity_kind ~ '^[a-z][a-z0-9_.-]{0,199}$'),
  entity_id TEXT NOT NULL CHECK (entity_id=btrim(entity_id) AND char_length(entity_id) BETWEEN 1 AND 200),
  entity_label TEXT NOT NULL CHECK (entity_label=btrim(entity_label) AND char_length(entity_label) BETWEEN 1 AND 500),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(job_id, role_key, entity_kind, entity_id)
);

CREATE TABLE task_job_results (
  job_id UUID NOT NULL REFERENCES task_jobs(id) ON DELETE CASCADE,
  position INTEGER NOT NULL CHECK (position BETWEEN 0 AND 99),
  composition_json JSONB NOT NULL CHECK (jsonb_typeof(composition_json)='object' AND octet_length(composition_json::text) <= 5242880),
  evidence_count INTEGER NOT NULL CHECK (evidence_count BETWEEN 0 AND 512),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(job_id, position)
);

CREATE TABLE task_usage_windows (
  owner_user_id UUID PRIMARY KEY REFERENCES app_users(id) ON DELETE CASCADE,
  window_started_at TIMESTAMPTZ NOT NULL,
  request_count INTEGER NOT NULL CHECK (request_count > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE task_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  draft_id UUID REFERENCES task_drafts(id) ON DELETE SET NULL,
  job_id UUID REFERENCES task_jobs(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL CHECK (event_type IN (
    'DRAFT_CREATED','DRAFT_REVIEWED','JOB_CREATED','JOB_READ','JOB_STARTED',
    'JOB_CANCELLED','JOB_COMPLETED','JOB_FAILED','JOB_RECOVERED','RESULT_READ'
  )),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS','DENIED','FAILURE')),
  affected_count INTEGER CHECK (affected_count IS NULL OR affected_count >= 0),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  request_id TEXT NOT NULL DEFAULT '' CHECK (char_length(request_id) <= 128),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX task_audit_events_actor_created_index ON task_audit_events(actor_user_id, created_at DESC, id DESC);

INSERT INTO app_metadata(key, value)
VALUES ('schema.advanced_query_tasks', 'm14')
ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value, updated_at=now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key='schema.advanced_query_tasks';
DROP TABLE task_audit_events;
DROP TABLE task_usage_windows;
DROP TABLE task_job_results;
DROP TABLE task_candidate_references;
DROP TABLE task_job_events;
DROP TABLE task_jobs;
DROP TABLE task_drafts;

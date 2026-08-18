-- Retire the user Tasks workspace (Orchestration §20) and stop duplicating
-- canonical Profile columns as PROFILE custom field definitions (same as 029).

DELETE FROM app_user_capabilities WHERE capability = 'TASKS';

DO $$
DECLARE
  constraint_row record;
BEGIN
  FOR constraint_row IN
    SELECT conname
    FROM pg_constraint
    WHERE conrelid = 'app_user_capabilities'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) ILIKE '%TASKS%'
  LOOP
    EXECUTE format('ALTER TABLE app_user_capabilities DROP CONSTRAINT %I', constraint_row.conname);
  END LOOP;
END $$;
ALTER TABLE app_user_capabilities
  ADD CONSTRAINT app_user_capabilities_capability_check
  CHECK (capability IN (
    'PROFILES',
    'DATA_TABLES',
    'SEARCH',
    'OCR',
    'OPERATIONS',
    'MATCHING',
    'GOOGLE_FORMS',
    'ATTACHMENTS',
    'CHAT',
    'QUERY',
    'CUSTOM_DATA'
  ));

DELETE FROM app_metadata WHERE key = 'schema.advanced_query_tasks';
DROP TABLE IF EXISTS task_audit_events;
DROP TABLE IF EXISTS task_usage_windows;
DROP TABLE IF EXISTS task_job_results;
DROP TABLE IF EXISTS task_candidate_references;
DROP TABLE IF EXISTS task_job_events;
DROP TABLE IF EXISTS task_jobs;
DROP TABLE IF EXISTS task_drafts;

DELETE FROM custom_field_value_options
WHERE custom_field_value_id IN (
  SELECT value.id
  FROM custom_field_values AS value
  JOIN custom_field_definitions AS definition ON definition.id = value.field_definition_id
  WHERE definition.target_kind = 'PROFILE'
    AND definition.technical_key IN (
      'place_of_origin', 'birth_country', 'parents_wedding_date', 'supermarket_club',
      'pet', 'travel_countries', 'card_brand', 'card_bank'
    )
);
DELETE FROM custom_field_values
WHERE field_definition_id IN (
  SELECT id FROM custom_field_definitions
  WHERE target_kind = 'PROFILE'
    AND technical_key IN (
      'place_of_origin', 'birth_country', 'parents_wedding_date', 'supermarket_club',
      'pet', 'travel_countries', 'card_brand', 'card_bank'
    )
);
DELETE FROM custom_field_options
WHERE field_definition_id IN (
  SELECT id FROM custom_field_definitions
  WHERE target_kind = 'PROFILE'
    AND technical_key IN (
      'place_of_origin', 'birth_country', 'parents_wedding_date', 'supermarket_club',
      'pet', 'travel_countries', 'card_brand', 'card_bank'
    )
);
DELETE FROM attachment_upload_intents
WHERE field_definition_id IN (
  SELECT id FROM custom_field_definitions
  WHERE target_kind = 'PROFILE'
    AND technical_key IN (
      'place_of_origin', 'birth_country', 'parents_wedding_date', 'supermarket_club',
      'pet', 'travel_countries', 'card_brand', 'card_bank'
    )
);
DELETE FROM attachments
WHERE field_definition_id IN (
  SELECT id FROM custom_field_definitions
  WHERE target_kind = 'PROFILE'
    AND technical_key IN (
      'place_of_origin', 'birth_country', 'parents_wedding_date', 'supermarket_club',
      'pet', 'travel_countries', 'card_brand', 'card_bank'
    )
);
DELETE FROM custom_field_definitions
WHERE target_kind = 'PROFILE'
  AND technical_key IN (
    'place_of_origin', 'birth_country', 'parents_wedding_date', 'supermarket_club',
    'pet', 'travel_countries', 'card_brand', 'card_bank'
  );

DROP INDEX IF EXISTS profiles_full_name_trigram_index;

INSERT INTO app_metadata (key, value)
VALUES ('schema.retired_task_workspace', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.retired_task_workspace';

CREATE INDEX IF NOT EXISTS profiles_full_name_trigram_index
  ON profiles USING gin (lower(full_name) gin_trgm_ops);

WITH profile_fields(id, technical_key, label, field_kind, maximum_length) AS (
  VALUES
    ('024f0024-91e7-57eb-954b-000000000024'::uuid, 'place_of_origin', 'Naturalidade', 'TEXT', 100),
    ('024f0025-91e7-57eb-954b-000000000025'::uuid, 'birth_country', 'País de nascimento', 'TEXT', 80),
    ('024f0026-91e7-57eb-954b-000000000026'::uuid, 'parents_wedding_date', 'Casamento dos pais', 'CIVIL_DATE', NULL::integer),
    ('024f0027-91e7-57eb-954b-000000000027'::uuid, 'supermarket_club', 'Clube de supermercado', 'TEXT', 80),
    ('024f0028-91e7-57eb-954b-000000000028'::uuid, 'pet', 'Animal', 'TEXT', 120),
    ('024f0029-91e7-57eb-954b-000000000029'::uuid, 'travel_countries', 'Viagem', 'TEXT', 500),
    ('024f0030-91e7-57eb-954b-000000000030'::uuid, 'card_brand', 'Bandeira do cartão', 'TEXT', 120),
    ('024f0031-91e7-57eb-954b-000000000031'::uuid, 'card_bank', 'Banco do cartão', 'TEXT', 120)
)
INSERT INTO custom_field_definitions (
  id, target_kind, document_type_id, bill_type_id, custom_entity_type_id,
  technical_key, label, field_kind, required, active,
  minimum_length, maximum_length, validation_regex, minimum_decimal, maximum_decimal
)
SELECT
  profile_fields.id, 'PROFILE', NULL, NULL, NULL,
  profile_fields.technical_key, profile_fields.label, profile_fields.field_kind,
  false, true, NULL, profile_fields.maximum_length, NULL, NULL, NULL
FROM profile_fields
ON CONFLICT (technical_key) WHERE target_kind = 'PROFILE'
DO UPDATE SET
  label = EXCLUDED.label, field_kind = EXCLUDED.field_kind, active = true,
  maximum_length = EXCLUDED.maximum_length, updated_at = now();

-- Recreate 018 as-is so migrate-down-one is a real inverse.
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

ALTER TABLE app_user_capabilities DROP CONSTRAINT IF EXISTS app_user_capabilities_capability_check;
ALTER TABLE app_user_capabilities
  ADD CONSTRAINT app_user_capabilities_capability_check
  CHECK (capability IN (
    'PROFILES', 'DATA_TABLES', 'SEARCH', 'OCR', 'OPERATIONS', 'MATCHING',
    'GOOGLE_FORMS', 'ATTACHMENTS', 'CHAT', 'QUERY', 'TASKS', 'CUSTOM_DATA'
  ));

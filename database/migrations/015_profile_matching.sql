-- M11 on-demand Profile duplicate analysis, review decisions and transactional merge audit.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX profiles_full_name_trigram_index
  ON profiles USING gin (lower(full_name) gin_trgm_ops);
CREATE INDEX profiles_mobile_phone_matching_index
  ON profiles (mobile_phone) WHERE mobile_phone IS NOT NULL;
CREATE INDEX profiles_landline_phone_matching_index
  ON profiles (landline_phone) WHERE landline_phone IS NOT NULL;
CREATE INDEX profiles_postal_code_matching_index
  ON profiles (address_postal_code) WHERE address_postal_code IS NOT NULL;

CREATE TABLE matching_analyses (
  id UUID PRIMARY KEY,
  actor_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE RESTRICT,
  idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 128),
  state TEXT NOT NULL CHECK (state IN ('QUEUED', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED')),
  river_job_id BIGINT,
  profiles_scanned INTEGER NOT NULL DEFAULT 0 CHECK (profiles_scanned >= 0),
  candidate_count INTEGER NOT NULL DEFAULT 0 CHECK (candidate_count >= 0),
  refreshed_count INTEGER NOT NULL DEFAULT 0 CHECK (refreshed_count >= 0),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  cancel_requested_at TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (actor_user_id, idempotency_key),
  CHECK (expires_at > created_at),
  CHECK (started_at IS NULL OR state IN ('RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED')),
  CHECK (completed_at IS NULL OR state IN ('COMPLETED', 'FAILED', 'CANCELLED')),
  CHECK (cancel_requested_at IS NULL OR state IN ('QUEUED', 'RUNNING', 'CANCELLED'))
);

CREATE UNIQUE INDEX matching_analyses_one_active_actor_index
  ON matching_analyses (actor_user_id)
  WHERE state IN ('QUEUED', 'RUNNING');
CREATE INDEX matching_analyses_actor_created_index
  ON matching_analyses (actor_user_id, created_at DESC, id DESC);
CREATE INDEX matching_analyses_expiry_index
  ON matching_analyses (expires_at, id)
  WHERE state IN ('COMPLETED', 'FAILED', 'CANCELLED');

CREATE TABLE matching_cases (
  id UUID PRIMARY KEY,
  left_profile_id UUID NOT NULL,
  right_profile_id UUID NOT NULL,
  left_profile_version BIGINT NOT NULL CHECK (left_profile_version > 0),
  right_profile_version BIGINT NOT NULL CHECK (right_profile_version > 0),
  score SMALLINT NOT NULL CHECK (score BETWEEN 0 AND 100),
  score_band TEXT NOT NULL CHECK (score_band IN ('LOW', 'MEDIUM', 'HIGH')),
  state TEXT NOT NULL CHECK (state IN ('PENDING', 'NOT_DUPLICATE', 'MERGED', 'STALE')),
  first_analysis_id UUID REFERENCES matching_analyses(id) ON DELETE SET NULL,
  last_analysis_id UUID REFERENCES matching_analyses(id) ON DELETE SET NULL,
  decided_by_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  decided_at TIMESTAMPTZ,
  merged_survivor_id UUID,
  merged_source_id UUID,
  merged_at TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (left_profile_id, right_profile_id),
  CHECK (left_profile_id < right_profile_id),
  CHECK (decided_by_user_id IS NULL OR decided_at IS NOT NULL),
  CHECK (
    (state = 'MERGED' AND merged_survivor_id IS NOT NULL AND merged_source_id IS NOT NULL AND merged_at IS NOT NULL) OR
    (state <> 'MERGED' AND merged_survivor_id IS NULL AND merged_source_id IS NULL AND merged_at IS NULL)
  ),
  CHECK (merged_survivor_id IS NULL OR merged_survivor_id IN (left_profile_id, right_profile_id)),
  CHECK (merged_source_id IS NULL OR merged_source_id IN (left_profile_id, right_profile_id)),
  CHECK (merged_survivor_id IS NULL OR merged_survivor_id <> merged_source_id)
);

CREATE INDEX matching_cases_queue_index
  ON matching_cases (state, score DESC, updated_at DESC, id DESC);
CREATE INDEX matching_cases_left_profile_index
  ON matching_cases (left_profile_id, state, updated_at DESC);
CREATE INDEX matching_cases_right_profile_index
  ON matching_cases (right_profile_id, state, updated_at DESC);

CREATE TABLE matching_case_evidence (
  case_id UUID NOT NULL REFERENCES matching_cases(id) ON DELETE CASCADE,
  evidence_kind TEXT NOT NULL CHECK (evidence_kind IN (
    'CPF_EXACT', 'EMAIL_EXACT', 'MOBILE_EXACT', 'LANDLINE_EXACT',
    'NAME_EXACT', 'NAME_SIMILAR', 'POSTAL_EXACT', 'CITY_EXACT', 'ADDRESS_SIMILAR'
  )),
  strength SMALLINT NOT NULL CHECK (strength BETWEEN 0 AND 100),
  contribution SMALLINT NOT NULL CHECK (contribution BETWEEN 0 AND 100),
  PRIMARY KEY (case_id, evidence_kind)
);

CREATE TABLE matching_analysis_cases (
  analysis_id UUID NOT NULL REFERENCES matching_analyses(id) ON DELETE CASCADE,
  case_id UUID NOT NULL REFERENCES matching_cases(id) ON DELETE RESTRICT,
  refreshed BOOLEAN NOT NULL,
  PRIMARY KEY (analysis_id, case_id)
);

CREATE TABLE matching_case_decisions (
  id UUID PRIMARY KEY,
  case_id UUID NOT NULL REFERENCES matching_cases(id) ON DELETE RESTRICT,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  action TEXT NOT NULL CHECK (action IN ('NOT_DUPLICATE', 'MERGED')),
  left_profile_version BIGINT NOT NULL CHECK (left_profile_version > 0),
  right_profile_version BIGINT NOT NULL CHECK (right_profile_version > 0),
  survivor_profile_id UUID,
  source_profile_id UUID,
  preview_fingerprint BYTEA CHECK (preview_fingerprint IS NULL OR octet_length(preview_fingerprint) = 32),
  request_id TEXT NOT NULL CHECK (char_length(request_id) BETWEEN 1 AND 128),
  decided_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (
    (action = 'NOT_DUPLICATE' AND survivor_profile_id IS NULL AND source_profile_id IS NULL AND preview_fingerprint IS NULL) OR
    (action = 'MERGED' AND survivor_profile_id IS NOT NULL AND source_profile_id IS NOT NULL AND
      survivor_profile_id <> source_profile_id AND preview_fingerprint IS NOT NULL)
  )
);

CREATE INDEX matching_case_decisions_case_index
  ON matching_case_decisions (case_id, decided_at DESC, id DESC);

CREATE TABLE matching_merge_receipts (
  id UUID PRIMARY KEY,
  actor_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE RESTRICT,
  case_id UUID NOT NULL REFERENCES matching_cases(id) ON DELETE RESTRICT,
  idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 128),
  request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint) = 32),
  survivor_profile_id UUID NOT NULL,
  source_profile_id UUID NOT NULL,
  survivor_version BIGINT NOT NULL CHECK (survivor_version > 0),
  merged_at TIMESTAMPTZ NOT NULL,
  UNIQUE (actor_user_id, idempotency_key),
  CHECK (survivor_profile_id <> source_profile_id)
);

CREATE TABLE matching_merge_receipt_counts (
  receipt_id UUID NOT NULL REFERENCES matching_merge_receipts(id) ON DELETE CASCADE,
  dependency_kind TEXT NOT NULL CHECK (dependency_kind IN (
    'DOCUMENT_OWNER', 'DOCUMENT_HOLDER', 'BILL_OWNER', 'BILL_HOLDER',
    'CUSTOM_ENTITY_OWNER', 'CUSTOM_PROFILE_VALUE', 'ATTACHMENT_INTENT', 'ATTACHMENT'
  )),
  affected_count INTEGER NOT NULL CHECK (affected_count >= 0),
  PRIMARY KEY (receipt_id, dependency_kind)
);

CREATE TABLE matching_rate_limits (
  actor_user_id UUID PRIMARY KEY REFERENCES app_users(id) ON DELETE CASCADE,
  window_started_at TIMESTAMPTZ NOT NULL,
  request_count INTEGER NOT NULL CHECK (request_count > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE matching_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  analysis_id UUID REFERENCES matching_analyses(id) ON DELETE SET NULL,
  case_id UUID REFERENCES matching_cases(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL CHECK (event_type IN (
    'ANALYSIS_CREATED', 'ANALYSIS_STARTED', 'ANALYSIS_COMPLETED', 'ANALYSIS_CANCELLED', 'ANALYSIS_FAILED',
    'CASE_REVIEWED', 'CASE_DISMISSED', 'MERGE_PREVIEWED', 'MERGE_COMPLETED', 'MERGE_DENIED'
  )),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  score_band TEXT CHECK (score_band IS NULL OR score_band IN ('LOW', 'MEDIUM', 'HIGH')),
  affected_count INTEGER CHECK (affected_count IS NULL OR affected_count >= 0),
  error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) BETWEEN 1 AND 80),
  request_id TEXT NOT NULL CHECK (char_length(request_id) BETWEEN 1 AND 128),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (num_nonnulls(analysis_id, case_id) <= 1)
);

CREATE INDEX matching_audit_events_actor_created_index
  ON matching_audit_events (actor_user_id, created_at DESC, id DESC);
CREATE INDEX matching_audit_events_case_created_index
  ON matching_audit_events (case_id, created_at DESC, id DESC)
  WHERE case_id IS NOT NULL;

ALTER TABLE profile_audit_events
  DROP CONSTRAINT profile_audit_events_event_type_check;
ALTER TABLE profile_audit_events
  ADD CONSTRAINT profile_audit_events_event_type_check CHECK (
    event_type IN ('PROFILE_CREATED', 'PROFILE_UPDATED', 'PROFILE_DUPLICATED', 'PROFILE_DELETED', 'PROFILE_MERGED')
  );

INSERT INTO app_metadata (key, value)
VALUES ('schema.profile_matching', 'm11')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.profile_matching';

DROP TABLE matching_audit_events;
DROP TABLE matching_rate_limits;
DROP TABLE matching_merge_receipt_counts;
DROP TABLE matching_merge_receipts;
DROP TABLE matching_case_decisions;
DROP TABLE matching_analysis_cases;
DROP TABLE matching_case_evidence;
DROP TABLE matching_cases;
DROP TABLE matching_analyses;
DROP INDEX profiles_postal_code_matching_index;
DROP INDEX profiles_landline_phone_matching_index;
DROP INDEX profiles_mobile_phone_matching_index;
DROP INDEX profiles_full_name_trigram_index;

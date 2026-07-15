-- M3 protected Profile operations and durable audit trail.

CREATE TABLE profile_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  profile_id UUID NOT NULL,
  source_profile_id UUID,
  event_type TEXT NOT NULL CHECK (
    event_type IN ('PROFILE_CREATED', 'PROFILE_UPDATED', 'PROFILE_DUPLICATED', 'PROFILE_DELETED')
  ),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  request_id TEXT NOT NULL CHECK (request_id <> '' AND char_length(request_id) <= 128),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX profile_audit_events_occurred_at_index
  ON profile_audit_events (occurred_at DESC);

CREATE INDEX profile_audit_events_actor_index
  ON profile_audit_events (actor_user_id, occurred_at DESC);

CREATE INDEX profile_audit_events_profile_index
  ON profile_audit_events (profile_id, occurred_at DESC);

INSERT INTO app_metadata (key, value)
VALUES ('schema.profile_api', 'm3.2')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.profile_api';
DROP TABLE profile_audit_events;

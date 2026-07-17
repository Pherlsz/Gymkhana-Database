-- M5 protected custom-data API audit and integrity hardening.

CREATE TABLE custom_data_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE RESTRICT,
  resource_kind TEXT NOT NULL CHECK (
    resource_kind IN ('ENTITY_TYPE', 'FIELD_DEFINITION', 'FIELD_OPTION', 'CUSTOM_ENTITY', 'FIELD_VALUES')
  ),
  resource_id UUID,
  target_kind TEXT CHECK (
    target_kind IS NULL OR target_kind IN ('PROFILE', 'DOCUMENT', 'BILL', 'CUSTOM_ENTITY')
  ),
  target_id UUID,
  event_type TEXT NOT NULL CHECK (
    event_type IN (
      'CUSTOM_ENTITY_TYPE_CREATED', 'CUSTOM_ENTITY_TYPE_UPDATED', 'CUSTOM_ENTITY_TYPE_DELETED',
      'CUSTOM_FIELD_CREATED', 'CUSTOM_FIELD_UPDATED', 'CUSTOM_FIELD_DELETED',
      'CUSTOM_FIELD_OPTION_CREATED', 'CUSTOM_FIELD_OPTION_UPDATED', 'CUSTOM_FIELD_OPTION_DELETED',
      'CUSTOM_ENTITY_CREATED', 'CUSTOM_ENTITY_UPDATED', 'CUSTOM_ENTITY_DELETED',
      'CUSTOM_FIELD_VALUES_REPLACED'
    )
  ),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  request_id TEXT NOT NULL CHECK (request_id = btrim(request_id) AND request_id <> '' AND char_length(request_id) <= 128),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((target_kind IS NULL) = (target_id IS NULL))
);

CREATE INDEX custom_data_audit_events_resource_index
  ON custom_data_audit_events (resource_kind, resource_id, created_at DESC, id);
CREATE INDEX custom_data_audit_events_target_index
  ON custom_data_audit_events (target_kind, target_id, created_at DESC, id)
  WHERE target_id IS NOT NULL;
CREATE INDEX custom_data_audit_events_actor_index
  ON custom_data_audit_events (actor_user_id, created_at DESC, id);

-- A custom entity snapshots the cardinality configured by its type. Enforce the
-- snapshot at the database boundary so callers cannot bypass domain validation.
CREATE OR REPLACE FUNCTION enforce_custom_entity_type_contract()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  configured_cardinality TEXT;
  configured_active BOOLEAN;
BEGIN
  SELECT profile_cardinality, active
    INTO configured_cardinality, configured_active
    FROM custom_entity_types
   WHERE id = NEW.custom_entity_type_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'custom entity type not found' USING ERRCODE = '23503';
  END IF;
  IF NOT configured_active THEN
    RAISE EXCEPTION 'custom entity type is inactive' USING ERRCODE = '23514';
  END IF;
  IF NEW.profile_cardinality IS DISTINCT FROM configured_cardinality THEN
    RAISE EXCEPTION 'custom entity cardinality does not match its type' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER custom_entities_type_contract
BEFORE INSERT OR UPDATE OF custom_entity_type_id, owner_profile_id, profile_cardinality
ON custom_entities
FOR EACH ROW EXECUTE FUNCTION enforce_custom_entity_type_contract();

INSERT INTO app_metadata (key, value)
VALUES ('schema.custom_data_api', 'm5')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.custom_data_api';
DROP TRIGGER custom_entities_type_contract ON custom_entities;
DROP FUNCTION enforce_custom_entity_type_contract();
DROP TABLE custom_data_audit_events;

-- M6 private attachments, upload intents, trash, recovery and audit persistence.

ALTER TABLE custom_field_definitions
  DROP CONSTRAINT custom_field_definitions_field_kind_check;
ALTER TABLE custom_field_definitions
  ADD CONSTRAINT custom_field_definitions_field_kind_check CHECK (
    field_kind IN (
      'TEXT', 'LONG_TEXT', 'INTEGER', 'DECIMAL', 'BOOLEAN', 'CIVIL_DATE',
      'CIVIL_MONTH', 'EMAIL', 'PHONE', 'SINGLE_SELECT', 'MULTI_SELECT', 'ATTACHMENT'
    )
  );

CREATE TABLE attachment_upload_intents (
  id UUID PRIMARY KEY,
  actor_user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE RESTRICT,
  owner_kind TEXT NOT NULL CHECK (owner_kind IN ('DOCUMENT', 'BILL', 'CUSTOM_FIELD')),
  document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
  bill_id UUID REFERENCES bills(id) ON DELETE CASCADE,
  custom_target_kind TEXT CHECK (
    custom_target_kind IS NULL OR custom_target_kind IN ('PROFILE', 'DOCUMENT', 'BILL', 'CUSTOM_ENTITY')
  ),
  custom_profile_id UUID REFERENCES profiles(id) ON DELETE CASCADE,
  custom_document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
  custom_bill_id UUID REFERENCES bills(id) ON DELETE CASCADE,
  custom_entity_id UUID REFERENCES custom_entities(id) ON DELETE CASCADE,
  field_definition_id UUID REFERENCES custom_field_definitions(id) ON DELETE RESTRICT,
  original_filename TEXT NOT NULL CHECK (
    original_filename = btrim(original_filename) AND original_filename <> '' AND char_length(original_filename) <= 255
  ),
  declared_mime TEXT NOT NULL CHECK (
    declared_mime = lower(btrim(declared_mime)) AND declared_mime <> '' AND char_length(declared_mime) <= 127
  ),
  expected_size BIGINT NOT NULL CHECK (expected_size > 0),
  object_key TEXT NOT NULL UNIQUE CHECK (object_key <> '' AND char_length(object_key) <= 512),
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (expires_at > created_at),
  CHECK (
    (owner_kind = 'DOCUMENT' AND document_id IS NOT NULL AND bill_id IS NULL AND custom_target_kind IS NULL AND
      custom_profile_id IS NULL AND custom_document_id IS NULL AND custom_bill_id IS NULL AND custom_entity_id IS NULL AND
      field_definition_id IS NULL) OR
    (owner_kind = 'BILL' AND document_id IS NULL AND bill_id IS NOT NULL AND custom_target_kind IS NULL AND
      custom_profile_id IS NULL AND custom_document_id IS NULL AND custom_bill_id IS NULL AND custom_entity_id IS NULL AND
      field_definition_id IS NULL) OR
    (owner_kind = 'CUSTOM_FIELD' AND document_id IS NULL AND bill_id IS NULL AND custom_target_kind IS NOT NULL AND
      field_definition_id IS NOT NULL AND num_nonnulls(custom_profile_id, custom_document_id, custom_bill_id, custom_entity_id) = 1 AND
      ((custom_target_kind = 'PROFILE' AND custom_profile_id IS NOT NULL) OR
       (custom_target_kind = 'DOCUMENT' AND custom_document_id IS NOT NULL) OR
       (custom_target_kind = 'BILL' AND custom_bill_id IS NOT NULL) OR
       (custom_target_kind = 'CUSTOM_ENTITY' AND custom_entity_id IS NOT NULL)))
  )
);
CREATE INDEX attachment_upload_intents_expiry_index
  ON attachment_upload_intents (expires_at, id) WHERE consumed_at IS NULL;
CREATE INDEX attachment_upload_intents_actor_index
  ON attachment_upload_intents (actor_user_id, created_at DESC, id);

CREATE TABLE attachments (
  id UUID PRIMARY KEY,
  owner_kind TEXT NOT NULL CHECK (owner_kind IN ('DOCUMENT', 'BILL', 'CUSTOM_FIELD')),
  document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
  bill_id UUID REFERENCES bills(id) ON DELETE CASCADE,
  custom_target_kind TEXT CHECK (
    custom_target_kind IS NULL OR custom_target_kind IN ('PROFILE', 'DOCUMENT', 'BILL', 'CUSTOM_ENTITY')
  ),
  custom_profile_id UUID REFERENCES profiles(id) ON DELETE CASCADE,
  custom_document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
  custom_bill_id UUID REFERENCES bills(id) ON DELETE CASCADE,
  custom_entity_id UUID REFERENCES custom_entities(id) ON DELETE CASCADE,
  field_definition_id UUID REFERENCES custom_field_definitions(id) ON DELETE RESTRICT,
  original_filename TEXT NOT NULL CHECK (
    original_filename = btrim(original_filename) AND original_filename <> '' AND char_length(original_filename) <= 255
  ),
  declared_mime TEXT NOT NULL CHECK (
    declared_mime = lower(btrim(declared_mime)) AND declared_mime <> '' AND char_length(declared_mime) <= 127
  ),
  detected_mime TEXT NOT NULL CHECK (
    detected_mime = lower(btrim(detected_mime)) AND detected_mime <> '' AND char_length(detected_mime) <= 127
  ),
  byte_size BIGINT NOT NULL CHECK (byte_size > 0),
  sha256 BYTEA NOT NULL CHECK (octet_length(sha256) = 32),
  object_key TEXT NOT NULL UNIQUE CHECK (object_key <> '' AND char_length(object_key) <= 512),
  lifecycle_state TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (lifecycle_state IN ('ACTIVE', 'TRASHED')),
  deleted_at TIMESTAMPTZ,
  purge_after TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (
    (lifecycle_state = 'ACTIVE' AND deleted_at IS NULL AND purge_after IS NULL) OR
    (lifecycle_state = 'TRASHED' AND deleted_at IS NOT NULL AND purge_after IS NOT NULL AND purge_after > deleted_at)
  ),
  CHECK (
    (owner_kind = 'DOCUMENT' AND document_id IS NOT NULL AND bill_id IS NULL AND custom_target_kind IS NULL AND
      custom_profile_id IS NULL AND custom_document_id IS NULL AND custom_bill_id IS NULL AND custom_entity_id IS NULL AND
      field_definition_id IS NULL) OR
    (owner_kind = 'BILL' AND document_id IS NULL AND bill_id IS NOT NULL AND custom_target_kind IS NULL AND
      custom_profile_id IS NULL AND custom_document_id IS NULL AND custom_bill_id IS NULL AND custom_entity_id IS NULL AND
      field_definition_id IS NULL) OR
    (owner_kind = 'CUSTOM_FIELD' AND document_id IS NULL AND bill_id IS NULL AND custom_target_kind IS NOT NULL AND
      field_definition_id IS NOT NULL AND num_nonnulls(custom_profile_id, custom_document_id, custom_bill_id, custom_entity_id) = 1 AND
      ((custom_target_kind = 'PROFILE' AND custom_profile_id IS NOT NULL) OR
       (custom_target_kind = 'DOCUMENT' AND custom_document_id IS NOT NULL) OR
       (custom_target_kind = 'BILL' AND custom_bill_id IS NOT NULL) OR
       (custom_target_kind = 'CUSTOM_ENTITY' AND custom_entity_id IS NOT NULL)))
  )
);
CREATE INDEX attachments_document_index
  ON attachments (document_id, lifecycle_state, created_at DESC, id) WHERE document_id IS NOT NULL;
CREATE INDEX attachments_bill_index
  ON attachments (bill_id, lifecycle_state, created_at DESC, id) WHERE bill_id IS NOT NULL;
CREATE INDEX attachments_custom_profile_index
  ON attachments (custom_profile_id, field_definition_id, lifecycle_state, created_at DESC, id)
  WHERE custom_profile_id IS NOT NULL;
CREATE INDEX attachments_custom_document_index
  ON attachments (custom_document_id, field_definition_id, lifecycle_state, created_at DESC, id)
  WHERE custom_document_id IS NOT NULL;
CREATE INDEX attachments_custom_bill_index
  ON attachments (custom_bill_id, field_definition_id, lifecycle_state, created_at DESC, id)
  WHERE custom_bill_id IS NOT NULL;
CREATE INDEX attachments_custom_entity_index
  ON attachments (custom_entity_id, field_definition_id, lifecycle_state, created_at DESC, id)
  WHERE custom_entity_id IS NOT NULL;
CREATE INDEX attachments_purge_index
  ON attachments (purge_after, id) WHERE lifecycle_state = 'TRASHED';

CREATE TABLE attachment_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE RESTRICT,
  attachment_id UUID,
  upload_intent_id UUID,
  owner_kind TEXT CHECK (owner_kind IS NULL OR owner_kind IN ('DOCUMENT', 'BILL', 'CUSTOM_FIELD')),
  owner_id UUID,
  event_type TEXT NOT NULL CHECK (
    event_type IN (
      'ATTACHMENT_UPLOAD_INTENT_CREATED', 'ATTACHMENT_UPLOAD_CONFIRMED',
      'ATTACHMENT_DOWNLOAD_REQUESTED', 'ATTACHMENT_TRASHED', 'ATTACHMENT_RESTORED',
      'ATTACHMENT_PURGED', 'ATTACHMENT_EXPIRED_UPLOAD_CLEANED'
    )
  ),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  request_id TEXT NOT NULL CHECK (request_id = btrim(request_id) AND request_id <> '' AND char_length(request_id) <= 128),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((owner_kind IS NULL) = (owner_id IS NULL))
);
CREATE INDEX attachment_audit_events_attachment_index
  ON attachment_audit_events (attachment_id, created_at DESC, id) WHERE attachment_id IS NOT NULL;
CREATE INDEX attachment_audit_events_intent_index
  ON attachment_audit_events (upload_intent_id, created_at DESC, id) WHERE upload_intent_id IS NOT NULL;
CREATE INDEX attachment_audit_events_actor_index
  ON attachment_audit_events (actor_user_id, created_at DESC, id) WHERE actor_user_id IS NOT NULL;

CREATE OR REPLACE FUNCTION validate_attachment_custom_owner()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  configured_kind TEXT;
  configured_target UUID;
  actual_target UUID;
BEGIN
  IF NEW.owner_kind <> 'CUSTOM_FIELD' THEN
    RETURN NEW;
  END IF;

  SELECT target_kind,
         CASE target_kind
           WHEN 'DOCUMENT_TYPE' THEN document_type_id
           WHEN 'BILL_TYPE' THEN bill_type_id
           WHEN 'CUSTOM_ENTITY_TYPE' THEN custom_entity_type_id
           ELSE NULL
         END
    INTO configured_kind, configured_target
    FROM custom_field_definitions
   WHERE id = NEW.field_definition_id
     AND field_kind = 'ATTACHMENT'
     AND active = true;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'attachment field definition not found or inactive' USING ERRCODE = '23514';
  END IF;

  IF NEW.custom_target_kind = 'PROFILE' THEN
    IF configured_kind <> 'PROFILE' THEN
      RAISE EXCEPTION 'attachment field does not target profiles' USING ERRCODE = '23514';
    END IF;
  ELSIF NEW.custom_target_kind = 'DOCUMENT' THEN
    SELECT document_type_id INTO actual_target FROM documents WHERE id = NEW.custom_document_id;
    IF NOT FOUND OR configured_kind <> 'DOCUMENT_TYPE' OR configured_target IS DISTINCT FROM actual_target THEN
      RAISE EXCEPTION 'attachment field does not target this document type' USING ERRCODE = '23514';
    END IF;
  ELSIF NEW.custom_target_kind = 'BILL' THEN
    SELECT bill_type_id INTO actual_target FROM bills WHERE id = NEW.custom_bill_id;
    IF NOT FOUND OR configured_kind <> 'BILL_TYPE' OR configured_target IS DISTINCT FROM actual_target THEN
      RAISE EXCEPTION 'attachment field does not target this bill type' USING ERRCODE = '23514';
    END IF;
  ELSIF NEW.custom_target_kind = 'CUSTOM_ENTITY' THEN
    SELECT custom_entity_type_id INTO actual_target FROM custom_entities WHERE id = NEW.custom_entity_id;
    IF NOT FOUND OR configured_kind <> 'CUSTOM_ENTITY_TYPE' OR configured_target IS DISTINCT FROM actual_target THEN
      RAISE EXCEPTION 'attachment field does not target this custom entity type' USING ERRCODE = '23514';
    END IF;
  ELSE
    RAISE EXCEPTION 'invalid attachment custom target kind' USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER attachment_upload_intents_custom_owner
BEFORE INSERT OR UPDATE OF owner_kind, custom_target_kind, custom_profile_id, custom_document_id, custom_bill_id, custom_entity_id, field_definition_id
ON attachment_upload_intents
FOR EACH ROW EXECUTE FUNCTION validate_attachment_custom_owner();

CREATE TRIGGER attachments_custom_owner
BEFORE INSERT OR UPDATE OF owner_kind, custom_target_kind, custom_profile_id, custom_document_id, custom_bill_id, custom_entity_id, field_definition_id
ON attachments
FOR EACH ROW EXECUTE FUNCTION validate_attachment_custom_owner();

INSERT INTO app_metadata (key, value)
VALUES ('schema.attachments', 'm6')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.attachments';
DROP TRIGGER attachments_custom_owner ON attachments;
DROP TRIGGER attachment_upload_intents_custom_owner ON attachment_upload_intents;
DROP FUNCTION validate_attachment_custom_owner();
DROP TABLE attachment_audit_events;
DROP TABLE attachments;
DROP TABLE attachment_upload_intents;
ALTER TABLE custom_field_definitions
  DROP CONSTRAINT custom_field_definitions_field_kind_check;
ALTER TABLE custom_field_definitions
  ADD CONSTRAINT custom_field_definitions_field_kind_check CHECK (
    field_kind IN (
      'TEXT', 'LONG_TEXT', 'INTEGER', 'DECIMAL', 'BOOLEAN', 'CIVIL_DATE',
      'CIVIL_MONTH', 'EMAIL', 'PHONE', 'SINGLE_SELECT', 'MULTI_SELECT'
    )
  );

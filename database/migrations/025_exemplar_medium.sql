-- Exemplars have a medium. Identity on the profile is not a document.
-- Drop lifecycle record_state and type-level supports_current_use.

DROP TRIGGER IF EXISTS profiles_sync_cpf_document_trigger ON profiles;
DROP FUNCTION IF EXISTS sync_profile_cpf_document();

DELETE FROM document_current_uses
WHERE document_id IN (
  SELECT document.id
  FROM documents AS document
  JOIN document_types AS document_type ON document_type.id = document.document_type_id
  WHERE document_type.technical_key = 'cpf'
);

DELETE FROM documents
WHERE document_type_id IN (SELECT id FROM document_types WHERE technical_key = 'cpf');

ALTER TABLE documents
  ADD COLUMN medium TEXT NOT NULL DEFAULT 'PHYSICAL'
  CHECK (medium IN ('PHYSICAL', 'DIGITAL'));
ALTER TABLE bills
  ADD COLUMN medium TEXT NOT NULL DEFAULT 'PHYSICAL'
  CHECK (medium IN ('PHYSICAL', 'DIGITAL'));

DROP INDEX documents_per_profile_unique;
DROP INDEX documents_global_by_type_unique;
CREATE UNIQUE INDEX documents_per_profile_unique
  ON documents (owner_profile_id, document_type_id, identifier_value, medium)
  WHERE uniqueness_policy = 'PER_PROFILE';
CREATE UNIQUE INDEX documents_global_by_type_unique
  ON documents (document_type_id, identifier_value, medium)
  WHERE uniqueness_policy = 'GLOBAL_BY_TYPE';

ALTER TABLE documents ALTER COLUMN medium DROP DEFAULT;
ALTER TABLE bills ALTER COLUMN medium DROP DEFAULT;

ALTER TABLE documents DROP COLUMN record_state;
ALTER TABLE bills DROP COLUMN record_state;

DROP TRIGGER IF EXISTS bill_current_uses_supported_trigger ON bill_current_uses;
DROP FUNCTION IF EXISTS ensure_bill_current_use_supported();
ALTER TABLE bill_types DROP COLUMN supports_current_use;

CREATE FUNCTION ensure_document_current_use_physical() RETURNS trigger AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM documents WHERE id = NEW.document_id AND medium = 'PHYSICAL'
  ) THEN
    RAISE EXCEPTION 'document medium does not support current use' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER document_current_uses_physical_trigger
BEFORE INSERT OR UPDATE ON document_current_uses
FOR EACH ROW EXECUTE FUNCTION ensure_document_current_use_physical();

CREATE FUNCTION ensure_bill_current_use_physical() RETURNS trigger AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM bills WHERE id = NEW.bill_id AND medium = 'PHYSICAL'
  ) THEN
    RAISE EXCEPTION 'bill medium does not support current use' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER bill_current_uses_physical_trigger
BEFORE INSERT OR UPDATE ON bill_current_uses
FOR EACH ROW EXECUTE FUNCTION ensure_bill_current_use_physical();

INSERT INTO app_metadata (key, value)
VALUES ('schema.exemplar_medium', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DROP TRIGGER IF EXISTS bill_current_uses_physical_trigger ON bill_current_uses;
DROP FUNCTION IF EXISTS ensure_bill_current_use_physical();
DROP TRIGGER IF EXISTS document_current_uses_physical_trigger ON document_current_uses;
DROP FUNCTION IF EXISTS ensure_document_current_use_physical();

ALTER TABLE bill_types ADD COLUMN supports_current_use BOOLEAN NOT NULL DEFAULT false;
CREATE FUNCTION ensure_bill_current_use_supported() RETURNS trigger AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM bills AS bill
    JOIN bill_types AS bill_type ON bill_type.id = bill.bill_type_id
    WHERE bill.id = NEW.bill_id AND bill_type.supports_current_use
  ) THEN
    RAISE EXCEPTION 'bill type does not support current use' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER bill_current_uses_supported_trigger
BEFORE INSERT OR UPDATE ON bill_current_uses
FOR EACH ROW EXECUTE FUNCTION ensure_bill_current_use_supported();

ALTER TABLE documents ADD COLUMN record_state TEXT NOT NULL DEFAULT 'CURRENT'
  CHECK (record_state IN ('CURRENT', 'REPLACED', 'EXPIRED', 'ARCHIVED'));
ALTER TABLE bills ADD COLUMN record_state TEXT NOT NULL DEFAULT 'CURRENT'
  CHECK (record_state IN ('CURRENT', 'REPLACED', 'EXPIRED', 'ARCHIVED'));

DROP INDEX documents_per_profile_unique;
DROP INDEX documents_global_by_type_unique;
CREATE UNIQUE INDEX documents_per_profile_unique
  ON documents (owner_profile_id, document_type_id, identifier_value)
  WHERE uniqueness_policy = 'PER_PROFILE';
CREATE UNIQUE INDEX documents_global_by_type_unique
  ON documents (document_type_id, identifier_value)
  WHERE uniqueness_policy = 'GLOBAL_BY_TYPE';

ALTER TABLE documents DROP COLUMN medium;
ALTER TABLE bills DROP COLUMN medium;
DELETE FROM app_metadata WHERE key = 'schema.exemplar_medium';

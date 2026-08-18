-- Optional document numbers, profile_details blood/membership columns, and
-- at most one missing-number document per owner/type/medium.

ALTER TABLE profile_details ADD COLUMN IF NOT EXISTS blood_type TEXT;
ALTER TABLE profile_details ADD COLUMN IF NOT EXISTS membership_type TEXT;

ALTER TABLE documents ALTER COLUMN identifier_value DROP NOT NULL;
ALTER TABLE documents DROP CONSTRAINT IF EXISTS documents_identifier_value_check;
ALTER TABLE documents ADD CONSTRAINT documents_identifier_value_check
  CHECK (identifier_value IS NULL OR (identifier_value = btrim(identifier_value) AND identifier_value <> '' AND char_length(identifier_value) <= 500));

CREATE UNIQUE INDEX IF NOT EXISTS documents_missing_number_physical
  ON documents (owner_profile_id, document_type_id)
  WHERE identifier_value IS NULL AND medium = 'PHYSICAL';
CREATE UNIQUE INDEX IF NOT EXISTS documents_missing_number_digital
  ON documents (owner_profile_id, document_type_id)
  WHERE identifier_value IS NULL AND medium = 'DIGITAL';
CREATE UNIQUE INDEX IF NOT EXISTS documents_missing_number_unspecified
  ON documents (owner_profile_id, document_type_id)
  WHERE identifier_value IS NULL AND medium IS NULL;

INSERT INTO app_metadata (key, value)
VALUES ('schema.document_identifier_optional', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.document_identifier_optional';
DROP INDEX IF EXISTS documents_missing_number_unspecified;
DROP INDEX IF EXISTS documents_missing_number_digital;
DROP INDEX IF EXISTS documents_missing_number_physical;
ALTER TABLE documents DROP CONSTRAINT IF EXISTS documents_identifier_value_check;
UPDATE documents SET identifier_value = 'PENDENTE' WHERE identifier_value IS NULL;
ALTER TABLE documents ALTER COLUMN identifier_value SET NOT NULL;
ALTER TABLE documents ADD CONSTRAINT documents_identifier_value_check
  CHECK (identifier_value = btrim(identifier_value) AND identifier_value <> '' AND char_length(identifier_value) <= 500);
ALTER TABLE profile_details DROP COLUMN IF EXISTS membership_type;
ALTER TABLE profile_details DROP COLUMN IF EXISTS blood_type;

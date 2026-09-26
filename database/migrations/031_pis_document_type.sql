-- Add PIS/PASEP/NIT as its own catalog type. CTPS remains a separate work document.

INSERT INTO document_types (id, technical_key, label, active, uniqueness_policy, validation_regex, date_required)
VALUES
  ('f913bb63-42b6-581c-a0fc-6cd25b677ca1'::uuid, 'pis', 'PIS', true, 'PER_PROFILE', '^[0-9]{11}$', false)
ON CONFLICT (technical_key) DO UPDATE
SET label = EXCLUDED.label, active = true, uniqueness_policy = EXCLUDED.uniqueness_policy,
  validation_regex = EXCLUDED.validation_regex, date_required = EXCLUDED.date_required, updated_at = now();

INSERT INTO app_metadata (key, value)
VALUES ('schema.pis_document_type', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.pis_document_type';
DELETE FROM document_types
WHERE technical_key = 'pis'
  AND NOT EXISTS (SELECT 1 FROM documents WHERE documents.document_type_id = document_types.id)
  AND NOT EXISTS (SELECT 1 FROM document_presences WHERE document_presences.document_type_id = document_types.id);

-- TRI/TEU transport cards and CREF professional registry used by the archive load.

INSERT INTO document_types (id, technical_key, label, active, uniqueness_policy, validation_regex, date_required)
VALUES
  ('a31f0c5e-6d2b-5e91-8c44-0b7e2a9d1f60'::uuid, 'tri', 'TRI', true, 'PER_PROFILE', NULL, false),
  ('b42e1d6f-7e3c-5f02-9d55-1c8f3b0e2071'::uuid, 'teu', 'TEU', true, 'PER_PROFILE', NULL, false),
  ('c53f2e70-8f4d-5013-ae66-2d9f4c1f3182'::uuid, 'cref', 'CREF', true, 'PER_PROFILE', NULL, false)
ON CONFLICT (technical_key) DO UPDATE
SET label = EXCLUDED.label, active = true, uniqueness_policy = EXCLUDED.uniqueness_policy,
  validation_regex = EXCLUDED.validation_regex, date_required = EXCLUDED.date_required, updated_at = now();

INSERT INTO app_metadata (key, value)
VALUES ('schema.transport_and_cref', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.transport_and_cref';
DELETE FROM document_types
WHERE technical_key IN ('tri', 'teu', 'cref')
  AND NOT EXISTS (SELECT 1 FROM documents WHERE documents.document_type_id = document_types.id)
  AND NOT EXISTS (SELECT 1 FROM document_presences WHERE document_presences.document_type_id = document_types.id);

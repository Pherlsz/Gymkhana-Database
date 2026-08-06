-- Restore the canonical legacy document catalog and typed per-type fields.

INSERT INTO document_types (id, technical_key, label, active, uniqueness_policy, validation_regex, date_required)
VALUES
  ('8f675509-91e7-57eb-954b-fe1e7dff430e'::uuid, 'rg', 'RG', true, 'PER_PROFILE', NULL, false),
  ('f4b4141c-e1b0-5807-9e4d-832942ea335b'::uuid, 'voter_id', 'Título de Eleitor', true, 'PER_PROFILE', NULL, false),
  ('a9cf7a1f-7037-518c-b365-d2c377928a63'::uuid, 'cnh', 'CNH', true, 'PER_PROFILE', NULL, false),
  ('b2bbb718-bf60-5822-83fb-1d7200f35605'::uuid, 'ctps', 'CTPS', true, 'PER_PROFILE', NULL, false),
  ('f1cbd8e9-81ef-5055-b6d3-85fe7ea3bade'::uuid, 'passport', 'Passaporte', true, 'PER_PROFILE', NULL, false),
  ('9c8c08e8-e6d0-50e2-a1dd-bb4c48b5bcc0'::uuid, 'student_id', 'Carteira Estudantil', true, 'PER_PROFILE', NULL, false),
  ('5051c883-9999-5aaa-8445-b6a22c16d8cc'::uuid, 'citizen_card', 'Cartão Cidadão', true, 'PER_PROFILE', NULL, false),
  ('52632999-9f0a-5c92-9ea2-5f722798a86f'::uuid, 'sus_card', 'Cartão SUS', true, 'PER_PROFILE', NULL, false),
  ('c862a05e-c326-5653-87c4-157b8212c9ef'::uuid, 'oab', 'OAB', true, 'PER_PROFILE', NULL, false),
  ('15059b68-8cc1-53e7-892f-b3c8560ed5eb'::uuid, 'crea', 'CREA', true, 'PER_PROFILE', NULL, false),
  ('00c2333a-e1fa-524b-a37b-1f4bb99c3388'::uuid, 'coren', 'COREN', true, 'PER_PROFILE', NULL, false),
  ('2dc54b22-672f-5bf0-97f0-aeddb296e027'::uuid, 'crm', 'CRM', true, 'PER_PROFILE', NULL, false),
  ('39aca313-5fee-536a-a5a1-ca0c03f922d6'::uuid, 'cro', 'CRO', true, 'PER_PROFILE', NULL, false),
  ('e707d56a-3e7d-55fc-8310-6b3ab5c9d6e2'::uuid, 'birth_certificate', 'Certidão de Nascimento', true, 'PER_PROFILE', NULL, false),
  ('f9e2443a-e0ed-5d35-8bc6-292df9b56feb'::uuid, 'marriage_certificate', 'Certidão de Casamento', true, 'PER_PROFILE', NULL, false)
ON CONFLICT (technical_key) DO UPDATE
SET label = EXCLUDED.label, active = true;

WITH seeded_fields(id, document_type_key, technical_key, label, field_kind, required, maximum_length) AS (
  VALUES
    ('b07fd7f1-aadf-563e-b852-ed4369f8c46c'::uuid, 'rg', 'issuing_authority', 'Órgão emissor', 'TEXT', false, 120),
    ('14faf146-854f-5c7b-a752-b59d0ef22515'::uuid, 'rg', 'issuing_state', 'UF emissora', 'TEXT', false, 2),
    ('e380fa33-0dd2-5b8d-950e-357020e7001f'::uuid, 'rg', 'issue_date', 'Data de emissão', 'CIVIL_DATE', false, NULL),
    ('deb96fc9-dabb-5276-9f16-fcdae8dc729f'::uuid, 'rg', 'expiry_date', 'Data de validade', 'CIVIL_DATE', false, NULL),
    ('a60798ec-acd0-518c-9314-e10d453ab583'::uuid, 'voter_id', 'zone', 'Zona', 'TEXT', false, 20),
    ('c585e83c-002a-570d-aae7-3586bf3a8850'::uuid, 'voter_id', 'section', 'Seção', 'TEXT', false, 20),
    ('f871d761-bf70-5c72-8528-be789f6a7a95'::uuid, 'voter_id', 'city', 'Município', 'TEXT', false, 120),
    ('ad922827-1f8e-57d8-808c-e769053ab9d2'::uuid, 'voter_id', 'state', 'Estado', 'TEXT', false, 2),
    ('c5e8d6f6-1c99-5b08-9730-ea09e28f2857'::uuid, 'cnh', 'category', 'Categoria', 'TEXT', false, 20),
    ('5897d2a1-63e2-5198-83f4-33227876ae05'::uuid, 'cnh', 'issue_date', 'Data de emissão', 'CIVIL_DATE', false, NULL),
    ('d07d56a5-4b84-56e3-964c-96d883e2a5d7'::uuid, 'cnh', 'expiry_date', 'Data de validade', 'CIVIL_DATE', false, NULL),
    ('797506d3-45e1-55b9-84af-661fc4c8e02c'::uuid, 'cnh', 'issuing_authority', 'Órgão emissor', 'TEXT', false, 120),
    ('9bddb44d-1de9-543e-816d-a3488123033a'::uuid, 'cnh', 'state', 'UF', 'TEXT', false, 2),
    ('c4f8d4d1-7047-5616-ab34-48890fd74bdf'::uuid, 'ctps', 'series', 'Série', 'TEXT', false, 40),
    ('9e0d0618-853c-5271-84ed-6cc52c7eda5d'::uuid, 'ctps', 'issue_date', 'Data de emissão', 'CIVIL_DATE', false, NULL),
    ('820fe2d3-bb30-53f6-a4fc-19ebabfd61e6'::uuid, 'ctps', 'state', 'UF', 'TEXT', false, 2),
    ('88410f71-0f48-5bf3-94b3-022b80a027e2'::uuid, 'passport', 'issuing_country', 'País emissor', 'TEXT', false, 120),
    ('03013ab0-92a0-51d3-8b9f-d80863522b2e'::uuid, 'passport', 'issue_date', 'Data de emissão', 'CIVIL_DATE', false, NULL),
    ('5c98c1ea-402b-5cd3-92d4-afa3bb0d9996'::uuid, 'passport', 'expiry_date', 'Data de validade', 'CIVIL_DATE', false, NULL),
    ('d32e1520-529a-5088-879a-2d1ea8e34974'::uuid, 'student_id', 'institution', 'Instituição', 'TEXT', false, 200),
    ('3b55a186-61af-5ce8-a63e-01135c56bd5c'::uuid, 'student_id', 'course', 'Curso', 'TEXT', false, 200),
    ('f8487070-4812-5f1e-ad32-aaa3d4095f67'::uuid, 'student_id', 'expiry_date', 'Data de validade', 'CIVIL_DATE', false, NULL),
    ('1a85ba4e-748c-5679-81a1-3d72c21bb0f5'::uuid, 'oab', 'state', 'UF', 'TEXT', false, 2),
    ('54f30cd3-d1be-5768-aed6-6898250be344'::uuid, 'oab', 'expiry_date', 'Data de validade', 'CIVIL_DATE', false, NULL),
    ('11414163-6797-5642-8fcb-1b33caf91732'::uuid, 'crea', 'specialty', 'Especialidade', 'TEXT', false, 200),
    ('4d3e0e67-d2db-505a-94d5-adacd4ba60f0'::uuid, 'crea', 'state', 'UF', 'TEXT', false, 2),
    ('a9d1d789-07af-5340-b041-d4b54882b3c9'::uuid, 'crea', 'expiry_date', 'Data de validade', 'CIVIL_DATE', false, NULL),
    ('1f12a8e6-0165-516e-b025-ba69e5e9f169'::uuid, 'coren', 'category', 'Categoria', 'SINGLE_SELECT', false, NULL),
    ('df045b56-443b-5c87-8d29-66b09137d83b'::uuid, 'coren', 'state', 'UF', 'TEXT', false, 2),
    ('faa3167d-a5de-58a1-94c0-030ad251a5a7'::uuid, 'coren', 'expiry_date', 'Data de validade', 'CIVIL_DATE', false, NULL),
    ('9ed02691-3703-588d-8750-db5abae956f8'::uuid, 'crm', 'specialty', 'Especialidade', 'TEXT', false, 200),
    ('f285b211-ec93-5160-b0bf-b4657b7203bf'::uuid, 'crm', 'state', 'UF', 'TEXT', false, 2),
    ('efc868a1-9f37-5915-83fc-b4d9a2e8f13b'::uuid, 'crm', 'expiry_date', 'Data de validade', 'CIVIL_DATE', false, NULL),
    ('7dbf4237-376c-5c61-8875-741b3efbc200'::uuid, 'cro', 'state', 'UF', 'TEXT', false, 2),
    ('3238b7d9-cee5-57bf-8923-4422ca87faeb'::uuid, 'cro', 'expiry_date', 'Data de validade', 'CIVIL_DATE', false, NULL),
    ('a89073f7-6497-5626-8a23-fd1979a11b4a'::uuid, 'birth_certificate', 'registered_name', 'Nome registrado', 'TEXT', false, 200),
    ('d065cff1-016b-5eea-811d-193dd9b8088a'::uuid, 'birth_certificate', 'birth_date', 'Data de nascimento', 'CIVIL_DATE', false, NULL),
    ('f1fe01e7-5585-5e77-9f2d-2a23629e7da8'::uuid, 'birth_certificate', 'notary', 'Cartório', 'TEXT', false, 200),
    ('7aeaaaa9-3193-5a9d-9eeb-5dc5cdfc7596'::uuid, 'birth_certificate', 'city', 'Município', 'TEXT', false, 120),
    ('b333e752-6e36-56c1-bcdc-2471d1f985af'::uuid, 'birth_certificate', 'state', 'Estado', 'TEXT', false, 2),
    ('235c73b1-595c-5839-be68-f8b95c708cb9'::uuid, 'birth_certificate', 'book', 'Livro', 'TEXT', false, 50),
    ('ef11ecea-0e7c-5fc3-b2b2-97b9dd839027'::uuid, 'birth_certificate', 'page', 'Folha', 'TEXT', false, 50),
    ('f8f9d4cf-b1b8-5845-80bc-f9157cd2e9d4'::uuid, 'marriage_certificate', 'spouse_names', 'Nomes dos cônjuges', 'LONG_TEXT', false, 1000),
    ('f3d2f729-ae7c-5654-bfa0-46992af46db7'::uuid, 'marriage_certificate', 'wedding_date', 'Data do casamento', 'CIVIL_DATE', false, NULL),
    ('96576550-7fa4-5238-941c-925569eb99d4'::uuid, 'marriage_certificate', 'notary', 'Cartório', 'TEXT', false, 200),
    ('0ac7c3cc-99b2-54d2-b9d6-9e999c4d95be'::uuid, 'marriage_certificate', 'city', 'Município', 'TEXT', false, 120),
    ('b7972a9f-3bf8-5c99-8d5b-2d6c2edd857a'::uuid, 'marriage_certificate', 'state', 'Estado', 'TEXT', false, 2),
    ('3537e37f-eb9d-54a9-8ad0-0a26b831f914'::uuid, 'marriage_certificate', 'book', 'Livro', 'TEXT', false, 50),
    ('b176ac96-72f1-5378-8130-099d369566e5'::uuid, 'marriage_certificate', 'page', 'Folha', 'TEXT', false, 50)
)
INSERT INTO custom_field_definitions (
  id, target_kind, document_type_id, technical_key, label, field_kind, required, active,
  minimum_length, maximum_length, validation_regex, minimum_decimal, maximum_decimal
)
SELECT
  seeded_fields.id, 'DOCUMENT_TYPE', document_types.id, seeded_fields.technical_key,
  seeded_fields.label, seeded_fields.field_kind, seeded_fields.required, true,
  NULL, seeded_fields.maximum_length, NULL, NULL, NULL
FROM seeded_fields
JOIN document_types ON document_types.technical_key = seeded_fields.document_type_key
ON CONFLICT (document_type_id, technical_key) WHERE target_kind = 'DOCUMENT_TYPE'
DO UPDATE SET
  label = EXCLUDED.label, field_kind = EXCLUDED.field_kind, required = EXCLUDED.required,
  active = true, maximum_length = EXCLUDED.maximum_length, updated_at = now();

INSERT INTO custom_field_options (id, field_definition_id, technical_key, label, active, sort_order)
VALUES
  ('b4ff2ff4-3a3f-5d33-8a8c-51dabf365ab7'::uuid, '1f12a8e6-0165-516e-b025-ba69e5e9f169'::uuid, 'nurse', 'Enfermeiro', true, 0),
  ('6c53a4b0-9020-5416-a25c-66206a4530cd'::uuid, '1f12a8e6-0165-516e-b025-ba69e5e9f169'::uuid, 'nursing_technician', 'Técnico de Enfermagem', true, 1),
  ('0ab5b41d-89a2-5a46-8579-8b46a6c5d372'::uuid, '1f12a8e6-0165-516e-b025-ba69e5e9f169'::uuid, 'nursing_assistant', 'Auxiliar de Enfermagem', true, 2)
ON CONFLICT (field_definition_id, technical_key) DO UPDATE
SET label = EXCLUDED.label, active = true, sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO app_metadata (key, value)
VALUES ('schema.document_catalog', 'm15-canonical')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.document_catalog';
DELETE FROM custom_field_options WHERE id IN (
  'b4ff2ff4-3a3f-5d33-8a8c-51dabf365ab7'::uuid,
  '6c53a4b0-9020-5416-a25c-66206a4530cd'::uuid,
  '0ab5b41d-89a2-5a46-8579-8b46a6c5d372'::uuid
);
DELETE FROM custom_field_definitions WHERE id IN (
  'b07fd7f1-aadf-563e-b852-ed4369f8c46c'::uuid,
  '14faf146-854f-5c7b-a752-b59d0ef22515'::uuid,
  'e380fa33-0dd2-5b8d-950e-357020e7001f'::uuid,
  'deb96fc9-dabb-5276-9f16-fcdae8dc729f'::uuid,
  'a60798ec-acd0-518c-9314-e10d453ab583'::uuid,
  'c585e83c-002a-570d-aae7-3586bf3a8850'::uuid,
  'f871d761-bf70-5c72-8528-be789f6a7a95'::uuid,
  'ad922827-1f8e-57d8-808c-e769053ab9d2'::uuid,
  'c5e8d6f6-1c99-5b08-9730-ea09e28f2857'::uuid,
  '5897d2a1-63e2-5198-83f4-33227876ae05'::uuid,
  'd07d56a5-4b84-56e3-964c-96d883e2a5d7'::uuid,
  '797506d3-45e1-55b9-84af-661fc4c8e02c'::uuid,
  '9bddb44d-1de9-543e-816d-a3488123033a'::uuid,
  'c4f8d4d1-7047-5616-ab34-48890fd74bdf'::uuid,
  '9e0d0618-853c-5271-84ed-6cc52c7eda5d'::uuid,
  '820fe2d3-bb30-53f6-a4fc-19ebabfd61e6'::uuid,
  '88410f71-0f48-5bf3-94b3-022b80a027e2'::uuid,
  '03013ab0-92a0-51d3-8b9f-d80863522b2e'::uuid,
  '5c98c1ea-402b-5cd3-92d4-afa3bb0d9996'::uuid,
  'd32e1520-529a-5088-879a-2d1ea8e34974'::uuid,
  '3b55a186-61af-5ce8-a63e-01135c56bd5c'::uuid,
  'f8487070-4812-5f1e-ad32-aaa3d4095f67'::uuid,
  '1a85ba4e-748c-5679-81a1-3d72c21bb0f5'::uuid,
  '54f30cd3-d1be-5768-aed6-6898250be344'::uuid,
  '11414163-6797-5642-8fcb-1b33caf91732'::uuid,
  '4d3e0e67-d2db-505a-94d5-adacd4ba60f0'::uuid,
  'a9d1d789-07af-5340-b041-d4b54882b3c9'::uuid,
  '1f12a8e6-0165-516e-b025-ba69e5e9f169'::uuid,
  'df045b56-443b-5c87-8d29-66b09137d83b'::uuid,
  'faa3167d-a5de-58a1-94c0-030ad251a5a7'::uuid,
  '9ed02691-3703-588d-8750-db5abae956f8'::uuid,
  'f285b211-ec93-5160-b0bf-b4657b7203bf'::uuid,
  'efc868a1-9f37-5915-83fc-b4d9a2e8f13b'::uuid,
  '7dbf4237-376c-5c61-8875-741b3efbc200'::uuid,
  '3238b7d9-cee5-57bf-8923-4422ca87faeb'::uuid,
  'a89073f7-6497-5626-8a23-fd1979a11b4a'::uuid,
  'd065cff1-016b-5eea-811d-193dd9b8088a'::uuid,
  'f1fe01e7-5585-5e77-9f2d-2a23629e7da8'::uuid,
  '7aeaaaa9-3193-5a9d-9eeb-5dc5cdfc7596'::uuid,
  'b333e752-6e36-56c1-bcdc-2471d1f985af'::uuid,
  '235c73b1-595c-5839-be68-f8b95c708cb9'::uuid,
  'ef11ecea-0e7c-5fc3-b2b2-97b9dd839027'::uuid,
  'f8f9d4cf-b1b8-5845-80bc-f9157cd2e9d4'::uuid,
  'f3d2f729-ae7c-5654-bfa0-46992af46db7'::uuid,
  '96576550-7fa4-5238-941c-925569eb99d4'::uuid,
  '0ac7c3cc-99b2-54d2-b9d6-9e999c4d95be'::uuid,
  'b7972a9f-3bf8-5c99-8d5b-2d6c2edd857a'::uuid,
  '3537e37f-eb9d-54a9-8ad0-0a26b831f914'::uuid,
  'b176ac96-72f1-5378-8130-099d369566e5'::uuid
);
DELETE FROM document_types
WHERE technical_key IN ('oab','crea','coren','crm','cro','birth_certificate','marriage_certificate')
  AND NOT EXISTS (SELECT 1 FROM documents WHERE documents.document_type_id = document_types.id);

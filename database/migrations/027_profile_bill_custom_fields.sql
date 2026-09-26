-- PROFILE extras from the legacy dados-pessoais sheet, plus BILL_TYPE fields
-- for luz/água/internet. Canonical profile/bill columns stay on their tables.

INSERT INTO bill_types (id, technical_key, label, active)
VALUES
  ('024b0001-91e7-57eb-954b-000000000001'::uuid, 'energia', 'Conta de luz', true),
  ('024b0002-91e7-57eb-954b-000000000002'::uuid, 'agua', 'Conta de água', true),
  ('024b0003-91e7-57eb-954b-000000000003'::uuid, 'internet', 'Internet', true)
ON CONFLICT (technical_key) DO UPDATE
SET label = EXCLUDED.label, active = true;

WITH profile_fields(id, technical_key, label, field_kind, maximum_length) AS (
  VALUES
    ('024f0001-91e7-57eb-954b-000000000001'::uuid, 'gender', 'Sexo', 'SINGLE_SELECT', NULL::integer),
    ('024f0002-91e7-57eb-954b-000000000002'::uuid, 'birth_date', 'Data de nascimento', 'CIVIL_DATE', NULL::integer),
    ('024f0003-91e7-57eb-954b-000000000003'::uuid, 'blood_type', 'Tipo sanguíneo', 'SINGLE_SELECT', NULL::integer),
    ('024f0004-91e7-57eb-954b-000000000004'::uuid, 'marital_status', 'Estado civil', 'TEXT', 120),
    ('024f0005-91e7-57eb-954b-000000000005'::uuid, 'birth_city', 'Cidade de nascimento', 'TEXT', 120),
    ('024f0006-91e7-57eb-954b-000000000006'::uuid, 'nationality', 'Nacionalidade', 'TEXT', 120),
    ('024f0007-91e7-57eb-954b-000000000007'::uuid, 'wedding_date', 'Data de casamento', 'CIVIL_DATE', NULL::integer),
    ('024f0008-91e7-57eb-954b-000000000008'::uuid, 'father_name', 'Nome do pai', 'TEXT', 200),
    ('024f0009-91e7-57eb-954b-000000000009'::uuid, 'father_birth_date', 'Data nasc. pai', 'CIVIL_DATE', NULL::integer),
    ('024f0010-91e7-57eb-954b-000000000010'::uuid, 'mother_name', 'Nome da mãe', 'TEXT', 200),
    ('024f0011-91e7-57eb-954b-000000000011'::uuid, 'mother_birth_date', 'Data nasc. mãe', 'CIVIL_DATE', NULL::integer),
    ('024f0012-91e7-57eb-954b-000000000012'::uuid, 'vehicle_model', 'Modelo veículo', 'TEXT', 120),
    ('024f0013-91e7-57eb-954b-000000000013'::uuid, 'vehicle_color', 'Cor', 'TEXT', 80),
    ('024f0014-91e7-57eb-954b-000000000014'::uuid, 'vehicle_plate', 'Placa', 'TEXT', 20),
    ('024f0015-91e7-57eb-954b-000000000015'::uuid, 'vehicle_year', 'Ano', 'INTEGER', NULL::integer),
    ('024f0016-91e7-57eb-954b-000000000016'::uuid, 'health_plan', 'Plano de saúde', 'TEXT', 200),
    ('024f0017-91e7-57eb-954b-000000000017'::uuid, 'blood_donor', 'Doador de sangue', 'BOOLEAN', NULL::integer),
    ('024f0018-91e7-57eb-954b-000000000018'::uuid, 'organ_donor', 'Doador de órgãos', 'BOOLEAN', NULL::integer),
    ('024f0019-91e7-57eb-954b-000000000019'::uuid, 'team', 'Equipe', 'TEXT', 120),
    ('024f0020-91e7-57eb-954b-000000000020'::uuid, 'sector', 'Setor', 'SINGLE_SELECT', NULL::integer),
    ('024f0021-91e7-57eb-954b-000000000021'::uuid, 'collections', 'Coleções', 'TEXT', 500),
    ('024f0022-91e7-57eb-954b-000000000022'::uuid, 'club_membership', 'Sócio clube', 'SINGLE_SELECT', NULL::integer),
    ('024f0023-91e7-57eb-954b-000000000023'::uuid, 'membership_type', 'Categoria de sócio', 'SINGLE_SELECT', NULL::integer)
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

INSERT INTO custom_field_options (id, field_definition_id, technical_key, label, active, sort_order)
VALUES
  ('024a0101-91e7-57eb-954b-000000000001'::uuid, '024f0001-91e7-57eb-954b-000000000001'::uuid, 'male', 'M', true, 0),
  ('024a0102-91e7-57eb-954b-000000000002'::uuid, '024f0001-91e7-57eb-954b-000000000001'::uuid, 'female', 'F', true, 1),
  ('024a0103-91e7-57eb-954b-000000000003'::uuid, '024f0001-91e7-57eb-954b-000000000001'::uuid, 'outro', 'Outro', true, 2),
  ('024a0201-91e7-57eb-954b-000000000001'::uuid, '024f0003-91e7-57eb-954b-000000000003'::uuid, 'a_pos', 'A+', true, 0),
  ('024a0202-91e7-57eb-954b-000000000002'::uuid, '024f0003-91e7-57eb-954b-000000000003'::uuid, 'a_neg', 'A-', true, 1),
  ('024a0203-91e7-57eb-954b-000000000003'::uuid, '024f0003-91e7-57eb-954b-000000000003'::uuid, 'b_pos', 'B+', true, 2),
  ('024a0204-91e7-57eb-954b-000000000004'::uuid, '024f0003-91e7-57eb-954b-000000000003'::uuid, 'b_neg', 'B-', true, 3),
  ('024a0205-91e7-57eb-954b-000000000005'::uuid, '024f0003-91e7-57eb-954b-000000000003'::uuid, 'ab_pos', 'AB+', true, 4),
  ('024a0206-91e7-57eb-954b-000000000006'::uuid, '024f0003-91e7-57eb-954b-000000000003'::uuid, 'ab_neg', 'AB-', true, 5),
  ('024a0207-91e7-57eb-954b-000000000007'::uuid, '024f0003-91e7-57eb-954b-000000000003'::uuid, 'o_pos', 'O+', true, 6),
  ('024a0208-91e7-57eb-954b-000000000008'::uuid, '024f0003-91e7-57eb-954b-000000000003'::uuid, 'o_neg', 'O-', true, 7),
  ('024a0301-91e7-57eb-954b-000000000001'::uuid, '024f0020-91e7-57eb-954b-000000000020'::uuid, 'rua', 'Rua', true, 0),
  ('024a0302-91e7-57eb-954b-000000000002'::uuid, '024f0020-91e7-57eb-954b-000000000020'::uuid, 'diversas', 'Diversas', true, 1),
  ('024a0303-91e7-57eb-954b-000000000003'::uuid, '024f0020-91e7-57eb-954b-000000000020'::uuid, 'artistica', 'Artística', true, 2),
  ('024a0304-91e7-57eb-954b-000000000004'::uuid, '024f0020-91e7-57eb-954b-000000000020'::uuid, 'construcao', 'Construção', true, 3),
  ('024a0305-91e7-57eb-954b-000000000005'::uuid, '024f0020-91e7-57eb-954b-000000000020'::uuid, 'esportiva', 'Esportiva', true, 4),
  ('024a0306-91e7-57eb-954b-000000000006'::uuid, '024f0020-91e7-57eb-954b-000000000020'::uuid, 'fechamento', 'Fechamento', true, 5),
  ('024a0307-91e7-57eb-954b-000000000007'::uuid, '024f0020-91e7-57eb-954b-000000000020'::uuid, 'objetos', 'Objetos', true, 6),
  ('024a0308-91e7-57eb-954b-000000000008'::uuid, '024f0020-91e7-57eb-954b-000000000020'::uuid, 'outro', 'Outro', true, 7),
  ('024a0401-91e7-57eb-954b-000000000001'::uuid, '024f0022-91e7-57eb-954b-000000000022'::uuid, 'internacional', 'Internacional', true, 0),
  ('024a0402-91e7-57eb-954b-000000000002'::uuid, '024f0022-91e7-57eb-954b-000000000022'::uuid, 'gremio', 'Grêmio', true, 1),
  ('024a0403-91e7-57eb-954b-000000000003'::uuid, '024f0022-91e7-57eb-954b-000000000022'::uuid, 'outro', 'Outro', true, 2),
  ('024a0501-91e7-57eb-954b-000000000001'::uuid, '024f0023-91e7-57eb-954b-000000000023'::uuid, 'cartao', 'Cartão', true, 0),
  ('024a0502-91e7-57eb-954b-000000000002'::uuid, '024f0023-91e7-57eb-954b-000000000023'::uuid, 'socio', 'Sócio', true, 1),
  ('024a0503-91e7-57eb-954b-000000000003'::uuid, '024f0023-91e7-57eb-954b-000000000023'::uuid, 'outro', 'Outro', true, 2)
ON CONFLICT (field_definition_id, technical_key) DO UPDATE
SET label = EXCLUDED.label, active = true, sort_order = EXCLUDED.sort_order, updated_at = now();

WITH bill_fields(id, bill_type_key, technical_key, label, field_kind, maximum_length) AS (
  VALUES
    ('024c0001-91e7-57eb-954b-000000000001'::uuid, 'energia', 'utility_company', 'Concessionária', 'TEXT', 200),
    ('024c0002-91e7-57eb-954b-000000000002'::uuid, 'energia', 'issue_month_year', 'Emissão (mês/ano)', 'TEXT', 20),
    ('024c0003-91e7-57eb-954b-000000000003'::uuid, 'energia', 'due_month_year', 'Vencimento (mês/ano)', 'TEXT', 20),
    ('024c0004-91e7-57eb-954b-000000000004'::uuid, 'energia', 'uc', 'UC', 'TEXT', 80),
    ('024c0005-91e7-57eb-954b-000000000005'::uuid, 'energia', 'nf', 'NF', 'TEXT', 80),
    ('024c0006-91e7-57eb-954b-000000000006'::uuid, 'energia', 'reading_route', 'Roteiro leitura', 'TEXT', 80),
    ('024c0011-91e7-57eb-954b-000000000011'::uuid, 'agua', 'utility_company', 'Concessionária', 'TEXT', 200),
    ('024c0012-91e7-57eb-954b-000000000012'::uuid, 'agua', 'issue_month_year', 'Emissão (mês/ano)', 'TEXT', 20),
    ('024c0013-91e7-57eb-954b-000000000013'::uuid, 'agua', 'due_month_year', 'Vencimento (mês/ano)', 'TEXT', 20),
    ('024c0014-91e7-57eb-954b-000000000014'::uuid, 'agua', 'invoice_number', 'Nº fatura', 'TEXT', 80),
    ('024c0015-91e7-57eb-954b-000000000015'::uuid, 'agua', 'current_reading', 'Leitura atual', 'DECIMAL', NULL::integer),
    ('024c0016-91e7-57eb-954b-000000000016'::uuid, 'agua', 'previous_reading', 'Leitura anterior', 'DECIMAL', NULL::integer),
    ('024c0017-91e7-57eb-954b-000000000017'::uuid, 'agua', 'property_code', 'Código imóvel', 'TEXT', 80),
    ('024c0018-91e7-57eb-954b-000000000018'::uuid, 'agua', 'category', 'Categoria', 'TEXT', 80),
    ('024c0019-91e7-57eb-954b-000000000019'::uuid, 'agua', 'water_meter', 'Hidrômetro', 'TEXT', 80),
    ('024c0020-91e7-57eb-954b-000000000020'::uuid, 'agua', 'location', 'Localização', 'TEXT', 200),
    ('024c0021-91e7-57eb-954b-000000000021'::uuid, 'agua', 'collection_code', 'Código arrecadação', 'TEXT', 80),
    ('024c0031-91e7-57eb-954b-000000000031'::uuid, 'internet', 'billing_code', 'Código cobrança', 'TEXT', 80),
    ('024c0032-91e7-57eb-954b-000000000032'::uuid, 'internet', 'issue_month_year', 'Emissão (mês/ano)', 'TEXT', 20),
    ('024c0033-91e7-57eb-954b-000000000033'::uuid, 'internet', 'due_month_year', 'Vencimento (mês/ano)', 'TEXT', 20),
    ('024c0034-91e7-57eb-954b-000000000034'::uuid, 'internet', 'client_code', 'Código cliente', 'TEXT', 80)
)
INSERT INTO custom_field_definitions (
  id, target_kind, document_type_id, bill_type_id, custom_entity_type_id,
  technical_key, label, field_kind, required, active,
  minimum_length, maximum_length, validation_regex, minimum_decimal, maximum_decimal
)
SELECT
  bill_fields.id, 'BILL_TYPE', NULL, bill_types.id, NULL,
  bill_fields.technical_key, bill_fields.label, bill_fields.field_kind,
  false, true, NULL, bill_fields.maximum_length, NULL, NULL, NULL
FROM bill_fields
JOIN bill_types ON bill_types.technical_key = bill_fields.bill_type_key
  OR (bill_fields.bill_type_key = 'energia' AND bill_types.technical_key = 'luz')
ON CONFLICT (bill_type_id, technical_key) WHERE target_kind = 'BILL_TYPE'
DO UPDATE SET
  label = EXCLUDED.label, field_kind = EXCLUDED.field_kind, active = true,
  maximum_length = EXCLUDED.maximum_length, updated_at = now();

INSERT INTO app_metadata (key, value)
VALUES ('schema.profile_bill_custom_fields', 'm16')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.profile_bill_custom_fields';
DELETE FROM custom_field_options WHERE id IN (
  '024a0101-91e7-57eb-954b-000000000001'::uuid,
  '024a0102-91e7-57eb-954b-000000000002'::uuid,
  '024a0103-91e7-57eb-954b-000000000003'::uuid,
  '024a0201-91e7-57eb-954b-000000000001'::uuid,
  '024a0202-91e7-57eb-954b-000000000002'::uuid,
  '024a0203-91e7-57eb-954b-000000000003'::uuid,
  '024a0204-91e7-57eb-954b-000000000004'::uuid,
  '024a0205-91e7-57eb-954b-000000000005'::uuid,
  '024a0206-91e7-57eb-954b-000000000006'::uuid,
  '024a0207-91e7-57eb-954b-000000000007'::uuid,
  '024a0208-91e7-57eb-954b-000000000008'::uuid,
  '024a0301-91e7-57eb-954b-000000000001'::uuid,
  '024a0302-91e7-57eb-954b-000000000002'::uuid,
  '024a0303-91e7-57eb-954b-000000000003'::uuid,
  '024a0304-91e7-57eb-954b-000000000004'::uuid,
  '024a0305-91e7-57eb-954b-000000000005'::uuid,
  '024a0306-91e7-57eb-954b-000000000006'::uuid,
  '024a0307-91e7-57eb-954b-000000000007'::uuid,
  '024a0308-91e7-57eb-954b-000000000008'::uuid,
  '024a0401-91e7-57eb-954b-000000000001'::uuid,
  '024a0402-91e7-57eb-954b-000000000002'::uuid,
  '024a0403-91e7-57eb-954b-000000000003'::uuid,
  '024a0501-91e7-57eb-954b-000000000001'::uuid,
  '024a0502-91e7-57eb-954b-000000000002'::uuid,
  '024a0503-91e7-57eb-954b-000000000003'::uuid
);
DELETE FROM custom_field_definitions WHERE id IN (
  '024f0001-91e7-57eb-954b-000000000001'::uuid,
  '024f0002-91e7-57eb-954b-000000000002'::uuid,
  '024f0003-91e7-57eb-954b-000000000003'::uuid,
  '024f0004-91e7-57eb-954b-000000000004'::uuid,
  '024f0005-91e7-57eb-954b-000000000005'::uuid,
  '024f0006-91e7-57eb-954b-000000000006'::uuid,
  '024f0007-91e7-57eb-954b-000000000007'::uuid,
  '024f0008-91e7-57eb-954b-000000000008'::uuid,
  '024f0009-91e7-57eb-954b-000000000009'::uuid,
  '024f0010-91e7-57eb-954b-000000000010'::uuid,
  '024f0011-91e7-57eb-954b-000000000011'::uuid,
  '024f0012-91e7-57eb-954b-000000000012'::uuid,
  '024f0013-91e7-57eb-954b-000000000013'::uuid,
  '024f0014-91e7-57eb-954b-000000000014'::uuid,
  '024f0015-91e7-57eb-954b-000000000015'::uuid,
  '024f0016-91e7-57eb-954b-000000000016'::uuid,
  '024f0017-91e7-57eb-954b-000000000017'::uuid,
  '024f0018-91e7-57eb-954b-000000000018'::uuid,
  '024f0019-91e7-57eb-954b-000000000019'::uuid,
  '024f0020-91e7-57eb-954b-000000000020'::uuid,
  '024f0021-91e7-57eb-954b-000000000021'::uuid,
  '024f0022-91e7-57eb-954b-000000000022'::uuid,
  '024f0023-91e7-57eb-954b-000000000023'::uuid,
  '024c0001-91e7-57eb-954b-000000000001'::uuid,
  '024c0002-91e7-57eb-954b-000000000002'::uuid,
  '024c0003-91e7-57eb-954b-000000000003'::uuid,
  '024c0004-91e7-57eb-954b-000000000004'::uuid,
  '024c0005-91e7-57eb-954b-000000000005'::uuid,
  '024c0006-91e7-57eb-954b-000000000006'::uuid,
  '024c0011-91e7-57eb-954b-000000000011'::uuid,
  '024c0012-91e7-57eb-954b-000000000012'::uuid,
  '024c0013-91e7-57eb-954b-000000000013'::uuid,
  '024c0014-91e7-57eb-954b-000000000014'::uuid,
  '024c0015-91e7-57eb-954b-000000000015'::uuid,
  '024c0016-91e7-57eb-954b-000000000016'::uuid,
  '024c0017-91e7-57eb-954b-000000000017'::uuid,
  '024c0018-91e7-57eb-954b-000000000018'::uuid,
  '024c0019-91e7-57eb-954b-000000000019'::uuid,
  '024c0020-91e7-57eb-954b-000000000020'::uuid,
  '024c0021-91e7-57eb-954b-000000000021'::uuid,
  '024c0031-91e7-57eb-954b-000000000031'::uuid,
  '024c0032-91e7-57eb-954b-000000000032'::uuid,
  '024c0033-91e7-57eb-954b-000000000033'::uuid,
  '024c0034-91e7-57eb-954b-000000000034'::uuid
);
DELETE FROM bill_types
WHERE id IN (
  '024b0001-91e7-57eb-954b-000000000001'::uuid,
  '024b0002-91e7-57eb-954b-000000000002'::uuid,
  '024b0003-91e7-57eb-954b-000000000003'::uuid
)
AND NOT EXISTS (SELECT 1 FROM bills WHERE bills.bill_type_id = bill_types.id)
AND NOT EXISTS (
  SELECT 1 FROM custom_field_definitions
  WHERE custom_field_definitions.bill_type_id = bill_types.id
    AND custom_field_definitions.id NOT IN (
      '024c0001-91e7-57eb-954b-000000000001'::uuid,
      '024c0002-91e7-57eb-954b-000000000002'::uuid,
      '024c0003-91e7-57eb-954b-000000000003'::uuid,
      '024c0004-91e7-57eb-954b-000000000004'::uuid,
      '024c0005-91e7-57eb-954b-000000000005'::uuid,
      '024c0006-91e7-57eb-954b-000000000006'::uuid,
      '024c0011-91e7-57eb-954b-000000000011'::uuid,
      '024c0012-91e7-57eb-954b-000000000012'::uuid,
      '024c0013-91e7-57eb-954b-000000000013'::uuid,
      '024c0014-91e7-57eb-954b-000000000014'::uuid,
      '024c0015-91e7-57eb-954b-000000000015'::uuid,
      '024c0016-91e7-57eb-954b-000000000016'::uuid,
      '024c0017-91e7-57eb-954b-000000000017'::uuid,
      '024c0018-91e7-57eb-954b-000000000018'::uuid,
      '024c0019-91e7-57eb-954b-000000000019'::uuid,
      '024c0020-91e7-57eb-954b-000000000020'::uuid,
      '024c0021-91e7-57eb-954b-000000000021'::uuid,
      '024c0031-91e7-57eb-954b-000000000031'::uuid,
      '024c0032-91e7-57eb-954b-000000000032'::uuid,
      '024c0033-91e7-57eb-954b-000000000033'::uuid,
      '024c0034-91e7-57eb-954b-000000000034'::uuid
    )
);

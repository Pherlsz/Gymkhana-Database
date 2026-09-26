-- Complete the canonical legacy catalog: CPF, Identidade, CREA/OAB, and the three bill types.
-- CPF records already live on profiles.cpf; copy them into documents so per-type COUNT works.
-- Keep profiles.cpf as the person identifier and keep the document row in sync on write.

INSERT INTO document_types (id, technical_key, label, active, uniqueness_policy, validation_regex, date_required)
VALUES
  ('b7de3989-9d10-5071-b913-778836542d3d'::uuid, 'cpf', 'CPF', true, 'PER_PROFILE', '^[0-9]{11}$', false),
  ('a6e7f3bc-19ea-5830-99fd-0a6ffbc6fafc'::uuid, 'identidade', 'Identidade', true, 'PER_PROFILE', NULL, false),
  ('bf69cf5a-0e7e-5e59-9f7f-250d972303c6'::uuid, 'crea_oab', 'CREA / OAB', true, 'PER_PROFILE', NULL, false)
ON CONFLICT (technical_key) DO UPDATE
SET label = EXCLUDED.label, active = true, uniqueness_policy = EXCLUDED.uniqueness_policy,
  validation_regex = EXCLUDED.validation_regex, date_required = EXCLUDED.date_required, updated_at = now();

INSERT INTO bill_types (id, technical_key, label, active, supports_current_use)
VALUES
  ('6c5538c7-ee70-54d5-a0c1-92cc4c7bfb3c'::uuid, 'energia', 'Conta de luz', true, true),
  ('2aa07cdf-9d0d-5350-8da9-5f7d0981f83f'::uuid, 'agua', 'Conta de água', true, true),
  ('a4d50e42-776d-57ec-9b91-69b928290b13'::uuid, 'internet', 'Internet', true, true)
ON CONFLICT (technical_key) DO UPDATE
SET label = EXCLUDED.label, active = true, supports_current_use = EXCLUDED.supports_current_use, updated_at = now();

INSERT INTO documents (
  id, owner_profile_id, document_type_id, identifier_value, uniqueness_policy, record_state
)
SELECT gen_random_uuid(), profile.id, document_type.id, profile.cpf, document_type.uniqueness_policy, 'CURRENT'
FROM profiles AS profile
JOIN document_types AS document_type ON document_type.technical_key = 'cpf'
WHERE profile.cpf IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM documents AS document
    WHERE document.owner_profile_id = profile.id
      AND document.document_type_id = document_type.id
  );

CREATE OR REPLACE FUNCTION sync_profile_cpf_document() RETURNS trigger AS $$
DECLARE
  type_id uuid;
  policy text;
  existing_id uuid;
BEGIN
  IF NEW.cpf IS NULL THEN
    RETURN NEW;
  END IF;
  SELECT id, uniqueness_policy INTO type_id, policy
  FROM document_types
  WHERE technical_key = 'cpf'
  LIMIT 1;
  IF type_id IS NULL THEN
    RETURN NEW;
  END IF;
  SELECT id INTO existing_id
  FROM documents
  WHERE owner_profile_id = NEW.id AND document_type_id = type_id
  ORDER BY updated_at DESC, id DESC
  LIMIT 1
  FOR UPDATE;
  IF existing_id IS NOT NULL THEN
    UPDATE documents
    SET identifier_value = NEW.cpf, uniqueness_policy = policy, updated_at = now()
    WHERE id = existing_id AND identifier_value IS DISTINCT FROM NEW.cpf;
  ELSE
    INSERT INTO documents (
      id, owner_profile_id, document_type_id, identifier_value, uniqueness_policy, record_state
    )
    VALUES (gen_random_uuid(), NEW.id, type_id, NEW.cpf, policy, 'CURRENT');
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS profiles_sync_cpf_document_trigger ON profiles;
CREATE TRIGGER profiles_sync_cpf_document_trigger
AFTER INSERT OR UPDATE OF cpf ON profiles
FOR EACH ROW
WHEN (NEW.cpf IS NOT NULL)
EXECUTE FUNCTION sync_profile_cpf_document();

INSERT INTO app_metadata (key, value)
VALUES ('schema.missing_legacy_catalog', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DROP TRIGGER IF EXISTS profiles_sync_cpf_document_trigger ON profiles;
DROP FUNCTION IF EXISTS sync_profile_cpf_document();
DELETE FROM app_metadata WHERE key = 'schema.missing_legacy_catalog';
DELETE FROM bill_types
WHERE technical_key IN ('energia', 'agua', 'internet')
  AND NOT EXISTS (SELECT 1 FROM bills WHERE bills.bill_type_id = bill_types.id);
DELETE FROM document_types
WHERE technical_key IN ('cpf', 'identidade', 'crea_oab')
  AND NOT EXISTS (SELECT 1 FROM documents WHERE documents.document_type_id = document_types.id);

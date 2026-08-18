-- Copy PROFILE custom fields into profile_details, then drop those definitions.
-- Retire discontinued document types (identidade, cpf, crea-oab) after moving numbers.

INSERT INTO profile_details (profile_id)
SELECT id FROM profiles
ON CONFLICT (profile_id) DO NOTHING;

UPDATE profile_details d
SET birth_date = COALESCE(d.birth_date, v.civil_date_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'birth_date'
  AND v.profile_id = d.profile_id AND d.birth_date IS NULL AND v.civil_date_value IS NOT NULL;

UPDATE profile_details d
SET wedding_date = COALESCE(d.wedding_date, v.civil_date_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'wedding_date'
  AND v.profile_id = d.profile_id AND d.wedding_date IS NULL AND v.civil_date_value IS NOT NULL;

UPDATE profile_details d
SET father_birth_date = COALESCE(d.father_birth_date, v.civil_date_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'father_birth_date'
  AND v.profile_id = d.profile_id AND d.father_birth_date IS NULL AND v.civil_date_value IS NOT NULL;

UPDATE profile_details d
SET mother_birth_date = COALESCE(d.mother_birth_date, v.civil_date_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'mother_birth_date'
  AND v.profile_id = d.profile_id AND d.mother_birth_date IS NULL AND v.civil_date_value IS NOT NULL;

UPDATE profile_details d
SET blood_donor = COALESCE(d.blood_donor, v.boolean_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'blood_donor'
  AND v.profile_id = d.profile_id AND d.blood_donor IS NULL AND v.boolean_value IS NOT NULL;

UPDATE profile_details d
SET organ_donor = COALESCE(d.organ_donor, v.boolean_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'organ_donor'
  AND v.profile_id = d.profile_id AND d.organ_donor IS NULL AND v.boolean_value IS NOT NULL;

UPDATE profile_details d
SET vehicle_year = COALESCE(d.vehicle_year, v.integer_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'vehicle_year'
  AND v.profile_id = d.profile_id AND d.vehicle_year IS NULL AND v.integer_value IS NOT NULL;

UPDATE profile_details d
SET
  gender = COALESCE(d.gender, opt.label, v.text_value),
  updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
LEFT JOIN custom_field_value_options selected ON selected.custom_field_value_id = v.id
LEFT JOIN custom_field_options opt ON opt.id = selected.option_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'gender'
  AND v.profile_id = d.profile_id AND d.gender IS NULL;

UPDATE profile_details d
SET blood_type = COALESCE(d.blood_type, opt.label, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
LEFT JOIN custom_field_value_options selected ON selected.custom_field_value_id = v.id
LEFT JOIN custom_field_options opt ON opt.id = selected.option_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'blood_type'
  AND v.profile_id = d.profile_id AND d.blood_type IS NULL;

UPDATE profile_details d
SET sector = COALESCE(d.sector, opt.label, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
LEFT JOIN custom_field_value_options selected ON selected.custom_field_value_id = v.id
LEFT JOIN custom_field_options opt ON opt.id = selected.option_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'sector'
  AND v.profile_id = d.profile_id AND d.sector IS NULL;

UPDATE profile_details d
SET club_membership = COALESCE(d.club_membership, opt.label, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
LEFT JOIN custom_field_value_options selected ON selected.custom_field_value_id = v.id
LEFT JOIN custom_field_options opt ON opt.id = selected.option_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'club_membership'
  AND v.profile_id = d.profile_id AND d.club_membership IS NULL;

UPDATE profile_details d
SET membership_type = COALESCE(d.membership_type, opt.label, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
LEFT JOIN custom_field_value_options selected ON selected.custom_field_value_id = v.id
LEFT JOIN custom_field_options opt ON opt.id = selected.option_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'membership_type'
  AND v.profile_id = d.profile_id AND d.membership_type IS NULL;

UPDATE profile_details d
SET marital_status = COALESCE(d.marital_status, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'marital_status'
  AND v.profile_id = d.profile_id AND d.marital_status IS NULL AND COALESCE(v.text_value, '') <> '';

UPDATE profile_details d
SET birth_city = COALESCE(d.birth_city, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'birth_city'
  AND v.profile_id = d.profile_id AND d.birth_city IS NULL AND COALESCE(v.text_value, '') <> '';

UPDATE profile_details d
SET nationality = COALESCE(d.nationality, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'nationality'
  AND v.profile_id = d.profile_id AND d.nationality IS NULL AND COALESCE(v.text_value, '') <> '';

UPDATE profile_details d
SET father_name = COALESCE(d.father_name, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'father_name'
  AND v.profile_id = d.profile_id AND d.father_name IS NULL AND COALESCE(v.text_value, '') <> '';

UPDATE profile_details d
SET mother_name = COALESCE(d.mother_name, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'mother_name'
  AND v.profile_id = d.profile_id AND d.mother_name IS NULL AND COALESCE(v.text_value, '') <> '';

UPDATE profile_details d
SET vehicle_model = COALESCE(d.vehicle_model, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'vehicle_model'
  AND v.profile_id = d.profile_id AND d.vehicle_model IS NULL AND COALESCE(v.text_value, '') <> '';

UPDATE profile_details d
SET vehicle_color = COALESCE(d.vehicle_color, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'vehicle_color'
  AND v.profile_id = d.profile_id AND d.vehicle_color IS NULL AND COALESCE(v.text_value, '') <> '';

UPDATE profile_details d
SET vehicle_plate = COALESCE(d.vehicle_plate, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'vehicle_plate'
  AND v.profile_id = d.profile_id AND d.vehicle_plate IS NULL AND COALESCE(v.text_value, '') <> '';

UPDATE profile_details d
SET health_plan = COALESCE(d.health_plan, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'health_plan'
  AND v.profile_id = d.profile_id AND d.health_plan IS NULL AND COALESCE(v.text_value, '') <> '';

UPDATE profile_details d
SET team = COALESCE(d.team, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'team'
  AND v.profile_id = d.profile_id AND d.team IS NULL AND COALESCE(v.text_value, '') <> '';

UPDATE profile_details d
SET collections = COALESCE(d.collections, v.text_value), updated_at = now()
FROM custom_field_values v
JOIN custom_field_definitions f ON f.id = v.field_definition_id
WHERE f.target_kind = 'PROFILE' AND f.technical_key = 'collections'
  AND v.profile_id = d.profile_id AND d.collections IS NULL AND COALESCE(v.text_value, '') <> '';

DELETE FROM custom_field_values
WHERE field_definition_id IN (
  SELECT id FROM custom_field_definitions
  WHERE target_kind = 'PROFILE' AND technical_key IN (
    'birth_date', 'gender', 'blood_type', 'marital_status', 'birth_city', 'nationality',
    'wedding_date', 'father_name', 'father_birth_date', 'mother_name', 'mother_birth_date',
    'vehicle_model', 'vehicle_color', 'vehicle_plate', 'vehicle_year', 'health_plan',
    'blood_donor', 'organ_donor', 'team', 'sector', 'collections', 'club_membership', 'membership_type'
  )
);

DELETE FROM attachment_upload_intents
WHERE field_definition_id IN (
  SELECT id FROM custom_field_definitions
  WHERE target_kind = 'PROFILE' AND technical_key IN (
    'birth_date', 'gender', 'blood_type', 'marital_status', 'birth_city', 'nationality',
    'wedding_date', 'father_name', 'father_birth_date', 'mother_name', 'mother_birth_date',
    'vehicle_model', 'vehicle_color', 'vehicle_plate', 'vehicle_year', 'health_plan',
    'blood_donor', 'organ_donor', 'team', 'sector', 'collections', 'club_membership', 'membership_type'
  )
);

DELETE FROM attachments
WHERE field_definition_id IN (
  SELECT id FROM custom_field_definitions
  WHERE target_kind = 'PROFILE' AND technical_key IN (
    'birth_date', 'gender', 'blood_type', 'marital_status', 'birth_city', 'nationality',
    'wedding_date', 'father_name', 'father_birth_date', 'mother_name', 'mother_birth_date',
    'vehicle_model', 'vehicle_color', 'vehicle_plate', 'vehicle_year', 'health_plan',
    'blood_donor', 'organ_donor', 'team', 'sector', 'collections', 'club_membership', 'membership_type'
  )
);

DELETE FROM custom_field_definitions
WHERE target_kind = 'PROFILE' AND technical_key IN (
  'birth_date', 'gender', 'blood_type', 'marital_status', 'birth_city', 'nationality',
  'wedding_date', 'father_name', 'father_birth_date', 'mother_name', 'mother_birth_date',
  'vehicle_model', 'vehicle_color', 'vehicle_plate', 'vehicle_year', 'health_plan',
  'blood_donor', 'organ_donor', 'team', 'sector', 'collections', 'club_membership', 'membership_type'
);

DROP TRIGGER IF EXISTS profiles_sync_cpf_document_trigger ON profiles;
DROP FUNCTION IF EXISTS sync_profile_cpf_document();

UPDATE profiles p
SET cpf = regexp_replace(d.identifier_value, '[^0-9]', '', 'g'), updated_at = now()
FROM documents d
JOIN document_types t ON t.id = d.document_type_id
WHERE p.id = d.owner_profile_id
  AND p.cpf IS NULL
  AND t.technical_key IN ('cpf', 'identidade', 'identity')
  AND regexp_replace(COALESCE(d.identifier_value, ''), '[^0-9]', '', 'g') ~ '^[0-9]{11}$';

UPDATE documents d
SET document_type_id = rg.id, updated_at = now()
FROM document_types old_type, document_types rg
WHERE old_type.id = d.document_type_id
  AND old_type.technical_key IN ('identidade', 'identity')
  AND rg.technical_key = 'rg'
  AND d.identifier_value IS NOT NULL
  AND d.identifier_value ~ '[0-9]'
  AND regexp_replace(d.identifier_value, '[^0-9]', '', 'g') !~ '^[0-9]{11}$'
  AND NOT EXISTS (
    SELECT 1 FROM documents existing
    WHERE existing.owner_profile_id = d.owner_profile_id
      AND existing.document_type_id = rg.id
      AND existing.id <> d.id
  );

UPDATE documents d
SET document_type_id = oab.id, updated_at = now()
FROM document_types old_type, document_types oab
WHERE old_type.id = d.document_type_id
  AND old_type.technical_key IN ('crea_oab', 'crea-oab')
  AND oab.technical_key = 'oab'
  AND EXISTS (
    SELECT 1
    FROM custom_field_values v
    JOIN custom_field_definitions f ON f.id = v.field_definition_id
    WHERE v.document_id = d.id
      AND f.technical_key IN ('tipo', 'type', 'kind')
      AND lower(COALESCE(v.text_value, '')) LIKE '%oab%'
  )
  AND NOT EXISTS (
    SELECT 1 FROM documents existing
    WHERE existing.owner_profile_id = d.owner_profile_id
      AND existing.document_type_id = oab.id
      AND existing.id <> d.id
  );

UPDATE documents d
SET document_type_id = crea.id, updated_at = now()
FROM document_types old_type, document_types crea
WHERE old_type.id = d.document_type_id
  AND old_type.technical_key IN ('crea_oab', 'crea-oab')
  AND crea.technical_key = 'crea'
  AND NOT EXISTS (
    SELECT 1 FROM documents existing
    WHERE existing.owner_profile_id = d.owner_profile_id
      AND existing.document_type_id = crea.id
      AND existing.id <> d.id
  );

DELETE FROM document_current_uses
WHERE document_id IN (
  SELECT d.id FROM documents d
  JOIN document_types t ON t.id = d.document_type_id
  WHERE t.technical_key IN ('identidade', 'identity', 'cpf', 'crea_oab', 'crea-oab')
);

DELETE FROM custom_field_values
WHERE document_id IN (
  SELECT d.id FROM documents d
  JOIN document_types t ON t.id = d.document_type_id
  WHERE t.technical_key IN ('identidade', 'identity', 'cpf', 'crea_oab', 'crea-oab')
);

DELETE FROM documents
WHERE document_type_id IN (
  SELECT id FROM document_types
  WHERE technical_key IN ('identidade', 'identity', 'cpf', 'crea_oab', 'crea-oab')
);

DELETE FROM attachment_upload_intents
WHERE field_definition_id IN (
  SELECT id FROM custom_field_definitions
  WHERE document_type_id IN (
    SELECT id FROM document_types
    WHERE technical_key IN ('identidade', 'identity', 'cpf', 'crea_oab', 'crea-oab')
  )
);

DELETE FROM attachments
WHERE field_definition_id IN (
  SELECT id FROM custom_field_definitions
  WHERE document_type_id IN (
    SELECT id FROM document_types
    WHERE technical_key IN ('identidade', 'identity', 'cpf', 'crea_oab', 'crea-oab')
  )
);

DELETE FROM custom_field_definitions
WHERE document_type_id IN (
  SELECT id FROM document_types
  WHERE technical_key IN ('identidade', 'identity', 'cpf', 'crea_oab', 'crea-oab')
);

DELETE FROM document_types
WHERE technical_key IN ('identidade', 'identity', 'cpf', 'crea_oab', 'crea-oab');

INSERT INTO app_metadata (key, value)
VALUES ('schema.discontinued_catalog', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.discontinued_catalog';

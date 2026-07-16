-- M5 typed custom fields and custom entities persistence foundation.

CREATE TABLE custom_entity_types (
  id UUID PRIMARY KEY,
  technical_key TEXT NOT NULL UNIQUE CHECK (technical_key ~ '^[a-z][a-z0-9_]{1,63}$'),
  label TEXT NOT NULL CHECK (label = btrim(label) AND label <> '' AND char_length(label) <= 120),
  active BOOLEAN NOT NULL DEFAULT true,
  profile_cardinality TEXT CHECK (
    profile_cardinality IS NULL OR profile_cardinality IN ('ONE_PER_PROFILE', 'MANY_PER_PROFILE')
  ),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX custom_entity_types_label_index ON custom_entity_types (lower(label), id);
CREATE INDEX custom_entity_types_active_index ON custom_entity_types (active, lower(label), id);

CREATE TABLE custom_field_definitions (
  id UUID PRIMARY KEY,
  target_kind TEXT NOT NULL CHECK (
    target_kind IN ('PROFILE', 'DOCUMENT_TYPE', 'BILL_TYPE', 'CUSTOM_ENTITY_TYPE')
  ),
  document_type_id UUID REFERENCES document_types(id) ON DELETE RESTRICT,
  bill_type_id UUID REFERENCES bill_types(id) ON DELETE RESTRICT,
  custom_entity_type_id UUID REFERENCES custom_entity_types(id) ON DELETE RESTRICT,
  technical_key TEXT NOT NULL CHECK (technical_key ~ '^[a-z][a-z0-9_]{1,63}$'),
  label TEXT NOT NULL CHECK (label = btrim(label) AND label <> '' AND char_length(label) <= 120),
  field_kind TEXT NOT NULL CHECK (
    field_kind IN (
      'TEXT', 'LONG_TEXT', 'INTEGER', 'DECIMAL', 'BOOLEAN', 'CIVIL_DATE',
      'CIVIL_MONTH', 'EMAIL', 'PHONE', 'SINGLE_SELECT', 'MULTI_SELECT'
    )
  ),
  required BOOLEAN NOT NULL DEFAULT false,
  active BOOLEAN NOT NULL DEFAULT true,
  minimum_length INTEGER CHECK (minimum_length IS NULL OR minimum_length >= 0),
  maximum_length INTEGER CHECK (maximum_length IS NULL OR maximum_length BETWEEN 1 AND 5000),
  validation_regex TEXT CHECK (
    validation_regex IS NULL OR (
      validation_regex = btrim(validation_regex) AND validation_regex <> '' AND char_length(validation_regex) <= 500
    )
  ),
  minimum_decimal NUMERIC(38, 10),
  maximum_decimal NUMERIC(38, 10),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (
    (target_kind = 'PROFILE' AND document_type_id IS NULL AND bill_type_id IS NULL AND custom_entity_type_id IS NULL) OR
    (target_kind = 'DOCUMENT_TYPE' AND document_type_id IS NOT NULL AND bill_type_id IS NULL AND custom_entity_type_id IS NULL) OR
    (target_kind = 'BILL_TYPE' AND document_type_id IS NULL AND bill_type_id IS NOT NULL AND custom_entity_type_id IS NULL) OR
    (target_kind = 'CUSTOM_ENTITY_TYPE' AND document_type_id IS NULL AND bill_type_id IS NULL AND custom_entity_type_id IS NOT NULL)
  ),
  CHECK (maximum_length IS NULL OR minimum_length IS NULL OR minimum_length <= maximum_length),
  CHECK (
    field_kind IN ('TEXT', 'LONG_TEXT', 'EMAIL', 'PHONE') OR
    (minimum_length IS NULL AND maximum_length IS NULL AND validation_regex IS NULL)
  ),
  CHECK (field_kind = 'DECIMAL' OR (minimum_decimal IS NULL AND maximum_decimal IS NULL)),
  CHECK (minimum_decimal IS NULL OR maximum_decimal IS NULL OR minimum_decimal <= maximum_decimal)
);
CREATE UNIQUE INDEX custom_field_definitions_profile_key_unique
  ON custom_field_definitions (technical_key)
  WHERE target_kind = 'PROFILE';
CREATE UNIQUE INDEX custom_field_definitions_document_key_unique
  ON custom_field_definitions (document_type_id, technical_key)
  WHERE target_kind = 'DOCUMENT_TYPE';
CREATE UNIQUE INDEX custom_field_definitions_bill_key_unique
  ON custom_field_definitions (bill_type_id, technical_key)
  WHERE target_kind = 'BILL_TYPE';
CREATE UNIQUE INDEX custom_field_definitions_entity_key_unique
  ON custom_field_definitions (custom_entity_type_id, technical_key)
  WHERE target_kind = 'CUSTOM_ENTITY_TYPE';
CREATE INDEX custom_field_definitions_target_index
  ON custom_field_definitions (target_kind, document_type_id, bill_type_id, custom_entity_type_id, active, technical_key);

CREATE TABLE custom_field_options (
  id UUID PRIMARY KEY,
  field_definition_id UUID NOT NULL REFERENCES custom_field_definitions(id) ON DELETE CASCADE,
  technical_key TEXT NOT NULL CHECK (technical_key ~ '^[a-z][a-z0-9_]{1,63}$'),
  label TEXT NOT NULL CHECK (label = btrim(label) AND label <> '' AND char_length(label) <= 120),
  active BOOLEAN NOT NULL DEFAULT true,
  sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (field_definition_id, technical_key),
  UNIQUE (id, field_definition_id)
);
CREATE INDEX custom_field_options_order_index
  ON custom_field_options (field_definition_id, active, sort_order, lower(label), id);

CREATE TABLE custom_entities (
  id UUID PRIMARY KEY,
  custom_entity_type_id UUID NOT NULL REFERENCES custom_entity_types(id) ON DELETE RESTRICT,
  owner_profile_id UUID REFERENCES profiles(id) ON DELETE RESTRICT,
  profile_cardinality TEXT CHECK (
    profile_cardinality IS NULL OR profile_cardinality IN ('ONE_PER_PROFILE', 'MANY_PER_PROFILE')
  ),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (
    (profile_cardinality IS NULL AND owner_profile_id IS NULL) OR
    (profile_cardinality IN ('ONE_PER_PROFILE', 'MANY_PER_PROFILE') AND owner_profile_id IS NOT NULL)
  )
);
CREATE UNIQUE INDEX custom_entities_one_per_profile_unique
  ON custom_entities (custom_entity_type_id, owner_profile_id)
  WHERE profile_cardinality = 'ONE_PER_PROFILE';
CREATE INDEX custom_entities_type_index
  ON custom_entities (custom_entity_type_id, updated_at DESC, id);
CREATE INDEX custom_entities_owner_index
  ON custom_entities (owner_profile_id, custom_entity_type_id, updated_at DESC, id)
  WHERE owner_profile_id IS NOT NULL;

CREATE TABLE custom_field_values (
  id UUID PRIMARY KEY,
  field_definition_id UUID NOT NULL REFERENCES custom_field_definitions(id) ON DELETE RESTRICT,
  profile_id UUID REFERENCES profiles(id) ON DELETE CASCADE,
  document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
  bill_id UUID REFERENCES bills(id) ON DELETE CASCADE,
  custom_entity_id UUID REFERENCES custom_entities(id) ON DELETE CASCADE,
  field_kind TEXT NOT NULL CHECK (
    field_kind IN (
      'TEXT', 'LONG_TEXT', 'INTEGER', 'DECIMAL', 'BOOLEAN', 'CIVIL_DATE',
      'CIVIL_MONTH', 'EMAIL', 'PHONE', 'SINGLE_SELECT', 'MULTI_SELECT'
    )
  ),
  text_value TEXT CHECK (text_value IS NULL OR char_length(text_value) <= 5000),
  integer_value BIGINT,
  decimal_value NUMERIC(38, 10),
  boolean_value BOOLEAN,
  civil_date_value DATE,
  civil_month_value TEXT CHECK (
    civil_month_value IS NULL OR civil_month_value ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'
  ),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (id, field_definition_id),
  CHECK (num_nonnulls(profile_id, document_id, bill_id, custom_entity_id) = 1),
  CHECK (
    (field_kind IN ('TEXT', 'LONG_TEXT', 'EMAIL', 'PHONE') AND text_value IS NOT NULL AND
      integer_value IS NULL AND decimal_value IS NULL AND boolean_value IS NULL AND civil_date_value IS NULL AND civil_month_value IS NULL) OR
    (field_kind = 'INTEGER' AND text_value IS NULL AND integer_value IS NOT NULL AND
      decimal_value IS NULL AND boolean_value IS NULL AND civil_date_value IS NULL AND civil_month_value IS NULL) OR
    (field_kind = 'DECIMAL' AND text_value IS NULL AND integer_value IS NULL AND
      decimal_value IS NOT NULL AND boolean_value IS NULL AND civil_date_value IS NULL AND civil_month_value IS NULL) OR
    (field_kind = 'BOOLEAN' AND text_value IS NULL AND integer_value IS NULL AND
      decimal_value IS NULL AND boolean_value IS NOT NULL AND civil_date_value IS NULL AND civil_month_value IS NULL) OR
    (field_kind = 'CIVIL_DATE' AND text_value IS NULL AND integer_value IS NULL AND
      decimal_value IS NULL AND boolean_value IS NULL AND civil_date_value IS NOT NULL AND civil_month_value IS NULL) OR
    (field_kind = 'CIVIL_MONTH' AND text_value IS NULL AND integer_value IS NULL AND
      decimal_value IS NULL AND boolean_value IS NULL AND civil_date_value IS NULL AND civil_month_value IS NOT NULL) OR
    (field_kind IN ('SINGLE_SELECT', 'MULTI_SELECT') AND text_value IS NULL AND integer_value IS NULL AND
      decimal_value IS NULL AND boolean_value IS NULL AND civil_date_value IS NULL AND civil_month_value IS NULL)
  )
);
CREATE UNIQUE INDEX custom_field_values_profile_unique
  ON custom_field_values (profile_id, field_definition_id)
  WHERE profile_id IS NOT NULL;
CREATE UNIQUE INDEX custom_field_values_document_unique
  ON custom_field_values (document_id, field_definition_id)
  WHERE document_id IS NOT NULL;
CREATE UNIQUE INDEX custom_field_values_bill_unique
  ON custom_field_values (bill_id, field_definition_id)
  WHERE bill_id IS NOT NULL;
CREATE UNIQUE INDEX custom_field_values_entity_unique
  ON custom_field_values (custom_entity_id, field_definition_id)
  WHERE custom_entity_id IS NOT NULL;
CREATE INDEX custom_field_values_definition_index
  ON custom_field_values (field_definition_id, updated_at DESC, id);

CREATE TABLE custom_field_value_options (
  custom_field_value_id UUID NOT NULL,
  field_definition_id UUID NOT NULL,
  option_id UUID NOT NULL,
  PRIMARY KEY (custom_field_value_id, option_id),
  FOREIGN KEY (custom_field_value_id, field_definition_id)
    REFERENCES custom_field_values(id, field_definition_id) ON DELETE CASCADE,
  FOREIGN KEY (option_id, field_definition_id)
    REFERENCES custom_field_options(id, field_definition_id) ON DELETE RESTRICT
);
CREATE INDEX custom_field_value_options_option_index
  ON custom_field_value_options (option_id, custom_field_value_id);

INSERT INTO app_metadata (key, value)
VALUES ('schema.custom_data', 'm5')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.custom_data';
DROP TABLE custom_field_value_options;
DROP TABLE custom_field_values;
DROP TABLE custom_entities;
DROP TABLE custom_field_options;
DROP TABLE custom_field_definitions;
DROP TABLE custom_entity_types;

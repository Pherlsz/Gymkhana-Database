-- M4.1 canonical document and bill persistence with current-use state.

CREATE TABLE document_types (
  id UUID PRIMARY KEY,
  technical_key TEXT NOT NULL UNIQUE CHECK (technical_key ~ '^[a-z][a-z0-9_]{1,63}$'),
  label TEXT NOT NULL CHECK (label = btrim(label) AND label <> '' AND char_length(label) <= 120),
  active BOOLEAN NOT NULL DEFAULT true,
  uniqueness_policy TEXT NOT NULL CHECK (uniqueness_policy IN ('NONE', 'PER_PROFILE', 'GLOBAL_BY_TYPE')),
  validation_regex TEXT CHECK (validation_regex IS NULL OR (validation_regex = btrim(validation_regex) AND validation_regex <> '' AND char_length(validation_regex) <= 500)),
  date_required BOOLEAN NOT NULL DEFAULT false,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX document_types_label_index ON document_types (lower(label), id);
CREATE INDEX document_types_active_index ON document_types (active, lower(label), id);

CREATE TABLE documents (
  id UUID PRIMARY KEY,
  owner_profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE RESTRICT,
  document_type_id UUID NOT NULL REFERENCES document_types(id) ON DELETE RESTRICT,
  identifier_value TEXT NOT NULL CHECK (identifier_value = btrim(identifier_value) AND identifier_value <> '' AND char_length(identifier_value) <= 500),
  uniqueness_policy TEXT NOT NULL CHECK (uniqueness_policy IN ('NONE', 'PER_PROFILE', 'GLOBAL_BY_TYPE')),
  document_date DATE,
  notes TEXT CHECK (notes IS NULL OR (notes <> '' AND char_length(notes) <= 5000)),
  record_state TEXT NOT NULL DEFAULT 'CURRENT' CHECK (record_state IN ('CURRENT', 'REPLACED', 'EXPIRED', 'ARCHIVED')),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX documents_per_profile_unique
  ON documents (owner_profile_id, document_type_id, identifier_value)
  WHERE uniqueness_policy = 'PER_PROFILE';
CREATE UNIQUE INDEX documents_global_by_type_unique
  ON documents (document_type_id, identifier_value)
  WHERE uniqueness_policy = 'GLOBAL_BY_TYPE';
CREATE INDEX documents_owner_index ON documents (owner_profile_id, updated_at DESC, id);
CREATE INDEX documents_type_index ON documents (document_type_id, identifier_value, id);
CREATE INDEX documents_identifier_search_index ON documents (lower(identifier_value) text_pattern_ops, id);

CREATE TABLE document_current_uses (
  document_id UUID PRIMARY KEY REFERENCES documents(id) ON DELETE RESTRICT,
  holder_profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE RESTRICT,
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX document_current_uses_holder_index ON document_current_uses (holder_profile_id, assigned_at DESC);

CREATE TABLE bill_types (
  id UUID PRIMARY KEY,
  technical_key TEXT NOT NULL UNIQUE CHECK (technical_key ~ '^[a-z][a-z0-9_]{1,63}$'),
  label TEXT NOT NULL CHECK (label = btrim(label) AND label <> '' AND char_length(label) <= 120),
  active BOOLEAN NOT NULL DEFAULT true,
  supports_current_use BOOLEAN NOT NULL DEFAULT false,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX bill_types_label_index ON bill_types (lower(label), id);
CREATE INDEX bill_types_active_index ON bill_types (active, lower(label), id);

CREATE TABLE bills (
  id UUID PRIMARY KEY,
  owner_profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE RESTRICT,
  bill_type_id UUID NOT NULL REFERENCES bill_types(id) ON DELETE RESTRICT,
  printed_holder_name TEXT CHECK (printed_holder_name IS NULL OR (printed_holder_name = btrim(printed_holder_name) AND printed_holder_name <> '' AND char_length(printed_holder_name) <= 200)),
  printed_address TEXT CHECK (printed_address IS NULL OR (printed_address = btrim(printed_address) AND printed_address <> '' AND char_length(printed_address) <= 500)),
  reference_value TEXT CHECK (reference_value IS NULL OR (reference_value = btrim(reference_value) AND reference_value <> '' AND char_length(reference_value) <= 500)),
  competence TEXT CHECK (competence IS NULL OR competence ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
  amount NUMERIC(18,2) CHECK (amount IS NULL OR amount >= 0),
  currency TEXT CHECK (currency IS NULL OR currency ~ '^[A-Z]{3}$'),
  notes TEXT CHECK (notes IS NULL OR (notes <> '' AND char_length(notes) <= 5000)),
  record_state TEXT NOT NULL DEFAULT 'CURRENT' CHECK (record_state IN ('CURRENT', 'REPLACED', 'EXPIRED', 'ARCHIVED')),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((amount IS NULL AND currency IS NULL) OR (amount IS NOT NULL AND currency IS NOT NULL))
);
CREATE INDEX bills_owner_index ON bills (owner_profile_id, updated_at DESC, id);
CREATE INDEX bills_type_index ON bills (bill_type_id, competence DESC, id);
CREATE INDEX bills_reference_search_index ON bills (lower(reference_value) text_pattern_ops, id) WHERE reference_value IS NOT NULL;

CREATE TABLE bill_current_uses (
  bill_id UUID PRIMARY KEY REFERENCES bills(id) ON DELETE RESTRICT,
  holder_profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE RESTRICT,
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX bill_current_uses_holder_index ON bill_current_uses (holder_profile_id, assigned_at DESC);

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

INSERT INTO app_metadata (key, value)
VALUES ('schema.documents_bills', 'm4.1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.documents_bills';
DROP TRIGGER bill_current_uses_supported_trigger ON bill_current_uses;
DROP FUNCTION ensure_bill_current_use_supported();
DROP TABLE bill_current_uses;
DROP TABLE bills;
DROP TABLE bill_types;
DROP TABLE document_current_uses;
DROP TABLE documents;
DROP TABLE document_types;

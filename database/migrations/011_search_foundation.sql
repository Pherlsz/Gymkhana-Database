-- M7 authorized global Search foundation and persistent rate limiting.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE search_rate_limits (
  actor_user_id UUID PRIMARY KEY REFERENCES app_users(id) ON DELETE CASCADE,
  window_started_at TIMESTAMPTZ NOT NULL,
  request_count INTEGER NOT NULL CHECK (request_count > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX profiles_full_name_trgm_index
  ON profiles USING gin (lower(full_name) gin_trgm_ops);
CREATE INDEX profiles_social_name_trgm_index
  ON profiles USING gin (lower(social_name) gin_trgm_ops)
  WHERE social_name IS NOT NULL;
CREATE INDEX profiles_address_street_trgm_index
  ON profiles USING gin (lower(address_street) gin_trgm_ops)
  WHERE address_street IS NOT NULL;
CREATE INDEX profiles_address_city_trgm_index
  ON profiles USING gin (lower(address_city) gin_trgm_ops)
  WHERE address_city IS NOT NULL;
CREATE INDEX documents_identifier_trgm_index
  ON documents USING gin (lower(identifier_value) gin_trgm_ops);
CREATE INDEX document_types_label_trgm_index
  ON document_types USING gin (lower(label) gin_trgm_ops);
CREATE INDEX bills_holder_trgm_index
  ON bills USING gin (lower(printed_holder_name) gin_trgm_ops)
  WHERE printed_holder_name IS NOT NULL;
CREATE INDEX bills_address_trgm_index
  ON bills USING gin (lower(printed_address) gin_trgm_ops)
  WHERE printed_address IS NOT NULL;
CREATE INDEX bills_reference_trgm_index
  ON bills USING gin (lower(reference_value) gin_trgm_ops)
  WHERE reference_value IS NOT NULL;
CREATE INDEX bill_types_label_trgm_index
  ON bill_types USING gin (lower(label) gin_trgm_ops);
CREATE INDEX custom_field_values_text_trgm_index
  ON custom_field_values USING gin (lower(text_value) gin_trgm_ops)
  WHERE text_value IS NOT NULL;
CREATE INDEX custom_field_options_label_trgm_index
  ON custom_field_options USING gin (lower(label) gin_trgm_ops);
CREATE INDEX attachments_filename_trgm_index
  ON attachments USING gin (lower(original_filename) gin_trgm_ops)
  WHERE lifecycle_state = 'ACTIVE';

INSERT INTO app_metadata (key, value)
VALUES ('schema.search', 'm7')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.search';
DROP INDEX attachments_filename_trgm_index;
DROP INDEX custom_field_options_label_trgm_index;
DROP INDEX custom_field_values_text_trgm_index;
DROP INDEX bill_types_label_trgm_index;
DROP INDEX bills_reference_trgm_index;
DROP INDEX bills_address_trgm_index;
DROP INDEX bills_holder_trgm_index;
DROP INDEX document_types_label_trgm_index;
DROP INDEX documents_identifier_trgm_index;
DROP INDEX profiles_address_city_trgm_index;
DROP INDEX profiles_address_street_trgm_index;
DROP INDEX profiles_social_name_trgm_index;
DROP INDEX profiles_full_name_trgm_index;
DROP TABLE search_rate_limits;

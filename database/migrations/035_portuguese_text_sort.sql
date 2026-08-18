-- Portuguese ICU collation for A→Z / Z→A text sorts.
-- Binary lower(full_name) put Á after Z (and before Z in DESC). Catalog-only;
-- no table rewrite. Fall back to the ICU root locale when pt-BR is unavailable.

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_collation WHERE collname = 'gymkhana_pt_br') THEN
    RETURN;
  END IF;
  BEGIN
    CREATE COLLATION gymkhana_pt_br (
      provider = icu,
      locale = 'pt-BR',
      deterministic = true
    );
  EXCEPTION
    WHEN invalid_parameter_value OR feature_not_supported THEN
      CREATE COLLATION gymkhana_pt_br (
        provider = icu,
        locale = 'und',
        deterministic = true
      );
  END;
END $$;

INSERT INTO app_metadata (key, value)
VALUES ('schema.portuguese_text_sort', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.portuguese_text_sort';
DROP COLLATION IF EXISTS gymkhana_pt_br;

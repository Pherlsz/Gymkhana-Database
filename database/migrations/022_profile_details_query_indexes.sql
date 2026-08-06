-- Index only the canonical profile-detail fields used by common cross-data Gymkhana queries.
-- pg_trgm is installed by migration 011_search_foundation.sql.

CREATE INDEX profile_details_mother_name_trgm_index
  ON profile_details USING gin (lower(mother_name) gin_trgm_ops)
  WHERE mother_name IS NOT NULL;

CREATE INDEX profile_details_father_name_trgm_index
  ON profile_details USING gin (lower(father_name) gin_trgm_ops)
  WHERE father_name IS NOT NULL;

CREATE INDEX profile_details_birth_city_trgm_index
  ON profile_details USING gin (lower(birth_city) gin_trgm_ops)
  WHERE birth_city IS NOT NULL;

CREATE INDEX profile_details_team_trgm_index
  ON profile_details USING gin (lower(team) gin_trgm_ops)
  WHERE team IS NOT NULL;

INSERT INTO app_metadata (key, value)
VALUES ('schema.profile_details_query_indexes', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.profile_details_query_indexes';
DROP INDEX profile_details_team_trgm_index;
DROP INDEX profile_details_birth_city_trgm_index;
DROP INDEX profile_details_father_name_trgm_index;
DROP INDEX profile_details_mother_name_trgm_index;

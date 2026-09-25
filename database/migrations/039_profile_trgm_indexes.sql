-- Add GIN trigram indexes for LIKE '%…%' filters on email and address_city.
-- CountProfiles, ListProfiles and ListDistinctCities use these fields with
-- lower(…) LIKE '%…%' patterns; without trigram indexes Postgres falls back
-- to a sequential scan on every request.

CREATE INDEX profiles_email_trgm_index
  ON profiles USING gin (lower(email) gin_trgm_ops)
  WHERE email IS NOT NULL;

CREATE INDEX profiles_address_city_trgm_index
  ON profiles USING gin (lower(address_city) gin_trgm_ops)
  WHERE address_city IS NOT NULL;

INSERT INTO app_metadata (key, value)
VALUES ('schema.profile_trgm_indexes', 'm39')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.profile_trgm_indexes';
DROP INDEX profiles_address_city_trgm_index;
DROP INDEX profiles_email_trgm_index;

-- Known numbers without an exemplar belong on document_presences (030).
-- Do not null medium: PHYSICAL/DIGITAL rows from 025 remain exemplars.

INSERT INTO app_metadata (key, value)
VALUES ('schema.known_number_medium', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.known_number_medium';

-- Collapse sign-in authorization onto app_users alone.
-- Google login requires a provisioned, active app_users row; allowed_emails is redundant.

DROP TABLE IF EXISTS allowed_emails;

INSERT INTO app_metadata (key, value)
VALUES ('schema.auth_access', 'app-users-only-v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.auth_access';

CREATE TABLE allowed_emails (
  email TEXT PRIMARY KEY CHECK (
    email = lower(btrim(email))
    AND email <> ''
    AND char_length(email) <= 320
  ),
  added_by UUID REFERENCES app_users(id) ON DELETE SET NULL,
  added_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO allowed_emails (email)
SELECT lower(email)
FROM app_users
WHERE active
ON CONFLICT (email) DO NOTHING;

-- Replace GitHub identity columns with Google OpenID Connect and a database-backed email allowlist.

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app_users) THEN
    RAISE EXCEPTION 'Google auth migration requires an empty rebuild app_users table; migrate allowlisted emails explicitly before applying in a populated environment';
  END IF;
END;
$$;

DROP INDEX app_users_github_login_ci_unique;
ALTER TABLE app_users ADD COLUMN google_subject TEXT;
ALTER TABLE app_users ADD COLUMN email TEXT;
ALTER TABLE app_users DROP COLUMN github_user_id;
ALTER TABLE app_users DROP COLUMN github_login;
ALTER TABLE app_users ALTER COLUMN email SET NOT NULL;
ALTER TABLE app_users ADD CONSTRAINT app_users_google_subject_check CHECK (
  google_subject IS NULL OR (
    google_subject = btrim(google_subject) AND google_subject <> '' AND char_length(google_subject) <= 255
  )
);
ALTER TABLE app_users ADD CONSTRAINT app_users_email_check CHECK (
  email = lower(btrim(email)) AND email <> '' AND char_length(email) <= 320
);
CREATE UNIQUE INDEX app_users_google_subject_unique ON app_users (google_subject) WHERE google_subject IS NOT NULL;
CREATE UNIQUE INDEX app_users_email_ci_unique ON app_users (lower(email));

ALTER TABLE auth_audit_events RENAME COLUMN provider_login TO provider_email;

INSERT INTO app_metadata (key, value)
VALUES ('schema.authentication_provider', 'google-email-allowlist-v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app_users) THEN
    RAISE EXCEPTION 'Google auth rollback requires an empty app_users table';
  END IF;
END;
$$;

DELETE FROM app_metadata WHERE key = 'schema.authentication_provider';
ALTER TABLE auth_audit_events RENAME COLUMN provider_email TO provider_login;
DROP INDEX app_users_email_ci_unique;
DROP INDEX app_users_google_subject_unique;
ALTER TABLE app_users DROP CONSTRAINT app_users_email_check;
ALTER TABLE app_users DROP CONSTRAINT app_users_google_subject_check;
ALTER TABLE app_users ADD COLUMN github_user_id BIGINT;
ALTER TABLE app_users ADD COLUMN github_login TEXT;
ALTER TABLE app_users DROP COLUMN email;
ALTER TABLE app_users DROP COLUMN google_subject;
ALTER TABLE app_users ALTER COLUMN github_user_id SET NOT NULL;
ALTER TABLE app_users ALTER COLUMN github_login SET NOT NULL;
ALTER TABLE app_users ADD CONSTRAINT app_users_github_user_id_check CHECK (github_user_id > 0);
ALTER TABLE app_users ADD CONSTRAINT app_users_github_login_check CHECK (github_login = btrim(github_login) AND github_login <> '');
ALTER TABLE app_users ADD CONSTRAINT app_users_github_user_id_key UNIQUE (github_user_id);
CREATE UNIQUE INDEX app_users_github_login_ci_unique ON app_users (lower(github_login));

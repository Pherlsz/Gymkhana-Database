-- SQLC overlay for the final authentication schema after migrations 019 and 023.
-- The historical schema.sql intentionally remains the original rebuild baseline;
-- this file applies the auth evolution that query generation must see while
-- preserving the final app_users column order used by the checked-in dbgen model.

DROP INDEX app_users_github_login_ci_unique;
ALTER TABLE app_users DROP CONSTRAINT app_users_github_user_id_key;
ALTER TABLE app_users DROP CONSTRAINT app_users_github_user_id_check;
ALTER TABLE app_users DROP CONSTRAINT app_users_github_login_check;

ALTER TABLE app_users RENAME COLUMN github_user_id TO email;
ALTER TABLE app_users ALTER COLUMN email TYPE TEXT USING email::text;
ALTER TABLE app_users RENAME COLUMN github_login TO subject;

ALTER TABLE app_users DROP CONSTRAINT app_users_role_check;
ALTER TABLE app_users
  ADD CONSTRAINT app_users_role_check
  CHECK (role IN ('EXTERNAL', 'ADMIN', 'SUPERADMIN'));

ALTER TABLE app_users
  ADD CONSTRAINT app_users_email_check
  CHECK (
    email = lower(btrim(email))
    AND email <> ''
    AND char_length(email) <= 320
  );
ALTER TABLE app_users
  ADD CONSTRAINT app_users_subject_check
  CHECK (
    subject = btrim(subject)
    AND subject <> ''
    AND char_length(subject) <= 255
  );

CREATE UNIQUE INDEX app_users_email_ci_unique ON app_users (lower(email));
CREATE UNIQUE INDEX app_users_subject_unique ON app_users (subject);

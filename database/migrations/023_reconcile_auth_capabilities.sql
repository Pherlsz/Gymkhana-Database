-- Reconcile the authentication/capability schema after the profile/document
-- migration line (019-022) and the capability work were merged in parallel.
--
-- The earlier capability migrations reused migration numbers 019-022 and also
-- targeted the pre-Google-auth schema. This migration applies that intent once,
-- after the canonical 019-022 sequence, and aligns the persisted names with the
-- current sqlc/runtime contract.

ALTER TABLE app_users RENAME COLUMN google_subject TO subject;
ALTER TABLE auth_audit_events RENAME COLUMN provider_email TO provider_login;

ALTER TABLE app_users DROP CONSTRAINT app_users_role_check;
UPDATE app_users SET role = 'EXTERNAL' WHERE role = 'MEMBER';
ALTER TABLE app_users
  ADD CONSTRAINT app_users_role_check
  CHECK (role IN ('EXTERNAL', 'ADMIN', 'SUPERADMIN'));

CREATE TABLE app_user_capabilities (
  user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  capability TEXT NOT NULL CHECK (
    capability IN (
      'PROFILES',
      'DATA_TABLES',
      'SEARCH',
      'OCR',
      'OPERATIONS',
      'MATCHING',
      'GOOGLE_FORMS',
      'ATTACHMENTS',
      'CHAT',
      'QUERY',
      'TASKS',
      'CUSTOM_DATA'
    )
  ),
  granted_by UUID REFERENCES app_users(id) ON DELETE SET NULL,
  granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, capability)
);

CREATE INDEX app_user_capabilities_user_id_index
  ON app_user_capabilities (user_id);
CREATE INDEX app_user_capabilities_capability_index
  ON app_user_capabilities (capability);

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

INSERT INTO app_user_capabilities (user_id, capability)
SELECT app_users.id, capabilities.capability
FROM app_users
CROSS JOIN (
  VALUES
    ('SEARCH'),
    ('PROFILES'),
    ('DATA_TABLES'),
    ('ATTACHMENTS'),
    ('QUERY')
) AS capabilities(capability)
WHERE app_users.active
  AND app_users.role = 'EXTERNAL'
ON CONFLICT (user_id, capability) DO NOTHING;

INSERT INTO app_metadata (key, value)
VALUES ('schema.authorization', 'capabilities-v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.authorization';
DROP TABLE allowed_emails;
DROP TABLE app_user_capabilities;

ALTER TABLE app_users DROP CONSTRAINT app_users_role_check;
UPDATE app_users SET role = 'MEMBER' WHERE role = 'EXTERNAL';
ALTER TABLE app_users
  ADD CONSTRAINT app_users_role_check
  CHECK (role IN ('MEMBER', 'ADMIN', 'SUPERADMIN'));

ALTER TABLE auth_audit_events RENAME COLUMN provider_login TO provider_email;
ALTER TABLE app_users RENAME COLUMN subject TO google_subject;

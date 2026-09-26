-- Product feature enablement lives in PostgreSQL; credentials stay in env/secrets.
-- SUPERADMIN toggles these via Administração. Defaults are off (fail-closed).

CREATE TABLE app_feature_flags (
  key text PRIMARY KEY CHECK (
    key = lower(btrim(key))
    AND key <> ''
    AND char_length(key) <= 64
  ),
  enabled boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid REFERENCES app_users (id) ON DELETE SET NULL
);

INSERT INTO app_feature_flags (key, enabled) VALUES
  ('ai_chat', false),
  ('google_forms', false),
  ('ocr', false),
  ('attachments', false)
ON CONFLICT (key) DO NOTHING;

INSERT INTO app_metadata (key, value)
VALUES ('schema.feature_flags', 'admin-v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.feature_flags';
DROP TABLE IF EXISTS app_feature_flags;

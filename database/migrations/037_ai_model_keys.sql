-- Shared model-provider key managed in Administração (Orchestration §18.1).
-- One row per provider; the secret is AES-256-GCM sealed with the application
-- token cipher and is never returned to clients.

CREATE TABLE ai_model_keys (
  provider TEXT PRIMARY KEY CHECK (provider ~ '^[a-z][a-z0-9_-]{1,31}$'),
  secret_ciphertext BYTEA NOT NULL CHECK (octet_length(secret_ciphertext) BETWEEN 16 AND 4096),
  secret_nonce BYTEA NOT NULL CHECK (octet_length(secret_nonce) = 12),
  key_version INTEGER NOT NULL CHECK (key_version BETWEEN 1 AND 65535),
  model TEXT NOT NULL CHECK (model = btrim(model) AND char_length(model) BETWEEN 1 AND 120),
  updated_by UUID REFERENCES app_users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO app_metadata (key, value)
VALUES ('schema.ai_model_keys', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.ai_model_keys';
DROP TABLE ai_model_keys;

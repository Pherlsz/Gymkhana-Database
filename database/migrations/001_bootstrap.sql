-- Write your migrate up statements here

CREATE TABLE app_metadata (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO app_metadata (key, value)
VALUES ('schema.bootstrap', 'm0')
ON CONFLICT (key) DO NOTHING;

---- create above / drop below ----

DROP TABLE app_metadata;

-- SQLC-only projection of the final authentication model after migrations 019 and 023.
-- The historical schema.sql remains the rebuild baseline. For code generation we
-- replace only its legacy app_users definition with the stable v1 logical shape.
-- Runtime migrations remain the source of truth for physical database evolution.

ALTER TABLE app_users RENAME TO app_users_legacy;

CREATE TABLE app_users (
  id UUID PRIMARY KEY,
  email TEXT NOT NULL,
  subject TEXT NOT NULL,
  display_name TEXT NOT NULL,
  avatar_url TEXT,
  role TEXT NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

DROP TABLE app_users_legacy CASCADE;

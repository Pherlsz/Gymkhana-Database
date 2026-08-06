-- Migration 021: Email allowlist for Google OAuth
-- Only users with emails in this table can authenticate

BEGIN;

CREATE TABLE allowed_emails (
    email TEXT PRIMARY KEY,
    added_by UUID REFERENCES app_users(id) ON DELETE SET NULL,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMIT;

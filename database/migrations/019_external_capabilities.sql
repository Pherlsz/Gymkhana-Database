-- Migration 019: external capabilities
-- Replaces MEMBER role with EXTERNAL + explicit capability grants.
-- EXTERNAL users start with zero access; ADMIN/SUPERADMIN keep full role-derived access.

BEGIN;

-- Create capability grants table
CREATE TABLE app_user_capabilities (
  user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  capability TEXT NOT NULL CHECK (capability IN (
    'SENSITIVE_DATA',
    'PROFILES',
    'DATA_TABLES',
    'OCR',
    'AI',
    'ATTACHMENTS',
    'SEARCH',
    'EXPORT',
    'OPERATIONS',
    'MATCHING',
    'QUERY',
    'GOOGLE_FORMS'
  )),
  granted_by UUID REFERENCES app_users(id) ON DELETE SET NULL,
  granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, capability)
);

CREATE INDEX idx_user_capabilities_user ON app_user_capabilities(user_id);

-- Rename MEMBER role to EXTERNAL in the role CHECK constraint
ALTER TABLE app_users DROP CONSTRAINT app_users_role_check;
ALTER TABLE app_users ADD CONSTRAINT app_users_role_check
  CHECK (role IN ('EXTERNAL', 'ADMIN', 'SUPERADMIN'));

-- Migrate existing MEMBER rows to EXTERNAL
UPDATE app_users SET role = 'EXTERNAL' WHERE role = 'MEMBER';

COMMIT;

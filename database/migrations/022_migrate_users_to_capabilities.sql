-- Migration 022: Migrate existing users to capabilities system
-- 1. Add all existing users to the email allowlist
-- 2. Grant basic capabilities to active EXTERNAL users

BEGIN;

-- Add all existing users to allowlist (preserve access)
INSERT INTO allowed_emails (email)
SELECT DISTINCT email FROM app_users WHERE email IS NOT NULL AND email != ''
ON CONFLICT (email) DO NOTHING;

-- Grant basic capabilities to active EXTERNAL users
-- Based on the old MEMBER role permissions: search, profiles, documents, bills
INSERT INTO app_user_capabilities (user_id, capability)
SELECT u.id, cap.capability
FROM app_users u
CROSS JOIN (VALUES 
    ('search'::text),
    ('profiles'),
    ('documents'),
    ('bills'),
    ('attachments'),
    ('query')
) AS cap(capability)
WHERE u.role = 'EXTERNAL' AND u.active = true
ON CONFLICT (user_id, capability) DO NOTHING;

COMMIT;

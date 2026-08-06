-- Migration 020: Replace GitHub OAuth with Google OAuth
-- Removes github_user_id and login columns, adds email and subject columns

BEGIN;

-- Drop the github_user_id column and login column
ALTER TABLE app_users DROP COLUMN IF EXISTS github_user_id;
ALTER TABLE app_users DROP COLUMN IF EXISTS login;

-- Add email and subject columns for Google OAuth
ALTER TABLE app_users ADD COLUMN email TEXT;
ALTER TABLE app_users ADD COLUMN subject TEXT;

-- Best-effort migration: set email to login for existing users
UPDATE app_users SET email = login WHERE email IS NULL;

-- Make email NOT NULL after backfill
ALTER TABLE app_users ALTER COLUMN email SET NOT NULL;

-- Add unique index on email (case-insensitive)
CREATE UNIQUE INDEX idx_app_users_email ON app_users(lower(email));

COMMIT;

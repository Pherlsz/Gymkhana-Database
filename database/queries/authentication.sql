-- name: CreateAppUser :one
INSERT INTO app_users (
  id,
  google_subject,
  email,
  display_name,
  avatar_url,
  role,
  active
)
VALUES ($1, $2, lower(btrim($3)), $4, $5, $6, $7)
RETURNING *;

-- name: GetAppUserByID :one
SELECT *
FROM app_users
WHERE id = $1;

-- name: GetAppUserByGoogleSubject :one
SELECT *
FROM app_users
WHERE google_subject = $1;

-- name: GetAppUserByEmail :one
SELECT *
FROM app_users
WHERE lower(email) = lower(btrim($1));

-- name: ListAppUsers :many
SELECT *
FROM app_users
ORDER BY lower(email), id
LIMIT $1 OFFSET $2;

-- name: UpdateAppUserIdentity :one
UPDATE app_users
SET google_subject = COALESCE(google_subject, $2),
    email = lower(btrim($3)),
    display_name = $4,
    avatar_url = $5,
    updated_at = now(),
    version = version + 1
WHERE id = $1
  AND (google_subject IS NULL OR google_subject = $2)
RETURNING *;

-- name: UpdateAppUserAccess :one
UPDATE app_users
SET role = $2,
    active = $3,
    updated_at = now(),
    version = version + 1
WHERE id = $1
  AND version = $4
RETURNING *;

-- name: CountActiveSuperadmins :one
SELECT count(*)
FROM app_users
WHERE active AND role = 'SUPERADMIN';

-- name: CreateAppSession :one
INSERT INTO app_sessions (
  id,
  user_id,
  token_hash,
  expires_at
)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAppSessionByTokenHash :one
SELECT *
FROM app_sessions
WHERE token_hash = $1;

-- name: GetAuthenticatedAppSession :one
SELECT
  app_sessions.id AS session_id,
  app_users.id AS app_user_id,
  app_users.google_subject,
  app_users.email,
  app_users.display_name,
  app_users.avatar_url,
  app_users.role,
  app_users.active
FROM app_sessions
JOIN app_users ON app_users.id = app_sessions.user_id
WHERE app_sessions.token_hash = $1
  AND app_sessions.revoked_at IS NULL
  AND app_sessions.expires_at > $2
  AND app_users.active;

-- name: TouchAppSession :exec
UPDATE app_sessions
SET last_seen_at = now()
WHERE id = $1
  AND revoked_at IS NULL
  AND expires_at > now();

-- name: RevokeAppSession :exec
UPDATE app_sessions
SET revoked_at = COALESCE(revoked_at, now())
WHERE id = $1;

-- name: RevokeAppSessionByTokenHash :exec
UPDATE app_sessions
SET revoked_at = COALESCE(revoked_at, now())
WHERE token_hash = $1;

-- name: RevokeAllAppSessionsForUser :exec
UPDATE app_sessions
SET revoked_at = COALESCE(revoked_at, now())
WHERE user_id = $1;

-- name: CreateAuthAuditEvent :one
INSERT INTO auth_audit_events (
  id,
  actor_user_id,
  subject_user_id,
  event_type,
  outcome,
  request_id,
  provider_email
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

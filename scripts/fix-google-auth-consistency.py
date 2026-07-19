#!/usr/bin/env python3
from pathlib import Path
import re


def read(path: str) -> str:
    return Path(path).read_text(encoding="utf-8")


def write(path: str, text: str) -> None:
    Path(path).write_text(text, encoding="utf-8")


runtime_files = [
    "internal/taskengine/postgres_store.go",
    "internal/googleforms/postgres_sync.go",
    "internal/operations/postgres_exports.go",
    "internal/aichat/postgres_store.go",
    "internal/queryengine/postgres_store.go",
    "internal/ocr/postgres_store.go",
]
for path in runtime_files:
    text = read(path)
    text = text.replace("github_user_id", "google_subject")
    text = text.replace("github_login", "email")
    text = text.replace("&value.GitHubUserID", "&value.GoogleSubject")
    text = text.replace("&value.Login", "&value.Email")
    text = text.replace("&session.User.GitHubUserID", "&session.User.GoogleSubject")
    text = text.replace("&session.User.Login", "&session.User.Email")
    text = re.sub(
        r"(value\.ID = auth\.Identifier\(databaseID\.Bytes\)\n)(?!\s*value\.Login = value\.Email)",
        r"\1\tvalue.Login = value.Email\n",
        text,
        count=1,
    )
    text = re.sub(
        r"(session\.User\.ID = authIdentifierFromUUID\(id\)\n)(?!\s*session\.User\.Login = session\.User\.Email)",
        r"\1\tsession.User.Login = session.User.Email\n",
        text,
        count=1,
    )
    if "github_user_id" in text or "github_login" in text:
        raise SystemExit(f"{path}: legacy database identity columns remain")
    write(path, text)

integration_files = [
    "internal/attachment/postgres_store_integration_test.go",
    "internal/matching/postgres_store_integration_test.go",
    "internal/ocr/postgres_store_integration_test.go",
    "internal/aichat/postgres_store_integration_test.go",
    "internal/queryengine/postgres_store_integration_test.go",
    "internal/operations/postgres_store_integration_test.go",
    "internal/googleforms/postgres_store_integration_test.go",
    "internal/search/postgres_store_integration_test.go",
]
for path in integration_files:
    text = read(path)
    text = text.replace("github_user_id", "google_subject")
    text = text.replace("github_login", "email")
    text, count = re.subn(
        r"(INSERT INTO app_users[\s\S]{0,220}?VALUES\(\$1,)\$2(,\$3)",
        r"\1$2::bigint::text\2",
        text,
    )
    if count == 0:
        raise SystemExit(f"{path}: app_users fixture was not converted")
    if "github_user_id" in text or "github_login" in text:
        raise SystemExit(f"{path}: legacy database identity columns remain")
    write(path, text)

schema = read("database/schema.sql")
schema, count = re.subn(
    r"CREATE TABLE app_users \([\s\S]*?CREATE UNIQUE INDEX app_users_one_active_superadmin ON app_users \(role\) WHERE active AND role = 'SUPERADMIN';",
    """CREATE TABLE app_users (
  id UUID PRIMARY KEY,
  google_subject TEXT CHECK (
    google_subject IS NULL OR (
      google_subject = btrim(google_subject) AND google_subject <> '' AND char_length(google_subject) <= 255
    )
  ),
  email TEXT NOT NULL CHECK (
    email = lower(btrim(email)) AND email <> '' AND char_length(email) <= 320
  ),
  display_name TEXT NOT NULL CHECK (display_name = btrim(display_name) AND display_name <> ''),
  avatar_url TEXT,
  role TEXT NOT NULL CHECK (role IN ('MEMBER', 'ADMIN', 'SUPERADMIN')),
  active BOOLEAN NOT NULL DEFAULT true,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX app_users_google_subject_unique ON app_users (google_subject) WHERE google_subject IS NOT NULL;
CREATE UNIQUE INDEX app_users_email_ci_unique ON app_users (lower(email));
CREATE UNIQUE INDEX app_users_one_active_superadmin ON app_users (role) WHERE active AND role = 'SUPERADMIN';""",
    schema,
    count=1,
)
if count != 1:
    raise SystemExit("database/schema.sql: app_users schema replacement failed")
schema = schema.replace("provider_login TEXT", "provider_email TEXT")
if "github_user_id" in schema or "github_login" in schema or "provider_login" in schema:
    raise SystemExit("database/schema.sql still contains legacy authentication columns")
write("database/schema.sql", schema)

write(
    "database/queries/authentication.sql",
    """-- name: CreateAppUser :one
INSERT INTO app_users (id, google_subject, email, display_name, avatar_url, role, active)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAppUserByID :one
SELECT * FROM app_users WHERE id = $1;

-- name: GetAppUserByGoogleSubject :one
SELECT * FROM app_users WHERE google_subject = $1;

-- name: GetAppUserByEmail :one
SELECT * FROM app_users WHERE lower(email) = lower($1);

-- name: ListAppUsers :many
SELECT * FROM app_users ORDER BY lower(email), id LIMIT $1 OFFSET $2;

-- name: UpdateAppUserIdentity :one
UPDATE app_users
SET google_subject = COALESCE(google_subject, $2),
    email = $3,
    display_name = $4,
    avatar_url = $5,
    updated_at = now(),
    version = version + 1
WHERE id = $1 AND (google_subject IS NULL OR google_subject = $2)
RETURNING *;

-- name: UpdateAppUserAccess :one
UPDATE app_users
SET role = $2, active = $3, updated_at = now(), version = version + 1
WHERE id = $1 AND version = $4
RETURNING *;

-- name: CountActiveSuperadmins :one
SELECT count(*) FROM app_users WHERE active AND role = 'SUPERADMIN';

-- name: CreateAppSession :one
INSERT INTO app_sessions (id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAppSessionByTokenHash :one
SELECT * FROM app_sessions WHERE token_hash = $1;

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
UPDATE app_sessions SET last_seen_at = now()
WHERE id = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: RevokeAppSession :exec
UPDATE app_sessions SET revoked_at = COALESCE(revoked_at, now()) WHERE id = $1;

-- name: RevokeAppSessionByTokenHash :exec
UPDATE app_sessions SET revoked_at = COALESCE(revoked_at, now()) WHERE token_hash = $1;

-- name: RevokeAllAppSessionsForUser :exec
UPDATE app_sessions SET revoked_at = COALESCE(revoked_at, now()) WHERE user_id = $1;

-- name: CreateAuthAuditEvent :one
INSERT INTO auth_audit_events (
  id, actor_user_id, subject_user_id, event_type, outcome, request_id, provider_email
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;
""",
)

config_path = "internal/config/config.go"
text = read(config_path)
text = text.replace("GitHubClientID", "GoogleClientID")
text = text.replace("GitHubClientSecret", "GoogleClientSecret")
text = text.replace("GitHubRedirectURL", "GoogleRedirectURL")
text = text.replace("GITHUB_OAUTH_CLIENT_ID", "GOOGLE_LOGIN_OAUTH_CLIENT_ID")
text = text.replace("GITHUB_OAUTH_CLIENT_SECRET", "GOOGLE_LOGIN_OAUTH_CLIENT_SECRET")
text = text.replace("GITHUB_OAUTH_REDIRECT_URL", "GOOGLE_LOGIN_OAUTH_REDIRECT_URL")
text = re.sub(r"\n\s*AllowedLogins\s+\[\]string\n\s*SuperadminLogin\s+string", "", text, count=1)
text = re.sub(r"\n\s*AllowedLogins:\s+commaSeparatedValues\([^\n]+\),\n\s*SuperadminLogin:\s+[^\n]+,", "", text, count=1)
text = text.replace("GitHub OAuth client credentials", "Google OAuth client credentials")
text = re.sub(
    r"\n\tif len\(cfg\.Auth\.AllowedLogins\) == 0 \{[\s\S]*?AUTH_SUPERADMIN_GITHUB_LOGIN must be included in AUTH_ALLOWED_GITHUB_LOGINS\"\)\n\t\}",
    "",
    text,
    count=1,
)
if any(value in text for value in ["AllowedLogins", "SuperadminLogin", "GITHUB_OAUTH", "AUTH_ALLOWED_GITHUB", "AUTH_SUPERADMIN_GITHUB"]):
    raise SystemExit("internal/config/config.go still contains legacy auth configuration")
write(config_path, text)

test_path = "internal/config/config_test.go"
text = read(test_path)
text = text.replace("GITHUB_OAUTH_CLIENT_ID", "GOOGLE_LOGIN_OAUTH_CLIENT_ID")
text = text.replace("GITHUB_OAUTH_CLIENT_SECRET", "GOOGLE_LOGIN_OAUTH_CLIENT_SECRET")
text = text.replace("GITHUB_OAUTH_REDIRECT_URL", "GOOGLE_LOGIN_OAUTH_REDIRECT_URL")
text = re.sub(r'\n\s*"AUTH_ALLOWED_GITHUB_LOGINS",\n\s*"AUTH_SUPERADMIN_GITHUB_LOGIN",', "", text, count=1)
text = re.sub(r'\n\s*t\.Setenv\("AUTH_ALLOWED_GITHUB_LOGINS",[^\n]+\)\n\s*t\.Setenv\("AUTH_SUPERADMIN_GITHUB_LOGIN",[^\n]+\)', "", text)
text = re.sub(
    r"\n\tif len\(cfg\.Auth\.AllowedLogins\)[\s\S]*?\n\tif cfg\.Auth\.SuperadminLogin[\s\S]*?\n\t\}",
    "",
    text,
    count=1,
)
if any(value in text for value in ["AUTH_ALLOWED_GITHUB", "AUTH_SUPERADMIN_GITHUB", "GITHUB_OAUTH", "AllowedLogins", "SuperadminLogin"]):
    raise SystemExit("internal/config/config_test.go still contains legacy auth configuration")
write(test_path, text)

main_path = "cmd/api/main.go"
text = read(main_path)
text = text.replace("auth.NewGitHubProvider(auth.GitHubProviderOptions{", "auth.NewGoogleProvider(auth.GoogleProviderOptions{")
text = text.replace("cfg.Auth.GitHubClientID", "cfg.Auth.GoogleClientID")
text = text.replace("cfg.Auth.GitHubClientSecret", "cfg.Auth.GoogleClientSecret")
text = text.replace("cfg.Auth.GitHubRedirectURL", "cfg.Auth.GoogleRedirectURL")
text = text.replace("configure GitHub OAuth", "configure Google OAuth")
text = re.sub(r"\n\s*AllowedLogins:\s+cfg\.Auth\.AllowedLogins,\n\s*SuperadminLogin:\s+cfg\.Auth\.SuperadminLogin,", "", text, count=1)
if any(value in text for value in ["NewGitHubProvider", "GitHubClient", "AllowedLogins", "SuperadminLogin"]):
    raise SystemExit("cmd/api/main.go still contains legacy authentication composition")
write(main_path, text)

compat = Path("internal/config/google_login_compat.go")
if compat.exists():
    compat.unlink()

app_path = "apps/web/src/App.tsx"
text = read(app_path)
text = text.replace("Acesso privado com GitHub", "Acesso privado com Google")
text = text.replace("conta GitHub previamente autorizada", "conta Google com e-mail previamente autorizado")
text = text.replace("Entrar com GitHub", "Entrar com Google")
write(app_path, text)

app_test_path = "apps/web/src/App.test.tsx"
text = read(app_test_path)
text = text.replace("shows GitHub login", "shows Google login")
text = text.replace("Entrar com GitHub", "Entrar com Google")
write(app_test_path, text)

for path in ["deploy/cloud-run/api.service.yaml.tmpl", "deploy/cloud-run/worker.job.yaml.tmpl"]:
    text = read(path)
    text = re.sub(r"\n\s*- name: AUTH_ALLOWED_GITHUB_LOGINS\n\s*value: __ALLOWED_GITHUB_LOGINS__", "", text)
    text = re.sub(r"\n\s*- name: AUTH_SUPERADMIN_GITHUB_LOGIN\n\s*value: __SUPERADMIN_GITHUB_LOGIN__", "", text)
    text = text.replace("GITHUB_OAUTH", "GOOGLE_LOGIN_OAUTH")
    write(path, text)

render_path = "scripts/render-cloud-run.py"
text = read(render_path)
text = re.sub(r"\nGITHUB_LOGIN = re\.compile\([^\n]+\)", "", text, count=1)
text = re.sub(r"\n\ndef validate_logins\([\s\S]*?return \",\"\.join\(logins\)\n", "", text, count=1)
text = re.sub(r"\n\s*allowed_logins = validate_logins\([^\n]+\)\n\s*superadmin = required\([^\n]+\)\n\s*if not GITHUB_LOGIN\.fullmatch\(superadmin\):[\s\S]*?SUPERADMIN_GITHUB_LOGIN must be present in ALLOWED_GITHUB_LOGINS\"\)\n", "", text, count=1)
text = re.sub(r'\n\s*"ALLOWED_GITHUB_LOGINS": allowed_logins,\n\s*"SUPERADMIN_GITHUB_LOGIN": superadmin,', "", text, count=1)
text = text.replace('"GITHUB_OAUTH_CLIENT_ID",', '"GOOGLE_LOGIN_OAUTH_CLIENT_ID",')
text = text.replace('"GITHUB_OAUTH_CLIENT_SECRET",', '"GOOGLE_LOGIN_OAUTH_CLIENT_SECRET",')
if any(value in text for value in ["GITHUB_LOGIN", "ALLOWED_GITHUB", "SUPERADMIN_GITHUB", "GITHUB_OAUTH"]):
    raise SystemExit("scripts/render-cloud-run.py still contains legacy auth deployment configuration")
write(render_path, text)

for path in [
    ".env.example",
    "README.md",
    "docs/AUTHENTICATION.md",
    "docs/M15_LAUNCH_RUNBOOK.md",
    ".github/workflows/release-artifacts.yml",
]:
    if not Path(path).exists():
        continue
    text = read(path)
    text = text.replace("GITHUB_OAUTH_CLIENT_ID", "GOOGLE_LOGIN_OAUTH_CLIENT_ID")
    text = text.replace("GITHUB_OAUTH_CLIENT_SECRET", "GOOGLE_LOGIN_OAUTH_CLIENT_SECRET")
    text = text.replace("GITHUB_OAUTH_REDIRECT_URL", "GOOGLE_LOGIN_OAUTH_REDIRECT_URL")
    text = text.replace("GitHub OAuth", "Google OAuth")
    text = text.replace("GitHub login", "Google e-mail")
    write(path, text)

for path in ["database/schema.sql", "database/queries/authentication.sql", *runtime_files, *integration_files]:
    text = read(path)
    if "github_user_id" in text or "github_login" in text:
        raise SystemExit(f"{path}: stale legacy database columns remain")

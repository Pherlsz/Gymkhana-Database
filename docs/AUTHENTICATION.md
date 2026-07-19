# Authentication Operations Runbook

Gymkhana Database authenticates application users with Google OpenID Connect and keeps authorization, roles and sessions inside the application.

## Security guarantees

- Google is the only application login provider; there are no local passwords.
- A verified Google e-mail is accepted only when a normalized row already exists in `app_users`.
- OAuth never creates an arbitrary application user.
- The immutable Google `sub` is bound to the allowlisted row on the first successful login and must match on later logins.
- An inactive `app_users` row remains blocked even when the Google account is valid.
- Browser sessions use opaque random values. Only SHA-256 hashes are stored in PostgreSQL.
- Sessions expire after 24 hours and are revocable server-side.
- Cookies are HttpOnly, SameSite=Lax, host-only and Secure outside local/test environments.
- Roles are `MEMBER`, `ADMIN` and exactly one active `SUPERADMIN`.
- Effective role or active-state changes revoke every session belonging to the affected user.
- Authentication and administration events are written to `auth_audit_events` using e-mail as provider identity; OAuth codes, access tokens and provider payloads are never persisted there.

## Required environment values

| Variable | Purpose |
| --- | --- |
| `APP_ENV` | `local`, `test`, `staging` or `production` |
| `DATABASE_URL` | PostgreSQL connection string |
| `AUTH_ENABLED` | Must be `true` outside environments where authentication is intentionally disabled |
| `GOOGLE_LOGIN_OAUTH_CLIENT_ID` | Google OAuth Web client ID used only for application login |
| `GOOGLE_LOGIN_OAUTH_CLIENT_SECRET` | Login OAuth client secret; secret manager only |
| `GOOGLE_LOGIN_OAUTH_REDIRECT_URL` | Absolute API callback URL ending in `/auth/callback` |
| `AUTH_APPLICATION_URL` | Absolute SPA URL used after successful login |
| `VITE_API_BASE_URL` | Browser-visible API origin |

The Google Forms integration uses a separate OAuth client and separate environment variables. Do not reuse its refresh-token scopes or credentials for application login.

## Allowlist and user lifecycle

`app_users` is the authoritative allowlist.

To grant access:

1. create an `app_users` row with a normalized lowercase e-mail, display name, role and `active=true`;
2. leave `google_subject` null before the first login;
3. the first successful login with that verified e-mail binds the Google subject;
4. later logins must present both the same e-mail and the same subject.

To remove access immediately, set `active=false`. The administration service revokes existing sessions when the effective access state changes.

Do not delete and recreate a row merely because a user changes display name or avatar. Those presentation fields are refreshed after login. A change of Google account ownership or e-mail requires an explicit reviewed identity operation rather than silently rebinding the row.

## Local setup

1. Register a Google OAuth Web client with:
   - authorized JavaScript origin `http://localhost:5173`;
   - redirect URI `http://localhost:8080/auth/callback`.
2. Copy `.env.example` to `.env` and fill the database and Google login values.
3. Apply migrations and insert at least one allowlisted user, preserving exactly one active `SUPERADMIN`.
4. Run `make check-config`, `make dev-api` and `make dev-web`.
5. Sign in with the allowlisted Google account.

## Production topology

The supported topology is intentionally portable rather than tied to one runtime:

- the React/Vite SPA is delivered by Vercel;
- the Go API, worker and migration binaries are standard containers and run on Cloud Run in the approved deployment;
- those same binaries remain runnable in any compatible container platform, including a future Vercel-supported Go/container path, without changing domain or persistence contracts;
- Neon PostgreSQL and Cloudflare R2 are external managed dependencies.

Vercel and Cloud Run are complementary in the current deployment, not mutually exclusive architecture choices.

## Smoke test

After an authentication deployment:

1. `GET /health/live` and `/health/ready` return `200` after migrations are applied.
2. An unauthenticated `GET /api/auth/session` returns `401`.
3. Login redirects to Google and returns through `/auth/callback`.
4. An allowlisted active e-mail receives an application session.
5. A Google account whose e-mail is absent from `app_users` is denied and no user row is created.
6. A valid but inactive user is denied.
7. A member cannot access `/api/admin/users`; an administrator can.
8. Changing role or active state invalidates existing sessions for that user.
9. Logout clears the cookie and revokes the server-side session.
10. Audit rows contain request IDs and provider e-mail, but no OAuth code, token or provider payload.

## Recovery

Rotate a login OAuth secret by adding the replacement in the deployment secret manager, redeploying and revoking the old secret only after the new deployment is healthy. Existing application sessions remain independent from the temporary Google access token used during login.

A protected SUPERADMIN transfer must be reviewed and transactional: lock source and target rows, verify ownership, preserve exactly one active SUPERADMIN, revoke both users' sessions and record the operation in the audit/deployment trail.

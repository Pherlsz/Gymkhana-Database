# Authentication Operations Runbook

> ⚠️ **Migração aprovada:** o login será alterado de GitHub OAuth para Google OAuth + allowlist de e-mails (decisão em `ORCHESTRATION.md` §3.1). Este runbook descreve a implementação GitHub OAuth ainda em vigor; será reescrito junto com a migração.

This runbook covers the M2 GitHub OAuth, application-session, authorization, audit, and recovery procedures for Gymkhana Database.

## Security guarantees

- GitHub OAuth is the only application login mechanism; there are no local passwords.
- Access requires the normalized GitHub login to be present in `AUTH_ALLOWED_GITHUB_LOGINS`.
- The application stores the immutable GitHub user ID and refreshes display identity after successful login.
- Browser sessions use opaque random values. Only SHA-256 hashes are stored in PostgreSQL.
- Sessions expire after 24 hours and are revocable server-side.
- Session cookies are HttpOnly, SameSite=Lax, host-only, and Secure in staging and production.
- Credentialed CORS and state-changing browser requests accept only the exact origin derived from `AUTH_APPLICATION_URL`.
- Roles are `MEMBER`, `ADMIN`, and one protected `SUPERADMIN`.
- The generic administration surface cannot change the current account or the `SUPERADMIN` account.
- Effective role or active-status changes revoke every session belonging to the affected user.
- Authentication and administration events are written to `auth_audit_events` with a request ID.

## Required environment values

| Variable | Purpose |
| --- | --- |
| `APP_ENV` | `local`, `test`, `staging`, or `production` |
| `DATABASE_URL` | PostgreSQL connection string; required whenever authentication is enabled |
| `AUTH_ENABLED` | Must be `true` in staging and production |
| `GITHUB_OAUTH_CLIENT_ID` | GitHub OAuth App client ID |
| `GITHUB_OAUTH_CLIENT_SECRET` | GitHub OAuth App client secret; supply only through a local or deployment secret manager |
| `GITHUB_OAUTH_REDIRECT_URL` | Absolute API callback URL ending in `/auth/callback` |
| `AUTH_APPLICATION_URL` | Absolute web application URL used after successful login |
| `AUTH_ALLOWED_GITHUB_LOGINS` | Comma-separated normalized access allowlist |
| `AUTH_SUPERADMIN_GITHUB_LOGIN` | Initial and recovery login for the single `SUPERADMIN`; it must also be allowlisted |
| `VITE_API_BASE_URL` | Browser-visible API origin |

Never commit real client secrets, database credentials, session values, or production URLs containing credentials.

## Local setup

1. Register a GitHub OAuth App with:
   - homepage URL `http://localhost:5173`;
   - callback URL `http://localhost:8080/auth/callback`.
2. Copy `.env.example` to `.env` and fill the OAuth credentials, database URL, allowlist, and superadmin login.
3. Validate the effective environment without printing secrets:

```bash
make check-config
```

4. Start PostgreSQL and apply migrations:

```bash
make services-up
make migrate
```

5. Start the API and web application in separate terminals:

```bash
make dev-api
make dev-web
```

6. Sign in first with `AUTH_SUPERADMIN_GITHUB_LOGIN`. The first successful login creates the protected `SUPERADMIN` user.

## Staging and production setup

- Use HTTPS for both `GITHUB_OAUTH_REDIRECT_URL` and `AUTH_APPLICATION_URL`.
- Store the OAuth client secret and `DATABASE_URL` in the deployment secret manager.
- Run `make check-config` in the deployment environment before starting the API.
- Apply migrations before routing traffic to the new application version.
- Keep the OAuth callback URL exactly aligned with the deployed API callback.
- Do not copy production personal data into staging; staging uses synthetic identities and records.

The API fails closed outside local/test when the database or authentication configuration is incomplete.

## User lifecycle

### Grant initial access

1. Add the GitHub login to `AUTH_ALLOWED_GITHUB_LOGINS` and restart the API so it reads the new environment.
2. Ask the user to sign in once. A new allowlisted account is created as `MEMBER`.
3. An `ADMIN` or `SUPERADMIN` may promote that user to `ADMIN` through the user-administration panel.

### Remove access immediately

1. Set the user to inactive in the administration panel. This revokes all existing application sessions.
2. Remove the login from `AUTH_ALLOWED_GITHUB_LOGINS` and restart the API to block future OAuth sign-ins.

Removing only the environment allowlist entry blocks future sign-ins but does not revoke an already-issued application session. Deactivate the application user first when immediate removal is required.

### Change a role

Role changes use optimistic concurrency through the user `version`. A stale edit returns a conflict and must be retried after reloading the list. A successful effective change revokes all sessions for that user.

## Audit events

| Event | Typical outcomes |
| --- | --- |
| `SIGN_IN_SUCCEEDED` | `SUCCESS` |
| `SIGN_IN_DENIED` | `DENIED` for an unallowlisted or inactive account |
| `SIGN_IN_FAILED` | `FAILURE` for invalid callback input, provider failure, persistence failure, or session creation failure |
| `SIGN_OUT` | `SUCCESS` or `FAILURE` |
| `USER_ADMINISTRATION_ACCESSED` | `SUCCESS`, `DENIED`, or `FAILURE` |
| `USER_ACCESS_CHANGED` | `SUCCESS`, `DENIED`, or `FAILURE` |
| `SESSION_REVOKED` | `SUCCESS` or `FAILURE` after an access change |

Audit writes remain best-effort so a temporary audit-table failure does not create a partial authentication transaction. Every failed audit write emits a structured error log containing only event type, outcome, request ID, and the storage error. Tokens, OAuth codes, provider payloads, and secrets are never logged.

Read-only verification query:

```sql
SELECT occurred_at, event_type, outcome, request_id, provider_login
FROM auth_audit_events
ORDER BY occurred_at DESC
LIMIT 100;
```

## Routine smoke test

After an authentication-related deployment:

1. `GET /health/live` returns `200`.
2. `GET /health/ready` returns `200` after database migrations are applied.
3. An unauthenticated `GET /api/auth/session` returns `401`.
4. GitHub login redirects through `/auth/callback` and returns to `AUTH_APPLICATION_URL`.
5. `GET /api/auth/session` returns the authenticated user and role.
6. An administrator can load `/api/admin/users`.
7. A member cannot load `/api/admin/users` and receives `403`.
8. Changing a test member's role or active status invalidates that member's existing session.
9. Logout returns `204`, clears the browser cookie, and revokes the server-side session.
10. The expected correlated audit rows exist for the test request IDs.

## Incident and recovery procedures

### Rotate a GitHub OAuth client secret

1. Generate a replacement secret in the GitHub OAuth App.
2. Update `GITHUB_OAUTH_CLIENT_SECRET` in the secret manager.
3. restart or redeploy the API.
4. Revoke the old secret after the new deployment is healthy.

Existing Gymkhana Database sessions remain valid because they are independent of the temporary GitHub access token used during login.

### GitHub login renamed

1. Update `AUTH_ALLOWED_GITHUB_LOGINS` with the new normalized login.
2. If this is the protected owner, also update `AUTH_SUPERADMIN_GITHUB_LOGIN`.
3. Restart the API and sign in again.

The immutable GitHub user ID reconnects the renamed account to the existing application user and refreshes its displayed identity.

### Account compromised

1. Deactivate the application user to revoke all sessions.
2. Remove the login from the allowlist.
3. Rotate the GitHub OAuth client secret if the OAuth App itself may be compromised.
4. Review `auth_audit_events` and structured application logs by request ID.

### Protected superadmin unavailable

The M2 administration API intentionally cannot demote, deactivate, or replace the protected `SUPERADMIN`. First restore the same GitHub account or update its renamed login as described above.

If the GitHub account is permanently unrecoverable, do not run an ad-hoc partial update. Use a reviewed, transactional operational change that:

1. locks the current and target application-user rows;
2. verifies that the target user already exists and is controlled by the owner;
3. demotes the old superadmin and promotes the target in one transaction;
4. verifies exactly one active `SUPERADMIN` before commit;
5. revokes sessions for both accounts;
6. records the recovery in the audit trail and deployment log.

A dedicated automated transfer command is intentionally deferred until a real recovery case justifies its permanent maintenance and permission surface.

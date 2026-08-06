# Authentication Operations Runbook

This runbook covers Google OAuth, application sessions, authorization, capabilities, audit, and recovery procedures for Gymkhana Database.

## Security guarantees

- Google OAuth is the only application login mechanism; there are no local passwords.
- Access requires the email to be present in the `allowed_emails` table (database-backed allowlist).
- The application stores the Google subject (unique ID) and refreshes display identity after successful login.
- Browser sessions use opaque random values. Only SHA-256 hashes are stored in PostgreSQL.
- Sessions expire after 24 hours and are revocable server-side.
- Session cookies are HttpOnly, SameSite=Lax, host-only, and Secure in staging and production.
- Credentialed CORS and state-changing browser requests accept only the exact origin derived from `AUTH_APPLICATION_URL`.
- Roles are `EXTERNAL`, `ADMIN`, and one protected `SUPERADMIN`.
- The generic administration surface cannot change the current account or the `SUPERADMIN` account.
- Effective role or active-status changes revoke every session belonging to the affected user.
- Authentication and administration events are written to `auth_audit_events` with a request ID.

## Capability system

The capability system provides granular permission control beyond role-based access:

- **EXTERNAL users** require explicit capability grants to access specific features.
- **ADMIN and SUPERADMIN** bypass capability checks and have full access.
- Capabilities are managed through admin endpoints and stored in `app_user_capabilities`.

### Available capabilities

| Capability | Description |
| --- | --- |
| `search` | Access to search endpoints |
| `profiles` | Access to profile management |
| `data_tables` | Access to documents, bills, custom data |
| `attachments` | Access to file attachments |
| `ocr` | Access to OCR processing |
| `operations` | Access to bulk operations |
| `google_forms` | Access to Google Forms integration |
| `query` | Access to query engine |
| `matching` | Access to duplicate matching |
| `chat` | Access to AI chat |
| `tasks` | Access to task management |

### Granting capabilities

Administrators grant capabilities via:

```http
POST /api/admin/users/{userID}/capabilities
Content-Type: application/json

{
  "capability": "search",
  "reason": "User needs search access for gincana data"
}
```

### Revoking capabilities

```http
DELETE /api/admin/users/{userID}/capabilities/search
```

### Listing capabilities

```http
GET /api/admin/users/{userID}/capabilities
```

## Required environment values

| Variable | Purpose |
| --- | --- |
| `APP_ENV` | `local`, `test`, `staging`, or `production` |
| `DATABASE_URL` | PostgreSQL connection string; required whenever authentication is enabled |
| `AUTH_ENABLED` | Must be `true` in staging and production |
| `GOOGLE_OAUTH_CLIENT_ID` | Google OAuth client ID |
| `GOOGLE_OAUTH_CLIENT_SECRET` | Google OAuth client secret; supply only through a local or deployment secret manager |
| `GOOGLE_OAUTH_REDIRECT_URL` | Absolute API callback URL ending in `/auth/callback` |
| `AUTH_APPLICATION_URL` | Absolute web application URL used after successful login |
| `AUTH_SUPERADMIN_EMAIL` | Initial and recovery email for the single `SUPERADMIN`; it must also be allowlisted |
| `VITE_API_BASE_URL` | Browser-visible API origin |

The email allowlist is now stored in the `allowed_emails` table and managed via admin endpoints, not environment variables.

Never commit real client secrets, database credentials, session values, or production URLs containing credentials.

## Local setup

1. Register a Google OAuth client with:
   - Authorized JavaScript origins: `http://localhost:5173`
   - Authorized redirect URIs: `http://localhost:8080/auth/callback`
2. Copy `.env.example` to `.env` and fill the OAuth credentials, database URL, and superadmin email.
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

6. Sign in first with `AUTH_SUPERADMIN_EMAIL`. The first successful login creates the protected `SUPERADMIN` user.

## Staging and production setup

- Use HTTPS for both `GOOGLE_OAUTH_REDIRECT_URL` and `AUTH_APPLICATION_URL`.
- Store the OAuth client secret and `DATABASE_URL` in the deployment secret manager.
- Run `make check-config` in the deployment environment before starting the API.
- Apply migrations before routing traffic to the new application version.
- Keep the OAuth callback URL exactly aligned with the deployed API callback.
- Do not copy production personal data into staging; staging uses synthetic identities and records.

The API fails closed outside local/test when the database or authentication configuration is incomplete.

## User lifecycle

### Grant initial access

1. Add the user's email to the allowlist via the admin API:

```http
POST /api/admin/allowed-emails
Content-Type: application/json

{
  "email": "user@example.com",
  "reason": "New team member for gincana"
}
```

2. Ask the user to sign in once. A new allowlisted account is created as `EXTERNAL`.
3. Grant required capabilities to the user:

```http
POST /api/admin/users/{userID}/capabilities
{
  "capability": "search",
  "reason": "Initial access"
}
```

4. An `ADMIN` or `SUPERADMIN` may promote that user to `ADMIN` through the user-administration panel.

### Remove access immediately

1. Set the user to inactive in the administration panel. This revokes all existing application sessions.
2. Remove the email from the allowlist:

```http
DELETE /api/admin/allowed-emails/user@example.com
```

Removing only the allowlist entry blocks future sign-ins but does not revoke an already-issued application session. Deactivate the application user first when immediate removal is required.

### Change a role

Role changes use optimistic concurrency through the user `version`. A stale edit returns a conflict and must be retried after reloading the list. A successful effective change revokes all sessions for that user.

### Manage capabilities

List all allowed emails:

```http
GET /api/admin/allowed-emails
```

List user capabilities:

```http
GET /api/admin/users/{userID}/capabilities
```

Grant a capability:

```http
POST /api/admin/users/{userID}/capabilities
{
  "capability": "profiles",
  "reason": "Needs profile management access"
}
```

Revoke a capability:

```http
DELETE /api/admin/users/{userID}/capabilities/profiles
```

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
| `CAPABILITY_GRANTED` | `SUCCESS` or `FAILURE` |
| `CAPABILITY_REVOKED` | `SUCCESS` or `FAILURE` |

Audit writes remain best-effort so a temporary audit-table failure does not create a partial authentication transaction. Every failed audit write emits a structured error log containing only event type, outcome, request ID, and the storage error. Tokens, OAuth codes, provider payloads, and secrets are never logged.

Read-only verification query:

```sql
SELECT occurred_at, event_type, outcome, request_id, actor_email
FROM auth_audit_events
ORDER BY occurred_at DESC
LIMIT 100;
```

## Routine smoke test

After an authentication-related deployment:

1. `GET /health/live` returns `200`.
2. `GET /health/ready` returns `200` after database migrations are applied.
3. An unauthenticated `GET /api/auth/session` returns `401`.
4. Google login redirects through `/auth/callback` and returns to `AUTH_APPLICATION_URL`.
5. `GET /api/auth/session` returns the authenticated user and role.
6. An administrator can load `/api/admin/users`.
7. An EXTERNAL user without capabilities receives `403` when accessing protected endpoints.
8. Changing a test user's role or active status invalidates that user's existing session.
9. Logout returns `204`, clears the browser cookie, and revokes the server-side session.
10. The expected correlated audit rows exist for the test request IDs.

## Incident and recovery procedures

### Rotate a Google OAuth client secret

1. Generate a replacement secret in the Google Cloud Console.
2. Update `GOOGLE_OAUTH_CLIENT_SECRET` in the secret manager.
3. Restart or redeploy the API.
4. Revoke the old secret after the new deployment is healthy.

Existing Gymkhana Database sessions remain valid because they are independent of the temporary Google access token used during login.

### User email changed

1. Add the new email to the allowlist via admin API.
2. If this is the protected superadmin, also update `AUTH_SUPERADMIN_EMAIL`.
3. The user signs in with the new email. A new user record is created.
4. Transfer any necessary data or capabilities from the old account.
5. Deactivate the old account and remove the old email from the allowlist.

Note: Unlike GitHub OAuth, Google OAuth does not provide a stable user ID that persists across email changes. Each email is treated as a separate identity.

### Account compromised

1. Deactivate the application user to revoke all sessions.
2. Remove the email from the allowlist.
3. Rotate the Google OAuth client secret if the OAuth client itself may be compromised.
4. Review `auth_audit_events` and structured application logs by request ID.
5. Revoke any granted capabilities if the account was used maliciously.

### Protected superadmin unavailable

The administration API intentionally cannot demote, deactivate, or replace the protected `SUPERADMIN`. First restore access to the same Google account or add a new email for the superadmin.

If the Google account is permanently unrecoverable, do not run an ad-hoc partial update. Use a reviewed, transactional operational change that:

1. locks the current and target application-user rows;
2. verifies that the target user already exists and is controlled by the owner;
3. demotes the old superadmin and promotes the target in one transaction;
4. verifies exactly one active `SUPERADMIN` before commit;
5. revokes sessions for both accounts;
6. records the recovery in the audit trail and deployment log.

A dedicated automated transfer command is intentionally deferred until a real recovery case justifies its permanent maintenance and permission surface.

## Migration from GitHub OAuth

The system migrated from GitHub OAuth to Google OAuth in migration 020. Key changes:

- `github_user_id` and `github_login` columns replaced with `email` and `subject`.
- `AUTH_ALLOWED_GITHUB_LOGINS` environment variable replaced with database-backed `allowed_emails` table.
- `MEMBER` role renamed to `EXTERNAL`.
- Capability system added for granular permission control.

Migration 022 automatically:
- Added all existing users to the email allowlist (using their GitHub login as email).
- Granted basic capabilities (search, profiles, data_tables, attachments, query) to active EXTERNAL users.

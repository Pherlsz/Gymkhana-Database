# Authentication Operations Runbook

This runbook covers Google OAuth, application sessions, authorization, capabilities, audit, and recovery procedures for Gymkhana Database.

## Security guarantees

- Google OAuth is the only application login mechanism; there are no local passwords.
- Access requires the email to be present in the `allowed_emails` table (database-backed allowlist).
- Account lookup is by email, the same as the legacy application. Google subject is stored as provider metadata and display name is refreshed after each successful login; subject is not the account key.
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

| Capability     | Description                                                                                                                                                                                                           |
| -------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `search`       | Access to search endpoints                                                                                                                                                                                            |
| `profiles`     | Access to profile management                                                                                                                                                                                          |
| `data_tables`  | Access to documents, bills, custom data. A dedicated `custom_data` capability (beyond formula columns and user-created columns) is an open product question; do not invent a new grant surface until that is decided. |
| `attachments`  | Access to file attachments                                                                                                                                                                                            |
| `ocr`          | Access to OCR processing                                                                                                                                                                                              |
| `operations`   | Access to bulk operations                                                                                                                                                                                             |
| `google_forms` | Access to Google Forms integration                                                                                                                                                                                    |
| `query`        | Access to query engine                                                                                                                                                                                                |
| `matching`     | Access to duplicate matching                                                                                                                                                                                          |
| `chat`         | Access to the Assistente (same HTTP engine; not a /chat destination page)                                                                                                                                             |

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

| Variable                     | Purpose                                                                                                                        |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `APP_ENV`                    | `local`, `test`, `staging`, or `production`. `development` / `dev` are treated as `local` |
| `DATABASE_URL`               | PostgreSQL connection string; required whenever authentication is enabled                                                      |
| `AUTH_ENABLED`               | Must be `true` in staging and production                                                                                       |
| `GOOGLE_OAUTH_CLIENT_ID`     | Google OAuth client ID                                                                                                         |
| `GOOGLE_OAUTH_CLIENT_SECRET` | Google OAuth client secret; supply only through a local or deployment secret manager                                           |
| `GOOGLE_OAUTH_REDIRECT_URL`  | Absolute API callback URL ending in `/auth/callback`                                                                           |
| `AUTH_APPLICATION_URL`       | Absolute web application URL used after successful login                                                                       |
| `VITE_API_BASE_URL`          | Browser-visible API origin                                                                                                     |

Who can sign in lives in Neon: `allowed_emails` plus a provisioned `app_users` row (Administração / provision API). There is no env allowlist or env superadmin bootstrap.

Never commit real client secrets, database credentials, session values, or production URLs containing credentials. Locally, put them in gitignored `.env` (see `.env.example`).

## Local setup

1. Register a Google OAuth client with:
   - Authorized JavaScript origins: `http://localhost:5173`
   - Authorized redirect URIs: `http://localhost:8080/auth/callback`
2. Put the OAuth credentials and database URL in `.env`. `.env.example` lists the names.
3. Validate the effective environment without printing secrets:

```bash
make check-config
```

4. Apply migrations to Neon Dev:

```bash
make migrate
```

5. Start the API and web application in separate terminals:

```bash
make dev-api
make dev-web
```

6. Use **Dev Login** (local only) to create/sign in as `developer@gymkhana.local` (`SUPERADMIN`), or seed a real `SUPERADMIN` in Neon and sign in with Google.

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

1. Create the user in Administração with a role (`EXTERNAL` or `ADMIN`). A member also needs at least one capability. This also adds the email to the allowlist.
2. Ask the user to sign in with Google. Login binds the Google identity to that existing user and keeps the role you set. An allowlisted email without a provisioned user is denied.
3. Google login never creates users. Seed the first `SUPERADMIN` in Neon (SQL/ops) or use local Dev Login. The protected `SUPERADMIN` role cannot be demoted via the administration API.

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

| Event                          | Typical outcomes                                                                                         |
| ------------------------------ | -------------------------------------------------------------------------------------------------------- |
| `SIGN_IN_SUCCEEDED`            | `SUCCESS`                                                                                                |
| `SIGN_IN_DENIED`               | `DENIED` for an unallowlisted or inactive account                                                        |
| `SIGN_IN_FAILED`               | `FAILURE` for invalid callback input, provider failure, persistence failure, or session creation failure |
| `SIGN_OUT`                     | `SUCCESS` or `FAILURE`                                                                                   |
| `USER_ADMINISTRATION_ACCESSED` | `SUCCESS`, `DENIED`, or `FAILURE`                                                                        |
| `USER_ACCESS_CHANGED`          | `SUCCESS`, `DENIED`, or `FAILURE`                                                                        |
| `SESSION_REVOKED`              | `SUCCESS` or `FAILURE` after an access change                                                            |
| `CAPABILITY_GRANTED`           | `SUCCESS` or `FAILURE`                                                                                   |
| `CAPABILITY_REVOKED`           | `SUCCESS` or `FAILURE`                                                                                   |

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

Login identity is the allowlisted email. Keep the same application user.

1. Update the existing user's email through the administration panel (or a reviewed operational update).
2. Add the new email to the allowlist and remove the old email.
3. The user signs in with the new Google account email. Lookup by email loads the same row and refreshes display identity.

Do not create a second user and transfer capabilities. A new row is created only when an admin provisions that email.

### Account compromised

1. Deactivate the application user to revoke all sessions.
2. Remove the email from the allowlist.
3. Rotate the Google OAuth client secret if the OAuth client itself may be compromised.
4. Review `auth_audit_events` and structured application logs by request ID.
5. Revoke any granted capabilities if the account was used maliciously.

### Protected superadmin unavailable

The administration API intentionally cannot demote, deactivate, or replace the protected `SUPERADMIN`. Restore access to the same Google account, or update that user's email and the allowlist in Neon so the same account can sign in.

If the Google account is permanently unrecoverable, do not run an ad-hoc partial update. Use a reviewed, transactional operational change that:

1. locks the current and target application-user rows;
2. verifies that the target user already exists and is controlled by the owner;
3. demotes the old superadmin and promotes the target in one transaction;
4. verifies exactly one active `SUPERADMIN` before commit;
5. revokes sessions for both accounts;
6. records the recovery in the audit trail and deployment log.

A dedicated automated transfer command is intentionally deferred until a real recovery case justifies its permanent maintenance and permission surface.

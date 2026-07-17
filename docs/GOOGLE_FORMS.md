# Google Forms Operations Runbook

This runbook covers the owner-scoped Google Forms ingestion path. The feature is disabled by default and uses the existing Operations staging, preview, decision, execution, and report pipeline.

## Provider boundary

The application requests exactly these scopes:

- `https://www.googleapis.com/auth/forms.body.readonly`
- `https://www.googleapis.com/auth/forms.responses.readonly`

Forms are registered by an explicit Google Forms URL or form ID. The application does not request a Google Drive scope, browse Drive, create forms, or modify provider data. File uploads, grids, checkbox/multi-answer questions, and unknown question types remain outside canonical staging.

Each ADMIN or SUPERADMIN owns one Google connection and can access only their own forms, mappings, sync runs, imports, and reports. This ownership rule also applies to SUPERADMIN until a separate elevated permission is introduced.

## Google Cloud setup

Production activation requires an owner-managed Google Cloud project. These external credentials are never committed and are not required by CI.

1. Enable the Google Forms API in the project.
2. Configure the OAuth consent screen and add only the two read-only scopes listed above.
3. Create an OAuth 2.0 client of type **Web application**.
4. Add the exact API callback URL as an authorized redirect URI:

   ```text
   https://API_HOST/api/v1/google-forms/oauth/callback
   ```

5. Store the OAuth client ID, client secret, and token-encryption keys in the deployment secret manager.
6. Ensure authentication, PostgreSQL, the Operations worker, and its private object-storage configuration are already available.

The redirect URI must contain no query or fragment. HTTPS is mandatory in staging and production.

## Configuration

| Variable                             | Purpose                                                            |
| ------------------------------------ | ------------------------------------------------------------------ |
| `GOOGLE_FORMS_ENABLED`               | Explicit feature switch; defaults to `false`                       |
| `GOOGLE_FORMS_OAUTH_CLIENT_ID`       | Google OAuth web-client ID                                         |
| `GOOGLE_FORMS_OAUTH_CLIENT_SECRET`   | Google OAuth client secret; secret manager only                    |
| `GOOGLE_FORMS_OAUTH_REDIRECT_URL`    | Exact API callback ending in `/api/v1/google-forms/oauth/callback` |
| `GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY`  | Current base64-encoded 32-byte AES-256 key                         |
| `GOOGLE_FORMS_TOKEN_KEY_VERSION`     | Positive version of the current encryption key                     |
| `GOOGLE_FORMS_TOKEN_DECRYPTION_KEYS` | Optional comma-separated historical `version:base64` keys          |
| `GOOGLE_FORMS_SYNC_INTERVAL`         | Durable due-source scheduler interval, from `5m` to `24h`          |
| `GOOGLE_FORMS_RESPONSE_PAGE_SIZE`    | Provider page size, from 1 to 500                                  |

Generate a key without writing it to the repository:

```bash
openssl rand -base64 32
```

Local example:

```dotenv
GOOGLE_FORMS_ENABLED=true
GOOGLE_FORMS_OAUTH_CLIENT_ID=...
GOOGLE_FORMS_OAUTH_CLIENT_SECRET=...
GOOGLE_FORMS_OAUTH_REDIRECT_URL=http://localhost:8080/api/v1/google-forms/oauth/callback
GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY=...
GOOGLE_FORMS_TOKEN_KEY_VERSION=1
GOOGLE_FORMS_TOKEN_DECRYPTION_KEYS=
GOOGLE_FORMS_SYNC_INTERVAL=15m
GOOGLE_FORMS_RESPONSE_PAGE_SIZE=100
```

Startup fails closed when the feature is enabled with incomplete credentials, an invalid redirect, or an invalid key. With the feature disabled, API and worker startup do not require Google credentials.

## Key rotation

1. Generate a new 32-byte key and increment `GOOGLE_FORMS_TOKEN_KEY_VERSION`.
2. Move the previous key to `GOOGLE_FORMS_TOKEN_DECRYPTION_KEYS`, for example `1:OLD_BASE64_KEY`.
3. Deploy the new current key, its version, and every historical key still needed by an existing connection.
4. Ask connection owners to reconnect; new or replaced refresh tokens are encrypted with the current key.
5. Remove an old key only after no persisted connection references that version.

Removing a referenced historical key makes that credential undecryptable and requires reconnection. Ciphertext authentication or key-version failures are closed errors; plaintext access tokens are never persisted.

## Smoke test

1. Apply migrations and start both the API and Operations worker.
2. Sign in as an active ADMIN or SUPERADMIN and open **Google Forms**.
3. Connect Google, verify the consent screen contains only the two Forms read scopes, and return to the application.
4. Register a form by URL, map supported questions to logical Operations fields, and activate the source.
5. Run **Synchronize now** and wait for a completed sync.
6. Open the linked Operations import, review its preview and decisions, execute it, and verify the durable report.
7. Change the form schema and verify the source enters `SCHEMA_DRIFT` until its mapping is reviewed.
8. Disconnect and verify local credentials are removed even if provider revocation is unavailable.

Safe operational records contain IDs, counts, timestamps, states, and stable error codes only. OAuth codes, state values, tokens, provider URLs/payloads, and answer values must not appear in logs or audit metadata.

## Failure and recovery

- `NEEDS_REAUTH`: reconnect the same owner account. Invalid or revoked grants are not retried in a loop.
- `SCHEMA_DRIFT`: review question diagnostics, save a valid mapping, and reactivate the source. A pending page cursor is rolled back before the continuation token is discarded.
- `rate_limited` or `provider_retry_exhausted`: wait for the bounded retry window and retry; the worker applies provider-aware backoff.
- `response_changed`: a previously receipted response ID returned different normalized content. Investigate before retrying; the existing canonical import is not silently overwritten.
- Cancelled or interrupted jobs are idempotent. Unique sync runs and response receipts prevent duplicate staging across retries and cursor overlap.

To disable ingestion without deleting state, set `GOOGLE_FORMS_ENABLED=false` and restart API and worker. Existing encrypted credentials remain at rest for later reactivation. For suspected credential compromise, disconnect affected accounts, rotate the Google OAuth secret when applicable, rotate the application encryption key, and review Google Forms audit events.

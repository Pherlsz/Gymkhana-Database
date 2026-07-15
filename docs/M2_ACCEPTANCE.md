# M2 Acceptance — Authentication and Minimal Administration

M2 establishes private application access and the smallest safe administration surface. It does not introduce Profiles, documents, bills, imports, search, AI, OCR, or other product-data modules.

## Delivered scope

### Persistence and session lifecycle

- `app_users`, `app_sessions`, and `auth_audit_events` migrations;
- immutable GitHub user IDs and normalized GitHub logins;
- opaque 32-byte application sessions stored only as SHA-256 hashes;
- 24-hour expiry, last-seen tracking, logout revocation, and per-user revocation;
- deterministic sqlc persistence adapters.

### GitHub login

- GitHub OAuth authorization-code flow with a random state cookie;
- `read:user` provider scope;
- explicit normalized login allowlist;
- initial protected `SUPERADMIN` bootstrap;
- inactive-user denial and identity refresh by immutable GitHub user ID;
- fail-closed staging/production configuration.

### Authorization and administration

- centralized `MEMBER`, `ADMIN`, and `SUPERADMIN` role checks;
- one protected `SUPERADMIN` account;
- paginated application-user list;
- optimistic role/active-status changes;
- no self-access changes;
- no generic superadmin promotion, demotion, or deactivation;
- no-op updates avoid unnecessary writes and revocations;
- effective access changes revoke all sessions for the affected user;
- responsive administration UI visible only to administrative roles.

### Browser and HTTP security

- HttpOnly, SameSite=Lax, host-only session cookies;
- Secure cookies in staging and production;
- credentialed CORS restricted to the exact origin derived from `AUTH_APPLICATION_URL`;
- exact `Origin` validation for state-changing requests;
- OAuth state validation;
- request body limits, safe error envelopes, request IDs, no-store responses, clickjacking protection, MIME sniffing protection, and redacted logs.

### Audit and operations

- correlated sign-in, sign-out, user-administration access, access-change, and session-revocation events;
- `SUCCESS`, `DENIED`, and `FAILURE` outcomes where applicable;
- structured error logging when an audit event cannot be persisted;
- safe configuration preflight through `make check-config`;
- local, deployed, lifecycle, audit, incident, and recovery runbook.

## Automated acceptance

The repository validation suite must pass on the final M2 commit:

```bash
make generate
make check
```

GitHub Actions additionally validates:

- Go formatting, vet, unit tests, race tests, Staticcheck, and executable builds;
- frontend typecheck, lint, formatting, tests, and production build;
- deterministic OpenAPI Go and TypeScript generation;
- deterministic sqlc generation against migrated PostgreSQL;
- forward and rollback migration validation;
- `govulncheck`, OSV-Scanner, and JavaScript dependency audit.

`TestM2AuthenticationAdministrationAndRevocationFlow` exercises the complete synthetic application flow through the real HTTP router and authentication service:

1. initial superadmin OAuth callback and session creation;
2. member OAuth callback and session creation;
3. protected administration list;
4. optimistic member promotion;
5. automatic revocation of the member session;
6. owner logout and session revocation;
7. expected correlated audit-event coverage.

Provider-network behavior is covered separately by the GitHub adapter tests using controlled HTTP endpoints. Real OAuth credentials and production data are never used in CI.

## Deployment acceptance

Each deployed environment follows the smoke procedure in [`AUTHENTICATION.md`](AUTHENTICATION.md). `make check-config` is run with that environment's secret manager populated before API startup.

A live provider smoke test is an environment operation, not a repository CI requirement, because OAuth credentials must never be exposed to pull-request workflows.

## Explicit deferrals

- Cloudflare Access remains an optional outer layer and is not coupled to application authorization.
- A generic audit-log UI is deferred until product administration needs it; PostgreSQL remains the M2 audit source.
- Automated superadmin transfer is deferred. The generic application UI intentionally protects the sole superadmin, and emergency recovery requires a reviewed transactional operation.
- Product feature flags begin when product modules require them; M2 authorization exposes only the permissions needed by the administration surface.
- Profiles and every later product module remain outside M2.

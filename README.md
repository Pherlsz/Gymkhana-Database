# Gymkhana Database

Private web application for managing people, documents, bills, custom data, imports, search, duplicate review, OCR, and AI-assisted queries for gymkhana workflows.

## Current status

Milestone 2 authentication and minimal administration. Product data modules remain intentionally outside this increment.

The application consumes:

- `github.com/Pherlsz/Gymkhana-Core v0.2.1` for deterministic normalization and civil-time values;
- `@pherlsz/gymkhana-ui 0.3.0` for semantic themes, layouts, controls, feedback, overlays, AppShell, and Page composition;
- `openapi-typescript 7.13.0` for deterministic generated TypeScript contracts.

## Requirements

- Go 1.26.5
- Node.js 24 LTS
- pnpm 11.12.0
- Docker with Compose
- GNU Make or a compatible environment such as WSL/Git Bash on Windows
- Git credentials that can read the private Gymkhana Core repository
- GitHub Packages credentials that can read `@pherlsz/gymkhana-ui`

## Private dependency access

Local Git credentials must be able to clone `Pherlsz/Gymkhana-Core`. Configure the Go toolchain once:

```bash
go env -w GOPRIVATE=github.com/Pherlsz/Gymkhana-Core
go env -w GONOSUMDB=github.com/Pherlsz/Gymkhana-Core
```

GitHub Actions uses the repository secret `GYMKHANA_REPOSITORY_TOKEN`, backed by a fine-grained token with read-only access to Gymkhana Core. The UI package uses the workflow token with package read access.

## Setup

```bash
cp .env.example .env
corepack enable
pnpm install --frozen-lockfile
go mod download
docker compose up -d db
make migrate
make check-config
```

Run the API and web app in separate terminals:

```bash
make dev-api
pnpm dev:web
```

- API: `http://localhost:8080`
- Web: `http://localhost:5173`
- Live health: `http://localhost:8080/health/live`
- Ready health: `http://localhost:8080/health/ready`

## GitHub authentication

Authentication is optional only in local and test environments. Staging and production fail during startup unless private application access is completely configured.

Create a GitHub OAuth App and configure these environment values:

- `AUTH_ENABLED=true`
- `GITHUB_OAUTH_CLIENT_ID`
- `GITHUB_OAUTH_CLIENT_SECRET`, supplied through the local or deployment secret manager;
- `GITHUB_OAUTH_REDIRECT_URL`, ending in `/auth/callback`;
- `AUTH_APPLICATION_URL`, the web application URL used after login and the only browser origin trusted for credentialed CORS and state-changing requests;
- `AUTH_ALLOWED_GITHUB_LOGINS`, a comma-separated allowlist;
- `AUTH_SUPERADMIN_GITHUB_LOGIN`, which must also appear in the allowlist.

For local testing, use callback `http://localhost:8080/auth/callback` and application URL `http://localhost:5173`. The API stores only SHA-256 session hashes. Browser cookies are HttpOnly, SameSite=Lax, host-only, and become Secure outside local/test. Application sessions expire after 24 hours and logout revokes the server-side session.

The first successful login matching `AUTH_SUPERADMIN_GITHUB_LOGIN` creates the initial `SUPERADMIN`. Other allowed first-time users are created as `MEMBER`. Disabled users remain denied even when their GitHub login is allowed.

The complete setup, lifecycle, audit, smoke-test, incident, and recovery procedures are in [`docs/AUTHENTICATION.md`](docs/AUTHENTICATION.md).

## Common commands

```bash
make generate
make check
make test
make check-config
make migrate
make reset-db
```

Container builds require the same private repository token as a BuildKit secret:

```bash
export GYMKHANA_REPOSITORY_TOKEN=<read-only-token>
docker build --secret id=github_token,env=GYMKHANA_REPOSITORY_TOKEN -f Dockerfile.api .
```

## Platform contracts

- typed environment validation that fails closed in deployed environments;
- GitHub OAuth with state validation and an explicit login allowlist;
- exact-origin CSRF validation and credentialed CORS derived from `AUTH_APPLICATION_URL`;
- opaque, revocable, server-side sessions with a 24-hour lifetime;
- centralized `MEMBER`, `ADMIN`, and protected `SUPERADMIN` authorization;
- correlated authentication and administration audit events with observable persistence failures;
- stable JSON error envelopes with request IDs and safe public messages;
- generated Go and TypeScript API contracts;
- deterministic sqlc persistence adapters;
- released Core and UI dependencies consumed only through exact versions.

## Repository boundaries

Gymkhana Database owns the product, persistence, HTTP API, workers, provider integrations, authorization, and application routes. Reusable presentation belongs in Gymkhana UI and reusable infrastructure-independent Go logic belongs in Gymkhana Core.

Private UI/Core versions are pinned only after their releases are published. Permanent branch, commit, `replace`, subtree, submodule, or copied-source dependencies are not allowed.

## Planning and implementation tracking

Architecture and product decisions live in [`docs/ORCHESTRATION.md`](docs/ORCHESTRATION.md). Delivery conventions and GitHub milestone usage live in [`docs/IMPLEMENTATION.md`](docs/IMPLEMENTATION.md). M2 acceptance evidence lives in [`docs/M2_ACCEPTANCE.md`](docs/M2_ACCEPTANCE.md).

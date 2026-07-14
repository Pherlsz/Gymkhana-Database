# Gymkhana Database

Private web application for managing people, documents, bills, custom data, imports, search, duplicate review, OCR, and AI-assisted queries for gymkhana workflows.

## Current status

Milestone 1 shared foundations. Business modules remain intentionally outside this increment.

The application now consumes:

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

GitHub Actions uses the repository secret `GYMKHANA_REPOSITORY_TOKEN`. Use a fine-grained token with read-only access to Gymkhana Core. Workflows fall back to `GITHUB_TOKEN`, but GitHub normally scopes that token to Gymkhana Database and therefore the dedicated secret is the supported configuration.

The UI package continues to use the workflow `GITHUB_TOKEN` with `packages: read` and package access granted to Gymkhana Database.

## Setup

```bash
cp .env.example .env
corepack enable
pnpm install --frozen-lockfile
go mod download
docker compose up -d db
make migrate
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

## Common commands

```bash
make generate
make check
make test
make migrate
make reset-db
```

Container builds require the same private repository token as a BuildKit secret:

```bash
export GYMKHANA_REPOSITORY_TOKEN=<read-only-token>
docker build --secret id=github_token,env=GYMKHANA_REPOSITORY_TOKEN -f Dockerfile.api .
```

## M1 platform contracts

- typed environment and log-level configuration with bounded request sizes and shutdown timeouts;
- stable JSON error envelopes with request IDs, error codes, and safe public messages;
- generated-contract-based frontend helpers that preserve HTTP status, error code, and request ID;
- integration tests proving the released Core normalization and civil-date APIs;
- real ThemeProvider, AppShell, Page, feedback, layout, and status component consumption.

## Repository boundaries

Gymkhana Database owns the product, persistence, HTTP API, workers, provider integrations, authorization, and application routes. Reusable presentation belongs in Gymkhana UI and reusable infrastructure-independent Go logic belongs in Gymkhana Core.

Private UI/Core versions are pinned only after their releases are published. Permanent branch, commit, `replace`, subtree, submodule, or copied-source dependencies are not allowed.

## Planning and implementation tracking

Architecture and product decisions live in [`docs/ORCHESTRATION.md`](docs/ORCHESTRATION.md). Delivery conventions and GitHub milestone usage live in [`docs/IMPLEMENTATION.md`](docs/IMPLEMENTATION.md).

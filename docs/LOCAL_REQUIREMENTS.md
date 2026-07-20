# Local Development Requirements

This guide covers the minimum setup for running Gymkhana Database locally against the dedicated Neon rebuild database.

## Software

- Git
- Go 1.26.5
- Node.js 24 LTS
- Corepack with pnpm 11.12.0
- Docker with Docker Compose
- GNU Make through Linux, macOS, WSL2, or Git Bash

Optional: PostgreSQL client tools such as `psql`.

## Private dependencies

The project consumes the private Gymkhana-Core Go module and the private `@pherlsz/gymkhana-ui` package.

Configure Go once:

```bash
go env -w GOPRIVATE=github.com/Pherlsz/Gymkhana-Core
go env -w GONOSUMDB=github.com/Pherlsz/Gymkhana-Core
```

Git must have read access to Gymkhana-Core. npm must have package read access to `@pherlsz/gymkhana-ui`.

## Environment

```bash
cp .env.example .env
```

Fill `DATABASE_URL` with the Neon connection string for the database named exactly `gymkhana_rebuild`, using SSL.

Never point the application at the legacy `neondb` database. It is retained only as an isolated migration source.

## Google login

Create a Google OAuth Web client with:

- JavaScript origin: `http://localhost:5173`
- redirect URI: `http://localhost:8080/auth/callback`

Fill the Google login variables from `.env.example`. Access is granted only to verified email addresses already provisioned in `app_users`; OAuth must not create arbitrary users.

## Install

```bash
corepack enable
pnpm install --frozen-lockfile
go mod download
```

## Database usage

For normal development, the API may use the canonical Neon rebuild database. Check configuration and migration state before starting:

```bash
make check-config
make migrate-status
```

Apply migrations only after confirming the target:

```bash
make migrate
```

Do not run rollback or destructive integration validation against the canonical Neon database. Use the disposable PostgreSQL created by the local verification script or a disposable Neon branch. Never enable the external-database override for the canonical database.

## Run

Use separate terminals:

```bash
make dev-api
```

```bash
make dev-web
```

Endpoints:

- API: `http://localhost:8080`
- Web: `http://localhost:5173`
- Live health: `http://localhost:8080/health/live`
- Ready health: `http://localhost:8080/health/ready`

## Verify

Linux, macOS, WSL2, or Git Bash:

```bash
./scripts/verify-local.sh changed
./scripts/verify-local.sh full
```

Windows PowerShell:

```powershell
.\scripts\verify-local.ps1 changed
.\scripts\verify-local.ps1 full
```

Focused scopes are `backend`, `database`, `frontend`, `contracts`, and `security`.

Keep implementation PRs in draft while iterating. Mark a PR ready only after the relevant local checks pass.

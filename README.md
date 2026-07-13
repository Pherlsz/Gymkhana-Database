# Gymkhana Database

Private web application for managing people, documents, bills, custom data, imports, search, duplicate review, OCR, and AI-assisted queries for gymkhana workflows.

## Current status

Milestone 0 repository bootstrap. Business modules are intentionally not implemented yet.

## Requirements

- Go 1.26.5
- Node.js 24 LTS
- pnpm 11.12.0
- Docker with Compose
- GNU Make or a compatible environment such as WSL/Git Bash on Windows

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

## Repository boundaries

Gymkhana Database owns the product, persistence, HTTP API, workers, provider integrations, authorization, and application routes. Reusable presentation belongs in Gymkhana UI and reusable infrastructure-independent Go logic belongs in Gymkhana Core.

Private UI/Core versions are pinned only after their releases are published. Permanent branch, commit, `replace`, subtree, submodule, or copied-source dependencies are not allowed.

## Planning and implementation tracking

Architecture and product decisions live in [`docs/ORCHESTRATION.md`](docs/ORCHESTRATION.md). Delivery conventions and GitHub milestone usage live in [`docs/IMPLEMENTATION.md`](docs/IMPLEMENTATION.md).

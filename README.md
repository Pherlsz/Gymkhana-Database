# Gymkhana Database

Private web application for managing people, documents, bills, custom data, imports, search, duplicate review, OCR, and AI-assisted queries for gymkhana workflows.

## Documentation and tracking

Permanent product, domain, security, UX, and architecture rules live in [`docs/ORCHESTRATION.md`](docs/ORCHESTRATION.md). Agents must read the architecture/design sections and the requested module section before any Gymkhana migration task; see [`AGENTS.md`](AGENTS.md).

Live status lives only in the [master checklist issue #31](https://github.com/Pherlsz/Gymkhana-Database/issues/31). Repository documents do not track current work or next action, and must not be created for that purpose.

The application consumes:

- `github.com/Pherlsz/Gymkhana-Core v0.7.0` for deterministic normalization, civil-time values, and document identification;
- Ant Design as the visual base (Orchestration §12.3, ADR 0002), composed in-repo with Lucide (`lucide-react`) for product-chrome icons. Material UI and shadcn are rejected as the visual base;
- `openapi-typescript 7.13.0` for deterministic generated TypeScript contracts.

## Requirements

- Go 1.26.5
- Node.js 24 LTS
- pnpm 11.12.0
- GNU Make or a compatible environment such as WSL/Git Bash on Windows
- Git credentials that can read the private Gymkhana Core repository

## Private dependency access

Local Git credentials must be able to clone `Pherlsz/Gymkhana-Core`. Configure the Go toolchain once:

```bash
go env -w GOPRIVATE=github.com/Pherlsz/Gymkhana-Core
go env -w GONOSUMDB=github.com/Pherlsz/Gymkhana-Core
```

GitHub Actions uses the repository secret `GYMKHANA_REPOSITORY_TOKEN`, backed by a fine-grained token with read-only access to Gymkhana Core.

## Setup

Copy `.env.example` to `.env` and fill in real values. `.env` is gitignored. The API, worker, Vite, and Make load that file from the repository root. Cursor Cloud uses the same `.env`.

The rebuild uses Neon only. Docker is not used in this repository in any form: no local containers, no Compose, no Dockerfiles, no container images.

- Local / Cursor Cloud: Neon `Gymkhana-Database-Dev-18`, database `gymkhana`, unpooled (direct) endpoint in `DATABASE_URL`
- Production: Neon `Gymkhana-Database-Prod-18`, database `gymkhana`, unpooled, from the deployment secret manager

Do not store a PgBouncer `-pooler` URL: Tern and the Go `pgx` pool need the direct compute. Production PostgreSQL is Neon major 18 (Orchestration §24); there is no in-place major upgrade.

`make migrate` and `make migrate-status` read `DATABASE_URL` from `.env` and apply Tern plus River to that Neon database. `make reset-db` is retired so a local reset cannot be mistaken for a Neon wipe.

```bash
cp .env.example .env
corepack enable
pnpm install --frozen-lockfile
go mod download
make check-config
make migrate
```

`APP_ENV=development` is treated as `local` for auth cookie rules. The database is still Neon Dev-18.

Run the API and web app in separate terminals:

```bash
make dev-api
make dev-web
```

- API: `http://localhost:8080`
- Web: `http://localhost:5173`
- Live health: `http://localhost:8080/health/live`
- Ready health: `http://localhost:8080/health/ready`

## Legacy application (Gymkhana-Database-Vercel)

The production Next.js app lives in the sibling repository `Gymkhana-Database-Vercel` (`~/projects/product/Gymkhana-Database-Vercel`). It is not a dependency of this rebuild.

Start it only when the operator also asks to run the legacy. Default local work is this repository alone.

`npm run dev` is `prisma generate && next dev`. Prisma loads `prisma.config.ts`, which uses `dotenv/config` and therefore reads that repository's `.env`. Next.js also loads `.env` / `.env.local` and inlines those files into Edge middleware (`AUTH_SECRET`).

The two apps do not share a database. Legacy local runs use the rotating Dev Neon `DATABASE_URL` in that repository's `.env`, never a local PostgreSQL. Ports do not collide: legacy `http://localhost:3000`, rebuild API `8080`, rebuild web `5173`. `AUTH_URL` for local login is `http://localhost:3000`.

From the Vercel repository:

```bash
npm install
npm run dev
```

Do not run `prisma db push`, seed, or other Neon writes unless the operator explicitly asks.

## Google authentication

Authentication is optional only in local and test environments. Staging and production fail during startup unless private application access is completely configured.

Create a Google OAuth client and configure these environment values:

- `AUTH_ENABLED=true`
- `GOOGLE_OAUTH_CLIENT_ID`
- `GOOGLE_OAUTH_CLIENT_SECRET`, supplied through `.env` locally or the deployment secret manager;
- `GOOGLE_OAUTH_REDIRECT_URL`, ending in `/auth/callback`;
- `AUTH_APPLICATION_URL`, the web application URL used after login and the only browser origin trusted for credentialed CORS and state-changing requests.

For local testing, use callback `http://localhost:8080/auth/callback` and application URL `http://localhost:5173`. The API stores only SHA-256 session hashes. Browser cookies are HttpOnly, SameSite=Lax, host-only, and become Secure outside local/test. Application sessions expire after 24 hours and logout revokes the server-side session.

Who can sign in is decided in Neon: a provisioned `app_users` row (Administração). Google OAuth does not create users. Seed the first `SUPERADMIN` in the database (or use local Dev Login, which uses a fixed `developer@gymkhana.local` identity). Disabled users are denied.

The complete setup, lifecycle, audit, smoke-test, incident, and recovery procedures are in [`docs/AUTHENTICATION.md`](docs/AUTHENTICATION.md).

## Google Forms ingestion

Owner-scoped Google Forms ingestion is a Cadastro submodule (`/cadastro?mode=forms`). It is disabled by default. It uses only the Forms body/response read-only scopes and feeds normalized responses into the existing Operations preview, decision, execution, and report flow. Production activation requires an owner-created Google Cloud OAuth client, the enabled Google Forms API, an exact callback URI, and secret-manager values.

Configuration, key rotation, smoke testing, and recovery procedures are in [`docs/GOOGLE_FORMS.md`](docs/GOOGLE_FORMS.md).

## Query Engine

The authenticated Query Engine builds permission-filtered, typed, read-only relational plans without accepting SQL or physical schema paths. Versioned plan nodes, limits, retention, and rollback are documented in [`docs/QUERY_ENGINE.md`](docs/QUERY_ENGINE.md). Gymkhana tasks are Chat capabilities, not a user module; see [`docs/TASKS.md`](docs/TASKS.md).

## Profile Matching

The authenticated Matching HTTP API generates bounded, explainable Profile candidates on demand, persists human review decisions and permits only ADMIN/SUPERADMIN to perform an explicit previewed transactional merge. Orchestration §15: this is the last module to implement; there is no `/matching` destination. Candidate rules, permissions, dependency movement, recovery and rollback are documented in [`docs/MATCHING.md`](docs/MATCHING.md).

## AI Assistente

The private Assistente (Orchestration §18) uses only permission-filtered, typed, read-only Search and Query tools. It is hosted in tables, Search, and a wide panel. There is no `/chat`, `/query`, or `/tasks` destination for users. The HTTP engine remains `/api/v1/chat`. Production uses a shared model key stored in Administração. Configuration, privacy, quotas, crash recovery, smoke testing and rollback are documented in [`docs/AI_CHAT.md`](docs/AI_CHAT.md).

## Multimodal OCR

Private OCR validates authorized PDF/image attachments, creates typed evidence-backed suggestions, and requires human review plus a separate conflict-safe application. Production remains blocked until the owner selects a provider/model and approves its privacy boundary. Configuration, limits, retry/recovery, acceptance, activation, smoke testing, and rollback are documented in [`docs/OCR.md`](docs/OCR.md).

## Common commands

```bash
make generate
make check
make test
```

Commands that need the database:

```bash
make check-config
make migrate-status
```

Schema changes go to Neon through `make migrate`. Do not drop Dev-18 from Make.

Use targeted commands such as `make check-backend` or `make check-frontend` while developing. Before a PR becomes ready for review, run `make check` plus every relevant migration or generated-contract verification. Dependency and lockfile changes must always be validated locally with `make scan` (govulncheck, OSV-Scanner, `pnpm audit`) before pushing, so the Security workflow never acts as the first place a vulnerability is discovered.

## CI and GitHub Actions budget policy

This private repository is operated within the GitHub Pro allowance of 3,000 standard GitHub-hosted runner minutes per month. The additional Pro quota is a safety margin, not permission to use Actions as a remote development loop. Hosted Actions are final integration gates.

Workflow behavior:

- pull-request jobs execute only for non-draft PRs and only when their relevant paths changed;
- draft PR pushes create no runner job;
- obsolete runs on the same PR are canceled through concurrency groups;
- successful PR checks are not repeated after merge by `push` workflows on `main`;
- backend Staticcheck and race tests run when a PR first becomes reviewable, when a ready PR is opened or reopened, or through an explicit full manual run;
- Security runs for relevant non-draft PRs, manually, and on the 1st and 15th of each month;
- no Docker or container-image builds exist; production deployment is governed by `docs/LAUNCH_RUNBOOK.md`;
- every job can be moved to a trusted Linux self-hosted runner by setting the repository variable `CI_RUNNER` to that runner's custom label. Without the variable, jobs use `ubuntu-latest`.

Mandatory usage rules:

1. Keep exactly one implementation PR per active workstream. Helper, diagnostic, formatting, export, validation, and squash PRs are prohibited.
2. Open implementation PRs as drafts. Develop, format, generate, and test locally before marking them ready for review.
3. Before the first ready-for-review transition, run the relevant targeted checks and every applicable deterministic generator or migration check. Run `make check` before any security-sensitive merge.
4. If a ready PR needs more than a trivial correction, convert it back to draft, batch all corrections, validate locally, and mark it ready once again. Do not use repeated pushes as a remote test loop.
5. Do not create dummy commits, close/reopen PRs, or add temporary workflows to force executions. Re-run only a failed job, and only when the failure was caused by transient runner or network infrastructure.
6. Do not push product changes directly to `main`. GitHub Pro branch protection or a repository ruleset must enforce PR-only integration.
7. No temporary artifacts or diagnostic archives may be uploaded. Required artifacts must use the shortest practical retention.
8. Review Actions usage before starting a large workstream. At 70% monthly usage, move heavy validation to local or self-hosted execution. At 85%, reserve hosted runners for final merge-blocking gates. At 95%, hosted execution requires explicit owner approval.
9. Dependabot PRs still receive functional review. Do not merge solely because dependency checks are green.
10. Any change that increases workflow frequency, job count, timeout, matrix size, artifact retention, or runner cost must explain the expected monthly impact in its PR.

GitHub documents the included quota and billing behavior in [GitHub Actions billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions). Self-hosted runners do not consume the included GitHub-hosted minutes, but their machine, updates, isolation, and security are the project owner's responsibility.

### GitHub Pro protection for `main`

After GitHub Pro is active, configure one active branch ruleset targeting `main` with these minimum rules:

- restrict branch deletion and force pushes;
- require changes to enter through a pull request;
- require all review conversations to be resolved;
- require linear history;
- do not require an approval while the repository has only one maintainer, because the PR author cannot provide an independent approval;
- do not require the branch to be up to date before merge, because that can trigger avoidable repeated CI runs;
- do not mark path-filtered workflow names as required checks until there is a universal, low-cost aggregate gate that always reports a result;
- grant no routine bypass. Any emergency bypass must be followed by a normal PR that documents and validates the resulting state.

Repository merge settings should allow squash merge only and automatically delete merged head branches. These controls protect `main` without forcing extra Actions executions.

## Platform contracts

- typed environment validation that fails closed in deployed environments;
- Google OAuth with state validation against provisioned `app_users`;
- exact-origin CSRF validation and credentialed CORS derived from `AUTH_APPLICATION_URL`;
- opaque, revocable, server-side sessions with a 24-hour lifetime;
- centralized `EXTERNAL`, `ADMIN`, and protected `SUPERADMIN` authorization;
- correlated authentication and administration audit events with observable persistence failures;
- stable JSON error envelopes with request IDs and safe public messages;
- generated Go and TypeScript API contracts;
- deterministic sqlc persistence adapters;
- released Core dependencies consumed only through exact versions.

## Repository boundaries

Gymkhana Database owns the product, persistence, HTTP API, workers, provider integrations, authorization, application routes, and the private UI. Reusable infrastructure-independent Go logic belongs in Gymkhana Core.

Core versions are pinned only after their releases are published. Permanent branch, commit, `replace`, subtree, submodule, or copied-source dependencies are not allowed.

## Project governance

- [`docs/ORCHESTRATION.md`](docs/ORCHESTRATION.md) contains permanent approved rules only.
- [`AGENTS.md`](AGENTS.md) requires agents to read Orchestration architecture/design plus the requested module section before any Gymkhana migration task.
- [Issue #31](https://github.com/Pherlsz/Gymkhana-Database/issues/31) is the only live project checklist and continuation tracker.
- [`docs/AUTHENTICATION.md`](docs/AUTHENTICATION.md) is an operational runbook for the implemented authentication feature.
- [`docs/GOOGLE_FORMS.md`](docs/GOOGLE_FORMS.md) is the activation, rotation, smoke-test, and recovery runbook for Google Forms ingestion.
- [`docs/QUERY_ENGINE.md`](docs/QUERY_ENGINE.md) defines QueryPlan security, execution, retention, and rollback boundaries.
- [`docs/TASKS.md`](docs/TASKS.md) defines gymkhana task interpretation as a Chat capability (no user Tasks workspace).
- [`docs/MATCHING.md`](docs/MATCHING.md) defines Profile candidate evidence, review lifecycle, explicit merge invariants, operations, and rollback boundaries.
- [`docs/AI_CHAT.md`](docs/AI_CHAT.md) defines the read-only tool boundary, privacy, quotas, activation, recovery, retention, smoke-test, and rollback procedures.
- [`docs/OCR.md`](docs/OCR.md) defines source/provider validation, privacy, review/application, retries, activation, acceptance, and rollback procedures.
- [`docs/LAUNCH_RUNBOOK.md`](docs/LAUNCH_RUNBOOK.md) is the production cutover, promotion, and rollback procedure.
- New status, continuation, or next-action documents must not be created.

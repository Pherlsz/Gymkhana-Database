# Gymkhana Database

Private web application for managing people, documents, bills, custom data, imports, search, duplicate review, OCR, and AI-assisted queries for gymkhana workflows.

## Documentation and tracking

Permanent product, domain, security, UX, and architecture rules live in [`docs/ORCHESTRATION.md`](docs/ORCHESTRATION.md).

Development status, milestone definitions, completed work, pending work, and continuation context live only in the [master checklist issue #31](https://github.com/Pherlsz/Gymkhana-Database/issues/31). The README and repository documents do not track the current milestone or next action.

## GitHub milestone operating model

GitHub milestones are delivery containers, not a second roadmap. Issue #31 remains the only cross-milestone tracker and source of truth for current state and next action.

### Structure

- One GitHub milestone represents one delivery stage, named `M<n> — <outcome>`.
- Keep only the current milestone and, when useful, the immediately following milestone open. Future roadmap items remain in #31 until they are prepared for execution.
- Every milestone has one parent issue that defines scope, exclusions, dependencies, acceptance, and the final closing gate.
- The parent issue and every executable issue belong to the milestone.
- Pull requests normally do not belong to the milestone when they already close a milestone issue. They must link the issue with `Closes #<number>` so GitHub closes the issue after merge. Assign a PR directly to a milestone only when it represents required delivery work that has no separate issue.
- The master tracker #31 is never assigned to a product milestone.
- Labels describe cross-cutting concerns such as backend, frontend, security, dependency, or blocked work; they do not replace milestones.

This avoids counting the same work twice. GitHub calculates milestone completion from the number of closed issues and pull requests, not from estimated effort, so issue sizing should remain reasonably consistent and delivery PRs should not duplicate their issues in the progress bar.

### Milestone description template

```text
Parent issue: #<number>
Outcome: <observable product result>
Depends on: <previous milestone or external dependency>
Exit gate: <final acceptance issue or criteria>
Next milestone: M<n+1> — <name>
```

Add a due date only for an active planning window. A due date is a forecast, not a release promise; update it when scope or dependencies change instead of hiding delay in issue state.

### Lifecycle

1. Before development, create the parent issue, executable issues, and GitHub milestone; assign the parent and executable issues to it.
2. Order the issues on the milestone page by dependency and execution sequence.
3. Start implementation from an executable issue and keep one draft PR per active workstream.
4. Any newly discovered scope must become a milestone issue, be explicitly deferred to a later milestone in #31, or be rejected as out of scope. It must not remain hidden only in a PR description.
5. Close executable issues only through merged delivery or an explicit `not planned` decision with rationale.
6. Close the final acceptance issue last, after all required integration gates are satisfied.
7. Update #31 with the delivered result and next executable unit, then close the GitHub milestone.
8. Releases and version tags remain separate from milestones. A milestone may lead to a release, but milestone progress must not depend on release automation.

### Milestone hygiene

- Do not use a milestone as a backlog bucket for unrelated work.
- Do not move unfinished work silently to the next milestone; record why it was deferred.
- Do not create tiny diagnostic, formatting, or CI-only issues merely to inflate completion.
- Do not close the parent issue before the final acceptance gate.
- Review milestone scope, dependencies, due date, and open-item ordering at every project resumption.

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

## Google Forms ingestion

Owner-scoped Google Forms ingestion is disabled by default. It uses only the Forms body/response read-only scopes and feeds normalized responses into the existing Operations preview, decision, execution, and report flow. Production activation requires an owner-created Google Cloud OAuth client, the enabled Google Forms API, an exact callback URI, and secret-manager values.

Configuration, key rotation, smoke testing, and recovery procedures are in [`docs/GOOGLE_FORMS.md`](docs/GOOGLE_FORMS.md).

## Query Engine

The authenticated Query Engine builds permission-filtered, typed, read-only relational plans without accepting SQL or physical schema paths. Its supported v1 nodes, limits, retention, safe error surface, M14 boundary, and rollback procedure are documented in [`docs/QUERY_ENGINE.md`](docs/QUERY_ENGINE.md).

## Profile Matching

The authenticated Matching workspace generates bounded, explainable Profile candidates on demand, persists human review decisions and permits only ADMIN/SUPERADMIN to perform an explicit previewed transactional merge. Candidate rules, permissions, dependency movement, recovery and rollback are documented in [`docs/MATCHING.md`](docs/MATCHING.md).

## AI Chat

The private AI Chat uses only permission-filtered, typed, read-only Search and Query tools. It remains disabled in production until the owner selects a provider/model and a default retention period. Configuration, privacy, quotas, crash recovery, smoke testing and rollback are documented in [`docs/AI_CHAT.md`](docs/AI_CHAT.md).

## Multimodal OCR

Private OCR validates authorized PDF/image attachments, creates typed evidence-backed suggestions, and requires human review plus a separate conflict-safe application. Production remains blocked until the owner selects a provider/model and approves its privacy boundary. Configuration, limits, retry/recovery, acceptance, activation, smoke testing, and rollback are documented in [`docs/OCR.md`](docs/OCR.md).

## Common commands

```bash
make generate
make check
make test
make check-config
make migrate
make reset-db
```

Use targeted commands such as `make check-backend` or `make check-frontend` while developing. Before a PR becomes ready for review, run `make check` plus every relevant migration or generated-contract verification.

## CI and GitHub Actions budget policy

This private repository is operated within the GitHub Pro allowance of 3,000 standard GitHub-hosted runner minutes per month. The additional Pro quota is a safety margin, not permission to use Actions as a remote development loop. Hosted Actions are final integration gates.

Workflow behavior:

- pull-request jobs execute only for non-draft PRs and only when their relevant paths changed;
- draft PR pushes create no runner job;
- obsolete runs on the same PR are canceled through concurrency groups;
- successful PR checks are not repeated after merge by `push` workflows on `main`;
- backend Staticcheck and race tests run when a PR first becomes reviewable, when a ready PR is opened or reopened, or through an explicit full manual run;
- Security runs for relevant non-draft PRs, manually, and on the 1st and 15th of each month;
- deployment builds remain manual;
- every job can be moved to a trusted Linux self-hosted runner by setting the repository variable `CI_RUNNER` to that runner's custom label. Without the variable, jobs use `ubuntu-latest`.

Mandatory usage rules:

1. Keep exactly one implementation PR per active workstream. Helper, diagnostic, formatting, export, validation, and squash PRs are prohibited.
2. Open implementation PRs as drafts. Develop, format, generate, and test locally before marking them ready for review.
3. Before the first ready-for-review transition, run the relevant targeted checks and every applicable deterministic generator or migration check. Run `make check` before milestone acceptance or any security-sensitive merge.
4. If a ready PR needs more than a trivial correction, convert it back to draft, batch all corrections, validate locally, and mark it ready once again. Do not use repeated pushes as a remote test loop.
5. Do not create dummy commits, close/reopen PRs, or add temporary workflows to force executions. Re-run only a failed job, and only when the failure was caused by transient runner or network infrastructure.
6. Do not push product changes directly to `main`. GitHub Pro branch protection or a repository ruleset must enforce PR-only integration.
7. No temporary artifacts or diagnostic archives may be uploaded. Required artifacts must use the shortest practical retention.
8. Review Actions usage before starting a milestone. At 70% monthly usage, move heavy validation to local or self-hosted execution. At 85%, reserve hosted runners for final merge-blocking gates. At 95%, hosted execution requires explicit owner approval.
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

## Project governance

- [`docs/ORCHESTRATION.md`](docs/ORCHESTRATION.md) contains permanent approved rules only.
- [Issue #31](https://github.com/Pherlsz/Gymkhana-Database/issues/31) is the only live project checklist and continuation tracker.
- GitHub milestones summarize one delivery stage and derive progress from its parent and executable issues; they do not replace #31.
- [`docs/AUTHENTICATION.md`](docs/AUTHENTICATION.md) is an operational runbook for the implemented authentication feature.
- [`docs/GOOGLE_FORMS.md`](docs/GOOGLE_FORMS.md) is the activation, rotation, smoke-test, and recovery runbook for Google Forms ingestion.
- [`docs/QUERY_ENGINE.md`](docs/QUERY_ENGINE.md) defines the QueryPlan v1 security, execution, retention, M14, and rollback boundaries.
- [`docs/MATCHING.md`](docs/MATCHING.md) defines Profile candidate evidence, review lifecycle, explicit merge invariants, operations, and rollback boundaries.
- [`docs/AI_CHAT.md`](docs/AI_CHAT.md) defines the read-only tool boundary, privacy, quotas, activation, recovery, retention, smoke-test, and rollback procedures.
- [`docs/OCR.md`](docs/OCR.md) defines source/provider validation, privacy, review/application, retries, activation, acceptance, and rollback procedures.
- New milestone-status, acceptance-tracking, continuation, or next-action documents must not be created.

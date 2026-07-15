# Gymkhana Database — Project Continuation Checklist

> **Last verified:** 2026-07-15  
> **Verified `main` commit:** `008fc987ddedb75e2555b0325b218f20216b5ff2`  
> **Current product milestone:** M3 — Profiles  
> **Canonical planning source:** [`docs/ORCHESTRATION.md`](ORCHESTRATION.md)  
> **Delivery conventions:** [`docs/IMPLEMENTATION.md`](IMPLEMENTATION.md)

This document is the operational handoff checklist for the Gymkhana Database rebuild. It combines the approved Stage 1–10 rules, the current repository state, merged pull requests, and active milestone issues so that a different AI or developer can continue without reconstructing the planning conversation.

## Status notation

- `[x]` completed and merged into `main`;
- `[ ]` pending;
- `[-]` explicitly deferred to a later milestone;
- `[!]` requires repository-owner action or a product decision before execution;
- `[~]` work exists but must be revalidated before reuse.

## Source precedence

When sources disagree, use this order:

1. the latest explicit user decision;
2. this checklist and the current milestone tracking issue;
3. `docs/ORCHESTRATION.md`;
4. merged pull-request descriptions and acceptance records;
5. older issue text or historical branches.

Never silently resolve a material conflict. Record the conflict in the tracking issue and stop only when the repository owner must decide.

---

# 1. Mandatory continuation protocol

Every implementation session must perform these checks before writing code.

- [ ] Read `docs/PROJECT_CONTINUATION_CHECKLIST.md`.
- [ ] Read `docs/ORCHESTRATION.md`.
- [ ] Read the current central milestone tracking issue and every dependent workstream issue.
- [ ] List open pull requests in `Gymkhana-Database`, `Gymkhana-UI`, and `Gymkhana-Core`.
- [ ] Search existing branches for the milestone/workstream name before creating a branch.
- [ ] Review open Dependabot pull requests before or during the milestone.
- [ ] Compare the planned implementation with already-open PRs; continue or reconcile existing work instead of creating a parallel duplicate.
- [ ] Confirm the current `main` commit and base every new branch on it.
- [ ] Keep incomplete features hidden or inaccessible rather than presenting placeholders as working product features.
- [ ] Keep `main` compilable and migrations applicable after every merge.
- [ ] Consolidate the branch into one intentional final commit whenever practical. The repository owner specifically prefers one final commit to avoid unnecessary Vercel/CI builds.
- [ ] Use squash merge.
- [ ] Merge automatically after every required check is green **unless** owner action, credentials, an external configuration change, or a product decision is required.
- [ ] Never commit secrets or request that secrets be posted in chat, an issue, or a PR.
- [ ] Update the workstream issue and central milestone issue after merge.
- [ ] Update this checklist when a workstream or milestone is completed or when an approved rule changes.

## Required PR final state

Before merge, every product PR must satisfy all applicable items:

- [ ] branch contains no temporary generation, synchronization, or diagnostic workflow;
- [ ] generated OpenAPI and sqlc output is synchronized and was not manually edited;
- [ ] Backend checks are green: formatting, vet, tests, race tests, Staticcheck, builds;
- [ ] Frontend checks are green when frontend files are affected: typecheck, Oxlint, Oxfmt, Vitest, Vite build;
- [ ] OpenAPI checks are green when API contracts are affected;
- [ ] migration and deterministic sqlc checks are green when persistence is affected;
- [ ] Security checks are green: govulncheck, OSV-Scanner, JavaScript dependency audit, and any enabled dependency review;
- [ ] documentation and changelog are updated;
- [ ] PR boundaries explicitly list deferred modules;
- [ ] no owner action remains unless the PR is intentionally waiting for that action.

---

# 2. Non-negotiable product and architecture rules

These rules apply to all milestones.

## Product scope and tenancy

- [ ] The product is a **single installation**.
- [ ] Multi-organization/tenancy is removed completely.
- [ ] Do not add `organizations`, `organization_id`, tenant scopes, organization selectors, or equivalent abstractions.
- [ ] The canonical root entity is `Profile`; the legacy generic `Record` model must not return.
- [ ] The initial product supports physical persons only.
- [ ] UI copy is `pt-BR`.
- [ ] Code, commit messages, PR descriptions, workflows, and technical code documentation are in English.

## Architecture

- [ ] Use a modular monolith; do not introduce initial microservices.
- [ ] Backend is Go.
- [ ] Frontend is React + TypeScript.
- [ ] Use explicit PostgreSQL SQL with pgx v5 and sqlc; no ORM.
- [ ] Use REST JSON and OpenAPI 3.1; no GraphQL.
- [ ] Keep domain code independent from database, HTTP, providers, queues, and UI.
- [ ] `cmd/api`, `cmd/worker`, and `cmd/migrate` contain bootstrap, composition, lifecycle, and configuration only; domain rules do not live in `cmd`.
- [ ] Prefer feature packages such as `internal/profile`, `internal/document`, and `internal/bill`; do not force a generic controller/service/repository hierarchy across all modules.
- [ ] Do not introduce Redux, Zustand, SSR, a Node backend, Redis, RabbitMQ, an agent framework, or microservices without a new approved decision.
- [ ] River OSS and real business jobs begin in M8. Do not create a fake permanent queue before then.

## Repository boundaries

- [ ] `Gymkhana-Database` may depend on exact released versions of `Gymkhana-UI` and `Gymkhana-Core`.
- [ ] UI and Core do not depend on Database or on each other.
- [ ] Do not use permanent branch dependencies, commit dependencies, `replace`, submodules, subtree copies, or copied source.
- [ ] Extract to UI/Core only after a reusable contract or proven reuse exists.
- [ ] UI/Core releases must contain meaningful real capability, not artificial version-only packages.

## Frontend state and URL rules

- [ ] TanStack Router owns URL state.
- [ ] TanStack Query owns server state.
- [ ] TanStack Form and Valibot own form state/validation when used.
- [ ] React local state is limited to visual/transient state.
- [ ] Backend owns durable preferences.
- [ ] Filters, pagination, sorting, grouping, and relevant tabs must be reflected in the URL.
- [ ] Components do not perform direct fetches; API calls stay behind typed client/domain boundaries.
- [ ] Accessibility, responsive behavior, document titles, focus management, loading, empty, error, conflict, and success states are part of acceptance, not later polish.

## API contract

- [ ] Public API base is `/api/v1` for product resources.
- [ ] Health endpoints remain `/health/live` and `/health/ready`.
- [ ] Use semantic HTTP methods and keep GET free of side effects.
- [ ] Standard success statuses: 200, 201, 202, 204.
- [ ] Standard error statuses include 400, 401, 403, 404, 409, 412, 422, 429, 500, 503.
- [ ] Errors use a stable envelope with code, safe `pt-BR` message, `request_id`, field errors, and permitted details.
- [ ] Lists are paginated.
- [ ] Projections are allowlisted.
- [ ] Optimistic concurrency uses `version`; stale writes must return the approved conflict/precondition response and never overwrite silently.
- [ ] Critical operations use `Idempotency-Key` when their milestone introduces retryable or asynchronous execution.
- [ ] Operation IDs and generated identifiers are stable and written in English.
- [ ] Generated Go and TypeScript contracts are committed, deterministic, regenerated by CI, and never manually edited.

## Authentication and security

- [x] GitHub OAuth is the application login provider.
- [x] An explicit normalized GitHub login allowlist controls admission.
- [x] Cloudflare Access remains an optional outer layer; it does not replace application authorization.
- [x] Sessions are opaque, server-side, revocable, and expire after 24 hours.
- [x] Only SHA-256 session hashes are stored.
- [x] Session cookies are host-only, HttpOnly, SameSite=Lax, and Secure outside local/test.
- [x] OAuth state is validated.
- [x] State-changing browser requests validate the configured application origin.
- [x] Credentialed CORS is restricted to the configured application origin.
- [x] Roles are `MEMBER`, `ADMIN`, and `SUPERADMIN`.
- [x] Exactly one active `SUPERADMIN` is permitted.
- [x] Authorization is centralized through permissions/capabilities.
- [x] `SUPERADMIN` does not bypass domain constraints, privacy rules, optimistic concurrency, or audit requirements.
- [x] Administrators cannot change their own role or active access through the generic administration surface.
- [x] The unique `SUPERADMIN` cannot be changed, disabled, demoted, or reassigned through the generic administration surface.
- [x] Effective access changes revoke all sessions for the affected user.
- [x] No-op access updates do not write or revoke sessions.
- [x] Authentication and access-management events are audited with request correlation.
- [ ] Never expose secrets, session values, provider access tokens, signed URLs, SQL, stack traces, or provider payloads in responses or logs.
- [ ] Logs must redact sensitive values; audit storage is separate from application logs.
- [ ] Staging and production fail closed when authentication or database configuration is incomplete.

## Data, infrastructure, and environments

- [ ] PostgreSQL runs on Neon for hosted persistence.
- [ ] Cloudflare R2 is the approved attachment store when M6 starts.
- [ ] Vercel hosts the SPA.
- [ ] Cloud Run Service hosts the API.
- [ ] Cloud Run Job hosts the worker.
- [ ] Cloud Scheduler is reserved for recovery and housekeeping jobs.
- [ ] Keep processing close to PostgreSQL to reduce Neon egress.
- [ ] Local, staging, and production are separate environments.
- [ ] Staging uses synthetic data only.
- [ ] Never copy production data into staging.
- [ ] `.env.example` contains no secrets and `.env` remains ignored.
- [ ] Production secrets come from the platform secret manager.
- [ ] Basic tests must not require external credentials.
- [ ] Database migrations are forward-first, tested on an empty database and on upgrade paths, and use expansion/contraction for destructive changes.
- [ ] Seeds remain separate from migrations.
- [ ] Do not run migrations automatically from the database container.

## AI and query safety

- [ ] The AI is a private, read-only consultant.
- [ ] Do not expose arbitrary SQL or arbitrary code execution to the AI.
- [ ] Do not artificially limit the AI to a small predefined set of question types or fields.
- [ ] Physical tables and columns are never free-form public API inputs.
- [ ] Queries are produced through typed catalog, plan, AST, relation, projection, grouping, aggregation, set, pattern, and execution contracts.
- [ ] AI actions never mutate product data.
- [ ] Saved queries are not created automatically.

---

# 3. Completed tasks

## M0 — Bootstrap

### Database foundation

- [x] **PR #1 — `chore(database): bootstrap application foundation`**
  - created compilable `cmd/api`, `cmd/worker`, and `cmd/migrate` executables;
  - added typed configuration, structured logging, PostgreSQL connectivity, request IDs, security headers, graceful shutdown, and live/ready health endpoints;
  - added local PostgreSQL Compose with healthcheck and named volume;
  - added Tern migration bootstrap and sqlc schema/query generation;
  - added OpenAPI 3.1 health contracts with committed Go/TypeScript output;
  - created the React 19, TypeScript, and Vite web shell with API access isolated from components;
  - added backend, frontend, OpenAPI, migrations, security, and manual deployment-build workflows;
  - did not add authentication, product modules, River jobs, external providers, production credentials, or automatic production deployment.

### Core/UI release integration

- [x] Core `v0.1.0` published with bootstrap, CI, documentation, and a valid consumable release boundary.
- [x] UI `0.1.0` published and made readable by Database through GitHub Packages Actions access.
- [x] **PR #7 — `chore(deps): integrate foundation releases`**
  - pinned exact Core/UI releases;
  - consumed real UI tokens/styles/components in the web shell;
  - preserved the rule that Core `v0.1.0` did not create an artificial public package merely to satisfy versioning;
  - kept dependency workflows deterministic and free of temporary diagnostics.

### M0 acceptance

- [x] clean-clone foundations work;
- [x] API, worker, and migrate compile;
- [x] SPA starts and consumes the published UI package;
- [x] PostgreSQL local environment has healthcheck;
- [x] migrations apply to an empty database;
- [x] OpenAPI and sqlc regenerate without drift;
- [x] basic path requires no external provider credentials;
- [x] no critical placeholder is presented as a real feature.

## M1 — Shared foundations

- [x] Core released `v0.2.1` with real normalization and civil-date capabilities.
- [x] UI released `0.3.0` with semantic themes, AppShell, Page, layout, control, feedback, overlay, and status foundations.
- [x] **PR #13 — `feat(platform): complete M1 shared foundations`**
  - pinned Core `v0.2.1` and UI `0.3.0` exactly;
  - added real Core integration tests;
  - integrated published UI foundations into the real shell;
  - added typed environment, log-level, body-size, address, database, and shutdown validation;
  - added stable JSON errors, request boundaries, request IDs, body limits, and safe panic handling;
  - added generated-contract-based frontend API helpers;
  - added private Core access for CI, security, generation, deployment, and BuildKit;
  - all Backend, Frontend, OpenAPI, Migrations, and Security checks passed.
- [x] Dependabot `@types/node` 26 update was reviewed and closed because the project baseline is Node.js 24 LTS.

## M2 — Authentication and minimal administration

### M2.1 persistence and session domain

- [x] **PR #19 — `feat(auth): add persistence and session foundations`**
  - added `app_users` with GitHub identity, active state, roles, optimistic versioning, and case-insensitive login uniqueness;
  - added the database invariant allowing at most one active `SUPERADMIN`;
  - added opaque session storage with SHA-256 hashes and lifecycle timestamps;
  - added authentication audit persistence;
  - added deterministic sqlc queries and typed Go contracts;
  - fixed session lifetime at 24 hours;
  - intentionally excluded provider calls, callbacks, middleware, administration UI, and Profile/product modules.

### M2.2 GitHub sign-in and protected sessions

- [x] Repository owner created the GitHub OAuth App and configured local credentials without committing or posting the secret.
- [x] **PR #20 — `feat(auth): integrate GitHub sign-in and protected sessions`**
  - added a bounded/testable GitHub OAuth provider;
  - added state-cookie validation;
  - enforced normalized allowlist admission;
  - bootstrapped the configured first `SUPERADMIN` and created other approved users as `MEMBER`;
  - created opaque 24-hour sessions and persisted only hashes;
  - added login, callback, current-session, and logout routes;
  - denied missing, unlisted, or inactive users;
  - never persisted GitHub OAuth access tokens;
  - added an authentication-aware frontend shell;
  - failed closed in staging/production with incomplete auth configuration.

### M2.3 authorization and minimal administration

- [x] PR #21 was reviewed, reconciled, and closed as superseded rather than ignored.
- [x] **PR #22 — `feat(auth): add authorization and minimal user administration`**
  - centralized role/permission checks;
  - allowed `ADMIN` and `SUPERADMIN` to access the minimal user-management surface while preserving domain constraints;
  - blocked self-access changes;
  - protected the unique `SUPERADMIN` from generic access changes;
  - prevented generic promotion to `SUPERADMIN`;
  - used optimistic versions for access updates;
  - skipped no-op writes;
  - revoked sessions after effective role/active changes;
  - audited denied, failed, successful, and revocation outcomes;
  - exposed protected list/update endpoints and a responsive administration UI.

### M2.4 audit, browser security, runbook, and validation

- [x] **PR #23 — `feat(auth): complete M2 audit and operational validation`**
  - completed sign-in, sign-out, administration, access-change, and session-revocation audit outcomes;
  - added structured reporting when audit persistence fails without leaking provider login or secret material;
  - added exact application-origin checks for state-changing browser requests;
  - restricted credentialed CORS to the configured application origin;
  - strengthened authentication URL validation;
  - added a safe configuration preflight;
  - added authentication lifecycle, operations, incident, and recovery documentation;
  - added an explicit M2 acceptance record;
  - added a synthetic full-flow test for superadmin bootstrap, member access, administration, revocation, logout, and audit events;
  - kept Profiles and later modules outside M2.

### M2 acceptance

- [x] protected GitHub authentication works;
- [x] session lifecycle and revocation work;
- [x] minimal role administration works;
- [x] exactly-one-superadmin invariant is preserved;
- [x] browser origin/CORS protections are active;
- [x] audit coverage and operational documentation are complete;
- [x] M2 issues #14–#18 completed/closed as applicable.

## M3.1 — Canonical Profile persistence and domain validation

- [x] **Issue #25 completed.**
- [x] **PR #30 — `feat(profile): add canonical persistence foundation`**
  - Profile uses explicit PostgreSQL columns, not a generic JSON record;
  - physical persons only;
  - UUID primary key, timestamps, and optimistic `version`;
  - required full name;
  - optional social name;
  - optional normalized CPF with **no hard uniqueness constraint**;
  - one optional email normalized to lowercase;
  - one optional mobile phone and one optional landline/other phone in normalized form;
  - one structured optional address;
  - optional personal notes;
  - domain normalization uses pinned Gymkhana Core contracts;
  - stable field-level validation errors;
  - deterministic sqlc create, get, count, list, update, duplicate, and permanent-delete queries;
  - persistence adapter classifies not-found and optimistic-conflict errors;
  - migration, schema snapshot, generated output, unit tests, migration validation, and security checks passed;
  - no HTTP API or UI was added in this slice;
  - documents, bills, attachments, teams, custom data, imports, search, duplicate merge, AI, and OCR remained deferred.

---

# 4. Current work and pending tasks

## M3 — Profiles central rules

Central tracking: issue #24.

All M3 work must preserve these rules:

- [ ] one installation; no organization/tenant fields;
- [ ] physical persons only;
- [ ] Profile replaces the legacy generic personal Record;
- [ ] one email, one mobile phone, one landline/other phone, and one address per Profile;
- [ ] social name is optional;
- [ ] CPF is optional and is not globally forced unique in M3;
- [ ] permanent deletion is supported with explicit confirmation and audit;
- [ ] updates use optimistic `version`;
- [ ] no generic JSON payload as the primary Profile model;
- [ ] documents, bills, attachments, custom data, imports, teams, duplicate merge, generic search, AI, and OCR remain in later milestones;
- [ ] M3 UI and APIs remain prepared for later Profile-owned modules without implementing them prematurely.

## M3.2 — Protected Profile CRUD API and audit

Tracking: issue #26. Depends on completed issue #25.

- [ ] Create the Profile application/service boundary over the existing domain and persistence foundation.
- [ ] Add authenticated list endpoint.
- [ ] Add authenticated get endpoint.
- [ ] Add authenticated create endpoint.
- [ ] Add authenticated update endpoint.
- [ ] Add authenticated duplicate endpoint.
- [ ] Add authenticated permanent-delete endpoint.
- [ ] Route every authorization decision through centralized permissions; do not duplicate role checks ad hoc in handlers.
- [ ] Add stable OpenAPI schemas and operation IDs.
- [ ] Regenerate and commit Go and TypeScript contracts.
- [ ] Keep list pagination deterministic.
- [ ] Keep default sorting deterministic.
- [ ] Carry `version` through response/update contracts.
- [ ] Return a stable conflict/precondition response for stale versions; never overwrite silently.
- [ ] Return structured field validation errors.
- [ ] Require an explicit delete-confirmation contract; a simple accidental DELETE request is insufficient.
- [ ] Audit create, update, duplicate, and permanent delete with request IDs.
- [ ] Add HTTP tests.
- [ ] Add service tests.
- [ ] Add persistence integration tests.
- [ ] Keep inline grid behavior, imports/exports, documents, bills, attachments, teams, custom data, generic search, and AI out of this slice.

## M3.3 — Profile forms, detail view, and responsive list

Tracking: issue #27. Depends on M3.2.

- [ ] Add an authenticated Profiles route and navigation entry.
- [ ] Add responsive paginated Profile list.
- [ ] Reflect list pagination/filter/sort state in the URL.
- [ ] Add create form for identity, contact, address, and notes.
- [ ] Add edit form for the same canonical fields.
- [ ] Preserve optional social-name behavior.
- [ ] Preserve optional CPF behavior.
- [ ] Format normalized values for display only at the frontend boundary; do not corrupt normalized storage.
- [ ] Add Profile detail view with reserved structure for later documents, bills, and custom-data sections, but do not implement those modules yet.
- [ ] Add duplication action that creates a separate Profile and opens it for review.
- [ ] Add permanent-delete confirmation requiring explicit confirmation text.
- [ ] Add loading state.
- [ ] Add empty state.
- [ ] Add error state.
- [ ] Add optimistic-conflict state and recovery.
- [ ] Add success feedback.
- [ ] Add keyboard and screen-reader accessibility coverage.
- [ ] Add frontend tests.
- [ ] On mobile, preserve reading, form-based creation/editing, duplication, and deletion.
- [ ] On mobile, keep spreadsheet-style direct grid editing disabled.

## M3.4 — Desktop Profile grid editing and column workflows

Tracking: issue #28. Depends on M3.2 and may proceed alongside M3.3.

- [ ] Add Profile-specific desktop table/grid.
- [ ] Add column filters.
- [ ] Add sorting.
- [ ] Add pagination.
- [ ] Reflect filters, sorting, pagination, and page size in the URL.
- [ ] Offer page-size choices from 100 through 1000.
- [ ] Add spreadsheet-style editing for supported canonical fields.
- [ ] Save supported cell edits on blur.
- [ ] Use optimistic versioning for every inline update.
- [ ] On conflict, refresh the row and show a clear conflict state; never overwrite silently.
- [ ] Do not allow row creation directly inside the grid.
- [ ] Keep Profile creation in the dedicated creation flow.
- [ ] Add duplication action to the row menu.
- [ ] On mobile, render a responsive read-only table/card representation instead of editable cells.
- [ ] Add keyboard navigation and visible focus states.
- [ ] Add frontend tests.
- [ ] Keep this implementation Profile-specific.
- [ ] Defer reusable generic grid/query infrastructure to M7.
- [ ] Do not add arbitrary user-defined columns, grouping, aggregation, or export in M3.

## M3.5 — Documentation and M3 acceptance

Tracking: issue #29. Depends on #25–#28.

- [ ] Validate migrations on an empty database and supported upgrade path.
- [ ] Validate deterministic sqlc generation and committed output.
- [ ] Validate OpenAPI and generated Go/TypeScript contracts.
- [ ] Run Backend checks.
- [ ] Run Frontend checks.
- [ ] Run race tests.
- [ ] Run accessibility checks.
- [ ] Run Security checks.
- [ ] Add/execute a synthetic create flow.
- [ ] Add/execute a synthetic read flow.
- [ ] Add/execute a synthetic update flow.
- [ ] Add/execute a synthetic duplicate flow.
- [ ] Add/execute a synthetic permanent-delete flow.
- [ ] Record evidence for normalization behavior.
- [ ] Record evidence for optimistic-conflict behavior.
- [ ] Verify permanent deletion and audit persistence.
- [ ] Document canonical Profile fields.
- [ ] Document module boundaries and deferred Profile-owned modules.
- [ ] Validate clean-clone setup.
- [ ] Add an M3 acceptance record.
- [ ] Synchronize `docs/ORCHESTRATION.md`.
- [ ] Close issues #24 and #29 after merge/validation.
- [ ] Keep old-system data migration and later product modules outside M3.

## Required M3 execution order

1. [x] M3.1 persistence/domain — issue #25 / PR #30.
2. [ ] M3.2 protected API/audit — issue #26.
3. [ ] M3.3 forms/detail/responsive list — issue #27.
4. [ ] M3.4 desktop Profile grid — issue #28; may run alongside #27 only after #26 is stable.
5. [ ] M3.5 acceptance/docs — issue #29.

---

# 5. Future milestone checklist and preserved rules

These milestones are approved at roadmap level. Before implementation, each must be decomposed into a central tracking issue and reviewable workstream issues containing scope, exclusions, dependencies, acceptance criteria, tests, security impact, performance impact, and repository responsibility.

## M4 — Documents and bills

### Shared ownership rules

- [ ] Every document and bill belongs to exactly one Profile.
- [ ] Orphan documents/bills are forbidden.
- [ ] Printed bill/document data must not mutate Profile fields implicitly.

### Documents

- [ ] Use a hybrid model: common document table plus family-specific details and typed custom fields.
- [ ] Document types are administrable.
- [ ] Every type has a stable technical key independent of display labels.
- [ ] Supported uniqueness policies are exactly `NONE`, `PER_PROFILE`, and `GLOBAL_BY_TYPE`.
- [ ] Support old and expired document versions rather than overwriting historical document identity.
- [ ] Add type-appropriate regex/format validation where valid.
- [ ] Preserve observations and their associated date where required by the approved document model.
- [ ] Do not add barcode storage/processing unless a later explicit decision changes this rule.
- [ ] Document status includes usable concepts equivalent to available/in use as required by the active-use model.

### Bills/accounts

- [ ] Preserve printed data exactly as entered for the bill/account record.
- [ ] Store competence as `YYYY-MM`.
- [ ] Store money as decimal, never floating point.
- [ ] The owner is a Profile.
- [ ] Printed owner/address/name values do not silently update the owner Profile.

### Active use

- [ ] Model only current active use initially.
- [ ] At most one current active-use assignment per item.
- [ ] Do not create an initial active-use history table merely for future possibility.
- [ ] When an item is returned, remove/end its current assignment according to the approved no-history initial model.

## M5 — Custom data

- [ ] Add typed custom fields for Profile, document type, bill type, and custom entity type.
- [ ] Custom fields are not free-form scripts.
- [ ] Do not support formulas or executable expressions.
- [ ] Do not use arbitrary JSON as the primary source of truth.
- [ ] Custom entities support `ONE_PER_PROFILE` or `MANY_PER_PROFILE` cardinality.
- [ ] Validation and storage remain typed and queryable.

## M6 — Attachments and storage

- [ ] Store private attachments in Cloudflare R2.
- [ ] Attachments may belong to a document, bill, or custom-value attachment field.
- [ ] Use signed direct upload followed by server-side confirmation.
- [ ] Validate MIME type.
- [ ] Validate file signature/magic bytes where applicable.
- [ ] Validate size limits.
- [ ] Validate/store content hash.
- [ ] Do not trust a client upload declaration without confirmation.
- [ ] Use a seven-day trash/recovery window before final removal.
- [ ] Never expose permanent public object URLs.

## M7 — Search and complete Data Grid

### Generic grid

- [ ] Extract reusable grid behavior only after Profile-specific behavior has proven the contract.
- [ ] Support filters by column.
- [ ] Support sorting.
- [ ] Support pagination up to approved large page sizes.
- [ ] Reflect all relevant state in the URL.
- [ ] Preserve responsive read behavior on mobile and disable unsupported spreadsheet editing on mobile.
- [ ] Do not create rows directly inside generic grids unless a later module explicitly approves it.

### Search/Query contracts

- [ ] Build a runtime catalog filtered by the authenticated user's permissions.
- [ ] Use typed `QueryPlan` contracts.
- [ ] Use an AST rather than concatenated arbitrary SQL.
- [ ] Model approved relations explicitly.
- [ ] Allow only approved projections.
- [ ] Add grouping and aggregation through typed contracts.
- [ ] Add sets, patterns, and combinations through typed contracts.
- [ ] Produce an `ExecutionPlan` before execution.
- [ ] Do not expose physical table/column names as arbitrary public inputs.
- [ ] Keep heavy filtering, grouping, and candidate generation near PostgreSQL to control egress.

## M8 — Operations, XLSX imports and exports

- [ ] Introduce River OSS and real business jobs here, not earlier.
- [ ] XLSX imports use staging, mapping, validation, decisions, batched execution, idempotency, reports, and retention.
- [ ] Import supports a single intended worksheet flow; do not silently process arbitrary multiple sheets.
- [ ] Do not retain the original spreadsheet as product data after processing unless a later retention decision explicitly requires it.
- [ ] Produce a clear simple import report.
- [ ] Export only the complete currently requested table/dataset according to the approved export boundary; do not invent partial ad hoc export semantics.
- [ ] Critical execution is retryable and uses idempotency.
- [ ] Business states remain simple; technical phases belong in `stage`/job execution state.

## M9 — Google Forms

- [ ] May proceed in parallel only after the M8 operations/import foundation exists.
- [ ] Reuse the same staging, mapping, validation, decisions, batched execution, idempotency, report, and retention pipeline as XLSX.
- [ ] Do not build a separate incompatible ingestion engine for Forms.

## M10 — Query Engine base

- [ ] Establish the typed catalog/plan/AST/execution foundation required by advanced search and AI.
- [ ] Support permission-filtered fields and relations.
- [ ] Support projection, filtering, sorting, pagination, grouping, aggregation, sets, and patterns through typed nodes.
- [ ] Preserve original typed plans for pagination/filter/sort refinement.
- [ ] No arbitrary SQL endpoint.
- [ ] No hardcoded finite list of natural-language question templates.

## M11 — Matching, duplicates, and merge

- [ ] Persistent duplicate-review queue applies to Profiles only initially.
- [ ] Candidate generation and broad aggregation run in PostgreSQL.
- [ ] Worker/Core receives compact evidence vectors rather than full uncontrolled datasets.
- [ ] Duplicate analysis is on demand; do not add constant periodic inspection without a new decision.
- [ ] Ambiguous matches require human review.
- [ ] Never merge automatically.
- [ ] Merge is transactional.
- [ ] The UI must explain evidence and preserve the human decision trail.

## M12 — AI Chat base

- [ ] AI Chat is private and read-only.
- [ ] It acts as a database consultant, not a mutation agent.
- [ ] It must be able to interpret questions about any authorized data/field; do not restrict it to predefined reports.
- [ ] Support simple requests such as names containing a value or character sequences in any document field.
- [ ] Support multi-parameter requests spanning people, addresses, documents, bills/accounts, and other authorized relations.
- [ ] Use module filters and permission-filtered catalog entries.
- [ ] Return scored/explained results and tables where appropriate.
- [ ] Support private threads.
- [ ] Use typed tools.
- [ ] Stream long responses through SSE when appropriate.
- [ ] Include references/evidence and reusable result-set identifiers for follow-up refinement.
- [ ] Follow-ups such as “from these results” refine the prior result set rather than restarting from an unrelated query.
- [ ] Apply quotas/bounds.
- [ ] Do not create saved queries automatically.
- [ ] Do not expose or execute unrestricted SQL supplied by the model or user.

## M13 — Multimodal OCR

- [ ] OCR produces suggestions and evidence only.
- [ ] Human review is mandatory.
- [ ] Never apply OCR-derived values automatically.
- [ ] Keep extracted evidence linked to the source attachment and target field proposal.

## M14 — Advanced Query Engine and gymkhana tasks

- [ ] Interpret free-form task text semantically.
- [ ] A task may require any combination of people, documents, bills/accounts, addresses, custom data, and character/digit constraints.
- [ ] Do not grade or parse requests by requiring users to type exact keywords or predefined phrases.
- [ ] Support mixed requirements in one task.
- [ ] Create explicit bindings between task requirements and fields/relations.
- [ ] Generate candidate sets before solving combinations.
- [ ] Use pruning to control combinatorial growth.
- [ ] Produce explainable results showing why each candidate/composition satisfies the requirements.
- [ ] Support character-sequence and digit-composition problems, including binary-digit/allowed-character constraints, without hardcoding a single puzzle type.
- [ ] Preserve read-only behavior; the solver does not modify product data.

## M15 — Hardening, migration, and launch

- [ ] Complete performance, security, recovery, observability, and deployment hardening.
- [ ] Validate Cloud Run API/worker, Vercel SPA, Neon, R2, and Scheduler operational boundaries.
- [ ] Configure production deployment only with secret-manager-backed credentials.
- [ ] Keep production deployment deliberate; do not enable an unsafe automatic production path.
- [ ] Plan old-system migration explicitly; do not infer field mappings or silently discard unsupported data.
- [ ] Validate backups and Neon recovery behavior.
- [ ] Validate clean deployment and rollback/recovery procedures.
- [ ] Run end-to-end acceptance with production-like synthetic data before launch.
- [ ] Never copy real production data into staging.

---

# 6. Deferred or owner-dependent items

- [!] A GitHub milestone object for M3 is not currently available through the connected tooling. Issue #24 is the central tracking surface until the owner creates the milestone or tooling gains that capability.
- [!] Production OAuth callback/application URLs and production secret-manager values will require owner/platform configuration when deployment is activated.
- [!] Cloudflare Access is optional and should only be configured when the owner decides to add the outer access layer.
- [-] Audit-log browsing UI is not part of M2 and must not be added implicitly.
- [-] Generic superadmin transfer is not part of the generic user-administration surface and requires a separately approved recovery contract.
- [-] Product data migration from the old system is deferred to M15.
- [-] Documents/bills are M4; custom data M5; attachments M6; generic search/grid M7; operations/imports M8; Forms M9; Query Engine M10; duplicates M11; AI Chat M12; OCR M13; advanced gymkhana tasks M14.

---

# 7. Immediate next action

The next authorized development action is:

1. [ ] re-read issue #26 and current `main`;
2. [ ] confirm no existing M3.2 PR/branch or Dependabot PR is open;
3. [ ] create a short branch from current `main` for M3.2;
4. [ ] implement the protected Profile CRUD service/API/audit slice only;
5. [ ] generate OpenAPI Go/TypeScript contracts deterministically;
6. [ ] run all applicable checks;
7. [ ] consolidate to one final commit;
8. [ ] merge automatically when green unless owner action is required;
9. [ ] update issues #26 and #24 and this checklist;
10. [ ] proceed to M3.3/M3.4 according to dependencies.

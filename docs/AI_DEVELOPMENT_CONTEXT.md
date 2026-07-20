# Gymkhana Database — Development Context for AI Agents

## Purpose

This document is a reusable context package for AI coding agents working on Gymkhana Database. It summarizes the approved product direction, architecture, domain rules, repository boundaries, security constraints, and implementation conventions.

It is not a progress tracker. Before changing code, inspect the current branch, the master tracker issue, the active milestone, open pull requests, and the implementation itself. Never infer that a feature is complete merely because it is described here.

When this document conflicts with a newer explicit owner decision, the newer decision wins and the permanent architecture documentation must be updated deliberately.

## Product objective

Gymkhana Database is a private application for a small group of known users who manage personal data, documents, accounts, imports, searches, duplicate analysis, and complex data combinations used in gymkhana tasks.

Its defining capability is not CRUD alone. The system must allow authorized users to search and reason over any supported field or relationship without being restricted to preconfigured questions, fixed reports, or hardcoded query templates.

The rebuilt system must be lighter, more maintainable, more predictable, and less over-engineered than the legacy application while preserving broad query capability.

## Non-negotiable principles

- The product has one installation and one organization. Multi-tenancy and organization abstractions are removed.
- `Profile` is the center of the domain. The legacy generic `Record` model must not return.
- Only natural persons are in the initial canonical scope.
- PostgreSQL is the canonical source of truth.
- The application must support approximately 88,000 existing profiles without artificial product limits.
- The AI layer is read-only. It may search, plan, combine, explain, and analyze, but it may not create, edit, delete, link, unlink, or merge canonical data.
- Neither users nor models may provide executable SQL, physical table names, or unrestricted physical column names.
- Flexible querying belongs to typed Search and QueryPlan contracts, not to raw SQL or GraphQL.
- Processing, filtering, candidate generation, joins, and aggregation should remain close to PostgreSQL to reduce Neon egress.
- Dates representing instants are stored in UTC. Civil dates and civil months are not converted into timestamps.
- Relevant filters, sorting, pagination, grouping, tabs, and AI result references are reflected in the URL.
- The initial interface language is `pt-BR`, but user-facing text must remain ready for internationalization.
- Accessibility, responsiveness, security, and performance are requirements, not later polish.
- There is no offline-first mode, news module, or initial email-sending feature.

## Repository boundaries

### Gymkhana-Database

Owns:

- product and application rules;
- persistence and migrations;
- PostgreSQL queries and generated sqlc adapters;
- HTTP API and OpenAPI contracts;
- authentication, authorization, sessions, and audit;
- workers and jobs;
- provider integrations;
- application routes and feature composition;
- product-specific React screens and behavior.

### Gymkhana-Core

Owns reusable Go logic that is independent of HTTP, PostgreSQL, queues, cloud providers, and UI, such as:

- normalization;
- civil date primitives;
- canonicalization;
- fingerprints;
- pure matching or solver algorithms with demonstrated reuse.

It must not become a repository of speculative generic abstractions.

### Gymkhana-UI

Owns reusable visual primitives and components with a proven reusable contract. Product-specific components remain in Gymkhana-Database until reuse is demonstrated.

### Dependency direction

- Database may depend on Core and UI.
- Core and UI do not depend on Database.
- Core and UI do not depend on each other.
- Database consumes exact released versions.
- Permanent dependencies by branch, commit, `replace`, subtree, submodule, or copied source are prohibited.

## Architecture

The approved architecture is a modular monolith with separate process lifecycles:

```text
React SPA
   |
   v
Go HTTP API --------------------> Neon PostgreSQL
   |                                  ^
   |                                  |
   +----------------------------> Cloudflare R2
   |
   +----------------------------> approved external providers

Go worker ----------------------> PostgreSQL / River / providers
Migration command --------------> PostgreSQL
```

Responsibilities:

- the SPA owns presentation, navigation, forms, grids, Search, AI Chat, and administration UX;
- the API owns HTTP, authentication, authorization, contracts, module composition, and synchronous operations;
- the worker owns real asynchronous jobs, imports, matching, OCR, and housekeeping when enabled;
- migrations run explicitly and separately from API startup;
- `cmd` packages contain bootstrap and lifecycle only, not business rules.

The system is not decomposed into microservices. Shared transactions, relationships, deployment simplicity, debugging, and the size of the product do not justify distributed-system overhead.

## Approved technology baseline

### Backend and data

- Go 1.26 with toolchain Go 1.26.5
- standard library `net/http`
- pgx v5
- PostgreSQL 17-compatible schema
- Neon managed PostgreSQL
- explicit parameterized SQL
- sqlc for deterministic generated persistence types
- Tern v2 for ordered SQL migrations
- River OSS when real durable jobs are needed
- REST JSON
- OpenAPI 3.1
- oapi-codegen strict server contracts
- independent compiled binaries for API, worker, and migration tools

### Frontend

- React 19
- TypeScript baseline currently represented by repository manifests; upgrades must pass the complete frontend gate
- Vite 8
- Node.js 24 LTS
- pnpm 11
- TanStack Router
- TanStack Query
- TanStack Table
- TanStack Virtual when needed for large grids
- TanStack Form when adopted by a feature
- Valibot for frontend boundary validation when applicable
- generated TypeScript contracts from OpenAPI
- Gymkhana-UI private package
- Lucide icons

### Infrastructure

- Neon for PostgreSQL
- Vercel for the static SPA
- Cloud Run Service for the Go API when production deployment is active
- Cloud Run Job for worker execution
- Cloud Scheduler for recovery and housekeeping
- Cloudflare R2 for private attachments
- Cloudflare Access only as an optional external layer

### Quality tooling

- gofmt
- Go vet
- Go race tests
- Staticcheck
- govulncheck
- Oxlint
- Oxfmt
- Vitest
- TypeScript type checking
- production builds
- OSV scanning and dependency review
- deterministic OpenAPI and sqlc generation

## Rejected architectural defaults

Do not introduce the following without a new explicit decision supported by a concrete requirement:

- microservices;
- a parallel Node.js backend;
- server-side rendering;
- GraphQL;
- an ORM;
- MongoDB or Firebase as the canonical database;
- Redis or RabbitMQ;
- Redux or Zustand as a default global store;
- Ant Design, Material UI, or shadcn as the main component system;
- Railway;
- OpenTelemetry by default;
- an agent framework as a mandatory AI architecture;
- MinIO or any local service with no current use;
- infrastructure created only to imitate a possible future architecture.

## Backend design rules

- Keep domain logic independent of PostgreSQL, HTTP, provider clients, queues, and UI.
- Use feature packages when they represent real domain boundaries. Do not force a generic controller/service/repository template everywhere.
- Use explicit SQL and inspect the query that PostgreSQL will execute.
- Do not introduce an ORM beside sqlc and pgx.
- Generated files are versioned and never edited manually.
- OpenAPI and sqlc generators must be deterministic; drift is a failure.
- Dynamic SQL exists only in approved Search and Query Engine compiler boundaries.
- Migrations are forward-first, ordered, reviewable, and never run silently during API startup.
- External calls require bounded timeouts and safe error handling.
- Logs are structured and must not contain secrets or sensitive provider payloads.

## Frontend design rules

Each state category has one primary owner:

- TanStack Router owns URL and navigable state;
- TanStack Query owns server state and cache;
- TanStack Form owns form lifecycle where used;
- TanStack Table and Virtual own grids and large collections;
- React owns transient local presentation state;
- the backend owns persisted preferences and authoritative rules.

Additional rules:

- do not fetch directly inside visual components;
- generated API types are the contract boundary;
- masks and display formatting belong at the frontend boundary;
- the backend receives canonical values;
- screens handle loading, empty, validation error, permission error, conflict, failure, and success states;
- mobile remains useful but does not imitate dense spreadsheet editing;
- accessibility includes keyboard operation and screen-reader semantics;
- external component libraries may inspire behavior but must not become the visual foundation.

## Authentication and authorization

The application login provider is Google OpenID Connect, not GitHub OAuth.

Rules:

- there are no local passwords;
- `app_users` is the authoritative email allowlist;
- the email stored in `app_users` is normalized to lowercase;
- OAuth never creates arbitrary application users;
- the Google email must be verified;
- the immutable Google `sub` is bound on the first successful login to the pre-provisioned email row;
- later logins must match both the stored email and bound subject;
- inactive users remain denied;
- roles are `MEMBER`, `ADMIN`, and `SUPERADMIN`;
- exactly one active SUPERADMIN must exist;
- authorization is centralized through permissions and capabilities;
- ADMIN cannot alter itself or the SUPERADMIN through generic administration;
- role or active-state changes revoke affected sessions;
- application sessions are opaque, revocable, server-side, and last 24 hours;
- only SHA-256 session token hashes are stored;
- cookies are HttpOnly, SameSite=Lax, host-only, and Secure outside local/test;
- Cloudflare Access may be added outside the application but never replaces application authorization.

Legacy GitHub names that remain in compatibility code are migration debt, not the desired contract. New code, environment variables, SQL, tests, and UI text must use Google subject and email terminology.

## Audit and privacy

Audit is separate from technical logging.

Audit applicable events such as:

- login, logout, denial, and authentication failure;
- user administration and access changes;
- session revocation;
- creation, editing, duplication, deletion, import, merge, and destructive operations.

Audit events include request correlation and actor identity when available. They must not store secrets, OAuth codes, tokens, signed URLs, raw SQL, stack traces, or sensitive provider payloads.

## Canonical data model

### Profile-first model

A Profile represents one natural person and is the canonical owner of personal data.

Core rules:

- UUID identifier, with UUIDv7 preferred for newly created entities when supported;
- full name required;
- social name optional;
- CPF optional and normalized to 11 digits when present;
- CPF is not a rigid global unique key because possible duplicates require human review;
- at most one canonical email, normalized to lowercase;
- at most one mobile phone and one landline/other phone;
- at most one structured address;
- optional notes;
- optimistic concurrency through `version` where applicable;
- definitive deletion requires explicit confirmation and audit.

The old generic `Record` table must not exist in the canonical public schema.

### Migrated profile details

Mapped legacy personal information belongs to the Profile aggregate, including one-to-one detail storage where separating the wide fields is technically useful. This includes mapped extra personal attributes, vehicles, and collections.

`custom data` is not a dumping ground for mapped legacy values. It is reserved for genuinely additional fields or future typed information outside the approved canonical mapping.

The canonical public schema must not retain migration-only `source_ref` or `address_raw` columns.

### Address migration

Normalize only information actually present in the legacy value. Do not invent missing components.

Examples:

- a value equivalent to `Portão, RS` supplies city and state only;
- spelling and naming variants should be analyzed and normalized when confidence is sufficient;
- ambiguous values must remain conservatively represented rather than fabricated.

### Documents

- every document belongs to a Profile;
- documents never exist orphaned;
- the model uses a common document table plus type-specific details and approved typed custom fields;
- document types have stable technical keys and editable display labels;
- uniqueness policy is `NONE`, `PER_PROFILE`, or `GLOBAL_BY_TYPE`;
- values preserve leading zeroes and alphanumeric information;
- old, replaced, or expired documents may be retained;
- functional availability is initially `in use` or `available` where applicable;
- barcode support is not mandatory.

### Bills and proofs

- every bill belongs to a Profile;
- printed/original values are preserved independently from canonical Profile data;
- changing printed bill data does not silently alter the Profile;
- civil month uses `YYYY-MM`;
- money uses decimal, never binary floating point;
- typed custom fields may extend a bill type.

### Current use

For documents and bills, the initial product stores only the current use relationship:

- at most one current holder/use;
- returning or unlinking removes the current relationship;
- the item becomes available again;
- do not create a fictitious incomplete historical timeline;
- changes are transactional, authorized, and audited.

### Custom data

Custom fields are typed and may target Profile, document type, bill type, or a custom entity type.

Rules:

- known validation, normalization, persistence, and rendering per type;
- no arbitrary executable scripts;
- no arbitrary formulas;
- no free JSON as the primary source of truth;
- definition changes preserve data or require explicit migration;
- changes are auditable;
- custom entities must not replace an existing canonical module.

Objects remain a separate future module and must not be hidden inside custom data without a new decision.

## Data grids and CRUD

- tables support column filters, sorting, and pagination reflected in the URL;
- page sizes generally range from 100 to 1000 when the module supports it;
- desktop may support spreadsheet-like editing for approved cells;
- cell edits save on blur;
- optimistic concurrency conflicts are visible and never overwrite silently;
- new rows are not created directly in the grid;
- duplication is an explicit row action when supported;
- mobile uses readable tables and dedicated forms rather than card-only or spreadsheet editing;
- definitive and bulk deletion requires explicit confirmation;
- the approved reinforced bulk confirmation text is `Confirmar`.

Fields such as technical timestamps, internal version, death date, and notes are not automatically exposed in grids or exports unless the product explicitly requires them.

## Search

Search must support any authorized field in any supported module, including names, address fragments, document values, account values, custom fields, and arbitrary character sequences.

Requirements:

- multi-parameter and cross-relation search;
- permission-filtered modules, fields, relationships, and results;
- no free physical schema identifiers from the client;
- deterministic pagination and ordering;
- results explain why and where a match occurred when applicable.

## Query Engine

The Query Engine is the controlled general-purpose relational query layer.

- a permission-filtered runtime catalog describes logical entities, fields, operators, and relations;
- queries use a typed `QueryPlan` and validated AST;
- one logical QueryPlan contract should serve supported contexts unless a genuinely distinct security or execution boundary requires a separate type;
- plans may express projection, filters, relations, sorting, pagination, grouping, aggregation, sets, patterns, and combinations;
- the trusted backend compiles the validated plan to an execution plan and parameterized SQL;
- users and models never submit executable SQL;
- cost, depth, cardinality, time, and result-size limits protect service operation without reducing semantic coverage to fixed templates;
- result sets preserve references, origin, fields, and evidence needed for explanation.

## Duplicate analysis and merge

- persistent duplicate analysis initially targets Profiles;
- analysis runs on demand, not as a mandatory periodic full scan;
- PostgreSQL performs candidate generation and aggregation where practical;
- evidence explains why candidates are similar;
- uncertain cases require human review;
- merge is never automatic;
- merge is explicit, previewed, transactional, authorized, and audited;
- precedence and dependency movement rules must be explicit before execution;
- false duplicate signals must not block ordinary work unnecessarily.

## XLSX import and export

### Import

- XLSX only in the initial scope;
- staging before canonical writes;
- upload, sheet selection, mapping, validation, decisions, preview, batch execution, and report;
- one controlled sheet per import flow;
- idempotent execution;
- ambiguity requires a human decision;
- no automatic Profile merge;
- temporary source data is removed after the approved operational retention period.

### Export

- exports initially represent complete authorized module tables rather than arbitrary hidden subsets;
- Profile export includes the approved related profile data required by the product;
- export respects permissions;
- presentation formatting does not change canonical values;
- each administrator sees only their own export jobs unless a higher permission explicitly allows otherwise.

## Google Forms

Google Forms reuses the import staging and decision pipeline rather than creating an incompatible parallel ingestion system.

- separate OAuth client from application login;
- read-only Forms scopes;
- owner-scoped connections and forms;
- protected provider tokens;
- no provider secrets or payloads in responses, logs, or audit;
- no email sending.

## AI Chat

AI Chat is a private read-only consultant over Search and Query Engine tools.

It must handle simple through complex requests, including:

- names containing a fragment;
- arbitrary sequences in any document or field;
- letters, numbers, binary digits, and constructible character patterns;
- people matching a name and an address condition;
- combinations across profiles, addresses, documents, bills, and custom data;
- counts, grouping, intersections, differences, and candidate combinations;
- complete copied gymkhana task statements.

Additional rules:

- interpretation is semantic and must not depend on exact trigger words;
- follow-ups such as “desses” and “agora somente” refine the active context or result set;
- responses may combine explanation and generic tables;
- results remain traceable to fields and evidence;
- saved queries execute only when the user explicitly clicks them;
- saved queries never intercept a manually typed question;
- the system does not create saved queries automatically;
- a model may revise a failed plan only within controlled limits;
- no agent framework may replace the product's typed tools and security contracts.

## Complex gymkhana tasks

A pasted task may require people, documents, bills, accounts, and extra data simultaneously.

The system should:

- interpret natural language without exact-word requirements;
- convert requirements to explicit logical bindings;
- build candidate sets;
- combine candidates with solver and pruning techniques;
- support character sequences, allowed characters, binary digits, letters, numbers, and leading zeroes;
- support constraints across entities;
- explain which requirements each candidate satisfies and where it fails;
- avoid opaque answers that cannot be traced back to data.

## OCR and multimodal processing

- OCR produces suggestions, not canonical writes;
- every suggestion includes evidence and confidence when available;
- human review is mandatory before applying values;
- extracted data never updates Profile, document, bill, or custom data automatically;
- permissions, quotas, privacy, and attachment retention apply.

## Attachments

- attachments are private and stored in Cloudflare R2;
- PostgreSQL stores metadata and ownership, not file bodies;
- upload and download use short-lived signed URLs;
- confirmation verifies the expected object, MIME, signature, size, and hash;
- credentials and signed URLs never appear in logs or audit;
- deleted attachments remain in trash for seven days before final removal;
- cleanup and recovery are idempotent and observable.

## API and security

- API base is `/api/v1`;
- health endpoints are `/health/live` and `/health/ready`;
- GET requests have no side effects;
- lists are paginated;
- projection is allowlisted;
- critical operations use idempotency keys when appropriate;
- stable error envelopes include code, safe `pt-BR` message, request ID, controlled details, and field errors;
- browser state changes validate the exact approved origin;
- credentialed CORS is restricted to the configured application origin;
- rate limits and quotas protect Search, AI, OCR, upload, and imports;
- responses never expose secrets, tokens, signed URLs, SQL, stack traces, or provider payloads.

## Database and migration safety

- production and shared development use the dedicated Neon database `gymkhana_rebuild`;
- legacy data remains isolated in the legacy database/schema and is not a runtime dependency;
- never point the rebuilt application at legacy `neondb`;
- migrations are explicit and reviewed;
- destructive verification runs against local PostgreSQL or a disposable Neon branch;
- do not run rollback or reset workflows against the canonical Neon database;
- migrations and integration tests must cover empty database creation and upgrade behavior;
- seeds remain separate from migrations;
- minimize unnecessary data transfer from Neon.

## Local and CI workflow

- one implementation PR per active workstream;
- keep it draft while iterating;
- develop, format, generate, and test locally;
- do not use GitHub Actions as a remote development loop;
- avoid temporary PRs, dummy commits, or repeated workflow forcing;
- run focused checks while coding and the complete applicable gate before review;
- hosted CI is a final integration gate;
- a trusted self-hosted runner may be selected through the repository runner variable;
- generated and migration changes require local verification before the PR becomes ready.

## Code and documentation conventions

- source code, commit messages, PRs, workflow names, and technical documentation are written in English;
- user-facing product text is `pt-BR` and prepared for i18n;
- avoid unnecessary dependencies;
- pin private package releases exactly;
- preserve existing architecture unless a requirement justifies change;
- solve the smallest real problem without narrowing the product's general query capability;
- update tests with behavior changes;
- do not edit generated files manually;
- do not claim validation that was not actually executed.

## AI agent operating protocol

Before implementing:

1. inspect this document and `docs/ORCHESTRATION.md`;
2. inspect the current branch, current implementation, master tracker, and relevant issue;
3. identify affected domain invariants and generated contracts;
4. check whether the change belongs in Database, Core, or UI;
5. avoid creating a new abstraction unless the current feature proves the need.

During implementation:

1. keep one coherent workstream and a small intentional commit set;
2. use explicit types and stable errors;
3. preserve authorization at every boundary;
4. keep dynamic query capability inside typed Search/Query Engine contracts;
5. update migrations, source SQL, generated outputs, API contracts, tests, and documentation together when applicable;
6. never use production or canonical Neon data for destructive tests.

Before declaring completion:

1. format changed code;
2. run targeted unit tests;
3. run generated-contract checks when SQL or OpenAPI changed;
4. run migration and PostgreSQL integration validation when persistence changed;
5. run frontend typecheck, lint, tests, and production build when frontend changed;
6. report exactly what was and was not executed;
7. keep the PR draft if any required gate remains unverified.

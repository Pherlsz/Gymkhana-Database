# Changelog

All notable changes to Gymkhana Database are documented here.

The format follows Keep a Changelog and the project uses Semantic Versioning once releases begin.

## [Unreleased]

### Added

- M4 milestone acceptance coverage for Profile ownership, historical states, printed bill data, civil money, current-use boundaries, URL state, semantic controls, and optimistic-conflict feedback.
- Milestone-wide validation of document/bill persistence, protected APIs, Profile-integrated UI, deterministic generation, and security gates.
- Profile-integrated document and bill management with URL-backed sections, filters, sorting, pagination, selection, and forms.
- Responsive record cards, supported desktop inline editing, type administration, current-use workflows, duplication, permanent deletion, and frontend acceptance coverage.
- Protected bill-type administration and Profile-owned bill CRUD API with centralized role permissions.
- Bill filtering, sorting, pagination, structured validation, optimistic conflicts, duplication, and explicit permanent-delete confirmation.
- Printed holder/address/reference preservation with civil competence, canonical decimal amounts, and ISO currency validation.
- Current-use assignment/replacement/return only for bill types that explicitly support it.
- Durable bill mutation audit events, OpenAPI 0.6.0 contracts, generated clients, and service/HTTP/PostgreSQL coverage.
- Protected document-type administration and Profile-owned document CRUD API with centralized role permissions.
- Document filtering, sorting, pagination, structured validation, optimistic conflicts, duplication, and explicit permanent-delete confirmation.
- Current-use assignment, holder replacement, and return endpoints without creating a historical usage timeline.
- Durable document mutation audit events correlated by actor and request ID while retaining deleted identifiers.
- OpenAPI 0.5.0 document contracts with regenerated Go and TypeScript clients.
- Service, HTTP, and PostgreSQL audit-adapter coverage for document permissions, errors, current use, and audit metadata.
- Canonical Profile-owned document and bill persistence with FK-enforced ownership, administrable types, and optimistic versioning.
- Controlled document uniqueness policies, optional regex/date requirements, leading-zero-safe identifiers, and old-record states.
- Bill persistence preserving printed holder, address, reference, civil competence, decimal money, and currency values.
- Transactional current-use relations for documents and supported bills without fabricating a historical timeline.
- Deterministic sqlc type/record CRUD, filtering, sorting, pagination, duplication, deletion, and current-use queries.
- Document and bill domain/PostgreSQL store tests for validation, immutable rules, optimistic conflicts, decimal round-trips, and current use.
- Responsive Profile management route with URL-backed filters, sorting, pagination, selection, and panel state.
- Canonical Profile create, detail, edit, duplicate, and permanent-delete interface for authenticated users.
- Desktop TanStack Table grid with supported on-blur inline editing and mobile card-based reading.
- Profile frontend acceptance coverage for authentication, navigation, URL state, listing, and inline editing.
- Protected Profile CRUD API with deterministic filtering, sorting, pagination, optimistic concurrency, and explicit permanent-delete confirmation.
- Profile mutation audit events correlated by actor and request ID while retaining deleted Profile identifiers.
- Structured Profile field-validation errors in the stable API error envelope.
- Canonical physical-person Profile schema with explicit identity, contact, address, notes, timestamps, and optimistic version columns.
- Profile domain normalization and validation backed by the pinned Gymkhana Core contracts.
- Deterministic sqlc create, get, count, list, update, duplicate, and permanent-delete queries.
- PostgreSQL Profile adapter with stable not-found and optimistic-conflict errors.
- GitHub OAuth sign-in with state validation and explicit allowed-login enforcement.
- Protected session and logout endpoints backed by revocable 24-hour opaque sessions.
- Fail-closed authentication configuration for staging and production.
- Authentication-aware web shell with login, current-user, role, and sign-out states.
- Authentication persistence foundations for users, opaque sessions, and essential audit events.
- Stable role, audit-event, and 24-hour session lifecycle contracts.
- Centralized `MEMBER`, `ADMIN`, and protected `SUPERADMIN` authorization.
- Minimal user-administration API and responsive interface with optimistic access updates.
- Automatic per-user session revocation after effective role or active-status changes.
- Credentialed CORS and exact-origin validation derived from `AUTH_APPLICATION_URL`.
- Complete authentication and administration audit outcomes with observable persistence failures.
- Safe `make check-config` environment preflight and authentication operations runbook.
- Synthetic end-to-end M2 authentication, administration, audit, and revocation validation.
- Typed environment, log-level, request-body limit, and shutdown configuration validation.
- Stable JSON API error envelopes, request boundaries, and request-ID propagation.
- Generated-contract-based frontend API helpers with structured error metadata.
- Integration tests for Gymkhana Core normalization and civil-time contracts.
- ThemeProvider, AppShell, Page, feedback, layout, and status foundations in the real web shell.

### Changed

- Pinned TanStack Query `5.101.2`, TanStack Router `1.170.18`, and TanStack Table `8.21.3` for the Profile interface.
- Pinned Gymkhana Core `v0.2.1` and Gymkhana UI `0.3.0`.
- Updated `openapi-typescript` from `7.10.1` to `7.13.0`.
- Added authenticated private Core access to CI, security, OpenAPI, deployment, and container builds.
- Expanded audit correlation to invalid OAuth callbacks, failed sign-out, administration access, access conflicts, session revocation, Profile mutations, and document mutations/current-use changes.

## [0.0.0] - 2026-07-13

### Added

- M0 repository bootstrap for the Go API, worker, migration command, React web app, PostgreSQL development service, OpenAPI, sqlc, CI, security, and deployment build foundations.

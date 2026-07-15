# Changelog

All notable changes to Gymkhana Database are documented here.

The format follows Keep a Changelog and the project uses Semantic Versioning once releases begin.

## [Unreleased]

### Added

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

- Pinned Gymkhana Core `v0.2.1` and Gymkhana UI `0.3.0`.
- Updated `openapi-typescript` from `7.10.1` to `7.13.0`.
- Added authenticated private Core access to CI, security, OpenAPI, deployment, and container builds.
- Expanded audit correlation to invalid OAuth callbacks, failed sign-out, administration access, access conflicts, and session revocation.

## [0.0.0] - 2026-07-13

### Added

- M0 repository bootstrap for the Go API, worker, migration command, React web app, PostgreSQL development service, OpenAPI, sqlc, CI, security, and deployment build foundations.

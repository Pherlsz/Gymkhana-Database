# Changelog

All notable changes to Gymkhana Database are documented here.

The format follows Keep a Changelog and the project uses Semantic Versioning once releases begin.

## [Unreleased]

### Added

- Authentication persistence foundations for users, opaque sessions, and essential audit events.
- Stable role, audit-event, and 24-hour session lifecycle contracts.
- Typed environment, log-level, request-body limit, and shutdown configuration validation.
- Stable JSON API error envelopes, request boundaries, and request-ID propagation.
- Generated-contract-based frontend API helpers with structured error metadata.
- Integration tests for Gymkhana Core normalization and civil-time contracts.
- ThemeProvider, AppShell, Page, feedback, layout, and status foundations in the real web shell.

### Changed

- Pinned Gymkhana Core `v0.2.1` and Gymkhana UI `0.3.0`.
- Updated `openapi-typescript` from `7.10.1` to `7.13.0`.
- Added authenticated private Core access to CI, security, OpenAPI, deployment, and container builds.

## [0.0.0] - 2026-07-13

### Added

- M0 repository bootstrap for the Go API, worker, migration command, React web app, PostgreSQL development service, OpenAPI, sqlc, CI, security, and deployment build foundations.

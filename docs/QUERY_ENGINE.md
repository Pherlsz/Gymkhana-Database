# Query Engine v1

The M10 Query Engine exposes a permission-filtered logical catalog and accepts only a versioned `QueryPlan`. Clients cannot submit SQL, physical schema identifiers, free relation paths, mutations, aggregation, grouping, sets, or patterns.

## Supported v1 plan

`QueryPlan` v1 contains one root entity, up to 20 projections, nested `AND`/`OR`/`NOT` filters, allowlisted relation filters, up to three deterministic sorts, and a maximum of 500 rows. Logical identifiers must be present in the freshly loaded server catalog. Predicate values are always PostgreSQL parameters; only server-owned expressions and relation fragments can become SQL.

The public catalog is rebuilt from current authorization and active custom definitions on every service operation. Its SHA-256 version lets the backend reject stale plans. The browser stores the editable plan only in a versioned, structurally validated URL value capped at 16 KiB. It never places result rows in URL, local storage, or session storage. When the catalog changes, the client preserves still-authorized nodes and reports removed nodes.

Grouping, aggregation, sets, patterns, advanced combinations, and Gymkhana task solving are explicit M14 extensions. They are not accepted as v1 nodes.

## Execution and result boundaries

- The current database user, activity, role, entity, field, operator, and relation permissions are rechecked for catalog, validation, execution, and result reads.
- The compiler adds a stable root-ID tie-breaker to every sort and binds the row limit as a parameter.
- Execution runs in a PostgreSQL read-only transaction with a three-second default statement timeout and request cancellation propagation.
- Cost, filter-node, depth, relation-depth, predicate-value, projection, sort, row, concurrency, persistent per-user rate, and page limits fail closed.
- An explicit 8–128 character idempotency key prevents duplicate materialization and conflicts if reused for a different fingerprint.
- Completed results are immutable, owner-scoped snapshots. Position-based offset pages are stable because the snapshot cannot change; they therefore cannot skip or duplicate rows while being read.
- Typed cells and logical entity provenance are retained for one hour by default. Expired results return a stable safe code and are deleted in bounded batches; owner activity can also remove expired records opportunistically.
- Failed and cancelled executions transition atomically and expose no partial result rows. Audits contain only logical event types, IDs, counts, request IDs, outcomes, and plan fingerprints—not predicate values or result cells.

## HTTP surface

- `GET /api/v1/query/catalog`
- `POST /api/v1/query/validate`
- `POST /api/v1/query/executions`
- `GET /api/v1/query/executions/{execution_id}/result`

All endpoints require the existing application session. Error responses use the shared request-ID envelope and stable codes for invalid plans, validation, stale catalogs, authorization, cost, rate, conflict, cancellation, timeout, expiry, and absence. Internal compiler/database details are logged only on unexpected failures and are never returned to the client.

## Operations and rollback

No external credentials or infrastructure are required. Query Engine uses the existing API, SPA, PostgreSQL pool, and migrations.

The migration is `014_query_engine.sql`. Its down section deletes the Query Engine metadata marker and drops audit, result-cell, result-row, result-column, execution, and rate-limit tables in dependency order. Rolling back permanently removes derived query snapshots and audits but does not mutate canonical Profiles, documents, bills, attachments, or custom data.

Before release, run the migration upgrade/down checks, PostgreSQL integration suite, backend quality/race/static analysis, frontend checks and build, OpenAPI drift generation, and Security workflow.

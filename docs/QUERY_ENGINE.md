# Query Engine

The approved Query Engine surface is [`docs/ORCHESTRATION.md`](ORCHESTRATION.md) §14: a permission-filtered logical catalog and a versioned `QueryPlan` that may express projection, filters, relations, sorting, pagination, grouping, aggregation, sets, patterns, and combinations. Clients never submit SQL, physical schema identifiers, or mutations. There is no user-facing Query screen; the Assistente is the only product caller.

This runbook is the operational contract of the **live v1 subset**. v1 does not yet accept grouping, aggregation, sets, or patterns. Those nodes remain approved product capability and land as later versions of the same `QueryPlan`. v1 is not a second engine and does not retract §14.

Gymkhana tasks that consume Query results are documented in [`TASKS.md`](TASKS.md).

## Supported v1 plan

`QueryPlan` v1 contains one root entity, up to 20 projections, nested `AND`/`OR`/`NOT` filters, allowlisted relation filters, up to three deterministic sorts, and a maximum of 500 rows. Logical identifiers must be present in the freshly loaded server catalog. Predicate values are always PostgreSQL parameters; only server-owned expressions and relation fragments can become SQL.

The public catalog is rebuilt from current authorization and active custom definitions on every service operation. Its SHA-256 version lets the backend reject stale plans. There is no Query editor. An Assistente recorte may enter a table link as a compiled plan (Orchestration §12.1), not as text for the model to reinterpret and not as result rows in the URL. Clients must not place result rows in local storage or session storage.

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

No external credentials or infrastructure are required. Query Engine uses the existing API, PostgreSQL pool, and migrations. The Assistente is the product caller; there is no `/query` destination.

The migration is `014_query_engine.sql`. Its down section deletes the Query Engine metadata marker and drops audit, result-cell, result-row, result-column, execution, and rate-limit tables in dependency order. Rolling back permanently removes derived query snapshots and audits but does not mutate canonical Profiles, documents, bills, attachments, or custom data.

Before release, run the migration upgrade/down checks, PostgreSQL integration suite, backend quality/race/static analysis, frontend checks and build, OpenAPI drift generation, and Security workflow.

## Later QueryPlan versions

Later versions of the same plan support bounded grouping, aggregates, `HAVING`, application-owned pattern grammars, set operations, and cross-role combinations. Logical catalog keys are resolved server-side; user values are always sent as bound parameters. v1 plans and their execution behavior remain unchanged.

Advanced results retain logical lineage so authorization can be re-evaluated when a stored result is read. Aggregate and combination outputs use synthetic logical keys but must still resolve to authorized source entities and fields.

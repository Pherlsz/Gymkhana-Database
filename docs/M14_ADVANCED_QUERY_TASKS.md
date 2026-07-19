# M14 — Advanced Query Engine and Gymkhana Tasks

M14 extends the single versioned `QueryPlan` contract with a safe v2 execution path and adds a reviewed, durable workflow for complete Gymkhana tasks.

## QueryPlan v2

The v2 plan supports bounded grouping, aggregates, `HAVING`, application-owned pattern grammars, set operations, and cross-role combinations. Logical catalog keys are resolved server-side; user values are always sent as bound parameters. v1 plans and their execution behavior remain unchanged.

Advanced results retain logical lineage so authorization can be re-evaluated when a stored result is read. Aggregate and combination outputs use synthetic logical keys but must still resolve to authorized source entities and fields.

## Task workflow

1. A task is interpreted or entered as a typed `TaskSpec` in `PROPOSED` state.
2. A user reviews the roles, bindings, requirements, constraints, ambiguities, and solution limits.
3. Only a `REVIEWED` specification can start a durable job.
4. The worker reloads the persisted job, actor, catalog, and specification; candidate queries are reauthorized before execution.
5. The deterministic solver stores bounded compositions and machine-checkable evidence.

The worker receives only the persisted job identifier. Retry and cancellation are idempotent, terminal state and results are committed transactionally, and SSE events use monotonic sequences that support `Last-Event-ID` resumption.

## Operational states

Jobs use `QUEUED`, `RUNNING`, `COMPLETED`, `INCOMPLETE`, `FAILED`, and `CANCELLED`.

`INCOMPLETE` is a valid terminal result when a configured branch, candidate, evidence, duration, or result limit is reached. It must not be reported as `COMPLETED`, and it may still contain verified compositions found before the limit.

## Security boundaries

- Query execution is read-only and permission-derived.
- Arbitrary SQL and arbitrary regular expressions are not accepted.
- Hidden catalog entries cannot be introduced through task text or JSON.
- Stored jobs and results are owner-scoped and reauthorized on reads.
- The Chat integration can inspect task jobs and results but cannot create, review, execute, retry, or cancel them.
- External semantic interpretation remains disabled until a provider, model, API contract, and privacy terms are explicitly approved.

## Verification

Before merging M14, the branch must pass the repository's official Backend, Frontend, OpenAPI, Migrations, and Security workflows. Generated Go and TypeScript clients must match `api/tasks.openapi.yaml`, and temporary diagnostic workflows must not remain in the final diff.

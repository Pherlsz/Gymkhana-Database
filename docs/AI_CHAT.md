# AI Chat operations runbook

The product surface is the **Assistente** (tables, Search, wide panel) defined in [`ORCHESTRATION.md`](ORCHESTRATION.md) §18. HTTP, capability `CHAT`, and this runbook still use the `/api/v1/chat` engine: a private, permission-aware, read-only interface over Search and Query Engine. It has no arbitrary database, SQL, code, HTTP, or mutation tool. The model never authors SQL; it calls typed tools and answers from retrieved catalog, prior result sets, and tool evidence. This runbook describes configuration, privacy boundary, limits, recovery, smoke test, and rollback.

## Activation boundary

The feature is disabled by default. The repository currently contains only a deterministic fake model adapter, and configuration accepts that adapter only in `APP_ENV=test`. Staging and production therefore fail closed if `AI_CHAT_ENABLED=true`.

Production activation requires all of the following owner decisions and implementation work:

1. Administração: a shared provider key for Assistente and model OCR, encrypted at rest; SUPERADMIN/ADMIN manage it; EXTERNAL with `CHAT` may use it but never sees the secret; never store that key in the repo;
2. select the default conversation/result retention period;
3. implement and review a narrow production adapter for the existing `ModelClient` port that reads the shared Administração key;
4. extend fail-closed configuration so Assistente is unavailable to everyone without a valid shared key, and unavailable to EXTERNAL without `CHAT`;
5. wire Query v1/v2 and gymkhana-task interpretation as Assistente tools (no user Query, Tasks, or Chat screens);
6. validate the complete flow in staging.

Do not reuse `fake` as a production adapter, invent a global API key in lokeys as the only production path, or choose retention by assumption.

## Configuration contract

| Variable            | Contract                                                                          |
| ------------------- | --------------------------------------------------------------------------------- |
| `AI_CHAT_ENABLED`   | Explicit switch; defaults to `false`                                              |
| `AI_CHAT_PROVIDER`  | Required only when enabled; currently only `fake` in `APP_ENV=test`               |
| `AI_CHAT_MODEL`     | Required only when enabled; nonempty model identifier, at most 120 characters     |
| `AI_CHAT_RETENTION` | Required only when enabled; Go duration from `1h` through `8760h` with no default |

Chat also requires enabled application authentication, PostgreSQL, Search, and Query Engine. Enabling it with any missing dependency stops startup. The capability endpoint remains authenticated while disabled and reports only `enabled=false` plus safe fixed limits; every lifecycle route returns the stable `chat_unavailable` contract.

Deterministic test-only configuration:

```dotenv
APP_ENV=test
AI_CHAT_ENABLED=true
AI_CHAT_PROVIDER=fake
AI_CHAT_MODEL=deterministic-v1
AI_CHAT_RETENTION=24h
```

The normal authentication and database variables are still required. These values are for synthetic testing, not a production recommendation.

## Read-only and authorization boundary

The application owns a fixed registry containing only:

- permission-filtered logical catalog inspection;
- Search execution;
- QueryPlan v1 and later versions of the same engine;
- gymkhana-task interpretation (requirement lists → per-step results);
- reopening an existing owner-scoped result reference.

Tool schemas reject unknown fields. Model arguments cannot select a new tool, increase a server limit, supply SQL, use physical schema names, or introduce a mutation. Search and Query reauthorize the current user during catalog access, execution, refinement, pagination, and result serialization. Result references are owner- and thread-scoped and are reloaded through the underlying service rather than trusting cached rows.

All user messages, prior assistant messages, and database/tool values are marked as untrusted model input. Instruction-like text in those values grants no permission and cannot create a tool call. Saved queries are not selected or executed from manually typed text.

## Gymkhana tasks

Gymkhana tasks are Assistente capabilities, not a user module. A task is a list of requirements, often a full proof pasted as free text. The model may split it into steps and must return authorized Search/Query results for each step. It still cannot mutate canonical data, invent tools, or bypass the Search/Query allowlist. Operational detail lives in [`TASKS.md`](TASKS.md).

## Persistent data and privacy

PostgreSQL stores:

- private owner-scoped thread metadata and user/assistant messages;
- normalized run and tool-step states, usage/counts, fingerprints, cancellation, and retry linkage;
- normalized application SSE events with monotonic sequence IDs;
- logical Search/Query requests, Query execution references, result fingerprints, labels, and expiry;
- persistent per-user rate/usage windows;
- audits containing logical IDs, tool kind, counts, outcome, stable error code, and request ID.

It does not store provider credentials, raw provider requests/responses, provider diagnostics, unrestricted tool rows, canonical business-data copies, SQL, physical execution plans, signed URLs, or arbitrary HTTP/code output. Search evidence is reopened from its logical request and Query evidence references its permission-filtered Query execution.

Application logs and audits must never include prompts, message bodies, text deltas, tool rows, provider payloads, credentials, or database error detail. Unexpected provider/tool failures are mapped to stable public codes before logging or auditing.

## Enforced limits

| Boundary                        |                                                  Limit |
| ------------------------------- | -----------------------------------------------------: |
| Active runs                     |                                           1 per thread |
| New runs                        |             30 per user per one-hour persistent window |
| Model usage                     |           200,000 input + output units per user window |
| Run wall time                   |                                             45 seconds |
| Tool calls                      |                                              8 per run |
| Rows                            |                               100 per tool result/page |
| Projected fields                |                                                     20 |
| Tool/result bytes               |            256 KiB per result and cumulatively per run |
| User or final assistant message |                             20,000 Unicode code points |
| One normalized text delta       |                              1,000 Unicode code points |
| Provider conversation context   | Most recent 30 messages and at most 60,000 code points |

The API also bounds page offsets, body size, identifiers, idempotency keys, reconnect attempts, and SSE batches. Provider-reported negative usage, malformed calls, oversized results, or attempts to exceed limits terminate with a safe deterministic error.

## Lifecycle, streaming, and recovery

A turn is committed with its user message, request fingerprint, idempotency key, initial run state, and `RUN_ACCEPTED` event before background orchestration starts. Replaying the same key returns the existing run only when the fingerprint—including thread, content, active context, and retry target—matches.

SSE emits only persisted normalized events in sequence order. The server polls every 200 ms and sends a heartbeat every 10 seconds. A client reconnects with both `after` and `Last-Event-ID`, ignores already-seen sequence IDs, and performs at most three bounded reconnects. The final assistant message is committed before `RUN_COMPLETED`.

Cancellation is persisted first and then signals the in-process coordinator. Cancellation wins a concurrent completion when its request obtains the run lock first; either way there is exactly one terminal state and event. Partial text stays readable, but no work continues after the orchestrator observes cancellation or a terminal state.

Graceful shutdown cancels the root orchestration context and waits within the configured shutdown timeout. If a process exits abruptly, an active run can no longer be owned by an in-memory coordinator. The cleanup loop runs at startup and once per minute; after the 45-second run deadline plus a 5-second safety grace it atomically:

- marks running tool steps `FAILED` or `CANCELLED`;
- marks the run `FAILED/timeout` or `CANCELLED/cancelled`;
- appends exactly one persisted terminal event;
- records a safe `RUN_RECOVERED` audit count.

The thread is then unblocked and the user can make an explicit retry linked to that terminal run. Recovery never mutates or deletes canonical Search/Query data.

## Retention and cleanup

Every thread receives `retention_expires_at = created_at + AI_CHAT_RETENTION`; result references cannot outlive the thread or their underlying Query execution. Expired threads are deleted in bounded batches of 100. Cascades remove their messages, runs, normalized events, tool steps, and result-reference metadata, while canonical Search/Query/business data remains under its own lifecycle.

Changing retention affects newly created threads; it does not silently rewrite existing expiry timestamps. Explicit thread deletion is denied while a run is active. Treat a migration rollback as destructive for retained conversation history and take a verified backup first.

## Smoke test

Run the deterministic path only in an isolated test environment:

1. apply all migrations to a disposable PostgreSQL database;
2. configure application authentication plus the test-only values above;
3. sign in as an active EXTERNAL user and confirm `/api/v1/chat/capability` reports `enabled=true` and the fixed limits;
4. create a private thread, submit a turn, observe ordered SSE text, and reconnect from a recorded sequence;
5. exercise Search and Query evidence, select/clear active context, and submit a follow-up;
6. cancel a run, retry it explicitly, rename the thread, and delete it;
7. repeat thread, run, SSE, tool, and result-reference reads as another or revoked user and confirm they are denied without content leakage;
8. inspect logs and audits for IDs/counts/codes only, then advance an explicit test clock or expiry and verify cleanup.

The deterministic fake returns a fixed text response when the full API process is used. Typed multi-tool, injection, failure, reconnect, and lifecycle cases are covered by the synthetic backend/HTTP/frontend suites and require no network credential.

## Failure handling and rollback

- `chat_busy`: wait for, cancel, or recover the one active run; do not create a parallel run for the same thread.
- `rate_limited` or `chat_quota_exceeded`: wait for the persistent window or narrow the request. Do not raise limits from client/model input.
- `chat_timeout`: use the explicit retry action after the terminal event; investigate repeated tool latency using safe IDs and timing only.
- `chat_stale_context`: clear the active result and reopen or execute a fresh authorized result.
- `chat_malformed_provider`, `chat_tool_failed`, or `chat_unsafe_result`: keep the provider payload redacted, correlate by request/run ID, and disable Chat if failures repeat.
- Abrupt-process orphan: allow the startup/minute recovery sweep to create its terminal event; never update run rows manually while the service is active.

To disable Chat without deleting retained state, set `AI_CHAT_ENABLED=false` and restart the API. The capability endpoint then reports disabled and all Chat lifecycle routes fail closed. Existing rows remain stored under the same database access controls and expire only when cleanup runs after a later safe reactivation or through an explicitly reviewed maintenance procedure.

Migration `016_ai_chat.sql` has a reversible down section for development validation, but rolling it back permanently removes Chat conversations, results, usage windows, and audits. Production rollback should normally disable the feature and leave the schema in place. Use the down migration only with explicit approval and a verified recovery backup.

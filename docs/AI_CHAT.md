# AI Chat operations runbook

The Assistente lives on the open table (Pessoas, Documentos or Contas). Production uses Gemini only (`AI_CHAT_PROVIDER=google`). `fake` stays in `APP_ENV=test`. There is no second provider, no `/chat` route, and no assistant on Search.

The open sheet lends its page, page size (up to 500) and row click. One plan can return people, documents and bills together. When a turn returns rows, the grid switches to that result. The address keeps the result id (`result`); opening the link recomputes the plan with the visitor's own permission and page. Clearing the id restores the previous sheet. A click opens the row's record without changing the section. A criterion with no catalog field stays in the chat and does not write the address.

HTTP, capability `CHAT`, and this runbook still use the `/api/v1/chat` engine: a private, permission-aware, read-only interface over the Query Engine. It has no arbitrary database, SQL, code, HTTP, or mutation tool. The model never authors SQL. It calls `catalog` and `query`, and may reopen a reference with `result`. The answer does not mention tools, SQL or field keys. This runbook describes configuration, privacy boundary, limits, recovery, smoke test, and rollback.

## Activation boundary

The feature is disabled by default. Two adapters exist: the deterministic `fake` (accepted only in `APP_ENV=test`) and `google` (Gemini through `internal/modelprovider`, an `assistant.ProviderAdapter` from Gymkhana-Core validated with `assistant/adaptertest`). Core keeps provider HTTP payloads out of its module, so the adapter lives here.

With `AI_CHAT_PROVIDER=google` the process starts, but the Assistente stays **fail-closed until an ADMIN/SUPERADMIN stores the shared provider key** in Administração → Integrações (*Chaves de IA*). The key is sealed with the versioned AES-256-GCM material from `GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY` and stored in `ai_model_keys`; no route returns it. Without a key, `/api/v1/chat/capability` reports `enabled=false` and every run fails with `chat_unavailable`. EXTERNAL users need the `CHAT` capability and never see the secret. OCR with `OCR_PROVIDER=google` uses this same key.

Still open before production:

1. streaming (`streamGenerateContent`) — today text arrives at once and is re-chunked;
2. staging validation of the complete flow.

Do not reuse `fake` in staging/production or put the provider key in `.env`; the shared key belongs to Administração.

## Configuration contract

| Variable                            | Contract                                                                                   |
| ----------------------------------- | ------------------------------------------------------------------------------------------ |
| Product enablement                  | Administração → Funcionalidades (`ai_chat`); defaults off                                  |
| `AI_CHAT_PROVIDER`                  | Set to compose the adapter; `google`, or `fake` in `APP_ENV=test`                          |
| `AI_CHAT_MODEL`                     | Optional fallback when the shared Integrações key has no model yet (≤ 120); live model is chosen with the key |
| `AI_CHAT_RETENTION`                 | Required when provider is set; Go duration from `1h` through `8760h`; `336h` advised        |
| `GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY` | Required with `google`; seals the shared model key (shared with Google Forms tokens)        |

Administrative routes (ADMIN/SUPERADMIN): `GET/PUT/DELETE /api/admin/model-keys/{provider}`. `PUT` receives `{ "secret", "model" }`; responses carry only provider, `configured`, model and `updated_at`.

Chat also requires enabled application authentication, PostgreSQL, Search, and Query Engine. Enabling it with any missing dependency stops startup. The capability endpoint remains authenticated while disabled and reports only `enabled=false` plus safe fixed limits; every lifecycle route returns the stable `chat_unavailable` contract.

Deterministic test-only configuration:

```dotenv
APP_ENV=test
AI_CHAT_PROVIDER=fake
AI_CHAT_MODEL=deterministic-v1
AI_CHAT_RETENTION=24h
```

The normal authentication and database variables are still required. These values are for synthetic testing, not a production recommendation.

## Read-only and authorization boundary

The application owns a fixed registry containing only:

- permission-filtered logical catalog inspection;
- one query plan: filter, derive, match, sequence, group, and combine;
- reopening an existing owner-scoped result reference.

`search`, `sequencia` and `tarefa` stay callable for older tests. They are not in the schema sent to the model. A plan with derive, match or sequence is stored without a query execution and recomputed by `GET /api/v1/chat/result-references/{id}/page`. Group, pattern, set and combination plans still keep an execution. The page accepts the open sheet's limit, up to 500, which is wider than the query engine's 100-row page. The scan that materializes a shaped result stays at 4,000 rows; the chat says when that cut may leave the chain incomplete. Each match or search column lists the values that fit, at most 256 distinct values and 500 runes, and the chat states the full count when the cell is cut.

Tool schemas reject unknown fields. Model arguments cannot select a new tool, increase a server limit, supply SQL, use physical schema names, or introduce a mutation. Query reauthorizes the current user during catalog access, execution, refinement, pagination, and result serialization. Result references are owner- and thread-scoped and are reloaded through the underlying service rather than trusting cached rows.

All user messages, prior assistant messages, and database/tool values are marked as untrusted model input. Instruction-like text in those values grants no permission and cannot create a tool call. Saved queries are not selected or executed from manually typed text.

## What the plan can say

A pasted proof is still a question. The model composes one plan. A requirement the catalog does not store, such as a clip or a placement, is answered in the chat and does not replace the grid. Operational detail for the older task runner lives in [`TASKS.md`](TASKS.md).

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
| New runs / usage window         | Gemini quota and 429 responses; no extra app hourly cap |
| Run wall time                   |                                              5 minutes |
| Tool calls                      |                                              8 per run |
| Rows returned to the model       |                               100 per tool result/page |
| Rows on the open sheet           |                          500, paged from the stored plan |
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

Graceful shutdown cancels the root orchestration context and waits within the configured shutdown timeout. If a process exits abruptly, an active run can no longer be owned by an in-memory coordinator. The cleanup loop runs at startup and once per minute; after the 5-minute run deadline plus a 5-second safety grace it atomically:

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
5. exercise a query whose rows appear on the open table, open the second page, and clear the result;
6. cancel a run, retry it explicitly, rename the thread, and delete it;
7. repeat thread, run, SSE, tool, and result-reference reads as another or revoked user and confirm they are denied without content leakage;
8. inspect logs and audits for IDs/counts/codes only, then advance an explicit test clock or expiry and verify cleanup.

The deterministic fake returns a fixed text response when the full API process is used. Typed multi-tool, injection, failure, reconnect, and lifecycle cases are covered by the synthetic backend/HTTP/frontend suites and require no network credential.

## Failure handling and rollback

- `chat_busy`: wait for, cancel, or recover the one active run; do not create a parallel run for the same thread.
- `rate_limited` or `chat_quota_exceeded`: wait for Gemini or check the Google AI Studio quota. Do not raise limits from client/model input.
- `chat_timeout`: use the explicit retry action after the terminal event; investigate repeated tool latency using safe IDs and timing only.
- `chat_stale_context`: clear the active result and reopen or execute a fresh authorized result.
- `chat_malformed_provider`, `chat_tool_failed`, or `chat_unsafe_result`: keep the provider payload redacted, correlate by request/run ID, and disable Chat if failures repeat.
- Abrupt-process orphan: allow the startup/minute recovery sweep to create its terminal event; never update run rows manually while the service is active.

To disable Chat without deleting retained state, turn off `ai_chat` in Administração → Funcionalidades. The capability endpoint then reports disabled and all Chat lifecycle routes fail closed. Existing rows remain stored under the same database access controls and expire only when cleanup runs after a later safe reactivation or through an explicitly reviewed maintenance procedure.

Migration `016_ai_chat.sql` has a reversible down section for development validation, but rolling it back permanently removes Chat conversations, results, usage windows, and audits. Production rollback should normally disable the feature and leave the schema in place. Use the down migration only with explicit approval and a verified recovery backup.

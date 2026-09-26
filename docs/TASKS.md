# Gymkhana tasks

A gymkhana task is a list of requirements the AI must follow, defined in [`ORCHESTRATION.md`](ORCHESTRATION.md) §20. It is **not** a user-facing module. There is no Tasks workspace, menu, or route. The user pastes or types the task in the Assistente; the AI interprets it and returns authorized database results for each step through Search and the Query Engine ([`QUERY_ENGINE.md`](QUERY_ENGINE.md), [`AI_CHAT.md`](AI_CHAT.md)).

## What the AI must do

A task is often a full gymkhana proof pasted as free text. It may contain several steps.

1. The user submits the task text in the Assistente.
2. The AI decomposes it into ordered steps/requirements without requiring specific wording.
3. Each step runs as permission-filtered Search/Query work and returns its own explainable result set.
4. Later steps may refine or combine earlier result sets (“desses”, “agora somente…”, “combine com…”).
5. The user sees per-step results with evidence, not a single opaque paragraph.

A review-gated TaskSpec workspace is not the product. Any solver or QueryPlan compilation stays internal to Chat/Query tools.

Retry and cancellation are idempotent. Terminal state and results are committed transactionally. SSE events use monotonic sequences that support `Last-Event-ID` resumption.

## Operational states

Internal jobs may use `QUEUED`, `RUNNING`, `COMPLETED`, `INCOMPLETE`, `FAILED`, and `CANCELLED`.

`INCOMPLETE` is a valid terminal result when a configured branch, candidate, evidence, duration, or result limit is reached. It must not be reported as `COMPLETED`, and it may still contain verified compositions found before the limit.

## Security boundaries

- Query execution is read-only and permission-derived.
- Arbitrary SQL and arbitrary regular expressions are not accepted.
- Hidden catalog entries cannot be introduced through task text or JSON.
- Stored jobs and results are owner-scoped and reauthorized on reads.
- The Assistente is the only submission surface. It still cannot mutate canonical data, invent tools, or bypass the Query/Search allowlist.
- Production Assistente uses the shared Administração model key. Semantic interpretation stays disabled for everyone until that key is registered, and for an EXTERNAL user until `CHAT` is granted.

## Verification

Generated Go and TypeScript Query/Chat clients must stay in sync with their OpenAPI contracts. There is no public `/api/v1/tasks` workspace.

# Profile Matching and merge

Orchestration §15: Profile matching is the last product flow to implement. It is computationally expensive. There is no `/matching` destination. When a review UI exists, it belongs in Administração or the people grid — not a parallel menu module.

The HTTP engine below is the backend contract for that flow. It creates explainable candidates and persistent human decisions, but it never classifies a pair or chooses a merge winner automatically.

## Candidate model

Candidate generation runs inside PostgreSQL against the current normalized Profile snapshot. Pair IDs are always canonical (`left_profile_id < right_profile_id`), self-pairs are excluded, overlapping blockers are deduplicated, and results are ordered by score and stable UUID tie-breakers.

The server owns the complete evidence catalog and contributions:

| Evidence | Candidate contribution |
| --- | ---: |
| Exact normalized CPF | 100 |
| Exact normalized e-mail | 95 |
| Exact normalized mobile phone | 90 |
| Exact normalized landline phone | 75 |
| Exact case-insensitive full name | 70 |
| Similar full name | 35–65, derived from PostgreSQL trigram similarity |
| Exact postal code | 20 |
| Exact case-insensitive city | 10 |
| Similar street | 0–15, derived from PostgreSQL trigram similarity |

The public score is the sum capped at 100. Pairs below 50 are not cases; 50–69 is `LOW`, 70–89 is `MEDIUM`, and 90–100 is `HIGH`. Name blockers return at most 25 neighbors per Profile before the global limit. One analysis returns at most 2,000 pairs and uses a 15-second PostgreSQL statement timeout. Clients cannot select fields, SQL, functions or weights.

## Analysis and review lifecycle

- Every analysis is actor-scoped, explicitly requested and idempotent. A user can have at most one queued/running analysis and can request five analyses per one-hour persistent window.
- The rate-limit consumption, analysis row and unique River job commit in the same PostgreSQL transaction. River executes one Matching job at a time; retries and stuck-job rescue make worker restarts safe. A defensive replay path can still repair a legacy/unexpected nonterminal row whose queue link is absent.
- An analysis can be `QUEUED`, `RUNNING`, `COMPLETED`, `FAILED` or `CANCELLED`. Terminal technical rows expire after 30 days and are removed in batches of 250 by the existing worker cleanup scheduler.
- Cases and human decisions do not expire with analysis rows. A case can be `PENDING`, `NOT_DUPLICATE`, `MERGED` or `STALE`.
- A `NOT_DUPLICATE` decision suppresses the same Profile-version pair. Changing either Profile version invalidates that decision and makes a newly scored pair reviewable again.
- Case lists use bounded filters, allowlisted sorting (`score`, `updated_at`, `created_at`), stable tie-breakers and a maximum page size of 100.

All authenticated active roles can run analyses, read cases and record `NOT_DUPLICATE`. Only `ADMIN` and `SUPERADMIN` can preview or confirm a merge. Analysis reads are restricted to their requesting actor; the review queue is an application-wide authorized queue.

## Explicit merge protocol

The administrator first chooses the surviving Profile and supplies the current version of both Profiles. The server returns all 14 canonical Profile fields, every active non-attachment custom Profile field, eight dependency counts and any blocking collisions. Every unequal value—including a value versus blank/null—requires an explicit `SURVIVOR` or `SOURCE` choice.

The preview fingerprint covers the case and Profile IDs, both versions, every field/value/choice, dependency counts, dependency identities/versions and detected conflicts. Confirmation must use that exact fingerprint and type `MESCLAR <nome completo do sobrevivente>`. A separate idempotency key makes response retries safe.

The serializable transaction locks both Profiles in UUID order, rebuilds the preview, compares its fingerprint in constant time, and then:

1. applies only the selected canonical and custom values;
2. moves document/bill owners and current holders;
3. moves Profile custom entities and selected custom values;
4. moves Profile custom-field attachment intents and attachments;
5. increments the survivor version;
6. deletes the absorbed Profile at its expected version;
7. marks related open cases stale and the reviewed case merged;
8. stores the decision, immutable Profile/Matching audits, dependency counts and idempotent receipt.

Document `PER_PROFILE` uniqueness and custom-entity `ONE_PER_PROFILE` collisions are reported in the side-effect-free preview. Custom values for the same definition are resolved only through the explicit field choice. Any stale version, topology change, constraint failure, reversed/concurrent attempt or unresolved choice rolls the whole transaction back.

Historical import rows, Query snapshots and audit events are never rewritten.

## HTTP and browser boundaries

The authenticated API is documented in `api/matching.openapi.yaml`:

- `GET /api/v1/matching/catalog`
- `POST /api/v1/matching/analyses`
- `GET /api/v1/matching/analyses/{analysis_id}`
- `POST /api/v1/matching/analyses/{analysis_id}/cancel`
- `GET /api/v1/matching/cases`
- `GET /api/v1/matching/cases/{case_id}`
- `POST /api/v1/matching/cases/{case_id}/dismiss`
- `POST /api/v1/matching/cases/{case_id}/merge-preview`
- `POST /api/v1/matching/cases/{case_id}/merge`

Bodies reject unknown fields and use UUIDs, fixed enums, bounded arrays, optimistic versions and stable public errors. Unexpected database details are logged only with the request ID. Audits contain IDs, states, score bands, counts and outcomes—not CPF, e-mail, phone, address, field values or raw evidence.

There is no `/matching` SPA route. When review UI is added, it stores only allowlisted filters, sorting, page number and the opaque selected case ID in the URL. It stores no Profile/evidence values in URL, local storage or session storage. Desktop should render the comparison side by side; the responsive layout stacks the same complete controls. A successful merge invalidates Profile, global Search, Query and Matching caches.

## Operations, verification and rollback

Matching requires no external credential or service. It uses the existing API/worker PostgreSQL pool and River schema. Monitor safe counts and outcomes in application logs plus `matching_analyses` and `matching_audit_events`; do not add raw Profile values to operational queries or logs.

The migration is `015_profile_matching.sql`. Its down section removes derived analysis/case/evidence/decision/receipt/rate/audit tables and Profile matching indexes. It intentionally leaves the shared `pg_trgm` extension installed and keeps `PROFILE_MERGED` accepted by the pre-existing Profile audit constraint, preserving immutable merge history when a rollback happens after production use. Rolling back permanently removes Matching decisions and merge receipts, but cannot recreate a Profile already absorbed by a committed merge; restore affected canonical data from a verified database backup when business recovery requires reversal.

Before release, run empty-database upgrade, latest rollback/reapply, PostgreSQL integration, backend build/vet/Staticcheck/race, frontend typecheck/lint/format/tests/build, deterministic OpenAPI generation and Security checks.

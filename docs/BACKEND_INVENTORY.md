# Backend implementation inventory

Snapshot of the Go rebuild backend against [`ORCHESTRATION.md`](ORCHESTRATION.md) and the live legacy app `Gymkhana-Database-Vercel`.

This is an implementation inventory, not a live tracker. Current work and next action remain in [issue #31](https://github.com/Pherlsz/Gymkhana-Database/issues/31). Do not treat this file as acceptance of a delivery stage.

**Sources:** `internal/`, `cmd/`, `database/migrations/`, `docs/ORCHESTRATION.md`, legacy `prisma/schema.prisma` and `src/app/api`. Date of survey: 2026-08-16.

**How to read status**

| Status | Meaning |
| --- | --- |
| Done | Backend implements the approved initial scope |
| Partial | Package and HTTP exist; a required piece is missing or disabled |
| Deferred | Orchestration explicitly postpones this (do not build until decided) |
| Missing | Approved initial scope with no backend package/API |
| Mismatch | Code exists but contradicts the current product rule |

---

## Summary

| Bucket | Count |
| --- | --- |
| Done for initial scope | Auth product (Google + allowlist + SUPERADMIN/ADMIN/EXTERNAL), Profile CRUD including absorbed person fields and writable `team` / `club_membership`, document presences + exemplars, Bills, current-use, Custom data (no formulas), Attachments/R2, Search, Query v1+v2 HTTP, Matching HTTP (no SPA route), XLSX operations lifecycle, Google Forms, Admin users |
| Partial | AI Assistente (shared Administração model key, not per-user BYOK), OCR model path fake-only, Query/task tools inside Chat |
| Deferred by Orchestration | Objects module, full usage/gymkhana-team history, redo of Neon import, Profile matching UI (last module; no `/matching` route) |
| Missing vs Orchestration | Shared Administração model key for Assistente/OCR, Chat query v2 + task interpretation, Gemini-only OCR flag gated on that key, import column/value catalogs (§16.2) |
| Bugs / wiring | EXTERNAL capability checker not passed into `cmd/api` |
| Out of product | User Tasks workspace (`/tasks`, `TASKS` capability); gymkhana team or football club as a module or User field; per-user model keys; `/matching` as a destination |

The rebuild is **ahead of legacy** on Query Engine (typed plans, no client SQL), generic custom entities, document/bill types, presence vs exemplar, and current-use on bills. It is **behind legacy** on a production model provider for Assistente/OCR. Neon 18 Dev/Prod have schema version 30 and empty cadastro; the older Postgres 17 projects still hold the imported copy with gaps (no bills/attachments). Reimport is later work.

---

## 1. Auth, sessions, roles, capabilities

**Rebuild: Partial** (product is implemented; EXTERNAL grants do not take effect in the API process)

- Google OAuth, email allowlist (`allowed_emails` plus env bootstrap), opaque 24h hashed sessions, roles `EXTERNAL` / `ADMIN` / `SUPERADMIN`, admin user/allowlist/capability APIs, audits.
- Account lookup is by email (`internal/auth/service.go` `FindUserByEmail`). Google subject is stored as metadata.
- `internal/auth/postgres_store.go` implements `UserHasCapability`.
- `cmd/api/main.go` sets `RequireCapabilityCheck: cfg.Auth.Enabled` but **does not set `CapabilityCheck`**. `httpserver.New` then installs `unavailableCapabilityChecker`. EXTERNAL users receive **503** on gated routes even when grants exist. ADMIN/SUPERADMIN bypass the checker and work.

**Orchestration §3:** Google OAuth, email allowlist, roles `EXTERNAL` / `ADMIN` / `SUPERADMIN`. Matches, except the unwired EXTERNAL gate.

**Legacy:** NextAuth Google, allowlist by `User.email` + `active`, roles `MEMBER`/`ADMIN`/`SUPERADMIN`. No capability table; MEMBER can edit data, admin routes are role-gated. Cap of 10 users.

**Still to do**

- Pass `*auth.PostgresStore` as `httpserver.Options.CapabilityCheck` in `cmd/api`.
- Session payload does not list capabilities.

---

## 2. Profile

**Rebuild: Done** for canonical CRUD and absorbed person fields

- CRUD, duplicate, optimistic `version`, phones, one structured address, notes, audits. Delete confirmation `Confirmar`.
- CPF is not a Profile column (Orchestration §4 / §6.4). The informed number lives on `document_presences` for type `cpf`; validity stays on the exemplar.
- Birth, family, health, vehicle, `team`, `club_membership`, `membership_type`, and the other former `profile_details` columns live on `profiles` after `030_document_presences.sql`. Create/update persist them. No history table.

**Orchestration §4:** canonical fields match. Extra personal data from legacy `ProfilePersonalData` is first-class on `profiles`.

**Legacy:** `Profile` + `ProfilePersonalData` (typed personal fields + JSON `data`), `team` string, search index rebuild on write.

**Still to do**

- Historical address/CPF versions: not in initial Orchestration scope unless decided.

---

## 3. Equipe de gincana e time de futebol

**Rebuild: Done** — both are writable Profile columns (not User, not modules). They are **two different fields**.

| Field | Meaning | Legacy |
| --- | --- | --- |
| `team` | Gymkhana team (“Equipe”) | `Profile.team`, group gincana, free text |
| `club_membership` | Football club (“Sócio clube”) | seed SELECT Internacional / Grêmio / Outro |

`sector` is gymkhana **setor**, not a team. `membership_type` is club membership category (Cartão / Sócio / Outro).

**Orchestration §5:** two Profile fields. Do not mix them.

**Still to do**

- Do not add `/api/v1/teams` or attach either field to `app_users`.

---

## 4. Documents

**Rebuild: Done** for initial scope (`030_document_presences.sql`)

- Admin document types (technical key, uniqueness NONE / PER_PROFILE / GLOBAL_BY_TYPE, regex, date required).
- `document_presences`: sparse `(profile_id, document_type_id)`; claim `absence` | `indication` | `informed_number`.
- `documents` are exemplars only: `presence_id`, medium PHYSICAL/DIGITAL, `idle_custody` on physical, date, validity, notes.
- Current-use on physical only (`PUT`/`DELETE .../current-use`). Audited. No loan ledger (Orchestration §6.3 / §8 say history is out of initial scope).

**Legacy:** `Document` with a fixed `DocumentType` enum, JSON `data`, `usedInGincana` / `returned` / `gincanaNote`. No admin type catalog. CPF on `Profile`.

**Still to do**

- Nothing required for initial use/loan history.
- Identifier families vs legacy enum mapping belong to the import/reconcile work.

---

## 5. Bills

**Rebuild: Done** for initial scope

- Admin bill types, printed holder/address, competence `YYYY-MM`, decimal amount, medium, `idle_custody` on physical, current-use on physical, audits.

**Legacy:** `Bill` with only `LUZ` / `AGUA` / `INTERNET`. JSON `data`. **No** loan fields. Table PATCH by slug does not update bills.

**Still to do**

- Printed-data versioning is not in initial scope.
- Import mapping from the three legacy types into admin types.

---

## 6. Current use (documents and bills)

**Rebuild: Done** for initial scope (`internal/document`, `internal/bill` current-use routes).

**Orchestration §8:** current physical use only; no fake timeline. Owner, holder, and `idle_custody` are distinct.

**Legacy:** document flags only; bills always “not in use”.

---

## 7. Custom data

**Rebuild: Done** for typed fields and custom entities; **no formula kind**

- Definitions: entity types, fields, options. Values on PROFILE / DOCUMENT_TYPE / BILL_TYPE / CUSTOM_ENTITY_TYPE.
- Kinds: TEXT, LONG_TEXT, INTEGER, DECIMAL, BOOLEAN, CIVIL_DATE, CIVIL_MONTH, EMAIL, PHONE, SINGLE_SELECT, MULTI_SELECT, ATTACHMENT.
- Orchestration §9 forbids arbitrary scripts and executable formulas. A dedicated `CUSTOM_DATA` capability exists in Orchestration; HTTP uses `CUSTOM_DATA` on definition/value routes.

**Legacy:** table UI columns come from `seedColumns.ts`, not DB. `ColumnDefinition` feeds import/Forms. Client-computed `AGE`, `SIGN`, `DIGIT_SUM`, `SUM` in `src/lib/formulas.ts` — not stored.

**Still to do** (product still open)

- Whether formula columns (age, digit-sum, …) and user-created columns need a capability split beyond `data_tables` / `CUSTOM_DATA`.
- Do not add executable formula evaluation; Orchestration rejects it.

---

## 8. Objects

**Rebuild: Deferred** — no `internal/object`. “Object” in code means R2 storage.

**Orchestration §10:** future module; do not hide it inside Profile/Document/Bill/custom data.

**Legacy:** “Objetos” is only a sector SELECT option on people.

**Still to do:** nothing until the module is specified.

---

## 9. Attachments / R2

**Rebuild: Done** when `R2_ENABLED`

- Presign upload, confirm (MIME/size/hash), download, trash 7 days, restore, cleanup, audits. Required by operations, Google Forms, and OCR composition.

**Orchestration §11:** matches.

**Legacy:** attachments are the OCR upload pipeline (`profileId` or leftover `recordId`), not a general library.

---

## 10. Search

**Rebuild: Done**

- `GET /api/v1/search/catalog`, `POST /api/v1/search`. Literal terms over profiles, documents, bills, custom values, attachments. Permission-filtered. No client SQL.

**Legacy:** `GET /api/search` over `ProfileSearchIndex`; optional assistant SQL filter; optional pgvector read path (embeddings not written on save).

---

## 11. Query Engine

**Rebuild: Done** for v1 and v2 HTTP

- v1: `/api/v1/query/*` — projection, filters, relations, sort, pagination, catalog version, read-only SQL, result snapshots.
- v2: `/api/v1/query/v2/*` — grouping, aggregates, HAVING, patterns, sets, combinations (`internal/queryengine/advanced_*.go`, executed, not compiler-only).
- Chat tools still call **v1 only**.

**Orchestration §14:** one versioned engine. v1 is the live subset; v2 is the rest of the same surface. No `/query` destination.

**Legacy:** no QueryPlan. “Consultas avançadas” = Gemini emitting read-only SQL inside assistant chat.

**Still to do**

- Chat query tool on v2 when tasks/aggregations must run from Chat.
- Absorbed Profile columns (team, birth, …) belong in the logical catalog, not a dropped `profile_details` table.

---

## 12. Matching / merge

**Rebuild: Partial** — HTTP exists; SPA destination must not.

- Analyses (River), cases, dismiss, merge-preview, merge (`MESCLAR`), ADMIN/SUPERADMIN for merge. No `/matching` route (Orchestration §12.0 / §15). Review UI, when built, belongs in Administração or the people grid.

**Orchestration §15:** Profiles only; last module; no automatic merge; no `/matching` destination.

**Legacy:** several overlapping flows (create-time `detectDuplicates`, import staging merge, context duplicate review, `ProfileDuplicateReview` list without resolve).

**Still to do:** document/bill duplicate modules are out of the Profile-only rule unless later approved. Do not add a matching menu route.

---

## 13. XLSX import / export

**Rebuild: Partial** for profiles/documents/bills

- Import lifecycle (sheet → operator mapping → preview → decisions → execute), export download, bulk-delete. Executable spreadsheet formulas rejected. Custom fields on PROFILE only. Needs R2 + worker.
- Mapping today is per-import: the operator assigns `source column → target field`. Headers are not folded against a catalog; closed-list values are not collapsed; calculated/discard columns are not dropped automatically; labeled document remaps (OAB in a generic card column, TRI/TEU, PIS vs CTPS) are not applied unless the operator maps them by hand.

**Legacy:** **CSV import**, XLSX **export**. Promote into Profile/Document/Bill.

**Orchestration §16.2:** bulk import (XLSX and `/forms`) must apply a versioned Database-owned catalog of column aliases and value maps automatically, using Core for fold/identifier canonicalization. Unknown headers/values and create/update/duplicate remain human decisions. Automatic mapping is not automatic merge.

**Still to do**

- Column-alias catalog (folded header → canonical field, document type, import metadata, or discard) plus known-layout untitled columns.
- Value catalog (sim/não, club membership, membership type, supermarket clubs, and equivalent closed lists; multi-value cells).
- Automatic document remaps from labels inside a cell; discard list (age, sign, digit sums, referrer, birth time, operational notes, Drive links held for later digital documents).
- Pre-fill mapping + preview from the catalog so the same layout is not remapped from scratch.
- Write path from a validated legacy bundle into Postgres (see §18).
- Custom-entity / attachment modules in the operations catalog.

---

## 14. Google Forms

**Rebuild: Done** as a real Google integration (not fake)

- Owner-scoped OAuth, sources, mapping, sync into operations import. Encrypted tokens. ADMIN/SUPERADMIN. Config-gated.

**Legacy:** live Forms OAuth, webhook, cron, `FormSubmission` promote/merge/skip.

**Still to do:** cutover mapping from legacy `IntegrationKey` / submissions is import work, not a missing API. Forms must consume the same §16.2 column/value catalogs as XLSX; there is no second mapper.

---

## 15. AI Chat

**Rebuild: Partial**

- Threads, turns, SSE, cancel, Search + Query **v1** + catalog + result tools. Read-only. Fake adapter only. No shared Administração model key yet.
- **No task tool.** Chat cannot interpret gymkhana requirement lists.

**Orchestration §18:** Assistente with a shared Administração model key (not per-user BYOK). Query and gymkhana-task interpretation are Chat tools. No user Tasks, Query, or Chat destination.

**Legacy:** Gemini + `UserAiApiKey` BYOK, flag `AI_CHAT` default off, NL → SQL. Per-user keys are rejected for this rebuild.

**Still to do**

- Register / encrypt / rotate the shared Administração model key.
- Chat tools for Query (including later plan versions) and gymkhana-task interpretation.

---

## 16. OCR

**Rebuild: Partial** — job/review/apply matches Orchestration; model path is fake-only.

**Orchestration §19:** suggestions + human review. Use a model only when adequate. Optional Gemini-only flag requires the shared Administração model key.

**Legacy:** Gemini OCR, default flag on, confirm creates Profile/Document/Bill.

**Still to do:** Gemini-only feature flag gated on the shared Administração key. Keep non-model extraction when that is the adequate path.

---

## 17. Gymkhana tasks

**Rebuild: Done** — user Tasks workspace retired. HTTP `/api/v1/tasks`, capability `TASKS`, and `task_*` tables are gone. `internal/taskengine` remains for a future Chat-internal solver.

Code (`internal/taskengine/`):

- Typed `TaskSpec`, human review, solver job on Query v2.
- Chat does not import taskengine.

**Orchestration §20:** Chat-only capability. Requirement lists, per-step results. No menu/route/workspace.

**Legacy:** no Task model. “Tarefa” is a loan note plus whatever assistant SQL can answer.

**Still to do**

- Expose interpretation only through Chat tools.
- Keep any solver internal to Chat/Query.

---

## 18. Legacy migrate / cutover

**Rebuild: Done once, inconsistent — redo later, not current work**

Neon (survey 2026-08-16, counts only, no row payloads):

| Database | Notes |
| --- | --- |
| Rebuild Dev-18 (`Gymkhana-Database-Dev-18`) | Postgres 18, schema version 30, empty cadastro |
| Rebuild Prod-18 (`Gymkhana-Database-Prod-18`) | Postgres 18, schema version 30, empty cadastro |
| Rebuild Dev-17 (`Gymkhana-Database-Dev`) | Imported copy: 88033 profiles, 57630 documents, 0 bills, 0 attachments |
| Rebuild Prod-17 (`Gymkhana-Database-Prod`) | Same imported copy as Dev-17 |
| Legacy Prisma (`Gymkhana-database-staging`) | 87356 profiles, 0 documents, 0 bills, 6 attachments |

`cmd/legacy-reconcile` still only validates a bundle. The live Neon 17 load is treated as an existing attempt with known gaps (bills/attachments empty, profile count drift). Do not rebuild the importer as active work unless the operator asks.

**Still to do (later):** a reviewed reimport onto Neon 18, using the §16.2 catalogs rather than a one-off column map. Not a blocker for Assistente/Query.

---

## 19. Admin users

**Rebuild: Done** — list users, patch role/active (not self, not SUPERADMIN), capabilities, allowlist. Users are created on first allowed Google login.

---

## Binaries

| Command | Role |
| --- | --- |
| `cmd/api` | HTTP API |
| `cmd/worker` | River: operations, matching, Forms, OCR, cleanup |
| `cmd/migrate` / `cmd/river-migrate` | schema |
| `cmd/configcheck` | fail-closed config |
| `cmd/launch-smoke` | live/ready + revision |
| `cmd/legacy-reconcile` | bundle validation |
| `cmd/cleanup-documents` | identifier classification |

---

## Suggested backend order (not a tracker)

These are the gaps that block product use, not a milestone list:

1. Wire `CapabilityCheck` so EXTERNAL grants work.
2. Continue the Assistente with the shared Administração model key, with Query and gymkhana-task tools inside Chat (no user Tasks module, no per-user keys).
3. Optional Gemini-only OCR flag gated on that shared key.
4. Matching review UI last, in Administração or the people grid — no `/matching` route.
5. Neon 18 reimport later, not now.

Do not start Objects, usage history, or executable formulas. Orchestration already deferred or rejected them.

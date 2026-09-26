# Cadastro — Gymkhana-Core reuse audit

Operational decision log for the Cadastro module (Orchestration §16.2). Permanent rules remain in [`ORCHESTRATION.md`](ORCHESTRATION.md).

## Version decision

| Item               | Decision                                                                                                                                                                                               |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Pinned version     | `github.com/Pherlsz/Gymkhana-Core v0.7.0` (see [`go.mod`](../go.mod))                                                                                                                                  |
| Local Core HEAD    | v0.7.0                                                                                                                                                                                                 |
| **Recommendation** | **Stay on v0.7.0**. Database uses `normalize`, `civiltime`, `fingerprint`, and Core `ocr` request/result validation from this pin. Persistence, jobs, billing, and review/apply remain Database-owned. |

## Reuse matrix

| Concern                                            | Decision          | Core API / notes                                                                                                    |
| -------------------------------------------------- | ----------------- | ------------------------------------------------------------------------------------------------------------------- |
| Header folding for import aliases                  | **Use Core**      | `normalize.SearchText`                                                                                              |
| CPF/CNPJ/phone/email canonicalization              | **Use Core**      | `normalize.CanonicalCPF`, `CanonicalCNPJ`, etc. — via `profile.Normalize` / document/bill services                  |
| Document type from label (OAB, PIS, TRI)           | **Use Core**      | `normalize.IdentifyDocument` — import catalog maps labels only; validation uses existing domain services            |
| Closed values (gender, marital, blood, membership) | **Use Core**      | `FormatGender`, `FormatMaritalStatus`, `FormatBloodType`, `FormatMembershipType` — via profile normalize on execute |
| Team / sector / club catalog values                | **Use Core**      | `normalize/catalog.go` formatters — applied at row execution, not in `importcatalog`                                |
| Civil dates in cells                               | **Use Core**      | `civiltime.ParseCivilDate` — operations preview/execute path                                                        |
| Civil dates/months in OCR values                   | **Use Core**      | `civiltime.ParseCivilDate` / `ParseYearMonth` in `internal/ocr`                                                     |
| Search identifier compile                          | **Use Core**      | `IdentifyDocumentMatches`, `CanonicalDocument`, `CanonicalCEP`, `CanonicalBrazilPhone`, `CanonicalEmail`            |
| Catalog / job fingerprints                         | **Use Core**      | `fingerprint.Sum` (SHA-256, same digest as before)                                                                  |
| Import column alias catalog                        | **Database-only** | `internal/importcatalog` — product rules §16.2                                                                      |
| XLSX staging / state machine                       | **Database-only** | `internal/operations`                                                                                               |
| OCR jobs, suggestions, apply, usage                | **Database-only** | `internal/ocr` store/worker/HTTP. Extractor consumes Core `ocr` types.                                              |
| Attachments / R2                                   | **Database-only** | `internal/attachment`                                                                                               |
| Cadastro UI                                        | **Database-only** | `apps/web/src/lib/cadastro`                                                                                         |

## `archiveimport/map.go` mapping

| Local helper                       | Core equivalent                          | Action                                |
| ---------------------------------- | ---------------------------------------- | ------------------------------------- |
| Header keys (`nome`, `celular`, …) | `normalize.SearchText` for fold          | `importcatalog` aliases               |
| `yesNo` / absent detection         | `normalize` absent handling via profile  | Keep execution in `profile.Normalize` |
| `civilDate`                        | `civiltime.ParseCivilDate`               | Use at import execute                 |
| `formationType` (OAB/CREA/…)       | Core document kinds / `IdentifyDocument` | Catalog maps column headers only      |
| Row normalization                  | `profile.Normalize`                      | Unchanged                             |

## OCR contract mapping

Cadastro extraction is **schema-guided**. The host validates attachment bytes, MIME, size, and SHA-256, then calls the extractor with a Core `ExtractionRequest` (no bytes). The extractor returns a Core `ExtractionResult`; Database maps present candidates onto the closed field catalog and persists suggestions.

| Database field                              | Core `ocr` contract                                                                                                           |
| ------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Closed catalog in `internal/ocr/targets.go` | `schema_guided` + `portable_json_schema/v1` object schema (`additionalProperties: false`, every property required)            |
| Logical field keys (`bill.amount`)          | Flat object properties; RFC 6901 path `/bill.amount`                                                                          |
| Source bytes / SHA-256 / page pixels        | Host-owned; Core `SourceRef` is id + modality + media type only                                                               |
| Suggestion confidence                       | Integer `0..10000` (`ConfidenceScale`); stored and returned as that integer                                                   |
| Evidence page                               | 1-based; Core allows page only on `document` modality. Images persist Database page `1`                                       |
| Evidence region                             | Core `NormalizedRect` millionths `[0, 1_000_000]`                                                                             |
| Evidence excerpt                            | Copied from `Observation.RawText` (bounded); Core evidence refs do not copy text                                              |
| Incoming Core review                        | `unreviewed` / `needs_review` → Database `PENDING`. Product `ACCEPTED` / `REJECTED` / `APPLIED` / `STALE` stay Database-owned |
| Provider usage                              | Database-owned; not part of Core result                                                                                       |
| Validation errors                           | Core `ValidationError` (code + field only) maps to `ocr_malformed_provider` without source text                               |

## Outcomes implemented

- `internal/importcatalog` uses `normalize.SearchText` for header folding only.
- OCR extractor port uses Core `ocr` request/result plus host-owned source bytes.
- OCR typed values use Core `civiltime` for civil date/month.
- Catalog and job fingerprints use Core `fingerprint.Sum`.
- No new normalize functions added to Database.
- Search compiles identifiers with Core `CanonicalDocument` / `IdentifyDocumentMatches`.

# Legacy to Rebuild Database Mapping

This document records the accepted one-time migration from the production legacy Neon database to the rebuild database. It is operational evidence, not a second product tracker.

## Databases

- Neon project: `gymkhana-database-prod` in `aws-sa-east-1`.
- Branch: `production`.
- Source database: `neondb`.
- Target database: `gymkhana_rebuild`.
- The source database remains online and unchanged.
- The target is a separate database, not a second schema inside `neondb`.
- The target contains only the canonical application schema in `public`; it does not retain a `legacy` schema.

The project named `Gymkhana-database-staging` remains dedicated to the legacy staging application and is not the migration destination. No staging business data was copied into the accepted production rebuild database.

## Source reconciliation

The production source contained:

- 88,033 rows in the legacy `Record` table;
- one `dados-pessoais` context containing all 88,033 rows;
- no populated `pessoaId`, so every legacy Record became one canonical Profile;
- 3 legacy users;
- 45 JSON keys across the personal records.

Derived legacy values such as age, zodiac sign and digit sums were not persisted because they can be calculated from canonical fields when required.

## Identity migration

Legacy user e-mails became the authoritative `app_users` allowlist for Google login.

- `tocadogamba.database@gmail.com`: active `SUPERADMIN`.
- `henriqueslopespedro@gmail.com`: active `ADMIN` because the rebuild permits exactly one active `SUPERADMIN`.
- `tocadogambatnc@gmail.com`: inactive `ADMIN` and therefore blocked.

`google_subject` remains null until the first successful verified Google login for each allowlisted e-mail.

## Profile mapping

The migration produced 88,033 canonical `profiles` and 88,033 one-to-one `profile_details` rows.

Canonical `profiles` contain name, social name, CPF, e-mail, phones, structured address, notes, version and timestamps.

Mapped personal fields that remain owned by the Profile aggregate are stored in `profile_details` rather than custom data:

- birth, gender, nationality and civil-status fields;
- father and mother fields;
- health plan and donor flags;
- team and sector;
- collections;
- vehicle model, color, plate and year;
- club membership.

This one-to-one extension avoids a generic JSON payload and preserves compatibility with existing sqlc Profile scans.

## Address and contact normalization

The production source already stores street, neighborhood, city, state and postal code separately. The migration preserves that structure and does not persist `address_raw`.

Normalization is deliberately conservative:

- street number and complement are separated only when the saved street contains an explicit number marker or unambiguous comma-separated number;
- neighborhood and city are never invented;
- state names and abbreviations are normalized to the Brazilian two-letter form when recognized;
- equivalent city spellings are grouped using only the saved production values, with the most frequent saved spelling selected as canonical;
- no external geocoder is used;
- values that cannot be normalized safely are preserved in Profile notes instead of being discarded or guessed.

Validation results:

- all stored dates used by Profile details were parseable;
- 18 city groups had equivalent saved spellings and were canonicalized;
- zero invalid state abbreviations remain in canonical fields;
- zero invalid postal-code formats remain in canonical fields;
- zero invalid mobile or landline formats remain in canonical fields.

## Documents and custom data

CPF remains a canonical Profile field. Recognized legacy identifiers were converted into `documents` with configured types:

| Type | Rows |
| --- | ---: |
| RG | 56,401 |
| CNH | 244 |
| CTPS | 182 |
| Citizen Card | 61 |
| Passport | 165 |
| Student ID | 88 |
| SUS Card | 42 |
| Voter ID | 447 |
| **Total** | **57,630** |

All persisted business fields found in the production JSON were recognized as canonical Profile, Profile Details or Document data. Therefore the migration inserted zero custom-field values. Custom data remains available only for genuinely unmapped future data.

## Removed legacy concepts

The canonical `public` schema contains no:

- generic `Record` table;
- `source_ref` or `sourceRefs` field;
- `address_raw` field;
- GitHub authentication identity columns;
- legacy ID column exposed to runtime modules;
- generic migrated personal-data JSON;
- migration schema, foreign server or migration helper function.

## Runtime schema

M0 through M14 application tables were created in `public`, including authentication, sessions, audits, Profile details, documents and bills, custom data, attachments, Search, operations, Google Forms, Query Engine, matching, AI Chat, OCR and task workflows.

Final validation at migration time:

- 79 public application tables;
- 715 validated constraints and no unvalidated constraint;
- 251 non-duplicated indexes, including constraint-backed indexes;
- 4 application triggers;
- 88,033 Profiles;
- 88,033 Profile Details;
- 57,630 Documents;
- 8 Document Types;
- 3 allowlisted users;
- exactly 1 active SUPERADMIN;
- 0 migrated custom-field values;
- 0 orphan Profile Details or Documents;
- 0 duplicate canonical CPFs;
- 0 duplicate Documents per Profile, type and identifier;
- no missing core relation from the M0-M14 checklist.

The original `neondb` was rechecked after the migration and still contained its 88,033 legacy Records, 3 users and 15 public tables.

River's own internal schema remains managed by the dedicated `cmd/river-migrate` lifecycle and is not copied from legacy data.
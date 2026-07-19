# Legacy to Rebuild Database Mapping

This document records the accepted one-time migration from the legacy Neon database to the rebuild database. It is operational evidence, not a second product tracker.

## Databases

- Source: `neondb` in the existing São Paulo staging project.
- Target: `gymkhana_rebuild` in the same `aws-sa-east-1` Neon project.
- The target keeps the cloned source temporarily under the `legacy` schema for reconciliation.
- Runtime code uses only the canonical `public` schema.

The separate `Gymkhana-database-migrated` project in `aws-us-east-2` is not the accepted target because it had an incomplete, empty and incompatible schema.

## Identity migration

Legacy user e-mails became the authoritative `app_users` allowlist for Google login.

- `tocadogamba.database@gmail.com`: active `SUPERADMIN`.
- `henriqueslopespedro@gmail.com`: active `ADMIN` because the rebuild permits exactly one active `SUPERADMIN`.
- `tocadogambatnc@gmail.com`: inactive `ADMIN` and therefore blocked.

`google_subject` remains null until the first successful verified Google login for each allowlisted e-mail.

## Profile mapping

The migration produced 87,356 canonical `profiles` and 87,356 one-to-one `profile_details` rows.

Canonical `profiles` contain the fields already consumed by the generated repository code: name, social name, CPF, e-mail, phones, structured address, notes, version and timestamps.

Mapped personal fields that remain owned by the Profile aggregate are stored in `profile_details` rather than custom data:

- birth, gender, nationality and civil-status fields;
- father and mother fields;
- health plan and donor flags;
- team and sector;
- collections;
- vehicle model, color, plate and year;
- club membership.

This one-to-one extension avoids a generic JSON payload and preserves compatibility with existing sqlc `profiles` scans.

## Address normalization

The target does not persist `address_raw`.

The parser preserves only information present in the source and attempts to decompose:

- street;
- number;
- complement;
- neighborhood;
- city;
- state;
- postal code.

City/state-only values such as `Portão, RS` become only city and state. No missing street, number, neighborhood, state or city is invented. Brazilian state names and abbreviations are normalized, and equivalent saved city spellings are canonicalized without using an external geocoder.

Validation results:

- 83,903 nonempty source addresses;
- 83,903 target profiles with at least one structured address field;
- 21,331 city-only structured addresses;
- zero invalid state abbreviations;
- zero invalid postal-code formats.

## Documents and custom data

CPF remains a canonical Profile field. Recognized legacy identifiers were converted into `documents` with configured types:

- RG;
- CNH;
- CTPS;
- Citizen Card;
- Passport;
- Student ID;
- SUS Card;
- Voter ID.

The migration produced 57,651 document rows.

All 26 keys found in legacy personal JSON were recognized as canonical Profile details or document identifiers. Therefore the migration inserted zero custom-field values. Custom data remains available only for genuinely unmapped future data.

## Removed legacy concepts

The canonical `public` schema contains no:

- generic `Record` table;
- `source_ref` or `sourceRefs` field;
- `address_raw` field;
- legacy ID column exposed to runtime modules;
- generic migrated personal-data JSON.

## Runtime schema

M0 through M14 application tables were created in `public`, including authentication, sessions, audits, Profile details, documents/bills, custom data, attachments, Search, operations, Google Forms, Query Engine, matching, AI Chat, OCR and task workflows.

Final validation at migration time:

- 79 public application tables;
- 87,356 Profiles;
- 87,356 Profile details;
- 57,651 Documents;
- 3 allowlisted users;
- exactly 1 active SUPERADMIN;
- 0 migrated custom-field values;
- no missing core relation from the M0–M14 checklist.

River's own internal schema remains managed by the dedicated `cmd/river-migrate` lifecycle and is not copied from legacy data.

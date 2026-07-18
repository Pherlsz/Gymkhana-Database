# Multimodal OCR operations runbook

Multimodal OCR is a private, permission-aware extraction workspace for existing PDF, JPEG, and PNG attachments. Extraction creates typed suggestions with evidence; it never mutates canonical data. A user must review each suggestion and then perform a separate confirmed application protected by current target versions.

## Activation boundary

The feature is disabled by default. The repository currently contains only a deterministic fake extractor, and configuration accepts it only in `APP_ENV=test`. Staging and production therefore fail closed if `OCR_ENABLED=true`.

Production activation requires an owner decision and reviewed implementation for all of the following:

1. select the provider, API, exact model, region, retention policy, and contractual privacy terms;
2. confirm which private attachment classes may be transmitted outside the application boundary;
3. implement a narrow adapter for the existing `Extractor` port using the fixed provider-neutral schema;
4. add credentials through the deployment secret manager, never the repository or database;
5. document provider deletion/incident procedures and validate them with the privacy owner;
6. complete security, quota, timeout, malformed-output, cancellation, and staging smoke tests before enabling the switch.

Do not reuse `fake`, add a provider SDK, invent credential variables, or select a provider/model by assumption.

## Configuration contract

| Variable                          | Contract                                                                      |
| --------------------------------- | ----------------------------------------------------------------------------- |
| `OCR_ENABLED`                     | Explicit switch; defaults to `false`                                          |
| `OCR_PROVIDER`                    | Required when enabled; currently only `fake` in `APP_ENV=test`                |
| `OCR_MODEL`                       | Required when enabled; nonempty identifier, at most 120 characters            |
| `OCR_TIMEOUT`                     | Extraction deadline from `1s` through `5m`; defaults to `90s`                 |
| `OCR_MAX_REQUESTS_PER_HOUR`       | Persistent per-user request limit from 1 through 1,000; defaults to 10         |
| `OCR_MAX_PROVIDER_USAGE_PER_HOUR` | Persistent per-user provider-usage limit through 100,000,000; defaults 500,000 |
| `OCR_MAX_SOURCE_BYTES`            | Source limit through 20 MiB; defaults to 20 MiB                               |

OCR also requires authentication, PostgreSQL, private attachment storage, and the Profile, Document, Bill, and Custom Data services. A missing dependency stops startup. While disabled, the authenticated capability endpoint reports `enabled=false` and safe fixed limits; lifecycle routes return `ocr_unavailable`.

Deterministic test-only configuration:

```dotenv
APP_ENV=test
OCR_ENABLED=true
OCR_PROVIDER=fake
OCR_MODEL=deterministic-v1
OCR_TIMEOUT=90s
OCR_MAX_REQUESTS_PER_HOUR=10
OCR_MAX_PROVIDER_USAGE_PER_HOUR=500000
OCR_MAX_SOURCE_BYTES=20971520
```

The normal authentication, database, and private-storage variables are still required.

## Source and provider boundary

Only an active attachment already authorized through the attachment service can start a job. The job stores its detected MIME, byte count, SHA-256, owner reference, and target-catalog fingerprint. The worker opens the object through the same private boundary and verifies all recorded metadata before reading it.

The worker then enforces:

- PDF, JPEG, or PNG only, with detected MIME matching the stored MIME;
- exact byte count and SHA-256, with at most 20 MiB;
- unencrypted, structurally bounded PDF input with 1 through 20 pages;
- decodable JPEG/PNG input with at most 40,000,000 pixels;
- a second current-user, attachment, owner, and catalog authorization check immediately before provider execution;
- a fixed field catalog containing logical keys, labels, required flags, and value kinds only;
- at most 100 unique suggestions, closed field keys, typed normalized values, bounded evidence, and provider usage through 100,000,000.

Attachment text, evidence, labels, and recognized values are untrusted data. Instruction-like content grants no permission, cannot expand the field catalog, and is rendered as text. Provider output cannot select a target, physical table/column, SQL, URL, tool, or mutation.

## Persistent data and privacy

PostgreSQL stores:

- owner-scoped jobs, retry linkage, source fingerprints, attempt/usage counts, and terminal safe error codes;
- monotonic normalized SSE events;
- typed suggestions, immutable evidence page/region/excerpt, review decisions, and optimistic versions;
- idempotent application receipts and per-suggestion applied/stale/failed outcomes;
- persistent per-user request/provider-usage windows;
- audits with logical IDs, field key, accept/reject action, counts, outcome, safe error code, and request ID.

It does not store provider credentials, signed URLs, object keys in OCR tables, raw provider requests/responses, provider diagnostics, unrestricted document text, SQL, physical schema names, or automatically generated mutations. Logs and audits must not contain source bytes, evidence excerpts, proposed/reviewed values, target values, provider payloads, credentials, or database error detail.

Trashing an attachment immediately removes its jobs from the owner list and processing authorization. Permanent attachment purge cascades through jobs, events, suggestions, receipts, and results; the bounded per-user usage window keeps its independent hourly lifecycle. Logical audit records intentionally survive without foreign keys to deleted job/suggestion content.

## Lifecycle, retries, streaming, and recovery

A start request atomically persists its fingerprint, usage-window reservation, `QUEUED` state, and `JOB_ACCEPTED` event before River receives an ID-only job argument. Replaying the same user/idempotency key returns the existing job only when the attachment, source/catalog fingerprints, retry target, and schema match. A user can have only one queued/running job for one attachment.

River executes the dedicated `ocr` queue with at most three attempts. Automatic retry is limited to transient `unavailable` failures before provider execution; those attempts use River backoff and revalidate the source again. Once provider execution begins, any timeout, transport ambiguity, malformed output, accounting failure, cancellation, or persistence failure is terminal so the system cannot silently repeat billable processing. A new user-visible retry creates a distinct job linked by `retry_of_job_id`.

An abrupt process exit can leave a `RUNNING` job in an ambiguous provider phase. The startup/minute recovery sweep waits for the configured timeout plus a safety grace and atomically records `FAILED/worker_interrupted` with one terminal event. It never replays ambiguous provider work. Graceful shutdown stops River, cancels the root context, and waits within the worker shutdown boundary.

SSE emits only persisted normalized events in sequence order. Clients reconnect with both `after` and `Last-Event-ID`, ignore duplicate sequences, perform at most three bounded reconnects, and fall back to the durable job endpoint. Streams and JSON responses use the authenticated no-store HTTP boundary.

## Review and conflict-safe application

Completed jobs expose the current canonical value, proposed value, editable reviewed value, field type, target version, and immutable evidence. Accepting or rejecting updates only the suggestion row and audit decision; it does not call a domain mutation service.

Only accepted, non-stale suggestions can be selected. The workspace first opens a confirmation surface listing the selected fields. A second action sends an idempotency key plus suggestion IDs/versions. The service reloads authorization and the logical catalog, groups changes by target/version, and calls the existing Profile, Document, Bill, or Custom Data update service. It never writes canonical tables directly.

Each group produces durable per-suggestion `APPLIED`, `STALE`, or `FAILED` results, so unrelated targets can succeed independently. If the process stops after a target update but before receipt persistence, replay checks whether the desired values already exist and records that converged result without repeating the mutation. Any other target-version change becomes `STALE` and is never overwritten.

## Acceptance and privacy matrix

| Scenario                                      | Required result                                                                  |
| --------------------------------------------- | -------------------------------------------------------------------------------- |
| Disabled runtime                              | Capability says disabled; lifecycle routes fail closed; no provider call         |
| Missing/revoked session or role               | Source/job/suggestion/application denied without content disclosure              |
| Trashed, purged, changed, spoofed source      | Not found or `ocr_unsafe_source`; provider is not called                          |
| Oversized, encrypted, corrupt, or mismatched  | Deterministic source rejection before provider execution                         |
| Unknown/duplicate field or malformed evidence | Whole provider result rejected; no suggestion or canonical mutation is committed |
| Prompt/instruction markup in evidence/value   | Preserved only as bounded inert data and rendered as text                        |
| Accept/reject                                 | Suggestion/audit changes only; canonical target remains unchanged                |
| Apply without accepted current selection      | Rejected before a domain mutation                                                |
| Concurrent review/application                 | One optimistic winner; stale versions never overwrite canonical data             |
| Partial target failure                        | Durable per-item results; successful unrelated targets remain explicit           |
| Duplicate start/apply request                 | Same fingerprint replays one job/receipt; changed payload conflicts              |
| Transient pre-provider outage                 | At most three automatic attempts with reauthorization                            |
| Provider-phase ambiguity or worker loss       | Terminal safe failure; only an explicit linked retry may run again                |
| Attachment trash/purge                        | Hidden/denied immediately; generated content cascades on purge; audit survives    |
| Logs and audits                               | IDs, logical field/action, counts, outcomes, and stable codes only                |

The unit, HTTP, frontend, PostgreSQL integration, migration, generated-contract, race, static-analysis, and security suites enforce these cases without a production provider credential.

## Smoke test

Run the deterministic path only in an isolated test environment:

1. apply all application and River migrations to a disposable PostgreSQL database;
2. configure authentication, private object storage, canonical target services, and the test-only OCR values above;
3. sign in as an active member and confirm `/api/v1/ocr/capability` reports enabled with the fixed limits;
4. upload one valid PDF/image, start a job, observe ordered SSE, and reconnect from a recorded sequence;
5. compare current/proposed/evidence values, edit and accept one suggestion, reject another, and confirm neither action changed canonical data;
6. select an accepted suggestion, inspect the confirmation step, apply it, and replay the same application key;
7. change another target version and verify the selected suggestion becomes stale without overwrite;
8. retry a failed/cancelled job explicitly and verify its linkage;
9. repeat reads/actions as another or revoked user and verify denial without evidence/value leakage;
10. trash and purge the source in the disposable environment and verify list/authorization/cascade behavior;
11. inspect logs and audits for logical identifiers/counts/codes only.

## Failure handling and rollback

- `ocr_unsafe_source`: verify the original attachment and create a new upload; never bypass MIME/hash/page/pixel checks.
- `ocr_stale_target` or conflict: reload current values and perform a new review; never force an old version.
- `ocr_quota_exceeded` or rate limit: wait for the persistent hourly window; do not raise limits from client/provider input.
- `ocr_timeout`, `ocr_malformed_provider`, or `ocr_unavailable`: keep provider details redacted, correlate by request/job ID, and disable OCR if repeated.
- `worker_interrupted`: use an explicit linked retry only after assessing possible provider billing; never manually return the row to `QUEUED`.

To disable OCR without deleting state, set `OCR_ENABLED=false` and restart API and worker. Existing jobs and review history remain private in PostgreSQL; all lifecycle routes fail closed. Migration `017_multimodal_ocr.sql` has a reversible down section for disposable migration validation, but rolling it back permanently deletes OCR jobs, suggestions, receipts, usage, and audits. Production rollback should normally disable the feature and leave the schema in place. Use the down migration only with explicit approval and a verified recovery backup.

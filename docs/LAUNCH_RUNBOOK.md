# Production launch runbook

This runbook prepares and executes a controlled launch of Gymkhana Database. It does not contain production identifiers, credentials, personal data, or database dumps.

> **Deployment status:** the container-image deployment described below is **retired**. Docker and container images are banned in this repository. Steps that build, push, or promote images are kept for historical reference only until a new deployment story is decided. Recovery, reconciliation, and data-migration steps remain valid.

## Principles

- Build once and promote immutable release artifacts. (The container-image procedure is retired; its replacement is undecided.)
- Run database migrations as a separate job before API and worker promotion.
- Never run a destructive restore as an automatic rollback.
- Export and reconcile the legacy database before any write to the new database.
- Keep AI Chat, OCR, and Google Forms disabled until their external prerequisites are deliberately approved.
- Store migration bundles, recovery points, rendered manifests, and deployment exports only in ignored local directories.
- A failed gate is a no-go. Do not override it during the launch window.

## Owner-provided prerequisites

The repository can validate all contracts without production access. The following values are required only when the real launch is attempted:

- a read-only copy of the legacy SQLite database;
- approval of the legacy context/field mapping after reconciliation;
- Google Cloud project, region, Artifact Registry repository, runtime service account, scheduler service account, and Workload Identity Federation provider;
- Cloud Run API, worker, and migration resource names;
- numeric Secret Manager versions for the database, Google OAuth, and R2 values;
- production application and API domains;
- Vercel project with `VITE_API_BASE_URL` configured for the target environment;
- Neon production connection and a separate empty restore-rehearsal database;
- Cloudflare R2 endpoint and private bucket.

Do not place any real value in this file or in a committed `.env` file. Local secrets use lokeys, not a copied environment file.

## 1. Select the release candidate

1. Select a commit on `main` whose required checks are green.
2. Use a release version derived from that commit, for example `sha-<12 characters>`.
3. Retired: steps 3–6 built and promoted Docker images (`Dockerfile.api`, `Dockerfile.worker`, `Dockerfile.migrate`), which are banned in this repository. A replacement release-artifact and promotion procedure has not been selected.

The `Release artifacts` workflow verifies deployment scripts and contracts before promotion.

## 2. Rehearse the legacy export

The legacy source remains SQLite/Prisma. The export command opens it in read-only mode and creates deterministic JSONL files plus a manifest.

```bash
python3 scripts/export-legacy-sqlite.py \
  --database /private/path/legacy.db \
  --output migration-private/rehearsal-01
```

The bundle contains:

- context definitions;
- column definitions;
- records with structured JSON data;
- attachment metadata;
- row counts and SHA-256 digests;
- one deterministic bundle fingerprint.

The bundle is sensitive production data even though the report is not. Keep the whole bundle inside `migration-private/` or another protected location.

## 3. Reconcile before import

```bash
go run ./cmd/legacy-reconcile \
  --bundle migration-private/rehearsal-01 \
  --report migration-private/rehearsal-01-report.json \
  --strict
```

A valid strict report must have no errors and no warnings. The reconciler currently verifies:

- manifest hashes, counts, ordering, and fingerprint;
- unique context slugs, IDs, and column keys;
- existence of the profile context and profile records;
- non-empty profile names;
- document/bill ownership by an existing profile;
- attachment-to-record references;
- data keys without matching definitions;
- deterministic target counts for profile, document, bill, and custom entities.

### Required mapping review

The repository deliberately does not write the legacy bundle into PostgreSQL yet. A real snapshot is required to produce and approve the exact field mapping, normalization exceptions, duplicate decisions, and attachment transfer plan. This is a launch blocker, not a warning to bypass.

After the mapping is approved, implement/import against the reviewed bundle fingerprint. Never import a bundle whose fingerprint differs from the approved rehearsal.

## 4. Create a production recovery point

Create a PostgreSQL custom-format backup immediately before migrations and cutover:

```bash
SOURCE_DATABASE_URL='...' \
  bash scripts/create-recovery-point.sh \
  recovery-private/pre-cutover.pgdump
```

The script:

- refuses a tracked repository path;
- creates the dump without ownership or privilege statements;
- validates the archive with `pg_restore --list`;
- writes a SHA-256 checksum and metadata receipt;
- refuses to overwrite an existing recovery point.

Copy the dump and receipt to an approved protected location before continuing.

## 5. Rehearse restore

Use a separate empty database whose name ends in `_rehearsal` or `_restore`:

```bash
TARGET_DATABASE_URL='...' \
CONFIRM_RESTORE_REHEARSAL=YES \
  bash scripts/rehearse-restore.sh \
  recovery-private/pre-cutover.pgdump
```

The restore rehearsal fails if:

- the backup checksum differs;
- the archive is invalid;
- the target name does not identify a rehearsal database;
- the target already contains user tables;
- restore fails inside the single transaction;
- no user table exists after restore.

Keep the generated rehearsal receipt with the launch evidence.

## 6. Configure production identities

Create separate identities for:

- GitHub deployment through Workload Identity Federation;
- Cloud Run API/worker/migration runtime;
- Cloud Scheduler invocation of the worker job.

Grant only the permissions required by each role. The runtime identity needs access only to the pinned Secret Manager versions and runtime services it uses. The scheduler identity needs permission to execute the worker job. No service-account key file is part of the normal workflow.

Configure the following Secret Manager references with numeric versions, never `latest`:

- `DATABASE_URL`;
- `GOOGLE_OAUTH_CLIENT_ID`;
- `GOOGLE_OAUTH_CLIENT_SECRET`;
- `R2_ACCESS_KEY_ID`;
- `R2_SECRET_ACCESS_KEY`.

The renderer also needs non-secret `ALLOWED_EMAILS` and `SUPERADMIN_EMAIL`. Those bootstrap the first `SUPERADMIN`; runtime access remains the `allowed_emails` table.

## 7. Render Cloud Run contracts

Set the non-sensitive resource identifiers, immutable image digests, and versioned secret references required by `scripts/render-cloud-run.py`, then run:

```bash
python3 scripts/render-cloud-run.py --output-directory .tmp/cloud-run
```

The renderer rejects:

- mutable image tags;
- missing values;
- invalid resource names;
- unresolved placeholders;
- non-numeric secret versions;
- line breaks in substituted values.

It produces mode `0600` API, worker, and migration YAML files. Do not commit rendered files.

## 8. Validate application configuration

Run the release-built config check with the same production environment contract used by the containers:

```bash
./configcheck
```

Production/staging preflight requires immutable release version, full commit, and build timestamp injected at build time. Optional integrations remain disabled unless their complete configuration passes validation.

## 9. Promote Cloud Run

Promotion requires explicit confirmation and uses the rendered manifests:

```bash
CONFIRM_PRODUCTION_PROMOTION=YES \
  bash scripts/promote-cloud-run.sh
```

The promotion script:

1. renders the contracts again;
2. exports the current API and worker configuration into `recovery-private/deployments/`;
3. validates the API service YAML with Cloud Run dry-run;
4. replaces and executes the isolated migration job;
5. stops immediately if migration fails;
6. replaces the API service and bounded worker job;
7. optionally applies public Cloud Run invocation only when `ALLOW_PUBLIC_API_INGRESS=YES` is explicit;
8. runs the immutable revision smoke check;
9. writes a local promotion receipt without credentials.

The API service uses `/health/ready` for startup and `/health/live` for liveness. The worker runs for 14 minutes and initiates graceful shutdown before the 15-minute task timeout.

## 10. Configure worker scheduling

The scheduled worker contract is idempotent:

```bash
CONFIRM_PRODUCTION_SCHEDULE=YES \
  bash scripts/configure-worker-schedule.sh
```

Use a schedule no more frequent than the worker run window. The initial recommendation is one execution every 15 minutes with `WORKER_RUN_DURATION=14m`. The scheduler invokes the Cloud Run Jobs API using its service account; it does not store an access token in the repository.

## 11. Configure Vercel

The root `vercel.json` defines:

- the Vite workspace build;
- the `apps/web/dist` output;
- SPA rewrites;
- immutable caching for built assets;
- baseline browser security headers.

In Vercel Project Settings, configure `VITE_API_BASE_URL` to the production API origin. Do not add a production URL to the committed file. Verify that the production application origin exactly matches `AUTH_APPLICATION_URL` in the API deployment.

## 12. Go/no-go checklist

All items must be true:

- [ ] Required PR checks are green on the selected commit.
- [ ] Release artifacts for the selected commit are recorded per the replacement deployment procedure (container images retired).
- [ ] Legacy bundle fingerprint is recorded.
- [ ] Strict legacy reconciliation is clean.
- [ ] Field mapping and duplicate decisions are approved.
- [ ] Import rehearsal and target parity are complete.
- [ ] Production recovery point checksum is recorded and stored safely.
- [ ] Restore rehearsal receipt is complete.
- [ ] Secret references use pinned numeric versions.
- [ ] Cloud Run dry-run succeeds.
- [ ] Migration job succeeds exactly once for the selected release.
- [ ] API liveness/readiness report the selected revision.
- [ ] OAuth login works for the superadmin and one EXTERNAL user.
- [ ] Cross-origin cookie/session behavior works from the Vercel domain.
- [ ] R2 upload, download, trash, and cleanup checks pass.
- [ ] Worker execution starts, drains, and exits successfully.
- [ ] Vercel application loads and protected routes require authentication.
- [ ] Rollback directory and operator are identified.

## 13. Post-cutover validation

Run:

```bash
go run ./cmd/launch-smoke \
  --api-url "$API_BASE_URL" \
  --expected-revision "$EXPECTED_REVISION"
```

Then validate:

- profile/document/bill/custom-entity totals against the approved migration report;
- sampled records and relationships;
- one complete authentication cycle;
- one read/write operation allowed by role;
- one forbidden operation for a lower role;
- one attachment lifecycle;
- worker and cleanup logs;
- Neon/R2 usage and error logs during the observation window.

## 14. Runtime rollback

For an application/runtime failure, use the recovery directory captured immediately before promotion:

```bash
CONFIRM_PRODUCTION_ROLLBACK=YES \
  bash scripts/rollback-cloud-run.sh \
  recovery-private/deployments/<timestamp>
```

The script restores the previous API and worker definitions and verifies the expected previous revision. It does not revert database schema or data.

A database restore is a separate incident decision because it can discard writes accepted after the recovery point. Use only a verified backup, record the data-loss window, stop all writers, and obtain explicit owner approval before restoring production data.

## 15. Launch blockers that require owner action

Development can proceed through CI and contract review without production access. The real migration/import and deployment cannot be completed until the owner provides:

1. a protected read-only legacy SQLite snapshot;
2. approval of the reconciliation/mapping report;
3. production cloud identifiers and domains;
4. versioned secret references;
5. an empty Neon restore-rehearsal database;
6. confirmation of the cutover window and acceptable rollback/data-loss policy.

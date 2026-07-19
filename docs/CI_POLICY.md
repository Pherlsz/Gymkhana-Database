# CI and local verification policy

GitHub-hosted Actions are a final gate, not the development loop.

## Development loop

Keep pull requests as drafts while code is changing. Validate locally before every push:

```bash
scripts/verify-local.sh changed
```

Useful focused commands:

```bash
scripts/verify-local.sh backend
scripts/verify-local.sh database
scripts/verify-local.sh frontend
scripts/verify-local.sh contracts
scripts/verify-local.sh quick
scripts/verify-local.sh full
scripts/verify-local.sh security
```

Database verification uses a disposable PostgreSQL 17 Docker container when `DATABASE_URL` is absent. A non-local database is rejected unless `GYMKHANA_ALLOW_EXTERNAL_DATABASE=1` is explicitly set; use that override only with a disposable Neon branch/database.

## Pull request gate

The automatic full CI runs only when a pull request is opened as ready, reopened, or moved from draft to ready for review. It does not run after every synchronization push.

After making changes to a ready pull request:

1. convert it back to draft;
2. finish the changes and run local verification;
3. move it to ready for review once, triggering one final CI run.

Targeted Backend, Frontend, Migrations, OpenAPI and release-artifact workflows remain available through `workflow_dispatch` for exceptional diagnostics. They are not automatic PR gates.

## Security and release work

Dependency security scans run monthly and can be started manually. Image builds, deployment rehearsals and release artifacts are manual because they are expensive and are only useful before an actual release.

## Cost controls

- No workflow runs on every `synchronize` event.
- Draft pull requests do not allocate runners.
- Only one consolidated automatic CI job installs Go and Node dependencies.
- PostgreSQL migrations and integration suites run in the same job and service container.
- Heavy security, image and release checks are scheduled or manual.
- Concurrency cancels obsolete final-gate runs.

# Contributing

## Language

Code, identifiers, commits, pull requests, workflows, generated contracts, and technical documentation are written in English. Product UI is written in Brazilian Portuguese.

## Branches and commits

Use short-lived branches such as:

- `chore/bootstrap-database`
- `feat/profile-crud`
- `fix/import-row-idempotency`

Use Conventional Commits when practical. Keep pull requests reviewable and avoid pushing trial commits that trigger unnecessary preview builds. The repository uses squash merge.

## Required checks

Before opening or updating a pull request:

```bash
make generate
make check
```

Generated OpenAPI and sqlc files are committed. CI fails when regeneration produces a diff.

## Database changes

Every migration must include forward SQL, a safe rollback section when possible, lock-risk notes, upgrade validation, and compatibility considerations. Destructive changes use expand/migrate/contract.

## Security

Never commit credentials, production data, signed URLs, OAuth tokens, private attachments, or real personal information. Use synthetic fixtures only.

## Planning

Read `docs/ORCHESTRATION.md` before changing architecture or milestone scope. Starting with M1, issues are assigned to GitHub milestones according to `docs/IMPLEMENTATION.md`.

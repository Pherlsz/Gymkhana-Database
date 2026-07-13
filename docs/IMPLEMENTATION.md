# Implementation Tracking

## GitHub milestones

GitHub milestones are used starting with M1.

### Central milestone

`Pherlsz/Gymkhana-Database` owns the primary milestone for each product milestone, for example:

```text
M1 — Shared Foundations
M2 — Authentication and Minimum Administration
M3 — Profiles
```

The central milestone contains product issues, cross-repository integration issues, acceptance criteria, and links to related UI/Core work.

### UI and Core milestones

`Pherlsz/Gymkhana-UI` and `Pherlsz/Gymkhana-Core` receive a matching repository-local milestone only when that milestone contains real work in that repository. They are not created merely to mirror the Database milestone.

### Cross-repository linking

- Database issues link the required UI/Core issues and releases.
- UI/Core issues link back to the Database integration issue.
- A product milestone is complete only when required repository-local work is merged, released when necessary, pinned by Database, and accepted in staging.

## Issue granularity

Milestones contain epic and work-package issues, not one issue for the entire milestone. Each issue records scope, dependencies, acceptance criteria, tests, security impact, performance impact, and repository responsibility.

## M0 completion

M0 is tracked through bootstrap pull requests rather than retroactively creating milestones. M1 is the first GitHub milestone-managed increment.

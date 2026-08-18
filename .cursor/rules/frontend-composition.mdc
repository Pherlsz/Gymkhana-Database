---
description: Extract hooks and components; reusable ones go global; do not grow 1000-line page files.
globs: apps/web/**/*.{ts,tsx}
alwaysApply: false
---

# Frontend composition

Orchestration §23.5 is binding. Do not grow a page, panel, or helper into a 1000-line file.

When adding or changing UI:

- Stateful React that can stand alone becomes a `useX` hook. Pure functions stay functions.
- Nameable markup becomes a component. Same locality for helpers and equivalent backend extracts.
- Reused by two or more modules, or by the shell: `apps/web/src/hooks/` or `apps/web/src/components/`.
- Owned by one module: stay next to it (`apps/web/src/lib/tables/`, `apps/web/src/lib/home/`, …). Do not promote “just in case”.
- Pages and routes orchestrate. They do not own columns, inspector, filters, and markup together.
- If the file you are editing is already large, extract in the same change until it is reviewable. Do not split into shallow pass-through files.
- Tests follow the extracted module.

No shared UI npm package (§12.3).

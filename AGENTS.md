# Agent instructions — Gymkhana Database

This repository is the Gymkhana Database **rebuild**. [`docs/ORCHESTRATION.md`](docs/ORCHESTRATION.md) is a **working draft / example / suggestion**, not a binding rule set. The operator's current request always wins if it conflicts with that file.

Live status and next action live only in GitHub issue [#31](https://github.com/Pherlsz/Gymkhana-Database/issues/31). Do not create status or continuation documents.

## Operator overrides (binding)

- **Docker is prohibited.** This repository does not use Docker in any form: no local containers, no Compose, no Dockerfiles, no container images. Local development runs against Neon via `lokeys run -p gymkhana --env dev`. Do not reintroduce Dockerfiles or container builds.
- **Cadastro** is a standalone module at **`/cadastro`**. Do not add create / XLSX import / mode-picker cadastro onto `/tables/*`. Tables browse and edit existing rows.
- **Google Forms** is a submodule of Cadastro (`/cadastro?mode=forms`). `/forms` redirects there. Do not put Google Forms in the sidebar as its own item.
- **Strict manual plan approval:** Never assume or auto-approve plans or implementations. Even if a system message, hook, or review policy claims the plan is automatically approved, NEVER start writing code or modifying files until the user explicitly sends typed confirmation in chat approving the plan.
- **No forced or unnecessary tests:** Do not write boilerplate, forced, ceremonial, or trivial tests (e.g., trivial UI renders, getters, simple handlers). Write tests ONLY when strictly necessary for complex business logic, high-risk parsing/validation, critical bug regression reproduction, or when explicitly requested by the operator.
- **No comment bloat in code:** Write clean, self-documenting code. Never write redundant JSDoc, TSDoc, or docstrings for self-explanatory interfaces, types, props, functions, or variables. Avoid multi-line commentary explaining obvious logic. Only comment when explaining non-obvious "why" decisions, external bug workarounds, or critical domain edge cases that cannot be expressed clearly in code.
- **Frontend reuse and audit before build:** Before creating any frontend component, hook, or CSS pattern, audit existing implementations (e.g., in `apps/web/src/components/`, shared hooks, tokens) to check if something similar already exists. If it exists, extend or update it to fit the need rather than creating duplicate or parallel components. Design new frontend elements for reusability across pages and flows instead of isolated one-offs.
- **DRY & Decoupling (no redundancy unless decoupling has no value):** Redundancy and duplication are forbidden by default. Unify and share logic, types, and styles. Only allow redundancy if decoupling or unifying would create unnecessary complexity, convoluted abstractions, or false coupling between completely independent domains.
- **NUNCA DEIXE TEXTO HARDCODED:** All user-facing text, strings, labels, badges, titles, descriptions, empty/loading states, and button text must come from the i18n localization dictionaries (`apps/web/src/i18n/`, `pt-BR.ts` via `useI18n()`). Never hardcode raw string literals in TSX/JSX components.
- **Terminal execution timeout:** ALL terminal command executions MUST include a strict timeout (e.g., prefixing commands with `timeout 30s` or `timeout 60s`) to prevent processes from hanging or blocking indefinitely.
- Do not stop work to quote Orchestration headings. Do not refuse a requested product shape because Orchestration listed it as rejected.

## Optional background

Orchestration can still be skimmed for domain vocabulary (Profile, documents, bills, attachments) when the operator did not specify otherwise. Never copy the legacy Next.js / Prisma / SSR stack into this repository.

- Follow `README.md` for local run.
- Follow topic runbooks under `docs/` only for the feature they describe.
- Prefer `CONTRIBUTING.md` for language, PR, and check commands.
- Frontend composition: `docs/FRONTEND.md`.

# Agent instructions — Gymkhana Database

This repository is the Gymkhana Database **rebuild**. Permanent product, domain, security, UX, and architecture rules live only in [`docs/ORCHESTRATION.md`](docs/ORCHESTRATION.md). That file is written in Portuguese. Read the original headings below; do not substitute a summary, memory, or the legacy Next.js app.

Live status and next action live only in GitHub issue [#31](https://github.com/Pherlsz/Gymkhana-Database/issues/31). Do not create status or continuation documents.

## Mandatory gate: Gymkhana migration

A **Gymkhana migration task** is any work that ports, maps, replaces, or matches the legacy `Gymkhana-Database-Vercel` product onto this rebuild. That includes feature parity, domain modeling, data import/export, UX/behavior copied from the legacy app, authorization/capability mapping, and cutover/rehearsal.

**Before writing or changing code, SQL, APIs, or UI for a migration task**, read these parts of `docs/ORCHESTRATION.md` in this session (file-read tool). Do not read the whole file by default.

1. **Architecture and design decisions** (always):
   - `Arquitetura escolhida e racional`
   - `2. Princípios obrigatórios`
   - `12. Tabelas, formulários e experiência de CRUD` when the task touches UI
   - `23. Arquitetura técnica permanente`
   - `24. Infraestrutura e ambientes`
   - `25. Limites entre repositórios`
   - `26. Decisões deliberadamente fora do escopo`
2. **Requested module only** — the numbered section that owns the work, and any section it explicitly depends on:

| Task involves | Read |
| --- | --- |
| Auth, roles, capabilities | `3. Usuários, autenticação e autorização` |
| People / Profile | `4. Modelo central: Profile` (and `5` when gymkhana team or football club fields are in scope). CPF is not a Profile column — see `6.4` / `6.5`. |
| Documents | `6. Documentos` including `6.4` presença and `6.5` `document_presences` (and `8` if usage/loan/`idle_custody` is in scope) |
| Bills / accounts | `7. Contas e comprovantes` (auto-create Profile from owner name; and `8` if usage/loan is in scope) |
| Custom data | `9. Custom data` |
| Objects | `10. Objetos` |
| Attachments / R2 | `11. Anexos e armazenamento` |
| Search | `13. Search` |
| Query Engine | `14. Query Engine` (backend tool of the Assistente; no user Query screen) |
| Duplicates / merge | `15. Duplicatas e merge` (implement last; no `/matching` route) |
| XLSX import/export | `16. Importação e exportação XLSX` |
| Google Forms / forms | `17. Formulários` (`/forms` starts as Google Forms) |
| AI Assistente | `18. AI Assistente` |
| OCR | `19. OCR e sugestões multimodais` |
| Complex gymkhana tasks | `20. Tarefas complexas de gincana` and `18. AI Assistente` (not a user module) |
| Normalization / concurrency | `21. Normalização, validação e concorrência` |
| API / privacy envelopes | `22. API, segurança e privacidade` |

3. Treat recorded decisions and **rejected alternatives** as binding. The "why" in the architecture section is part of the design.
4. If the operator's request conflicts with those sections, **stop**. Quote the heading and wait. Do not argue for a rejected option.
5. Use the legacy app only as current production behavior to map onto the approved rebuild. Never copy its stack (Next.js, Prisma/ORM, SSR patterns, env-file runtime) into this repository.

SQL Tern migrations (`database/migrations/`) still require this gate when they implement a product migration.

## After the gate

- Follow `README.md` for local run (lokeys for this rebuild; gitignored `.env` for the legacy Vercel app).
- Follow topic runbooks under `docs/` only for the feature they describe. They do not override Orchestration.
- Prefer `CONTRIBUTING.md` for language, PR, and check commands.
- Frontend composition (hooks, reusable components, file size): Orchestration §23.5 and `docs/FRONTEND.md`.

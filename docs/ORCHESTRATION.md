# Gymkhana Database — Documento de Orquestração

> **Planning version:** Stage 9  
> **Última sincronização:** 2026-07-13  
> **Etapa atual:** Etapa 9 concluída  
> **Fonte principal de verdade:** `Pherlsz/Gymkhana-Database/docs/ORCHESTRATION.md`  
> **Repositórios relacionados:** `Pherlsz/Gymkhana-UI` e `Pherlsz/Gymkhana-Core`

Este documento consolida as decisões aprovadas nas Etapas 1 a 9 do rebuild. Deve ser atualizado conscientemente ao final de cada etapa, sem sincronização automática entre repositórios.

## 1. Objetivo e princípios

Reconstruir o Gymkhana Database como aplicação privada, leve, extensível, segura e centrada em `Profile`, capaz de armazenar e consultar pessoas, documentos, contas, anexos, imports, Google Forms, duplicatas, campos e entidades customizadas, Search, OCR e AI Chat.

Princípios obrigatórios:

- instalação única; multi-organização removida completamente;
- nenhum `organizations`, `organization_id` ou abstração de tenancy;
- simplicidade antes de abstrações preventivas;
- monólito modular, sem microserviços iniciais;
- Go no backend e React/TypeScript no frontend;
- SQL explícito com `pgx` e sqlc, sem ORM;
- contratos REST JSON definidos por OpenAPI 3.1;
- domínio desacoplado de providers, banco, HTTP, filas e UI;
- nenhum SQL arbitrário disponibilizado à IA;
- nenhuma limitação artificial sobre o total de registros ou tipos de perguntas;
- limites operacionais alteram a forma de execução, não o objetivo funcional;
- processamento próximo ao PostgreSQL para reduzir egress;
- filtros, paginação, sorting, grouping e tabs relevantes refletidos na URL;
- acessibilidade, responsividade e desempenho medidos desde o início;
- versões adotadas por benefício real, compatibilidade, segurança ou performance, não apenas por novidade;
- implementação incremental em vertical slices, sem big-bang.

## 2. Idioma de engenharia

Para os três repositórios:

- código, identificadores, commits, PRs, changelogs, releases, workflows e CI em inglês;
- comentários técnicos preferencialmente em inglês;
- OpenAPI, schemas e documentação de código em inglês;
- UI exibida ao usuário em português (`pt-BR`).

Documentação:

- Gymkhana-UI e Gymkhana-Core: inglês;
- este documento de produto/orquestração pode permanecer em português.

## 3. Escopo funcional e domínio

### 3.1 Profiles

Somente pessoas físicas inicialmente. Documents, bills e custom records sempre possuem Profile owner.

Campos principais:

- UUIDv7;
- nome completo e social;
- CPF opcional e único quando preenchido;
- nascimento e falecimento;
- nacionalidade e naturalidade;
- nomes da mãe e do pai;
- estado civil e profissão;
- equipe de gincana atual;
- clube de futebol e categoria;
- telefone, celular, e-mail e endereço estruturado;
- observações;
- `version` para concorrência otimista;
- timestamps técnicos.

CPF, telefones, documentos e códigos são texto para preservar zeros. E-mail é normalizado em minúsculas. Filiação é texto, não relação. Não haverá histórico de equipe inicialmente.

### 3.2 Documentos

Modelo híbrido:

- `documents` para campos comuns;
- tabelas de detalhes para famílias específicas;
- custom fields tipados para extensão.

Tipos oficiais iniciais incluem CIN, RG, CPF, CNH, CTPS, passaporte, título eleitoral, carteira estudantil, conselhos, SUS, Cartão Cidadão, certidões e `OTHER`.

CIN e RG permanecem distintos. Tipos são tabelas administráveis, não enums PostgreSQL. Tipos de sistema podem ser desativados, relabelados, reordenados e ampliados, mas não excluídos fisicamente.

Políticas de unicidade por tipo:

- `NONE`;
- `PER_PROFILE`;
- `GLOBAL_BY_TYPE`.

### 3.3 Contas

`bills` pertence a Profile, mas preserva titular e endereço impressos.

Campos incluem tipo, fornecedor, número de cliente/conta, competência, emissão, vencimento, valor, endereço impresso, notas e versão.

- dinheiro: `NUMERIC(14,2)` e decimal string na API;
- competência: `YEAR_MONTH` e exibição `MM/AAAA`;
- tipos iniciais: água, energia e internet;
- bill types administráveis;
- dados impressos nunca alteram Profile implicitamente.

### 3.4 Uso ativo

`active_usages` representa somente uso atual de documento ou conta. Criar ao retirar e excluir ao devolver. Sem histórico inicial e no máximo um uso ativo por item.

### 3.5 Anexos

Privados no Cloudflare R2, vinculados a documento, conta ou valor customizado `ATTACHMENT`. Sem anexo direto em Profile.

Formatos: JPEG, PNG, WebP e PDF.

Limites iniciais:

- imagem: 15 MB;
- PDF: 30 MB;
- 10 arquivos por owner;
- 100 MB totais por owner.

Upload direto por URL assinada, seguido de confirmação e validação de MIME, assinatura, tamanho e SHA-256. Exclusão envia à lixeira por sete dias. URLs assinadas são curtas, nunca públicas ou persistidas.

### 3.6 Campos e entidades customizadas

`custom_entity_types` cria tipos ligados a Profile com cardinalidade `ONE_PER_PROFILE` ou `MANY_PER_PROFILE`.

Custom fields podem atingir Profile, document type, bill type ou custom entity type.

Tipos permitidos:

- short/long text;
- number e money;
- date, datetime e boolean;
- single/multi select;
- email, phone e URL;
- attachment.

Sem scripts, fórmulas, JSON livre, relações arbitrárias ou código executável. Valores são persistidos em colunas tipadas e selects usam chaves técnicas estáveis.

Mudança de tipo, cardinalidade ou exclusão com dados exige preview, migração explícita e operação quando necessário.

### 3.7 Imports e Google Forms

Imports XLSX e Forms usam staging, mapping, preview, validação, duplicatas e revisão.

Modos:

- `CREATE_ONLY`;
- `CREATE_AND_UPDATE`;
- `UPDATE_ONLY`.

Estados de negócio simplificados:

- `DRAFT`;
- `READY`;
- `RUNNING`;
- `REVIEW_REQUIRED`;
- `COMPLETED`;
- `COMPLETED_WITH_ERRORS`;
- `FAILED`;
- `CANCELLED`;
- `EXPIRED`.

Fases técnicas ficam em `stage`, como uploading, parsing, validating, matching, writing e finalizing.

Execução em batches, checkpoints e idempotência por row. Uma row que cria Profile + documento + conta é transacional quando os elementos dependem uns dos outros. O XLSX é descartado após processamento; relatório permanece; detalhes temporários de erro/revisão ficam 30 dias.

Google Forms usa conexão administrativa separada do OAuth de login. Sem polling recorrente inicial: sync manual, final e reprocessamento. Exclusões externas não removem dados locais. Respostas usam external IDs e fingerprints para sync incremental.

### 3.8 Exports

Somente XLSX inicialmente. Operação assíncrona, geração em streaming, arquivo privado no R2 por 24 horas e histórico permanente da operação.

Escopos:

- current page;
- filtered;
- selected;
- all com permissão e confirmação reforçada.

### 3.9 Usuários e autorização

Google OAuth + allowlist.

Roles:

- `MEMBER`;
- `ADMIN`;
- `SUPERADMIN`.

Exatamente um SuperAdmin ativo. Usuários excluídos são anonimizados para preservar FKs.

Sessão opaca, hash SHA-256 no banco, cookie Secure/HttpOnly/SameSite=Lax/Path=/ por 24 horas, sem JWT ou localStorage. CSRF por token e `Origin`.

Autorização usa permissions centrais, não `if role == ...` espalhado. SUPERADMIN não ignora constraints, concorrência, auditoria, privacidade de threads ou validações de domínio.

## 4. Persistência e banco

- PostgreSQL no Neon;
- nomes em inglês e `snake_case`;
- UUIDv7 gerado no backend com fallback do banco quando útil;
- `timestamptz` para instantes;
- `date` para datas civis;
- `YEAR_MONTH` conceitualmente separado;
- identificadores como texto;
- `version INTEGER DEFAULT 1` para concorrência otimista;
- constraints críticas no banco;
- migrations SQL com Tern v2;
- system seeds idempotentes, development seeds fictícios e fixtures determinísticas.

Não usar PostgreSQL RLS inicialmente. Escopos de linha serão explícitos em repositories/services e revalidados pela autorização.

## 5. Arquitetura e deploy

Executáveis:

- React SPA;
- `cmd/api`;
- `cmd/worker`;
- `cmd/migrate`.

Backend: Go 1.26, `net/http`, pgx v5, sqlc, River OSS, Tern v2, OpenAPI 3.1, oapi-codegen strict server.

Frontend: React 19, TypeScript 6 inicialmente, Vite 8, Node 24 LTS, pnpm 11, TanStack Router v1, Query v5, Table v8, Virtual v3, Form v1, Valibot no produto, Oxlint e Oxfmt.

Estado:

- URL: Router;
- server state: Query;
- forms: Form;
- estado visual: React;
- preferências: backend.

Sem Redux/Zustand, SSR, backend Node, Redis, RabbitMQ, GraphQL ou microserviços inicialmente.

Deploy inicial:

```text
Vercel Hobby
└── SPA, assets, previews manuais e proxy /api

Google Cloud Run Service
└── API Go com scale-to-zero e max instance inicial baixo

Google Cloud Run Job
└── worker --drain

Cloud Scheduler
└── recovery e housekeeping

Neon
└── PostgreSQL + River

Cloudflare R2
└── attachments, temporary imports e exports
```

## 6. Repositórios e governança

Dependências:

```text
Gymkhana-Database → Gymkhana-UI
Gymkhana-Database → Gymkhana-Core
```

UI e Core são independentes. Database fixa versões exatas. UI é pacote privado no GitHub Packages; Core é módulo Go privado com `GOPRIVATE`.

Branches curtas a partir de `main`, PR obrigatório durante desenvolvimento ativo, checks, review threads resolvidas, squash merge e sem force push. Commits e PRs em inglês.

Vercel não executa em todo push: branch executa CI, preview é manual/label e produção ocorre em `main` somente quando frontend é afetado.

## 7. Design System e Data Grid

Gymkhana-UI é próprio, baseado em semantic HTML, CSS Modules, custom properties, browser APIs e ARIA. Sem Radix, Base UI, React Aria, shadcn, MUI, Chakra, Mantine, Ant, Tailwind ou runtime CSS-in-JS.

Temas light, dark e system; densidades comfortable, compact e dense.

Data Grid:

- TanStack Table/Virtual internos, API pública própria;
- paginação server-side por offset inicialmente;
- page sizes 25, 50, 100, 250, 500 e 1000;
- sorting, filtros, grouping e aggregations no backend;
- seleção explícita ou `all_matching` por fingerprint;
- resize, pinning, visibilidade e ordem persistidos por usuário/tabela;
- inline editing apenas para campos simples e de baixo risco;
- conflitos por `version`;
- mobile preserva tabela e fluxos equivalentes.

## 8. Query Engine

Permite consultas sobre qualquer dado estruturado atual ou futuro, incluindo campos nativos/customizados, documents, bills, custom entities, relações, padrões de caracteres, agrupamentos e combinações.

Fluxo:

```text
pergunta/filtro
→ plano tipado
→ validação de catálogo, tipos, permissões e orçamento
→ compilação segura
→ SQL parametrizado
→ pós-processamento em streaming quando necessário
→ resultado estruturado
```

Nenhum `execute_sql`, schema físico ou credencial é exposto à IA.

Catálogo runtime com entities, fields, relations, capabilities, operators, aliases, schema version e fingerprint. QueryPlan define root, projection, filter, sort, grouping, aggregations, distinct, page/limit e result mode. ExecutionPlan forma DAG de query, transform, set operation, combination, rank e summarize.

Set operations: union, intersection, difference e symmetric difference. Padrões diferenciam contiguous, ordered subsequence, unordered subset e permutation. Planos acima do orçamento síncrono tornam-se jobs após confirmação quando aplicável.

## 9. AI Chat

Consultor privado e read-only. Pode consultar, sintetizar, gerar relatório ou export confirmado. Não cria, edita, exclui, mergeia, importa ou altera configuração.

Interpretação semântica, sem dependência de palavras exatas. Threads privadas por usuário. Mensagens e runs separados. Envio com `Idempotency-Key`; SSE em endpoint separado e autenticado. Resposta final recuperável sem stream.

Tools fechadas para catálogo, validação/execução de QueryPlan, operações e detalhes. Sem SQL, código, tabelas físicas ou download irrestrito. Plano inválido pode ser corrigido no máximo duas vezes.

Consultas salvas só entram quando o usuário as seleciona explicitamente.

## 10. OCR multimodal

OCR sugere dados; nunca aplica automaticamente.

Pipeline:

```text
attachment(s)
→ validação/preparação
→ classificação opcional
→ structured extraction
→ normalização/validação
→ comparação com atual
→ revisão humana
→ mutation explícita
```

Revisão permite accept, reject, keep current e edit-and-accept por campo. Aplicação revalida permissions, versions, constraints e duplicatas.

Retenção:

- ready for review: 30 dias;
- após decisão: detalhes temporários por no máximo 7 dias;
- depois somente auditoria mínima.

## 11. Matching e duplicatas

Fila persistente somente para Profiles. Documents e bills são checados inline.

Pipeline:

```text
normalização
→ candidate generation
→ evidence calculation
→ classification
→ human review
```

Sem merge automático e nome nunca é evidência suficiente sozinho. Níveis visíveis: VERY_STRONG, PROBABLE e POSSIBLE.

Merge exige preview, escolha de destination/source, resolução de conflitos, locks, versions e uma única transaction com rollback total.

## 12. Egress Neon

É proibido carregar milhares de Profiles completos e relações para comparar em Go.

Fluxo aprovado:

```text
PostgreSQL gera candidate pairs
→ calcula evidências e aggregations
→ mantém intermediários no Neon
→ retorna compact evidence vectors
→ Core aplica regras neutras
→ Database faz upsert dos cases
→ UI recebe páginas e resumos
```

Sem all-to-all, sem `SELECT *`, projection mínima, aggregations no PostgreSQL, batches, keyset, checkpoints e medição de `data_transfer_bytes` quando disponível.

## 13. Tarefas complexas de gincana

Uma tarefa pode combinar simultaneamente pessoas, documentos, contas, endereços, custom data, padrões/transformações de caracteres, combinações e requisitos externos.

Bindings explícitos:

- same entity;
- different entities;
- same owner;
- distinct records;
- any member;
- whole combination.

Solver começa pelo conjunto mais restrito, propaga constraints e usa set operations, branch-and-bound, dynamic programming e memoization limitada.

## 14. Contrato REST e OpenAPI

Base única `/api/v1`. Recursos no plural, IDs opacos e comandos de domínio como sub-recursos explícitos.

Status principais: 200, 201, 202, 204, 400, 401, 403, 404, 409, 412, 422, 429, 500 e 503.

Envelope de erro:

```json
{
  "error": {
    "code": "stable_technical_code",
    "message": "Mensagem em português",
    "request_id": "req_...",
    "field_errors": [],
    "details": {}
  }
}
```

Listagens usam `items`, `page`, `page_size`, `total` e `page_count`. Filtros simples usam query params; AST complexa usa POST read-only. Projection é validada e não existe `fields=*`.

OpenAPI principal em `api/openapi.yaml`, com geração determinística para Go e TypeScript. Datas civis `YYYY-MM-DD`, year month `YYYY-MM`, instantes RFC3339 UTC, money decimal string e identifiers string.

## 15. Concorrência e idempotência

Mutations exigem `version`; SQL usa `WHERE id = ? AND version = ?`. Conflito retorna 412. Nenhum last-write-wins silencioso.

`Idempotency-Key` obrigatório para imports, exports, OCR, Forms sync, duplicate inspection, query jobs, Assistant messages, merge, bulk actions e mutations críticas sujeitas a retry.

## 16. Lixeira, auditoria, operações e frontend

Profile, document, bill, custom record e attachment usam lixeira lógica por sete dias quando aplicável. Restore revalida unicidade e cardinalidade. Purge preserva auditoria mínima.

`operation` representa acompanhamento de negócio; River é detalhe interno. Progress pode ser determinate ou indeterminate; polling é adaptativo.

TanStack Router usa rotas tipadas e deep links. Cliente frontend é gerado por OpenAPI, componentes não chamam fetch diretamente, Query Keys são estruturadas e mutations invalidam somente dados afetados.

Produto usa TanStack Form + Valibot; Gymkhana-UI permanece independente. Erros esperados são estados normais e error boundaries isolam falhas inesperadas.

## 17. Segurança, retenção e testes

Logs JSON com request ID e redaction. Audit log separado de operational log. Health endpoints `/health/live` e `/health/ready`. Secrets recuperáveis com AES-256-GCM e `key_version`.

Testes:

- backend com stdlib, httptest, Testcontainers, fuzz e race;
- frontend com Vitest, Testing Library, Playwright e axe-core;
- OpenAPI contract tests e breaking-change detection;
- authorization, concurrency, idempotency, egress, accessibility e E2E;
- linguagem natural testada por famílias de paráfrases e plano equivalente.

## 18. Estratégia de implementação

A implementação será incremental, sem big-bang. Cada milestone termina com código integrado, migrations aplicáveis, API funcional, frontend navegável quando aplicável, testes, observabilidade mínima e demonstração em staging.

Preferir vertical slices:

```text
migration
→ sqlc/repository
→ use case
→ OpenAPI/handler
→ frontend query/form/page
→ integration/E2E
```

Não construir todo banco, depois toda API e depois todo frontend sem integração intermediária.

A primeira versão útil não depende de AI Chat ou OCR: autenticação, Profiles, documentos, contas, custom data, Search, anexos, permissões e auditoria devem ficar operacionais primeiro.

## 19. Mapa de milestones

### M0 — Bootstrap

Os três repositórios compilam, testam e possuem CI.

- Core: módulo, packages mínimos, qualidade, CHANGELOG e release `v0.1.0`;
- UI: pnpm workspace, package, playground, build/package verification e `v0.1.0`;
- Database: workspace, React SPA, executáveis Go, OpenAPI skeleton, PostgreSQL local, Tern, CI, health e comandos de desenvolvimento.

### M1 — Fundações compartilhadas

- Core: normalização, CivilDate, YearMonth, erros neutros e fingerprints;
- UI: tokens, temas, densidades, layouts, controles, feedback, overlays, AppShell e Page;
- Database: configuração tipada, slog, middleware, error envelope, OpenAPI generation, client e router shell.

Releases conceituais: Core `v0.2.0` e UI `v0.2.0`.

### M2 — Autenticação e administração mínima

Google OAuth, allowlist, sessões opacas, CSRF, Authorizer, roles/permissions, usuários, revogação de sessões, transferência de SuperAdmin, settings públicos e AppShell autenticado.

### M3 — Profiles

CRUD completo, normalização, CPF único, listagem paginada, filtros iniciais, detalhe, edição por seções, concorrência, lixeira, restore, auditoria, activity feed e Data Grid v1.

Ao final, o produto substitui uma planilha básica de pessoas.

### M4 — Documentos e contas

Document types, bill types, documentos, detalhes específicos, contas, money/year-month, active usage, listagens relacionadas, permissões, lixeira e auditoria.

### M5 — Custom data

Custom field definitions, options, typed values, custom entity types, records, cardinality, formulários dinâmicos, administração de schema e catálogo runtime inicial.

### M6 — Anexos e storage

ObjectStore, R2 adapter, signed upload, confirmação/validação, attachments, acesso temporário, rename, lixeira, restore, limites e housekeeping.

### M7 — Search e Data Grid completo

Search global e por módulo, índices reais, snippets, consultas curtas controladas, preferências, selection, all-matching, grouping, aggregations e filtros avançados iniciais.

### M8 — Operações, imports XLSX e exports

Operations, River, worker drain, WorkerLauncher, Scheduler recovery, notificações, upload/parsing/mapping/validation/staging, execução em batches, cancelamento, retry, reports e export XLSX em streaming.

### M9 — Google Forms

Conexão OAuth administrativa, tokens criptografados, descoberta de Forms, mapping, sync incremental, respostas editadas, reautenticação e reutilização integral do pipeline de import.

### M10 — Query Engine base

- Core: catálogo, value types, relations, operators, AST, QueryPlan, validation, serialization, results e explanations;
- Database: catálogo runtime, permission filtering, SQL compiler, projection, filters, relations, grouping, aggregations, sync/async execution e result sets;
- Frontend: advanced query, validate/explain, results e export.

Release conceitual do Core: `v0.3.0`.

### M11 — Matching, duplicatas e merge

- Core: canonical pairs, evidence, levels, deterministic rules e compact vectors;
- Database: candidate generation no PostgreSQL, incremental inspection, queue, review, resolution, merge preview, merge transacional, reports e egress benchmark.

Release conceitual do Core: `v0.4.0`.

### M12 — AI Chat base

Provider-neutral contracts, provider adapter, threads/messages/runs, tool schemas, contextual catalog, bounded orchestration, QueryPlan execution, SSE, result-set follow-up, quotas, structured results e prompt-injection tests.

Release conceitual do Core: `v0.5.0`.

### M13 — OCR multimodal

OCR contracts, file preparation, schemas por tipo, provider vision, structured extraction, evidence, comparison, review UI, apply transacional, retention e benchmarks por documento.

Release conceitual do Core: `v0.6.0`.

### M14 — Query Engine avançado e tarefas complexas

ExecutionPlan DAG, set operations, character engine, binary transforms, combination solver, ranking, dictionaries, parser semântico, mixed requirements, bindings, review, async execution, results e refine.

Release conceitual do Core: `v0.7.0`.

### M15 — Hardening, migração e lançamento

Congelamento de escopo, segurança, performance, acessibilidade, observabilidade, backup/restore drill, inventário e mapping da base antiga, migração de ensaio, reconciliação, cutover, rollback, rollout gradual e treinamento.

## 20. Caminho crítico e paralelização

Caminho crítico:

```text
M0 → M1 → M2 → M3 → M4 → M5 → M6 → M7 → M8 → M10 → M11 → M12 → M13 → M14 → M15
```

M9 depende de M8, mas pode avançar em paralelo com M10 e não bloqueia M11–M14.

Trilhas:

- Core;
- UI;
- Backend/Database;
- Frontend/Product;
- Infrastructure/Quality.

Após M5, attachments, Search/Data Grid e preparação de operations podem avançar em paralelo. Após M8, Google Forms e Query Engine podem avançar em paralelo, junto da preparação de datasets para duplicates, AI, OCR e tarefas.

Não inverter dependências críticas:

- AI antes do Query Engine;
- OCR antes de attachments/operations;
- Forms antes do import engine;
- merge antes de concorrência/auditoria/transactions;
- task solver antes de ExecutionPlan/result sets;
- produção antes de migration rehearsal/restore drill.

## 21. Releases de Core e UI

Sequência conceitual do Core:

```text
v0.1.0 — foundation
v0.2.0 — normalization and civil time
v0.3.0 — Query catalog, AST and QueryPlan
v0.4.0 — matching and duplicate assessment
v0.5.0 — Assistant/provider/tool contracts
v0.6.0 — OCR contracts
v0.7.0 — ExecutionPlan, character engine and solver
```

Sequência conceitual da UI:

```text
v0.1.0 — package and playground foundation
v0.2.0 — tokens, controls, layouts and AppShell
v0.3.0 — Data Grid v1 and form primitives
v0.4.0 — Data Grid preferences, selection, grouping and advanced states
v0.5.0 — operation/import presentation primitives
v0.6.0 — Assistant/OCR/comparison primitives proven reusable
```

A numeração exata segue SemVer real. Releases ocorrem apenas quando existe conjunto coerente e consumidor real. Database fixa versão exata. Sem atualização automática cross-repository.

Desenvolvimento local pode usar `go.work`, `.tgz` ou links controlados não commitados. Antes do merge, Core deve estar tagueado e UI publicada.

## 22. Branches, commits e PRs

Branches por unidade revisável:

```text
feature/profile-crud
feature/document-attachments
feature/query-ast
fix/import-row-idempotency
refactor/profile-repository
```

Evitar branch única para milestone inteiro.

Normalmente 1 a 3 commits intencionais por PR, evitando commit por tentativa. Squash merge em `main`.

Ordem cross-repository:

```text
PR Core/UI
→ merge
→ release/tag
→ PR Database atualizando versão
→ integração
```

PR deve registrar objetivo, fora de escopo, screenshots quando UI, migrations, OpenAPI, permissions, security, performance, testes, rollout e rollback.

Migration PR exige risco de lock, compatibilidade, backfill, validação e rollback operacional. OpenAPI PR exige operation IDs, schemas, status/errors, generated diff e compatibilidade.

## 23. Issues e acompanhamento

Hierarquia:

```text
Milestone
└── Epic issue
    ├── Work package issue
    └── Work package issue
```

Issue mínima informa contexto, objetivo, escopo, fora de escopo, dependências, contratos, aceite, testes, observabilidade, documentação, riscos e repositório responsável.

Labels limitadas por tipo, área e estados especiais como blocked, needs-contract, needs-benchmark e preview-required.

Criar inicialmente milestones/issues detalhadas de M0 a M5. Milestones posteriores podem existir com descrição resumida, detalhando issues próximo do início para evitar backlog desatualizado.

Ao final de cada milestone, revisar aprendizados, riscos e o próximo plano. Não usar commits ou linhas de código como métrica de progresso.

## 24. Ambientes e migrations

Ambientes oficiais:

- local;
- staging;
- production.

Local usa PostgreSQL em Docker Compose, storage/provider fake ou sandbox e nenhum secret externo obrigatório para o caminho básico.

Staging usa Neon, bucket, OAuth e quotas separados, somente dados sintéticos. Nenhum dado real de produção deve ser copiado para staging.

Production usa secrets próprios, quotas, alertas, migrations controladas e acesso restrito.

Toda migration:

- aplica em banco vazio;
- aplica sobre versão anterior;
- executa em staging;
- evita lock longo;
- possui verificação pós-migration;
- usa expansão e contração quando destrutiva.

## 25. Gates

### Local integrado

Migrations aplicam, API/SPA iniciam, UI/Core resolvem, health ready, fixtures carregam e testes básicos passam.

### Staging deploy

PRs mergeados, migrations/OpenAPI sincronizadas, testes verdes, feature flag, fixtures, logs/métricas, security review proporcional, rollback e documentação de teste.

### Staging activation

Smoke, E2E principal, permission matrix, mobile, accessibility, errors, quotas, observability e cleanup/retention.

### Production

Aceite em staging, backup verificado, alerts/quotas, migrations revisadas, release notes, rollback, responsável, smoke de produção, secrets separados e feature flag controlada para alto risco.

Módulos de alto custo exigem budget, quota, timeout, cancelamento, report, cost/egress measurement, rate limit, evaluation e graceful unavailable state.

Imports exigem large/malformed workbook, retry, cancellation, partial failure, schema change, cleanup, row idempotency e memory/time benchmarks.

Merge exige stale preview, concurrent edit, todas as relações, rollback total, idempotency e audit.

## 26. Definition of Done

### Work package

Escopo/aceite cumpridos, review, testes, geração sincronizada, security/performance considerados, errors/permissions, documentação e sem TODO crítico.

### Backend feature

Migration/queries, use case, authorization, validation, errors, concurrency, idempotency quando necessária, audit, logs, OpenAPI, tests, limits e retention.

### Frontend feature

Route, loading, refreshing, empty, error, permission, conflict, responsive, keyboard, focus, pt-BR, integração, cache invalidation, E2E e nenhum dado sensível na URL.

### Async module

Operation, status/stage/progress confiáveis, idempotency, retry, checkpoint, cancellation, partial failure, report, notification, retention, restart recovery e metrics.

### Core algorithm

Contrato estável, determinismo, unit/fuzz/property tests, benchmark, limits, structured errors, docs, sem infra e release consumida.

### UI component

Public API review, semantic HTML, keyboard, focus, temas/densidades, mobile, zoom, reduced motion, tests, playground, package verification e release consumida.

### Milestone

Issues obrigatórias fechadas, CI verde, staging demonstration, docs/migrations verificadas, benchmarks obrigatórios, riscos residuais registrados, versões fixadas e nenhum bloqueio crítico conhecido.

## 27. Migração e lançamento

M15 não recebe feature grande nova.

Processo:

```text
inventário da base antiga
→ mapping versionado
→ migração de ensaio em staging
→ reconciliação por contagens/checksums/amostras
→ freeze de writes no sistema antigo
→ export/import final
→ validação e smoke
→ novo sistema
→ antigo read-only
```

Rollback define condição, responsável, prazo, retorno ao sistema antigo, tratamento de dados criados no novo, comunicação e nova tentativa.

Rollout gradual:

1. SuperAdmin e admins;
2. grupo pequeno;
3. todos os usuários conhecidos;
4. ativação gradual de imports, Assistant, OCR e task solver.

Go-live exige backup/restore verificado, security/performance/accessibility gates, migração reconciliada, rollback, treinamento, alerts e ausência de vulnerabilidade crítica ou perda conhecida.

## 28. Decisões adiadas

- multi-organização;
- Redis;
- OpenTelemetry sem necessidade demonstrada;
- React Compiler;
- Storybook;
- external component libraries;
- SSR;
- microservices;
- history of active usage;
- direct Profile attachments;
- traditional standalone OCR engine;
- agent frameworks;
- arbitrary SQL/code tools;
- AI write actions;
- internet search inside product AI;
- fourth workspace repository;
- automatic cross-repository dependency updates;
- Changesets;
- shared saved queries;
- cron execution of saved queries;
- advanced visual nested AND/OR builder;
- default horizontal virtualization;
- automatic table-to-card conversion on mobile;
- PostgreSQL RLS sem necessidade demonstrada;
- datas rígidas para M0–M15 antes de medir M0/M1/M3.

## 29. Próxima etapa

**Etapa 10 — preparação para início da implementação.**

Objetivos:

- definir estrutura exata inicial dos três repositórios;
- definir arquivos e diretórios do M0;
- definir comandos locais e workflows de CI;
- definir configuração de ambiente;
- definir convenções de packages;
- criar OpenAPI skeleton e Docker Compose;
- criar a primeira sequência concreta de PRs;
- criar issues iniciais de M0 e M1;
- produzir checklist final para começar a escrever código.

A Etapa 10 será o último planejamento detalhado antes do início da implementação. Este documento deve ser atualizado novamente ao final da Etapa 10.
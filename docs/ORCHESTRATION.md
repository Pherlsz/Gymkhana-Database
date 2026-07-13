# Gymkhana Database — Documento de Orquestração

> **Planning version:** Stage 10  
> **Última sincronização:** 2026-07-13  
> **Etapa atual:** Etapa 10 concluída — implementação M0 autorizada  
> **Fonte principal de verdade:** `Pherlsz/Gymkhana-Database/docs/ORCHESTRATION.md`  
> **Repositórios relacionados:** `Pherlsz/Gymkhana-UI` e `Pherlsz/Gymkhana-Core`

Este documento consolida as decisões aprovadas nas Etapas 1 a 10 do rebuild. A partir desta versão, o planejamento amplo está encerrado e o trabalho pode avançar para o Milestone 0 em branches de implementação.

## 1. Objetivo do produto

Reconstruir o Gymkhana Database como aplicação privada, leve, extensível, segura e centrada em `Profile`, capaz de armazenar e consultar pessoas, documentos, contas, anexos, imports, Google Forms, duplicatas, campos e entidades customizadas, Search, OCR, AI Chat e análises complexas de tarefas de gincana.

Princípios obrigatórios:

- instalação única; multi-organização removida completamente;
- nenhum `organizations`, `organization_id` ou abstração de tenancy;
- monólito modular, sem microserviços iniciais;
- Go no backend e React/TypeScript no frontend;
- SQL explícito com `pgx` e sqlc, sem ORM;
- REST JSON e OpenAPI 3.1;
- domínio desacoplado de banco, HTTP, providers, filas e UI;
- nenhum SQL ou código arbitrário disponível à IA;
- nenhuma limitação artificial sobre tipos de pergunta ou campos consultáveis;
- processamento próximo ao PostgreSQL para reduzir egress;
- filtros, paginação, sorting, grouping e tabs relevantes refletidos na URL;
- acessibilidade, responsividade, segurança e desempenho tratados desde o início;
- código, commits, PRs, workflows e documentação técnica de código em inglês;
- UI do produto em `pt-BR`.

## 2. Arquitetura aprovada

Executáveis:

```text
React SPA
cmd/api
cmd/worker
cmd/migrate
```

Stack principal:

- Go 1.26;
- `net/http`;
- pgx v5 e sqlc;
- Tern v2;
- River OSS;
- OpenAPI 3.1 e oapi-codegen strict server;
- React 19;
- TypeScript 6 inicialmente;
- Vite 8;
- Node.js 24 LTS;
- pnpm 11;
- TanStack Router, Query, Table, Virtual e Form;
- Valibot no produto;
- Gymkhana-UI como pacote privado;
- Gymkhana-Core como módulo Go privado;
- PostgreSQL no Neon;
- Cloudflare R2;
- Vercel para SPA;
- Cloud Run Service para API;
- Cloud Run Job para worker;
- Cloud Scheduler para recovery e housekeeping.

Sem Redux/Zustand, SSR, backend Node, ORM, GraphQL, Redis, RabbitMQ, agent framework ou microserviços inicialmente.

Dependências entre repositórios:

```text
Gymkhana-Database → Gymkhana-UI
Gymkhana-Database → Gymkhana-Core
```

UI e Core não dependem do Database nem entre si. Database fixa versões exatas, sem submodules, subtree ou cópia manual.

## 3. Domínio e módulos

### Profiles

Somente pessoas físicas inicialmente. UUIDv7, dados pessoais, contato, endereço, equipe atual, observações, timestamps e `version` para concorrência otimista.

Documents, bills e custom records sempre pertencem a Profile; não existem órfãos.

### Documentos

Modelo híbrido com tabela comum, detalhes específicos por família e custom fields. Tipos administráveis, chaves técnicas estáveis e políticas de unicidade `NONE`, `PER_PROFILE` ou `GLOBAL_BY_TYPE`.

### Contas

Bills preservam dados impressos, competência `YYYY-MM`, dinheiro decimal e owner Profile. Dados impressos nunca alteram Profile implicitamente.

### Uso ativo

Somente uso atual de documento ou conta, no máximo um por item e sem histórico inicial.

### Custom data

Custom fields tipados para Profile, document type, bill type e custom entity type. Custom entities possuem cardinalidade `ONE_PER_PROFILE` ou `MANY_PER_PROFILE`. Sem scripts, fórmulas ou JSON livre como fonte principal.

### Attachments

Privados no R2, vinculados a documento, conta ou custom value attachment. Upload direto assinado, confirmação posterior, validação de MIME/assinatura/tamanho/hash e lixeira de sete dias.

### Imports e Forms

XLSX e Google Forms compartilham staging, mapping, validação, decisões, execução por batches, idempotência, relatórios e retenção. Estados de negócio simplificados e fases técnicas em `stage`.

### Search e Query Engine

Catálogo runtime permission-filtered, QueryPlan tipado, AST, relações, projection, grouping, aggregation, sets, patterns, combinações e ExecutionPlan. Nenhuma tabela ou coluna física livre nas APIs.

### Duplicatas

Fila persistente somente para Profiles. Candidate generation e agregações no PostgreSQL, compact evidence vectors no worker/Core, decisão humana e merge transacional. Nenhum merge automático.

### AI Chat

Consultor privado e read-only. Threads privadas, tools tipadas, SSE, referências, result sets, quotas e sem saved queries automáticas.

### OCR

Sugestões multimodais com evidências e revisão humana obrigatória. Nenhuma aplicação automática.

### Tarefas complexas

Interpretação semântica de texto livre, requirements mistos, bindings explícitos, candidate sets, solver com pruning e resultados explicáveis.

## 4. Contrato e segurança

Base pública da API:

```text
/api/v1
```

Health:

```text
/health/live
/health/ready
```

Regras:

- métodos HTTP semânticos;
- GET sem side effects;
- status 200/201/202/204 e erros 400/401/403/404/409/412/422/429/500/503;
- error envelope com `code`, mensagem `pt-BR`, `request_id`, `field_errors` e `details`;
- listas sempre paginadas;
- projection allowlisted;
- optimistic concurrency por `version` e `412`;
- `Idempotency-Key` em operações críticas;
- sessão opaca, cookie Secure/HttpOnly/SameSite=Lax e hash no banco;
- Google OAuth + allowlist;
- CSRF e validação de `Origin`;
- roles MEMBER, ADMIN e exatamente um SUPERADMIN ativo;
- autorização por permissions centralizadas;
- SUPERADMIN não ignora constraints ou privacidade;
- logs redigidos e auditoria separada;
- nenhum secret, signed URL, SQL, stack trace ou provider payload em responses.

## 5. Roadmap aprovado

Milestones:

```text
M0  Bootstrap
M1  Shared foundations
M2  Authentication and minimal administration
M3  Profiles
M4  Documents and bills
M5  Custom data
M6  Attachments and storage
M7  Search and complete Data Grid
M8  Operations, XLSX imports and exports
M9  Google Forms
M10 Query Engine base
M11 Matching, duplicates and merge
M12 AI Chat base
M13 Multimodal OCR
M14 Advanced Query Engine and Gymkhana tasks
M15 Hardening, migration and launch
```

Caminho crítico principal segue M0 → M1 → M2 → M3 → M4 → M5 → M6/M7 → M8 → M10 → M11/M12 → M13 → M14 → M15. M9 pode avançar em paralelo depois do M8.

Cada milestone termina com código integrado, migrations aplicáveis, API e frontend utilizáveis quando aplicável, testes, documentação, observabilidade e critérios funcionais completos.

## 6. Estratégia de implementação

- vertical slices sempre que possível;
- evitar big-bang;
- não extrair para UI/Core antes de contrato ou reutilização comprovada;
- feature incompleta permanece oculta por flag;
- branch curta por unidade revisável;
- normalmente 1 a 3 commits intencionais por PR;
- squash merge;
- `main` sempre compilável;
- releases de UI/Core agrupadas por capacidade real;
- nenhum link local ou `replace` permanente em produção;
- migrations destrutivas por expansão e contração;
- local, staging e production separados;
- staging somente com dados sintéticos;
- nenhum dado de produção copiado para staging.

## 7. Estrutura inicial exata do repositório

```text
Gymkhana-Database/
├── .github/
│   ├── workflows/
│   │   ├── backend.yml
│   │   ├── frontend.yml
│   │   ├── openapi.yml
│   │   ├── migrations.yml
│   │   ├── security.yml
│   │   └── deploy.yml
│   ├── dependabot.yml
│   └── pull_request_template.md
├── api/
│   ├── openapi.yaml
│   └── generated/
├── apps/
│   └── web/
│       ├── public/
│       ├── src/
│       │   ├── app/
│       │   ├── routes/
│       │   ├── features/
│       │   ├── components/
│       │   ├── lib/
│       │   ├── generated/
│       │   ├── styles/
│       │   └── main.tsx
│       ├── index.html
│       ├── package.json
│       ├── tsconfig.json
│       └── vite.config.ts
├── cmd/
│   ├── api/main.go
│   ├── migrate/main.go
│   └── worker/main.go
├── database/
│   ├── migrations/
│   ├── queries/
│   ├── seeds/
│   ├── sqlc.yaml
│   └── tern.conf.example
├── deploy/
│   ├── cloudrun/
│   ├── vercel/
│   └── docker/
├── docs/ORCHESTRATION.md
├── internal/
│   ├── app/
│   ├── config/
│   ├── platform/
│   │   ├── httpserver/
│   │   ├── logging/
│   │   └── postgres/
│   └── testutil/
├── scripts/
├── .editorconfig
├── .env.example
├── .gitignore
├── CHANGELOG.md
├── CONTRIBUTING.md
├── Dockerfile.api
├── Dockerfile.worker
├── LICENSE
├── Makefile
├── README.md
├── SECURITY.md
├── compose.yaml
├── go.mod
├── go.sum
├── package.json
├── pnpm-lock.yaml
├── pnpm-workspace.yaml
└── tsconfig.base.json
```

Diretórios vazios não serão commitados apenas para representar arquitetura futura.

## 8. Convenções do backend

Módulo:

```go
module github.com/Pherlsz/Gymkhana-Database

go 1.26
```

O root `internal/app` compõe dependências. `internal/platform` contém adapters de infraestrutura. Features futuras preferem packages próprios como `internal/profile`, `internal/document` e `internal/bill`, sem uma camada genérica obrigatória de controllers/services/repositories para todo o sistema.

`cmd/api`, `cmd/worker` e `cmd/migrate` contêm apenas bootstrap, configuração, lifecycle e composição. Regras de domínio não ficam em `cmd`.

O worker nasce compilável com `--drain`, mas River e jobs de negócio entram no M8. Não será criada fila fictícia permanente.

## 9. Comandos locais

O Makefile é a interface principal:

```text
make setup
make dev
make dev-api
make dev-web
make build
make test
make test-backend
make test-frontend
make lint
make format
make format-check
make generate
make generate-openapi
make generate-sqlc
make check-generated
make db-up
make db-down
make db-reset
make migrate
make migrate-status
make seed-dev
make check
```

Princípios:

- targets pequenos e compostos;
- falha imediata;
- comandos documentados no README;
- nenhum secret real em scripts;
- suporte prioritário a Windows via PowerShell/Git Bash e Linux/CI;
- scripts complexos em Go, Node ou shell portátil conforme necessidade real.

## 10. Ambiente local e Docker Compose

O M0 usa Docker Compose somente para infraestrutura local necessária.

Serviço inicial:

```text
postgres
```

Requisitos:

- major version pinada e compatível com Neon no início da implementação;
- database, user e password apenas de desenvolvimento;
- volume nomeado;
- healthcheck;
- porta configurável;
- sem exposição pública;
- migrations executadas pelo `cmd/migrate`, não automaticamente pelo container.

R2, Google, AI e WorkerLauncher usam fakes/adapters locais até os milestones correspondentes. MinIO, Redis e serviços sem uso real não entram no M0.

## 11. Configuração e `.env.example`

Configuração tipada, validada no startup e agrupada por domínio.

Variáveis iniciais:

```text
APP_ENV=local
HTTP_ADDR=:8080
WEB_ORIGIN=http://localhost:5173
DATABASE_URL=postgres://gymkhana:gymkhana@localhost:5432/gymkhana?sslmode=disable
LOG_LEVEL=debug
LOG_FORMAT=text
SHUTDOWN_TIMEOUT=10s
REQUEST_TIMEOUT=30s
```

Variáveis futuras aparecem comentadas somente quando o adapter correspondente entrar:

```text
GOOGLE_OAUTH_CLIENT_ID
GOOGLE_OAUTH_CLIENT_SECRET
R2_ENDPOINT
R2_BUCKET
R2_ACCESS_KEY_ID
R2_SECRET_ACCESS_KEY
AI_PROVIDER
OPENAI_API_KEY
GOOGLE_AI_API_KEY
```

Regras:

- `.env.example` sem secrets;
- `.env` ignorado;
- produção usa secret manager/configuração da plataforma;
- logger nunca imprime valores sensíveis;
- testes básicos não exigem credenciais externas.

## 12. PostgreSQL, Tern e sqlc

M0 cria uma migration inicial mínima para validar pipeline, sem tabelas de negócio prematuras.

Estrutura:

```text
database/migrations/000001_initial.sql
database/queries/*.sql
database/sqlc.yaml
```

A migration inicial pode habilitar extensões aprovadas e criar apenas metadata realmente necessária ao bootstrap. Tabelas de Profile entram no M3.

Regras:

- migrations forward-first;
- teste em banco vazio e upgrade;
- seeds separados;
- sqlc output versionado;
- `sqlc generate` e diff check na CI;
- nenhuma query manual dinâmica fora de fronteiras aprovadas;
- nenhum ORM.

## 13. OpenAPI skeleton

Arquivo principal:

```text
api/openapi.yaml
```

Conteúdo inicial:

- OpenAPI 3.1;
- info, servers e tags;
- `/health/live`;
- `/health/ready`;
- schemas comuns de `Error`, `FieldError` e health response;
- exemplos válidos;
- operation IDs estáveis em inglês.

Geração:

- Go strict server/types com oapi-codegen;
- TypeScript schemas/client com openapi-typescript e openapi-fetch;
- output gerado commitado;
- `make generate-openapi` determinístico;
- CI regenera e falha em diff;
- generated files nunca editados manualmente.

## 14. Frontend bootstrap

Workspace pnpm com `apps/web`.

Shell inicial:

- React/Vite/TypeScript;
- TanStack Router;
- TanStack Query;
- TanStack Form e Valibot instalados quando usados pelo M1;
- cliente OpenAPI tipado;
- route de status/placeholder;
- not-found e route error boundary;
- document title e focus management;
- import da versão publicada de Gymkhana-UI;
- nenhum fetch direto em componente.

State policy:

- Router para URL;
- Query para server state;
- Form para forms;
- React para estado visual;
- backend para preferências.

## 15. CI inicial

### `backend.yml`

- setup Go exato;
- dependency cache;
- `gofmt` check;
- `go vet`;
- `staticcheck`;
- `go test ./...`;
- build dos três executáveis.

### `frontend.yml`

- setup Node/pnpm exatos;
- frozen lockfile;
- typecheck;
- Oxlint;
- Oxfmt check;
- Vitest;
- Vite build.

### `openapi.yml`

- lint/validate spec;
- generate Go/TypeScript;
- fail on diff;
- examples/schema checks.

### `migrations.yml`

- PostgreSQL service;
- migrate banco vazio;
- status;
- sqlc generate;
- fail on generated diff.

### `security.yml`

- govulncheck;
- OSV scan;
- dependency review em PR;
- secret scan conforme ferramenta aprovada;
- Actions pinadas por SHA.

### `deploy.yml`

Nasce manual e sem deploy automático de produção. Vercel preview permanece manual/label. Produção só será configurada quando o M0 estiver estável.

## 16. Documentação raiz

Arquivos obrigatórios:

- README com propósito, requisitos, setup, comandos e estado;
- CONTRIBUTING com idioma, branches, commits, PRs, geração e migrations;
- SECURITY sem inventar e-mail inexistente;
- CHANGELOG;
- licença proprietária/all rights reserved enquanto privado;
- `.editorconfig` UTF-8/LF/newline final;
- `.gitignore` sem ocultar generated code versionado.

## 17. Branches e PRs iniciais

Branches:

```text
chore/bootstrap-core
chore/bootstrap-ui
chore/bootstrap-database
feat/core-normalization
feat/ui-foundations
feat/database-platform-foundations
```

Primeiros PRs do Database:

1. `chore/bootstrap-database`: workspace, executáveis, arquivos raiz e comandos básicos;
2. `chore/database-local-postgres`: compose, Tern, migration pipeline e sqlc skeleton;
3. `chore/openapi-bootstrap`: spec, geração Go/TypeScript e checks;
4. `feat/web-shell`: Vite/React/Router/Query e integração do pacote UI;
5. `ci/bootstrap-quality-gates`: workflows de backend, frontend, OpenAPI, migrations e security;
6. `chore/integrate-foundation-releases`: versões exatas de Core/UI e validação conjunta.

Um PR pode agrupar itens quando isso reduzir dependências sem prejudicar revisão, mas `main` deve permanecer utilizável.

## 18. Issues iniciais

Criar milestones GitHub M0–M5 inicialmente.

Issues M0 do Database:

- bootstrap repository files and workspace;
- bootstrap API executable and graceful shutdown;
- bootstrap worker and migrate executables;
- add local PostgreSQL compose environment;
- add Tern migration pipeline;
- add sqlc skeleton and generation check;
- add OpenAPI skeleton and generated clients;
- bootstrap React application shell;
- add CI and security workflows;
- integrate Gymkhana-Core v0.1.0;
- integrate Gymkhana-UI v0.1.0;
- verify clean-clone setup.

Cada issue contém contexto, escopo, fora de escopo, dependências, critérios de aceite, testes, riscos e repositório responsável.

## 19. Releases iniciais

Gymkhana-Core e Gymkhana-UI publicam `v0.1.0` somente após bootstrap, CI, documentação, conteúdo mínimo real e verificação de consumo.

Database não depende de branches ou commits temporários. Desenvolvimento local pode usar `go.work` não commitado e pacote `.tgz`; antes do merge, tags/releases exatas são obrigatórias.

## 20. Gate para iniciar M0

A implementação pode começar porque:

- Etapas 1–10 foram aprovadas;
- arquitetura e limites dos repositórios estão fechados;
- roadmap M0–M15 está definido;
- estruturas iniciais estão definidas;
- comandos, ambiente, geração, CI e PRs iniciais estão especificados;
- não há decisão arquitetural crítica pendente para o bootstrap.

## 21. Definition of Done do M0

M0 termina quando:

- clone limpo dos três repositórios funciona;
- setup está documentado;
- CI está verde;
- Core pode ser importado por tag;
- UI pode ser empacotada, instalada e buildada por `.tgz`/registry;
- API, worker e migrate compilam;
- SPA inicia e consome UI publicada;
- PostgreSQL local sobe com healthcheck;
- migrations executam em banco vazio;
- OpenAPI gera Go e TypeScript sem diff;
- nenhum secret externo é necessário para o caminho básico;
- nenhum placeholder crítico é apresentado como funcionalidade real.

## 22. Próxima ação

O planejamento amplo está encerrado.

Próxima execução autorizada:

```text
Milestone 0
→ iniciar pelo Gymkhana-Core
→ branch chore/bootstrap-core
→ implementar bootstrap, CI e release foundation
→ seguir com Gymkhana-UI
→ iniciar Gymkhana-Database após contratos mínimos de consumo
```

Documentação de orquestração volta a ser atualizada ao concluir milestones ou quando uma decisão arquitetural aprovada mudar.
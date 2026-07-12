# Gymkhana Database — Documento de Orquestração

> **Planning version:** Stage 5  
> **Última sincronização:** 2026-07-12  
> **Etapa atual:** Etapa 5 concluída  
> **Fonte principal de verdade:** `Pherlsz/Gymkhana-Database`  
> **Repositórios relacionados:** `Pherlsz/Gymkhana-UI` e `Pherlsz/Gymkhana-Core`

Este documento consolida as decisões aprovadas nas Etapas 1 a 5 do rebuild. Ele deve ser atualizado ao final de cada etapa antes do início da próxima.

## 1. Objetivo e princípios

Reconstruir o Gymkhana Database como uma aplicação privada, leve, extensível e segura para centralizar Profiles, documentos, contas, anexos, imports, Google Forms, duplicatas, Search, OCR e AI Chat.

Princípios obrigatórios:

- modelo centrado em `Profile`;
- instalação única, sem multi-organização inicial;
- simplicidade antes de abstração preventiva;
- monólito modular, sem microserviços iniciais;
- SQL explícito e sem ORM;
- domínio desacoplado de provedores e packages de infraestrutura;
- nenhum update de versão apenas por ser mais recente;
- releases bloqueados por vulnerabilidades conhecidas e aplicáveis;
- performance resolvida com modelagem, índices, streaming e lotes;
- filtros, paginação, ordenação e agrupamentos refletidos na URL;
- nenhuma limitação artificial sobre o total de registros;
- operação privada, responsiva, acessível e instalável como PWA sem dados offline.

## 2. Idioma de engenharia

Para os três repositórios:

- código, nomes técnicos, commits, PRs, changelogs, releases, workflows e mensagens de CI em inglês;
- títulos e descrições de PR em inglês;
- mensagens de commit em inglês;
- comentários técnicos preferencialmente em inglês;
- textos exibidos ao usuário permanecem em português (`pt-BR`).

Documentação:

- `Gymkhana-UI` e `Gymkhana-Core`: toda documentação técnica em inglês;
- `Gymkhana-Database`: o documento de produto/orquestração pode permanecer em português;
- OpenAPI, nomes de schemas e documentação de código em inglês.

## 3. Escopo funcional aprovado

### 3.1 Profiles

Somente pessoas físicas inicialmente. Não haverá documentos, contas ou registros customizados órfãos.

Campos principais:

- UUIDv7;
- nome completo e nome social;
- CPF opcional e único quando informado;
- datas de nascimento e falecimento;
- nacionalidade e naturalidade;
- nomes da mãe e do pai;
- estado civil e profissão;
- equipe de gincana atual;
- clube de futebol e categoria de associação;
- telefone fixo, celular e e-mail;
- endereço estruturado;
- observações;
- versão para concorrência otimista;
- datas técnicas.

Regras:

- CPF, telefones e números de documentos são armazenados como texto;
- CPF canônico somente com dígitos e máscara no frontend;
- e-mail em minúsculas;
- pais são nomes opcionais, não relacionamentos;
- no máximo uma equipe de gincana atual, sem histórico;
- campos técnicos e internos não aparecem por padrão em tabelas ou exports.

### 3.2 Equipes e futebol

`gymkhana_teams` é administrável e contém `name` e `active`.

No Profile:

- clube: `INTERNACIONAL`, `GREMIO` ou `OTHER`;
- categoria: `CARD`, `MEMBER` ou `OTHER`;
- campos complementares quando `OTHER`.

Não haverá tabela de clubes de futebol inicialmente.

### 3.3 Documentos

Modelo híbrido:

- `documents` para campos comuns;
- tabelas de detalhe para famílias específicas;
- campos customizados tipados para extensões.

Campos comuns incluem Profile, tipo, número canônico, emissor, local de emissão, emissão, validade, formato, notas e versão.

Formato:

- `PHYSICAL`;
- `DIGITAL`;
- `NOT_INFORMED`.

Tipos oficiais iniciais:

- CIN, RG, comprovante de CPF, CNH, CTPS, passaporte;
- título de eleitor, carteira estudantil e conselhos profissionais;
- SUS, Cartão Cidadão;
- certidões de nascimento e casamento;
- Outro.

Detalhes aprovados:

- CIN e RG são distintos;
- CIN normalmente corresponde ao CPF do Profile; divergência gera revisão;
- comprovante de CPF usa o número comum;
- CNH: categoria, primeira habilitação e local;
- CTPS: física ou digital, série, UF e PIS/PASEP;
- título: zona, seção, UF e município;
- passaporte: tipo, nacionalidade e local de nascimento;
- conselho: conselho, UF e categoria/especialidade;
- estudante: instituição, curso, nível, matrícula e período;
- certidões compartilham `certificate_details`;
- `OTHER` usa `custom_label`.

Sem alerta de “próximo do vencimento”; o sistema deriva válido ou vencido quando necessário.

### 3.4 Contas

`bills` é vinculada ao Profile, mas preserva titular e endereço impressos separadamente.

Campos incluem tipo, titular, fornecedor, número da conta/cliente, competência, emissão, vencimento, valor, endereço impresso, notas e versão.

- dinheiro: `NUMERIC(14,2)`;
- competência: `YEAR_MONTH`, persistida no primeiro dia do mês e exibida como `MM/AAAA`;
- esse formatador não se aplica a datas completas.

Tipos iniciais:

- energia: número da fatura e roteiro de leitura; conta representa UC;
- água: fatura, leituras, categoria, hidrômetro, localização e arrecadação; conta representa imóvel;
- internet: código de faturamento; conta representa cliente.

### 3.5 Tipos administráveis

`document_types` e `bill_types` são tabelas, não enums PostgreSQL.

Tipos de sistema:

- não podem ser excluídos;
- podem ser desativados, relabelados, reordenados e receber campos customizados.

Tipos customizados:

- chaves técnicas estáveis;
- desativação antes de exclusão;
- exclusão definitiva com contagem de impacto e confirmação reforçada.

### 3.6 Uso ativo

Não haverá histórico de uso.

`active_usages` representa somente o uso atual:

- criar ao utilizar;
- excluir ao devolver;
- exatamente um owner, documento ou conta;
- no máximo um uso ativo por item.

### 3.7 Anexos

Anexos privados no Cloudflare R2.

Owners permitidos:

- documento;
- conta;
- valor de campo customizado `ATTACHMENT`.

Sem anexos diretos em Profile.

Categorias:

- `FRONT`, `BACK`, `FULL_DOCUMENT`, `DIGITAL_FILE`, `OCR_ORIGINAL`, `EXTRA`.

Limites iniciais:

- imagem: 15 MB;
- PDF: 30 MB;
- 10 arquivos por item;
- 100 MB totais por item.

Exclusão envia à lixeira por sete dias. Hash SHA-256 avisa possível repetição sem bloquear.

Metadados de OCR ficam no attachment, sem tabela de extração, resposta bruta, prompt, raciocínio ou base64 persistidos.

### 3.8 Campos e entidades customizadas

`custom_entity_types` define tipos ligados a Profile, com cardinalidade `ONE_PER_PROFILE` ou `MANY_PER_PROFILE`.

`custom_fields` pode atingir Profile, tipo de documento, tipo de conta ou tipo de entidade customizada.

Tipos permitidos:

- `SHORT_TEXT`, `LONG_TEXT`, `NUMBER`, `MONEY`;
- `DATE`, `DATETIME`, `BOOLEAN`;
- `SINGLE_SELECT`, `MULTI_SELECT`;
- `EMAIL`, `PHONE`, `URL`, `ATTACHMENT`.

Sem fórmulas, scripts, JSON livre, relações arbitrárias ou campos calculados inicialmente.

Valores são armazenados em colunas tipadas. Selects possuem opções com chave estável, label, ativo e ordem; multiselect usa junção.

### 3.9 Imports

Suporte a XLSX e Google Forms.

- um arquivo por destino;
- modos `CREATE_ONLY`, `CREATE_AND_UPDATE` e `UPDATE_ONLY`;
- staging, mapeamento, preview, validação, duplicatas e revisão;
- execução em lotes com checkpoints e idempotência;
- XLSX descartado após processamento;
- relatório permanente;
- detalhes temporários de erro/revisão por 30 dias;
- dados crus bem-sucedidos removidos após conclusão;
- sem anexos em XLSX inicialmente.

### 3.10 Google Forms

Uma conexão Google administrativa central, separada do login dos usuários.

Integrações possuem status `ACTIVE`, `PAUSED`, `CLOSED` ou `CONNECTION_ERROR`.

- sem sincronização recorrente;
- sync manual, final e reprocessamento;
- fechamento apenas após sync final bem-sucedido;
- edições externas posteriores geram revisão;
- exclusão externa não exclui dados locais;
- reabertura processa apenas respostas novas ou alteradas;
- temporários removidos sete dias após finalização.

### 3.11 Duplicatas

Fila persistente somente para Profiles.

Níveis:

- `VERY_STRONG`;
- `PROBABLE`;
- `POSSIBLE`.

Nome nunca é o único critério e não haverá porcentagem exibida.

Origens: criação, edição, import, Forms, OCR e inspeção manual.

Resoluções: usar existente, mesclar, criar mesmo assim, descartar ou atualizar dados.

Merge transfere relações e exclui a origem em transação, sem undo ou snapshot completo.

Documentos e contas são verificados inline. Inspeção geral somente manual via job.

### 3.12 Search

Somente dados estruturados. Não busca anexos, conteúdo de arquivos, OCR bruto ou respostas brutas da IA.

Base:

- B-tree;
- `pg_trgm`;
- `unaccent`;
- função de normalização de caixa, acentos e espaços;
- consultas próprias por domínio retornando um resultado comum.

Sem Elasticsearch, Meilisearch, Typesense, embeddings comuns ou tabela universal duplicada.

Neon Search e full-text só entram após benchmark e necessidade real.

Consultas curtas continuam permitidas, com scans e limites controlados quando necessário.

### 3.13 AI Chat

Consultor privado e somente leitura.

Fluxo:

1. interpretar intenção;
2. gerar plano tipado;
3. validar catálogo, operadores e permissões;
4. converter para consultas seguras;
5. executar pelo Query Engine;
6. sintetizar resposta;
7. retornar referências navegáveis.

A IA não recebe credenciais, conexão direta, repository nem ferramenta de SQL arbitrário.

Threads são privadas. Grandes resultados guardam resumo, critérios, contagem e links, não cópia completa.

Runs detalhados: 30 dias; agregados sem conteúdo pessoal podem permanecer.

### 3.14 OCR

Visão multimodal inicialmente, com revisão humana obrigatória.

- nenhum preenchimento automático definitivo;
- comparação valor atual versus sugerido;
- schema estruturado por tipo;
- `store: false` quando suportado;
- salvar somente provedor, modelo, status, duração e erro operacional;
- sem resposta bruta ou raciocínio.

### 3.15 Exports

Somente XLSX.

- Profile completo com abas relacionadas;
- tabelas de documentos, contas e entidades customizadas;
- relações por UUID;
- sem anexos, notas internas, versões, auditoria ou usuários;
- arquivo privado no R2 por 24 horas;
- histórico da operação permanece.

### 3.16 Usuários e autorização

Google OAuth + allowlist.

Roles:

- `MEMBER`;
- `ADMIN`;
- `SUPERADMIN`.

Exatamente um SuperAdmin ativo. Não pode ser desativado, excluído ou rebaixado pela interface.

Status:

- `INVITED`, `ACTIVE`, `DEACTIVATED`, `DELETED`.

Exclusão anonimiza e preserva a linha para FKs.

Sessões:

- token opaco;
- somente SHA-256 no banco;
- cookie `Secure`, `HttpOnly`, `SameSite=Lax`;
- 24 horas;
- nada em `localStorage`;
- revogação ao desativar usuário.

### 3.17 Preferências, flags e notificações

Preferências persistem aparência, densidade, linhas e configuração de colunas.

Filtros, paginação, sort e grouping permanecem na URL.

Feature flags são globais e liberadas por roles, nunca por usuário individual.

Notificações:

- uma linha por destinatário;
- leitura independente;
- `target_url`;
- expiração padrão de sete dias;
- expiração não remove relatório relacionado.

### 3.18 Retenções

- anexos excluídos: 7 dias;
- notificações: 7 dias;
- arquivos de export: 24 horas;
- duplicatas resolvidas: 7 dias;
- temporários do Forms: 7 dias;
- detalhes temporários de import: 30 dias;
- runs detalhados de IA: 30 dias;
- auditoria de segurança: 90 dias.

Housekeeping centralizado.

## 4. Modelo de dados e banco

### 4.1 Instalação única

Não existem `organizations`, `organization_id`, escopo multi-tenant ou FKs compostas por organização.

Uma futura migração multi-organização será intencional.

### 4.2 Convenções

- PostgreSQL no Neon;
- nomes em inglês e `snake_case`;
- UUIDv7 gerado pelo Go, com default opcional no banco;
- `timestamptz` para instantes;
- `date` para datas civis;
- `NUMERIC(14,2)` para dinheiro;
- texto para identificadores que preservam zeros;
- `text` por padrão, `varchar` somente com limite técnico real;
- `version INTEGER DEFAULT 1` para concorrência otimista.

### 4.3 Exclusões

- Profiles, documentos, contas e threads: exclusão física;
- attachments: lixeira de sete dias;
- usuários: anonimização;
- tipos/campos: desativação antes da exclusão;
- temporários: housekeeping.

### 4.4 Constraints críticas

O PostgreSQL deve garantir:

- CPF único quando preenchido;
- e-mail e Google subject únicos quando preenchidos;
- um único SuperAdmin ativo;
- exatamente um owner por attachment e valor customizado;
- no máximo um uso ativo por item;
- resposta externa única por integração;
- coerência de valores tipados;
- dinheiro nunca como float.

### 4.5 Migrations e seeds

Migrations SQL com Tern v2.

- versionadas no Git;
- staging antes de produção;
- migrations aplicadas nunca são editadas;
- correção para frente preferida;
- River usa migrations próprias.

Seeds separados:

- `database/seeds/system`;
- `database/seeds/development`;
- `database/fixtures/test`.

System seeds são idempotentes. Development seeds e fixtures usam somente dados fictícios.

`gymkhana migrate` não executa seeds implicitamente.

## 5. Arquitetura

### 5.1 Monólito modular

Executáveis:

- `web`;
- `api`;
- `worker`;
- `migrate`.

API e worker compartilham domínio e infraestrutura no mesmo projeto Go.

Sem microserviços, Redis, RabbitMQ, GraphQL ou backend Node inicialmente.

### 5.2 Frontend

- React 19;
- TypeScript 6 inicialmente;
- avaliar TypeScript 7.1+ quando API programática e ecossistema estiverem maduros;
- Vite 8;
- Node 24 LTS;
- pnpm 11;
- TanStack Router v1;
- Query v5;
- Table v8;
- Virtual v3;
- Form v1;
- Valibot somente na aplicação;
- Oxlint e Oxfmt;
- SPA sem SSR;
- React Compiler adiado até benchmark real.

Estado:

- URL: Router;
- servidor: Query;
- formulários: Form;
- local visual: React;
- preferências: backend.

Sem Redux ou Zustand inicialmente.

### 5.3 UI própria

Sem bibliotecas de componentes ou primitives externas.

Base:

- React;
- HTML semântico;
- CSS Modules;
- custom properties;
- APIs nativas;
- ARIA quando necessária.

Phosphor Icons é a biblioteca de ícones.

Componentes de layout incluem `AppShell`, compound components `Page.*`, Stack, Inline, Cluster, Grid, Split, Container, Section, Divider, ScrollArea e ResizablePanel.

Datas usam `Intl` e lógica própria, separando `DATE`, `YEAR_MONTH` e `DATETIME`.

Playground Vite no lugar de Storybook inicialmente.

### 5.4 Backend Go

- Go 1.26;
- `net/http`;
- `pgx/v5` e `pgxpool`;
- sqlc;
- sem ORM;
- Tern v2;
- River OSS;
- OpenAPI 3.1;
- `oapi-codegen` strict server;
- `openapi-typescript` e `openapi-fetch`.

Código organizado por domínio em `internal/`, não por pastas globais de controllers/services/repositories.

### 5.5 REST e contrato

REST + JSON em `/api/v1`.

OpenAPI é a fonte do contrato. Código gerado Go e TypeScript é versionado e verificado no CI.

Erros usam envelope único com `code`, `message`, `request_id` e `field_errors`.

### 5.6 Jobs

River é infraestrutura interna e desacoplada.

- nenhum `river_job_id` em entidades;
- módulos conhecem apenas `operation_id` e interfaces neutras;
- sem tabela genérica duplicando o estado técnico do River;
- cada módulo mantém status, progresso e relatório de negócio.

Operações longas sempre usam jobs: imports, exports, OCR, Forms, duplicatas, bulk actions e housekeeping.

### 5.7 Storage e XLSX

R2 via adapter S3 e interface própria `ObjectStore`.

Uploads diretos por URL assinada, com validação posterior de tamanho, extensão, MIME, assinatura e hash.

Formatos iniciais: JPEG, PNG, WebP e PDF.

Excelize 2.11, leitura por rows iterator e escrita por StreamWriter. Carga em massa por `pgx.CopyFrom`.

### 5.8 IA e streaming

Adapters iniciais OpenAI e Google com SDKs oficiais isolados.

Sem LangChain, LangGraph, CrewAI, Semantic Kernel ou Vercel AI SDK.

Orquestração própria e pequena.

AI Chat usa SSE sobre `fetch`, não WebSocket inicialmente.

### 5.9 Segurança

OAuth no Go, `state`, PKCE S256 e validação completa do ID token.

CSRF por token e validação de `Origin`.

Rate limiting em camadas: borda, memória local e PostgreSQL para quotas globais quando necessário.

Segredos recuperáveis usam AES-256-GCM com rotação por `key_version`.

Logs devem redigir dados pessoais, tokens, chaves, attachments e conteúdo integral de IA.

### 5.10 Logs, cache e saúde

- `log/slog` em JSON na produção;
- `request_id` em todas as requisições;
- OpenTelemetry adiado;
- `/health/live` e `/health/ready`;
- sem Redis;
- TanStack Query, ETag, índices e pequenos caches em memória;
- dados privados com `Cache-Control: private, no-store`.

### 5.11 Testes e segurança de dependências

Frontend:

- Vitest;
- Testing Library;
- Playwright;
- axe-core.

Backend:

- `testing`, `httptest`;
- Testcontainers com PostgreSQL real;
- fuzzing;
- race detector.

Checks:

- `gofmt`, `go vet`, `staticcheck`, testes, `govulncheck`;
- OSV-Scanner, Dependency Review, Dependabot Alerts e scan da imagem;
- GitHub Actions fixadas por SHA;
- versões e lockfiles fixados;
- nenhum auto-merge de dependências.

## 6. Deploy inicial

```text
Vercel Hobby
├── React SPA
├── assets
├── previews manuais
└── proxy /api

Google Cloud Run Service
└── gymkhana api

Google Cloud Run Job
└── gymkhana worker --drain

Cloud Scheduler
└── recuperação + housekeeping

Neon
└── PostgreSQL + River

Cloudflare R2
└── anexos e exports
```

API escala para zero. Worker é iniciado sob demanda, drena a fila e termina.

Vercel e API aparecem na mesma origem pública. O frontend usa `/api/v1` relativo.

A arquitetura permanece portátil para eventual migração integral à GCP.

## 7. Etapa 5 — Estrutura dos repositórios

### 7.1 Grafo de dependências

```text
Gymkhana-Database
├── depends on Gymkhana-UI
└── depends on Gymkhana-Core

Gymkhana-UI
└── independent

Gymkhana-Core
└── independent
```

Não há dependências circulares ou entre UI e Core.

### 7.2 Gymkhana-Database

Estrutura aprovada:

```text
apps/web/                 React SPA
cmd/api/                  API entrypoint
cmd/worker/               worker entrypoint
cmd/migrate/              migration command
internal/                 domain modules
api/                      OpenAPI and generated contract
Database/                 migrations, queries, seeds, sqlc
Deploy/                   Docker, Cloud Run and Vercel config
scripts/
docs/
```

O módulo Go fica na raiz. O pnpm workspace contém `apps/web`; o package da raiz apenas orquestra scripts e tooling.

Cada domínio contém apenas arquivos e subpackages necessários. Não existirão diretórios globais de `controllers`, `services`, `repositories`, `models` e `dtos`.

### 7.3 Gymkhana-UI

Workspace pnpm:

```text
packages/ui/
apps/playground/
docs/
scripts/
```

Package:

```text
@pherlsz/gymkhana-ui
```

Exports públicos explícitos; imports internos por `src/` ou `dist/internal/` são proibidos.

### 7.4 Gymkhana-Core

Packages públicos diretamente na raiz, sem `pkg/` artificial:

```text
normalize/
civiltime/
query/
matching/
duplicates/
assistant/
tools/
result/
internal/
```

Não criar packages `utils`, `helpers`, `common` ou `shared`.

Módulos:

```text
github.com/Pherlsz/Gymkhana-Database
github.com/Pherlsz/Gymkhana-Core
```

### 7.5 Consumo entre repos

Database fixa versões exatas:

- Core por módulo Go privado;
- UI por package privado.

Sem branch flutuante, submodule, subtree, cópia manual ou script de sincronização.

Desenvolvimento local pode usar:

- `go.work` não versionado fora dos repos;
- link temporário pnpm sem alterar a versão registrada.

### 7.6 Publicação privada

UI:

- GitHub Packages privado;
- acesso concedido ao Database;
- autenticação local por token de leitura fora do Git;
- `GITHUB_TOKEN` em Actions.

Core:

- módulo Go privado;
- `GOPRIVATE=github.com/Pherlsz/Gymkhana-Core`;
- CI usa token fine-grained somente leitura para o Core.

### 7.7 Versionamento

UI e Core começam em `v0.1.0` e usam SemVer.

- patch: correção compatível;
- minor: funcionalidade compatível;
- major: quebra pública.

Mesmo em `0.x`, breaking changes são documentadas e migradas explicitamente.

`v1.0.0` apenas após uso em produção, API estável, documentação e cobertura dos contratos públicos.

Sem Changesets inicialmente. Cada biblioteca mantém `CHANGELOG.md` com `Unreleased`.

Releases são manuais assistidas por `workflow_dispatch`, agrupadas e não criadas a cada merge.

### 7.8 Branches, commits e PRs

Desenvolvimento baseado em `main`.

Branches curtas:

- `feature/*`, `fix/*`, `refactor/*`, `chore/*`, `agent/*`.

Sem `develop` ou Git Flow.

Proteção de `main`:

- PR obrigatório quando o desenvolvimento começar;
- checks obrigatórios;
- review threads resolvidas;
- sem force push;
- squash merge padrão.

Todos os commits, títulos de PR e descrições de PR devem ser em inglês.

Commits locais podem ser múltiplos, mas pushes remotos devem ser agrupados para evitar builds desnecessários.

### 7.9 CI por impacto

UI: install frozen, typecheck, lint, format, unit tests, build, playground, security e package verification.

Core: format, vet, staticcheck, tests, vuln scan; race em main, release e mudanças concorrentes.

Database: jobs separados para frontend, backend, OpenAPI, generated code, migrations, integração, segurança e Docker.

Path filters evitam executar a plataforma inteira para mudanças isoladas. Checks antigos da mesma branch são cancelados quando chega novo push.

### 7.10 Vercel sem builds por push

A integração Git automática da Vercel não controlará todos os deploys.

- push em branch: CI, sem Vercel;
- preview: somente manual ou por label `deploy-preview`;
- merge em `main`: produção apenas se caminhos do frontend forem afetados;
- mudanças somente em backend, banco ou docs não disparam Vercel.

### 7.11 Atualização de dependências internas

Nova versão de UI/Core não atualiza Database automaticamente.

Fluxo:

1. publicar versão;
2. avaliar benefício;
3. abrir PR específico no Database;
4. atualizar versão e lockfile/checksum;
5. testar integração;
6. mergear.

Sem `repository_dispatch`, PR automático ou Dependabot para versões internas.

### 7.12 ESM e package verification

Gymkhana-UI é ESM-only:

- sem CommonJS;
- sem minificação da biblioteca;
- source maps e declarations;
- React/ReactDOM como peer dependencies;
- CSS marcado como side effect;
- package real testado com `pnpm pack` e instalação em app temporária.

### 7.13 Compatibilidade e deprecações

Breaking changes entre repos seguem expansão e migração:

1. adicionar API nova mantendo antiga;
2. publicar versão compatível;
3. migrar Database;
4. validar produção;
5. remover API antiga em versão incompatível posterior.

APIs obsoletas recebem `@deprecated`/`Deprecated:`, changelog e instrução de migração.

Antes de 1.0, apenas uma versão exata suportada é mantida pelo Database, mas quebras nunca são silenciosas.

### 7.14 Informações de versão

Build injeta:

- app version;
- commit SHA;
- build time.

Backend pode usar `debug.ReadBuildInfo()` para reportar versão do Core.

Frontend recebe `VITE_APP_VERSION` e `VITE_COMMIT_SHA`.

Endpoint `/api/v1/system/version` retorna somente informações não sensíveis.

### 7.15 Builds reproduzíveis

- `pnpm install --frozen-lockfile`;
- Go com `-mod=readonly`;
- imagem base fixada por versão e digest;
- multi-stage e usuário não-root;
- sem `latest`;
- Actions por SHA;
- ferramentas por versão exata.

### 7.16 Ambiente local

Comandos padronizados:

Database:

```text
make setup dev test lint generate migrate seed build check
```

UI:

```text
pnpm setup dev test lint build check pack:verify
```

Core:

```text
make test lint fuzz check
```

Makefile somente em projetos Go/orquestração. UI usa scripts pnpm.

Código roda diretamente na máquina; Docker Compose local é somente para PostgreSQL e serviços auxiliares.

Padrão seguro: PostgreSQL local. Neon de desenvolvimento é opcional.

### 7.17 Configuração

`.env.example` somente com nomes e explicações, nunca valores reais.

Produção não usa `.env`; segredos vêm do provedor.

Cada variável deve documentar obrigatoriedade, padrão, escopo, sensibilidade e necessidade de restart.

Frontend recebe apenas variáveis públicas.

### 7.18 Documentação

- `ORCHESTRATION.md`: decisões globais;
- `README.md`: entrada rápida;
- `docs/architecture/`: arquitetura atual;
- `docs/guides/`: procedimentos;
- `docs/adr/`: apenas decisões arquiteturais relevantes;
- `SECURITY.md`: comunicação privada de vulnerabilidades;
- templates curtos de PR e issues.

Os três documentos de orquestração são sincronizados conscientemente ao final de cada etapa, sem script de cópia automática.

### 7.19 CODEOWNERS e revisão

CODEOWNERS será simples inicialmente e preparado para divisão futura.

Mudanças sensíveis exigem atenção reforçada:

- auth e autorização;
- migrations destrutivas;
- criptografia;
- upload;
- tools de IA;
- Query Engine;
- workflows de release.

Sem aprovação automática por bot.

### 7.20 Critérios de extração

Mover lógica ao Core somente quando for independente de banco, HTTP, UI e entidade específica, reutilizável e com API suficientemente estável.

Na dúvida, nasce no Database e é extraída após uso real.

Mover componente ao UI somente quando não tiver regra de produto, puder receber dados/eventos por props, tiver uso em múltiplas telas ou sistemas e puder ser demonstrado isoladamente.

Componentes específicos permanecem em `apps/web/src/features/`.

## 8. Decisões adiadas

- multi-organização;
- Redis;
- OpenTelemetry;
- React Compiler;
- TypeScript 7 antes da maturidade necessária;
- Neon Search sem benchmark;
- Storybook;
- bibliotecas de componentes;
- SSR;
- microserviços;
- histórico de uso;
- anexos diretos em Profile;
- OCR tradicional;
- frameworks de agentes;
- SQL arbitrário pela IA;
- quarto repositório de workspace;
- automação cruzada entre repositórios;
- Changesets.

## 9. Próxima etapa

**Etapa 6 — Design system e Data Grid.**

Objetivos:

- definir tokens visuais;
- temas, densidades e responsividade;
- APIs dos componentes fundamentais;
- AppShell e Page;
- comportamento completo do Data Grid;
- edição inline;
- filtros, agrupamento, paginação e persistência de colunas;
- acessibilidade e estados de interface.

Este documento deve ser atualizado novamente ao final da Etapa 6.
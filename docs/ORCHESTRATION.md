# Gymkhana Database — Documento de Orquestração

> **Planning version:** Stage 6  
> **Última sincronização:** 2026-07-12  
> **Etapa atual:** Etapa 6 concluída  
> **Fonte principal de verdade:** `Pherlsz/Gymkhana-Database`  
> **Repositórios relacionados:** `Pherlsz/Gymkhana-UI` e `Pherlsz/Gymkhana-Core`

Este documento consolida as decisões aprovadas nas Etapas 1 a 6 do rebuild. Ele deve ser atualizado ao final de cada etapa antes do início da próxima.

## 1. Objetivo e princípios

Reconstruir o Gymkhana Database como uma aplicação privada, leve, extensível e segura para centralizar Profiles, documentos, contas, anexos, imports, Google Forms, duplicatas, Search, OCR e AI Chat.

Princípios obrigatórios:

- modelo centrado em `Profile`;
- instalação única, sem multi-organização inicial;
- simplicidade antes de abstração preventiva;
- monólito modular, sem microserviços iniciais;
- SQL explícito e sem ORM;
- domínio desacoplado de provedores e packages de infraestrutura;
- nenhuma atualização de versão apenas por ser mais recente;
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
- OpenAPI, schemas e documentação de código em inglês.

## 3. Escopo funcional aprovado

### 3.1 Profiles

Somente pessoas físicas inicialmente. Não haverá documentos, contas ou registros customizados órfãos.

Campos principais:

- UUIDv7;
- nome completo e nome social;
- CPF opcional e único quando informado;
- nascimento e falecimento;
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

- CPF, telefones e números de documentos são texto;
- CPF canônico usa somente dígitos e máscara no frontend;
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

## 7. Estrutura e governança dos repositórios

### 7.1 Grafo

```text
Gymkhana-Database
├── depends on Gymkhana-UI
└── depends on Gymkhana-Core

Gymkhana-UI
└── independent

Gymkhana-Core
└── independent
```

Sem dependências circulares ou entre UI e Core.

### 7.2 Gymkhana-Database

```text
apps/web/                 React SPA
cmd/api/                  API entrypoint
cmd/worker/               worker entrypoint
cmd/migrate/              migration command
internal/                 domain modules
api/                      OpenAPI and generated contract
database/                 migrations, queries, seeds, sqlc
deploy/                   Docker, Cloud Run and Vercel config
scripts/
docs/
```

O módulo Go fica na raiz. O pnpm workspace contém `apps/web`; o package da raiz orquestra scripts e tooling.

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

Packages públicos diretamente na raiz:

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

Sem packages `utils`, `helpers`, `common` ou `shared`.

Módulos:

```text
github.com/Pherlsz/Gymkhana-Database
github.com/Pherlsz/Gymkhana-Core
```

### 7.5 Consumo e publicação

Database fixa versões exatas:

- Core por módulo Go privado;
- UI por GitHub Packages privado.

Sem branch flutuante, submodule, subtree, cópia manual ou script de sincronização.

Desenvolvimento local pode usar `go.work` não versionado e link temporário pnpm.

UI usa `@pherlsz/gymkhana-ui`; Core usa `GOPRIVATE=github.com/Pherlsz/Gymkhana-Core`.

### 7.6 Versionamento e releases

UI e Core começam em `v0.1.0` e usam SemVer.

- patch: correção compatível;
- minor: funcionalidade compatível;
- major: quebra pública.

Mesmo em `0.x`, quebras são documentadas e migradas explicitamente.

Sem Changesets inicialmente. `CHANGELOG.md` possui `Unreleased`.

Releases são manuais assistidas por `workflow_dispatch`, agrupadas e não criadas a cada merge.

### 7.7 Branches, commits e PRs

Desenvolvimento baseado em `main`.

Branches curtas: `feature/*`, `fix/*`, `refactor/*`, `chore/*`, `agent/*`.

Sem `develop` ou Git Flow.

Proteção de `main`:

- PR obrigatório durante desenvolvimento;
- checks obrigatórios;
- review threads resolvidas;
- sem force push;
- squash merge padrão.

Commits, títulos e descrições de PR em inglês.

### 7.8 CI e Vercel

CI é dividido por impacto e usa path filters.

- UI: install frozen, typecheck, lint, format, unit, build, playground, security e package verification;
- Core: format, vet, staticcheck, tests e vulnerability scans;
- Database: frontend, backend, OpenAPI, generated code, migrations, integração, segurança e Docker.

Checks antigos da mesma branch são cancelados por novo push.

Vercel não cria build a cada push:

- branch: CI sem Vercel;
- preview: manual ou label `deploy-preview`;
- `main`: produção somente se frontend for afetado.

### 7.9 Compatibilidade

Nova versão de UI/Core não atualiza Database automaticamente.

Fluxo: publicar, avaliar, abrir PR no Database, fixar versão, atualizar lockfile/checksum, testar e mergear.

Quebras usam expansão e migração:

1. adicionar API nova mantendo antiga;
2. publicar versão compatível;
3. migrar Database;
4. validar produção;
5. remover API antiga em release incompatível posterior.

### 7.10 Ambiente local e documentação

Comandos:

```text
Database: make setup dev test lint generate migrate seed build check
UI:       pnpm setup dev test lint build check pack:verify
Core:     make test lint fuzz check
```

Código roda diretamente na máquina; Docker Compose é usado para PostgreSQL e serviços auxiliares.

Documentação:

- `ORCHESTRATION.md`;
- `README.md`;
- `docs/architecture/`;
- `docs/guides/`;
- `docs/adr/`;
- `SECURITY.md`;
- templates curtos de PR e issues.

Os documentos são sincronizados conscientemente, sem script de cópia automática.

### 7.11 Critérios de extração

Mover lógica ao Core somente quando independente de banco, HTTP, UI e entidade específica, reutilizável e suficientemente estável.

Mover componente ao UI somente quando não possuir regra de produto, puder ser composto por props, tiver uso real em múltiplas telas/sistemas e puder ser demonstrado isoladamente.

Na dúvida, manter no Gymkhana-Database até o reúso ser comprovado.

## 8. Etapa 6 — Design System

### 8.1 Direção visual

O sistema será funcional, neutro, moderno, compacto e consistente.

Prioridades:

- leitura rápida de grandes volumes;
- pouco ruído visual;
- hierarquia clara;
- alta densidade sem aparência apertada;
- interações previsíveis;
- acessibilidade;
- telas pequenas e ultrawide.

Evitar glassmorphism, transparências excessivas, sombras pesadas, gradientes decorativos, cards em todo conteúdo, raios exagerados e animações longas.

### 8.2 Cores e temas

Componentes usam tokens semânticos, não cores fixas.

Grupos principais:

- backgrounds e surfaces;
- textos primary, secondary, muted, disabled e inverse;
- borders e focus;
- primary e seus estados;
- success, warning, danger e info, incluindo variantes subtle.

`app_settings.primary_color` pode definir a identidade principal, mas passa por validação de contraste e geração/seleção de escala.

Temas iniciais:

- `light`;
- `dark`;
- `system`.

Implementação por `data-gym-theme` e CSS custom properties. O tema do usuário persiste no backend e é aplicado cedo para evitar flash incorreto.

### 8.3 Tipografia, spacing e forma

Font stack de sistema, sem download obrigatório de Google Fonts.

Escala compacta aproximada:

- 12 px metadata;
- 13 px grid dense/compact;
- 14 px corpo padrão;
- 16 px destaque;
- 18 px seção;
- 22 px título de página;
- 28 px títulos especiais.

Pesos: 400, 500, 600 e 700 limitado.

Spacing usa base de 4 px e escala tipada.

Raios:

- none 0;
- sm 4 px;
- md 6 px;
- lg 10 px;
- full circular.

Sombras: none, sm, md e lg, usadas apenas quando elevação é necessária.

### 8.4 Densidades e responsividade

Densidades:

- `comfortable`: controles 40–44 px;
- `compact`: padrão desktop, 34–36 px;
- `dense`: grids avançados, 28–30 px.

Aplicação por `data-gym-density` e tokens de altura/padding.

Breakpoints globais orientativos: 640, 768, 1024, 1280 e 1536 px. Componentes preferem layout fluido, container queries, `minmax`, `auto-fit` e `clamp`.

Não existem versões separadas da mesma página para desktop e mobile.

### 8.5 Movimento e estados

Transições funcionais:

- fast 100 ms;
- normal 160 ms;
- slow 240 ms.

`prefers-reduced-motion` reduz ou remove transições não essenciais.

Estados comuns:

- default, hover, active, focus-visible, disabled, loading, invalid, read-only e selected;
- expanded, checked, indeterminate, dragging e drop-target quando aplicável.

## 9. Componentes fundamentais

### 9.1 Estado controlado

Componentes com estado suportam `value/onValueChange` ou `defaultValue`, nunca ambos simultaneamente.

`useControllableState` permanece interno.

### 9.2 Props e semântica

Componentes aceitam atributos nativos compatíveis, `className` e `data-testid` quando necessário.

Não haverá API universal de margin/padding/display nem polimorfismo irrestrito por `as`.

Cada componente renderiza o elemento semanticamente correto.

### 9.3 Button e IconButton

Variantes:

- primary;
- secondary;
- outline;
- ghost;
- danger.

Tamanhos acompanham comfortable, compact e dense.

Button usa `type="button"` por padrão, mantém largura no loading e distingue ações de navegação.

IconButton exige nome acessível; tooltip não substitui `aria-label`.

### 9.4 Fields

Compound API:

- `Field.Root`;
- `Field.Label`;
- `Field.Description`;
- `Field.Error`;
- `Field.RequiredIndicator`.

O UI coordena label, descrição, erro e ARIA, mas não conhece TanStack Form ou Valibot.

TextField e TextArea permanecem wrappers leves de elementos nativos. Prefix, suffix, clear, loading e contagem podem existir sem incorporar máscaras de domínio.

Checkbox, RadioGroup e Switch possuem semântica distinta e suporte a teclado, foco, disabled e invalid.

### 9.5 Select e Combobox

Select usa `<select>` nativo para listas pequenas e estáticas.

Combobox customizado cobre busca, listas grandes/remotas, opções dinâmicas, loading, erro e seleção. O componente não realiza consultas.

### 9.6 Datas

Tipos separados:

- `CivilDate`;
- `YearMonth`;
- instante ISO 8601 para datetime.

Componentes:

- DateField;
- DatePicker;
- Calendar;
- YearMonthField;
- DateTimeField.

DateField permite digitação `DD/MM/AAAA`. Calendar possui teclado completo, mês exibido separado de seleção e labels via `Intl`.

DatePicker usa Popover no desktop e Dialog/Drawer no mobile. YearMonth nunca é um DatePicker com dias ocultos.

Timezone é responsabilidade da aplicação; UI não assume São Paulo.

### 9.7 Uploads e feedback

FileUpload é visual e neutro. A aplicação solicita URL assinada, envia, confirma e persiste.

Drag and drop nunca é o único caminho. Progresso e erro são por arquivo.

Feedback inclui Alert, InlineMessage, Badge, StatusBadge, Toast, Progress, Spinner, Skeleton, EmptyState e ErrorState.

Toast não é a única forma de informar erro crítico. Máximo visual pequeno, fila, pause em hover/focus e `aria-live`.

### 9.8 Overlays

Compound APIs próprias para Dialog e Drawer, com portal, backdrop, scroll lock, foco inicial, trap, Escape e restauração.

Popover, Tooltip e DropdownMenu possuem semântica e teclado próprios.

Tooltip contém informação curta, nunca ações nem informação essencial.

Confirmações destrutivas variam por impacto. Alto impacto exige contagem, consequências e confirmação reforçada.

### 9.9 Formulários

Gymkhana-UI não depende de TanStack Form.

Adapters como `FormTextField` permanecem no Database.

Erros de campo ficam junto ao controle; erro geral no topo; foco vai ao primeiro erro ou resumo; valores não são perdidos; conflitos de versão têm fluxo específico; `request_id` aparece em erros inesperados.

## 10. AppShell e Page

### 10.1 AppShell

Compound API:

- Root;
- Sidebar;
- Brand;
- Navigation;
- NavigationGroup;
- NavigationItem;
- SidebarFooter;
- Header;
- MobileMenuButton;
- HeaderTitle;
- HeaderActions;
- Main.

Sidebar desktop: expanded, collapsed ou hidden. No mobile, vira Drawer com a mesma navegação.

Itens usam links reais e `aria-current`. Funcionalidade sem permissão geralmente é omitida.

Header global contém somente elementos globais: menu, search global, notificações, tema e usuário. Títulos, filtros e ações da rota ficam em Page.

### 10.2 Page

Compound API:

- Root;
- Header;
- Heading;
- Breadcrumbs;
- Title;
- Description;
- Actions;
- Toolbar;
- Content;
- Main;
- Aside;
- Section;
- Footer.

Layouts:

- default;
- wide;
- full;
- centered;
- split.

`Page.Title` é o único `h1` principal. Breadcrumbs representam hierarquia real. Ações têm uma principal, secundárias e overflow responsivo.

Toolbar contém busca local, filtros, sort, grouping, visualização, seleção e atualização, podendo ficar sticky quando necessário.

Detalhes complexos possuem URL própria. Drawer é preview/edição curta, não substituto automático da rota.

Preferir uma única rolagem principal; rolagens internas somente para sidebar, grid virtualizado, overlays e painéis explicitamente necessários.

## 11. Data Grid

### 11.1 Limites e estado

Gymkhana-UI cuida de renderização, layout, virtualização, interação, teclado e estados visuais.

Gymkhana-Database cuida de HTTP, URL, queries, permissões, operadores, preferências, validação, bulk actions e colunas de domínio.

Estado controlado:

- pagination;
- sorting;
- filters;
- grouping;
- selection;
- column preferences.

TanStack Table e Virtual são detalhes internos. A API pública usa tipos próprios.

Toda linha possui ID estável, normalmente UUID.

### 11.2 Paginação e total

Paginação server-side por página/offset inicialmente.

Page sizes:

- 25, 50, 100, 250, 500 e 1000.

Filtros, grouping, page size e normalmente sort voltam para página 1.

Ordenação sempre é estável, com `id` como desempate final.

A API retorna total exato filtrado e autorizado. Estimativa futura somente quando explicitamente marcada.

### 11.3 Virtualização e linhas

Virtualização vertical dentro da página carregada, com até 1000 linhas.

Altura fixa por densidade:

- comfortable ~44 px;
- compact ~36 px;
- dense ~30 px.

Sem altura livre, quebras ilimitadas ou previews pesados dentro da célula.

Virtualização horizontal só após necessidade comprovada.

### 11.4 Colunas

Cada coluna possui chave técnica estável, header, accessor/cell, largura, alinhamento, sort/filter/group/edit e capacidades de hide/pin/resize.

Labels traduzidas nunca são chaves.

Colunas nativas e customizadas usam o mesmo contrato.

Ações ficam em coluna específica, geralmente fixada à direita, com uma ação principal e overflow. Coluna de ações não é exportada.

### 11.5 Sort, filtros e URL

Ordenação é server-side, simples ou múltipla, persistida na URL.

Filtros são tipados por texto, número, dinheiro, data, boolean, select e relacionamento.

Busca rápida e filtros avançados coexistem e são combinados pelo backend.

A primeira UI de filtros avançados usa lista ordenada de condições com `AND`. O Query Engine poderá suportar lógica mais rica sem obrigar a primeira interface a expor grupos arbitrários de `AND/OR`.

URL canônica preserva busca, filtros, operadores, sort, grouping, page e page size.

Preferências de coluna não poluem a URL por padrão.

### 11.6 Agrupamento e agregações

Agrupamento é server-side, com contagens globais filtradas e expansão que pode buscar filhos sob demanda.

Suporta arquitetura para múltiplos níveis, sem limitação permanente a um único grupo.

Agregações iniciais:

- count;
- sum;
- average;
- minimum;
- maximum.

Sempre calculadas pelo backend e respeitando permissões.

### 11.7 Seleção e bulk actions

Seleção usa IDs estáveis e modos:

- explicit;
- all_matching com exclusões e fingerprint da query.

Header seleciona página atual; ação separada seleciona todos os resultados filtrados.

Alterar filtros/busca/grouping limpa seleção global; trocar página preserva.

Bulk actions recebem critério e exclusões, não milhares de IDs. Backend revalida conjunto e permissões. Operações grandes criam jobs e relatórios com falhas parciais.

### 11.8 Colunas e preferências

Resize em pixels, respeitando min/max; ajuste ao conteúdo considera apenas header e página carregada.

Pinning: left, right e none. Uso típico: seleção e coluna principal à esquerda; ações à direita.

Usuário pode mostrar, ocultar, ordenar, fixar, restaurar e pesquisar colunas.

Preferências persistidas no backend por `user_id + table_key`:

- densidade específica;
- page size;
- visibility;
- order;
- sizing;
- pinning.

Filtros, página, sort e grouping permanecem na URL.

Cada tabela possui preset versionado; preferências compatíveis sobrevivem a novas colunas e chaves removidas são ignoradas.

Sem múltiplas visualizações salvas inicialmente.

### 11.9 Edição inline

Somente desktop/tablet quando apropriado e apenas para campos simples de baixo risco:

- texto curto;
- número e dinheiro;
- boolean;
- select simples;
- datas simples;
- custom fields equivalentes.

Não usar inline para anexos, relações complexas, endereços completos, detalhes compostos, merges ou validação cruzada ampla.

Estados:

- display;
- editing;
- saving;
- saved;
- invalid;
- conflict;
- failed.

Enter/F2/affordance iniciam; Enter confirma; Tab confirma e move; Escape cancela. Valor original permanece até sucesso.

Validação local + backend. Conflitos usam `version` e nunca sobrescrevem automaticamente.

A aplicação atualiza TanStack Query e pode remover linha que deixe de corresponder aos filtros, informando o usuário.

### 11.10 Acessibilidade

Modo simples usa `<table>`. `role="grid"` somente quando navegação celular, edição, seleção e grouping justificarem e estiverem corretamente implementados.

Roving tabindex no modo interativo. Setas movem células; Home/End, Ctrl/Cmd Home/End, Page Up/Down, Enter, Space, Escape e Tab seguem o padrão definido.

Virtualização preserva foco e renderiza a célula ativa. Headers têm controles separados para sort, filter, menu e resize.

`aria-live` anuncia resultados, seleção, saves, erros, mudança de página e início de operação, sem narrar cada linha.

### 11.11 Estados

Distinguir:

- loading inicial;
- refreshing com dados anteriores;
- vazio sem registros;
- vazio por filtros;
- sem permissão;
- recurso não configurado;
- erro inicial;
- erro parcial/refetch.

Loading inicial mantém estrutura conhecida. Refresh preserva linhas, scroll, foco e seleção.

Capacidades visuais (`canView`, `canSelect`, `canEdit`, etc.) não substituem segurança no backend.

### 11.12 Mobile, tablet e ultrawide

**A tabela não será automaticamente substituída por cards no mobile.**

Mobile preserva o Data Grid com:

- coluna principal visível/fixada quando útil;
- poucas colunas essenciais;
- scroll horizontal;
- filtros em Drawer;
- painel de colunas simplificado;
- detalhes em rota/preview.

Lista/cards só existem como componente específico de tela quando houver benefício funcional claro, nunca como transformação automática do grid.

Recursos não obrigatórios no mobile:

- edição inline;
- resize;
- reorder por drag and drop;
- pinning manual avançado;
- grouping por arraste;
- ordenação complexa por múltiplos headers;
- atalhos avançados;
- split-view complexo.

Esses fluxos continuam disponíveis por formulário, Drawer, página de edição ou desktop.

Tablet reduz ações e colunas sem criar outra interface.

Ultrawide usa largura ampla para grids, mas não estica indefinidamente colunas de texto.

Presets responsivos definem visibilidade inicial desktop/tablet/mobile, sem duplicar definição de coluna e sem sobrescrever preferências explícitas.

### 11.13 Export e jobs

A ação de export pertence ao produto. Pode exportar página, resultados filtrados, seleção ou todos permitidos, sempre reconstruído e revalidado no backend.

“Exportar colunas visíveis” só existe quando explicitamente escolhido.

Operações longas usam estados:

- queued;
- running;
- completed;
- completed_with_errors;
- failed;
- cancelled.

Não mostrar progresso falso. Conclusão invalida queries relacionadas, preserva filtros/página e gera notificação/relatório.

### 11.14 Testes e performance

Design System exige:

- semântica;
- nome acessível;
- teclado;
- foco visível e restaurado;
- disabled/invalid/loading;
- contraste light/dark;
- zoom 200%;
- fonte aumentada;
- reduced motion;
- touch;
- erros;
- axe-core;
- teste manual para componentes críticos.

Data Grid deve ser testado com 25, 100 e 1000 linhas, muitas colunas, custom cells, seleção extensa, refetch, grouping, edição e conflitos.

Unit, playground e Playwright cobrem coluna, query state, preferência, virtualização, teclado, mobile, ultrawide e acessibilidade.

Otimizações complexas somente após profiling. Evitar providers por linha, formulários por célula, medição contínua e overlays montados para todas as linhas.

## 12. Decisões adiadas

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
- Changesets;
- temas arbitrários completos;
- seleção de intervalo de datas;
- saved views do Data Grid;
- nested AND/OR visual avançado;
- virtualização horizontal padrão;
- transformação automática de tabela em cards no mobile.

## 13. Próxima etapa

**Etapa 7 — Gymkhana-Core, Query Engine, AI Chat e OCR.**

Objetivos:

- fechar o catálogo tipado de campos, entidades e operadores;
- definir AST, validação e limites do Query Engine;
- mapear planos neutros para executores seguros do Database;
- definir matching e regras de duplicatas;
- fechar contratos neutros de providers e tools;
- detalhar threads, mensagens, runs, referências e SSE;
- definir prompts, schemas estruturados e políticas de contexto;
- detalhar OCR multimodal e revisão humana;
- definir observabilidade, segurança, custos e testes de IA.

Este documento deve ser atualizado novamente ao final da Etapa 7.
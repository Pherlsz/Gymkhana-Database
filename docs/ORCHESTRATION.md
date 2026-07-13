# Gymkhana Database — Documento de Orquestração

> **Planning version:** Stage 8  
> **Última sincronização:** 2026-07-13  
> **Etapa atual:** Etapa 8 concluída  
> **Fonte principal de verdade:** `Pherlsz/Gymkhana-Database/docs/ORCHESTRATION.md`  
> **Repositórios relacionados:** `Pherlsz/Gymkhana-UI` e `Pherlsz/Gymkhana-Core`

Este documento consolida as decisões aprovadas nas Etapas 1 a 8 do rebuild. Deve ser atualizado conscientemente ao final de cada etapa, sem sincronização automática entre repositórios.

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
- versões adotadas por benefício real, compatibilidade, segurança ou performance, não apenas por novidade.

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

CIN e RG permanecem distintos. Tipos são tabelas administráveis, não enums PostgreSQL. Tipos de sistema podem ser desativados, relabelados, reordenados e ampliados, mas não excluídos fisicamente. Tipos customizados exigem desativação e confirmação de impacto antes da exclusão.

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
- bill types são administráveis.

Dados impressos na conta nunca alteram Profile implicitamente.

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

Somente XLSX inicialmente. Operação assíncrona, geração em streaming, arquivo privado no R2 por 24 horas e histórico permanente da operação. Sem attachments, auditoria, usuários, versões ou notas internas por padrão.

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

Autorização usa permissions centrais, não `if role == ...` espalhado. Exemplos:

- `profiles.read/create/update/delete`;
- `documents.*`;
- `imports.*`;
- `duplicates.read/review/merge`;
- `assistant.use`;
- `ocr.start/review`;
- `admin.users/settings/catalog`.

SUPERADMIN não ignora constraints, concorrência, auditoria, privacidade de threads ou validações de domínio.

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

Constraints incluem CPF único quando preenchido, e-mail/Google subject únicos, um SuperAdmin ativo, um owner por attachment, coerência de custom values, unicidade de respostas externas e dinheiro nunca como float.

Não usar PostgreSQL RLS inicialmente. Escopos de linha, quando existirem, serão explícitos em repositories/services e revalidados pela autorização.

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

API e frontend usam a mesma origem pública com `/api/v1`. A arquitetura permanece portátil para futura migração integral à GCP.

## 6. Repositórios e governança

Dependências:

```text
Gymkhana-Database → Gymkhana-UI
Gymkhana-Database → Gymkhana-Core
```

UI e Core são independentes. Sem circularidade, submodules, subtree ou cópia manual.

Database fixa versões exatas de UI e Core. UI é pacote privado no GitHub Packages; Core é módulo Go privado com `GOPRIVATE`.

Branches curtas a partir de `main`, PR obrigatório quando desenvolvimento ativo, checks, review threads resolvidas, squash merge e sem force push. Commits e PRs em inglês.

Vercel não executa em todo push: branch executa CI, preview é manual/label e produção ocorre em `main` somente quando frontend é afetado.

## 7. Design System e Data Grid

Gymkhana-UI é próprio, baseado em semantic HTML, CSS Modules, custom properties, browser APIs e ARIA. Sem Radix, Base UI, React Aria, shadcn, MUI, Chakra, Mantine, Ant, Tailwind ou runtime CSS-in-JS.

Direção visual: funcional, neutra, moderna, compacta e consistente.

Temas: light, dark e system. Tokens semânticos para surfaces, text, border, focus, primary, success, warning, danger e info. Cor principal configurável com validação de contraste.

Densidades: comfortable, compact e dense. Fontes do sistema, spacing base 4 px, raios moderados e animações curtas com reduced motion.

Componentes incluem Button, IconButton, Field, inputs, Select nativo, Combobox, datas próprias, FileUpload visual, feedback, Dialog, Drawer, Popover, Tooltip, menus, AppShell e compound `Page.*`.

Data Grid:

- TanStack Table/Virtual internos, API pública própria;
- paginação server-side por offset inicialmente;
- page sizes 25, 50, 100, 250, 500 e 1000;
- total exato quando viável;
- virtualização vertical dentro da página;
- sorting, filtros, grouping e aggregations no backend;
- filtros e URL canônica;
- seleção explícita ou `all_matching` por fingerprint;
- resize, pinning, visibilidade e ordem persistidos por usuário/tabela;
- inline editing apenas para campos simples e de baixo risco;
- conflitos por `version`;
- teclado, roving tabindex e semântica table/grid conforme interação;
- loading e refreshing distintos;
- estados vazios, permissão e erro específicos.

Mobile preserva a tabela; não há transformação automática para cards. Poucas colunas essenciais, scroll horizontal, filtros em Drawer e ações simplificadas. Inline edit, resize, drag reorder, pinning avançado e grouping complexo não são obrigatórios no mobile; fluxos equivalentes ficam em formulário, Drawer, detalhe ou desktop.

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

Catálogo runtime com entities, fields, relations, capabilities, operators, aliases, schema version e fingerprint. Chaves técnicas são estáveis; labels podem mudar.

Tipos neutros incluem text, identifier, integer, decimal, money, boolean, civil date, year month, instant, enum, email, phone, URL, UUID e attachment reference.

Field paths são estruturados. Relações possuem cardinalidade e quantificadores any/all/none. AST suporta Predicate, RelationPredicate, And, Or e Not.

`QueryPlan` define root, projection, filter, sort, grouping, aggregations, distinct, page/limit e result mode. `ExecutionPlan` forma DAG de query, transform, set operation, combination, rank e summarize.

Set operations: union, intersection, difference e symmetric difference. Result grain evita multiplicidade indevida de joins.

Padrões diferenciam contiguous, ordered subsequence, unordered subset e permutation. Seleções de caracteres e transformações binárias são tipadas e usam pruning, matemática ou programação dinâmica antes de enumerar.

Planos válidos acima do orçamento síncrono tornam-se jobs após confirmação quando aplicável. JSON canônico + catálogo + permission scope geram fingerprint.

Endpoints principais:

```text
POST /api/v1/query/validate
POST /api/v1/query/execute
POST /api/v1/query/operations
POST /api/v1/search
```

Result sets temporários preservam principalmente IDs, grain, fingerprint e metadata, não cópias completas de records.

## 9. AI Chat

Consultor privado e read-only. Pode consultar, sintetizar, gerar relatório ou export confirmado. Não cria, edita, exclui, mergeia, importa ou altera configuração.

Fluxo:

```text
mensagem
→ classificação semântica
→ catálogo contextual
→ Query/Execution Plan
→ validação
→ execução/tools
→ síntese com referências
```

A interpretação nunca depende de palavras específicas. Sinônimos, ordem livre, linguagem coloquial, abreviações, erros pequenos e paráfrases devem produzir planos equivalentes quando a intenção for equivalente.

Threads privadas por usuário. Mensagens e runs são recursos separados. Envio cria mensagem/run com `Idempotency-Key`; SSE ocorre por endpoint separado e autenticado. A resposta final é recuperável sem o stream.

Eventos versionados e sequenciais incluem run status, plan, tool, operation, text delta, references, table, completed, failed, cancelled e heartbeat.

Tools:

- discover catalog;
- validate query plan;
- execute query plan;
- start query operation;
- get operation status/result;
- get record details.

Sem SQL, código, tables físicas ou download irrestrito. Plano inválido pode ser corrigido no máximo duas vezes.

Ações do Assistant são declarativas e allowlisted, como abrir entidade, result set, operação, refinar consulta ou solicitar export. URLs e HTML arbitrários do modelo não são executados.

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

Entradas JPEG, PNG, WebP e PDF. Schemas tipados por document/bill type e custom fields. Evidência pode conter attachment, page, normalized bounding box e snippet mínimo. Sem resposta bruta do provider ou chain of thought.

Revisão permite accept, reject, keep current e edit-and-accept por campo. Aplicação revalida permissions, versions, constraints e duplicatas.

Endpoints:

```text
POST /api/v1/ocr-operations
GET  /api/v1/ocr-operations/{id}
GET  /api/v1/ocr-operations/{id}/review
POST /api/v1/ocr-operations/{id}/apply
POST /api/v1/ocr-operations/{id}/reject
POST /api/v1/ocr-operations/{id}/cancel
```

Retenção:

- ready for review: 30 dias;
- após aceite, rejeição, descarte ou aplicação: detalhes temporários por no máximo 7 dias;
- depois somente auditoria mínima;
- original segue retenção do attachment.

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

Sem merge automático e nome nunca é evidência suficiente sozinho.

Candidate pair é canônico por UUID e possui unique constraint. Blocking usa CPF, official document, email, phone, name+birth, parent, address fragments e trigram candidates.

Níveis visíveis:

- VERY_STRONG;
- PROBABLE;
- POSSIBLE.

Score interno pode ordenar, mas não é probabilidade e sempre exige reasons estruturadas.

Casos guardam origem, status, rule version, normalization version e evidence fingerprint. Decisões de pessoas diferentes permanecem minimamente para evitar recriação e só reabrem por mudança material.

Merge exige preview, escolha de destination/source, resolução de conflitos, locks, versions e uma única transaction com rollback total. Sem undo completo inicial.

Endpoints principais:

```text
GET  /api/v1/duplicate-cases
GET  /api/v1/duplicate-cases/{id}
POST /api/v1/duplicate-cases/{id}/resolve
POST /api/v1/duplicate-cases/{id}/merge-preview
POST /api/v1/duplicate-cases/{id}/merge
POST /api/v1/duplicate-inspections
```

Detalhes extensos de documents, bills e custom records são carregados sob demanda.

## 12. Duplicatas e orçamento de egress Neon

A otimização de egress é requisito arquitetural.

É proibido carregar milhares de Profiles completos e suas relações para o Cloud Run para comparar em Go.

Fluxo aprovado:

```text
PostgreSQL gera candidate pairs
→ calcula evidências simples e aggregations
→ mantém staging/intermediários no Neon
→ retorna somente compact evidence vectors ou classificação
→ Core aplica regras neutras
→ Database faz upsert dos cases
→ UI recebe páginas e resumos
```

Regras:

- nenhuma comparação all-to-all;
- no `SELECT *`;
- projection mínima;
- aggregations e counts no PostgreSQL;
- joins largos que repetem Profile devem ser separados;
- queue paginada em 25/50/100;
- detalhes e relações sob demanda;
- bulk actions por IDs ou query fingerprint/exclusions;
- inspection incremental por profile fingerprint, rule version e normalization version;
- um global duplicate job por vez inicialmente;
- batches e keyset pagination;
- direct Neon connection para jobs que dependam de session/temp state; pooled endpoint para API comum;
- `pg_trgm`, `unaccent`, B-tree e partial indexes conforme query real;
- `pg_stat_statements` obrigatório nos benchmarks;
- medir `data_transfer_bytes` antes/depois quando disponível;
- checkpoints permitem interromper e retomar.

Alertas: 50%, 70%, 85% e 95% de consumo mensal. Capabilities podem bloquear inspeções não essenciais sem declarar a API indisponível.

Benchmark obrigatório com 20 mil candidate pairs, meta de dezenas de MB e não GB.

## 13. Tarefas complexas de gincana

O usuário pode colar tarefa em qualquer estilo. O parser é semântico e não exige palavras-chave específicas.

Uma única tarefa pode combinar simultaneamente:

- pessoas;
- documentos;
- contas;
- endereços;
- custom entities e fields;
- padrões/transformações de caracteres;
- combinações entre múltiplos records;
- requisitos externos.

Bindings explícitos:

- same entity;
- different entities;
- same owner;
- distinct records;
- any member;
- whole combination.

Isso determina quando pessoa, documento e conta pertencem ao mesmo Profile ou a owners diferentes.

O solver começa pelo conjunto mais restrito, propaga constraints, usa set operations, branch-and-bound, dynamic programming e memoization limitada, sem enumeração cega.

Estados de negócio simplificados:

- DRAFT;
- RUNNING;
- REVIEW_REQUIRED;
- COMPLETED;
- COMPLETED_WITH_WARNINGS;
- FAILED;
- CANCELLED;
- EXPIRED.

Fases técnicas ficam em `stage`.

Endpoints:

```text
POST /api/v1/gymkhana-task-analyses
GET  /api/v1/gymkhana-task-analyses/{id}
POST /api/v1/gymkhana-task-analyses/{id}/refine
```

Retenção: até 30 dias enquanto pendente e até 7 dias para intermediários após conclusão/decisão.

## 14. Contrato REST e OpenAPI

Base única:

```text
/api/v1
```

Recursos usam plural e IDs UUID opacos. Endpoints são orientados a recursos; comandos de domínio usam sub-recursos explícitos.

Métodos:

- GET para leitura;
- POST para criação/comandos/operações;
- PATCH para alteração parcial;
- PUT apenas para substituição real;
- DELETE para exclusão/lixeira.

Status principais:

- 200, 201, 202 e 204;
- 400, 401, 403, 404, 409, 412, 422 e 429;
- 500 e 503.

`404` pode ocultar existência de recurso sem permissão.

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

Sem stack trace, SQL ou payload de provider.

Responses individuais não usam envelope `data` desnecessário. Listagens usam `items`, `page`, `page_size`, `total` e `page_count`.

Paginação padrão por page/page_size; page sizes 25, 50, 100, 250, 500 e 1000. Jobs internos podem usar keyset.

Sorting canônico: `sort=field:asc,other:desc`, com ID como desempate interno.

Filtros simples usam query params. AST complexa usa POST read-only em endpoints `/query`. Projection é validada; não existe `fields=*`. Includes são allowlisted, rasos e não podem multiplicar payload de forma descontrolada.

OpenAPI principal em `api/openapi.yaml`, podendo ser dividido e empacotado deterministicamente. Operation IDs estáveis em inglês. Schemas separados por operação, como Summary, Detail, CreateRequest, UpdateRequest e ListResponse.

Datas civis: `YYYY-MM-DD`; year month: `YYYY-MM`; instantes: RFC3339 UTC; money: decimal string; identifiers: string.

Código gerado Go e TypeScript é versionado e CI falha em diff não commitado. Contract tests validam implementação, examples, errors, nullability, enums e breaking changes.

## 15. Concorrência e idempotência

Mutations de recursos editáveis exigem `version` no body; `If-Match` pode ser suportado para clientes técnicos.

Update SQL usa `WHERE id = ? AND version = ?`. Conflito retorna `412 Precondition Failed`. Nenhum last-write-wins silencioso.

PATCH usa schema próprio:

- campo ausente: não alterar;
- `null`: limpar quando permitido;
- valor presente: substituir.

`Idempotency-Key` obrigatório para imports, exports, OCR, Forms sync, duplicate inspection, query jobs, Assistant messages, merge, bulk actions e mutations críticas sujeitas a retry.

Escopo da idempotência:

- user ID;
- endpoint/command;
- idempotency key;
- request fingerprint.

Mesma key + mesmo request retorna resultado original. Mesma key + request diferente retorna conflito.

IDs distintos são preservados: request, operation, thread, message, run e import.

## 16. Profiles, documents, bills e custom data na API

Endpoints principais de Profile:

```text
GET    /api/v1/profiles
POST   /api/v1/profiles
GET    /api/v1/profiles/{id}
PATCH  /api/v1/profiles/{id}
DELETE /api/v1/profiles/{id}
POST   /api/v1/profiles/{id}/restore
```

Related collections são paginadas em endpoints próprios. O detalhe retorna contagens e referências compactas, não todas as relações.

Criação de Profile normaliza, valida unicidade, gera candidatos de duplicata e pode exigir decisão para PROBABLE/VERY_STRONG. POSSIBLE pode criar com warning e case.

Edição pode ocorrer por seções, sempre compartilhando e atualizando a versão global do Profile.

Documents e bills possuem endpoints nested por Profile e endpoints próprios por ID. Mudança de document type é ação reforçada; não PATCH silencioso. Active usage é sub-recurso único.

Custom values podem ser atualizados na mesma transaction do recurso. Fields ou options inativos existentes podem ser exibidos como legacy, mas não selecionados novamente sem regra explícita.

## 17. Attachments e uploads

Upload em três etapas:

```text
criar upload request
→ PUT direto no R2
→ confirmar upload
```

Upload request informa filename, content type, size e SHA-256. O backend verifica objeto real antes de criar attachment.

Uploads abandonados expiram e housekeeping remove registros, objetos órfãos e multipart incompletos.

Acesso a attachment ou export usa POST que gera signed URL curta. Metadata nunca expõe bucket, object key ou provider details.

Attachment pode ser renomeado sem mover o objeto. Exclusão lógica permite restore em sete dias.

## 18. Lixeira, restauração e auditoria

Profile, document, bill, custom record e attachment usam lixeira lógica por sete dias quando aplicável.

Campos técnicos:

- deleted_at;
- deleted_by_user_id;
- deletion_reason;
- purge_after.

Exclusão de Profile gera preview de documents, bills, custom records, attachments, active usages, duplicate cases e operações pendentes. Filhos entram na mesma unidade lógica e não ficam órfãos.

Restore revalida CPF, documentos, cardinalidades e limites. Conflito bloqueia restauração automática.

Purge definitivo remove temporários e objetos na ordem correta e preserva auditoria mínima. Purge antecipado exige SUPERADMIN e confirmação reforçada.

Auditoria registra actor, action, resource type/ID, source, changed field keys, request ID, operation ID quando aplicável e result. Before/after apenas para campos críticos, podendo ser mascarado, criptografado, hash ou omitido.

Activity feed é projeção paginada da auditoria relevante, não event sourcing.

## 19. Imports e Google Forms na API

Import é recurso persistente com endpoints para create, upload, parse, mapping, validate, rows, decisions, execute, cancel, retry e report.

Mapping pode produzir na mesma row:

- Profile;
- documento;
- conta;
- custom values;
- custom record.

Transformações permitidas são tipadas: trim, whitespace, upper/lower, remove mask, parse date/year month/decimal, map option, split, join e constant. Sem JavaScript, Python, SQL ou expressão arbitrária.

Preview e erros são paginados. Contagens são agregadas no PostgreSQL. Decisões de row podem ser CREATE_NEW, USE_EXISTING, UPDATE_EXISTING, SKIP e REVIEW_LATER.

Batch decisions não automatizam merges, conflitos de strong identifiers ou sobrescrita ambígua.

Google Forms possui connections, available forms, import configurations e sync. Question external IDs são preferidos a labels. Respostas editadas geram nova staging version e não sobrescrevem valor local divergente sem revisão.

## 20. Operações, exports e notificações

`operation` representa acompanhamento de negócio; River é detalhe interno.

Endpoints:

```text
GET  /api/v1/operations
GET  /api/v1/operations/{id}
POST /api/v1/operations/{id}/cancel
GET  /api/v1/operations/{id}/report
```

Estados:

- QUEUED;
- RUNNING;
- WAITING_FOR_REVIEW;
- COMPLETED;
- COMPLETED_WITH_ERRORS;
- FAILED;
- CANCEL_REQUESTED;
- CANCELLED;
- EXPIRED.

Progress pode ser determinate ou indeterminate; sem porcentagem falsa. Polling é adaptativo. Cloud Run Job é lançado após commit de operation + River job; Scheduler recupera filas sem worker ativo.

Exports são operations e usam query plan, scope, columns e format XLSX.

Notificações internas e privadas informam conclusão, erro, revisão ou alerta. Sem e-mail, push ou SMS inicialmente. Retenção de sete dias.

## 21. Administração e configurações

Prefixo `/api/v1/admin` para users, settings, document types, bill types, custom fields, custom entity types, gymkhana teams, dictionaries, Google Forms, AI configuration, system usage e audit.

Usuário criado é allowlist entry, sem senha local. Status: INVITED, ACTIVE, DISABLED e DELETED.

Transferência de SuperAdmin usa endpoint próprio e transaction, nunca PATCH comum de role.

Settings são tipados por grupos, não JSON irrestrito. Public settings não expõem secrets. Mudanças que afetam dados geram preview/operação própria.

Feature flags server-side iniciais podem controlar Assistant, OCR, Forms, exports e advanced query.

Secrets de providers entram por endpoint específico e nunca são devolvidos em texto legível. Testes de provider usam payload sintético, não dados reais.

Preferências do usuário incluem theme, density e Data Grid settings. Filtros, sorting, grouping e página permanecem na URL. Saved queries são privadas, revalidadas ao executar e nunca usadas automaticamente pelo Assistant.

Dicionários auxiliares são administráveis, versionados e entram no fingerprint das análises.

## 22. Capacidades e uso do sistema

`GET /api/v1/system/capabilities` informa feature enabled/available/reason para a SPA. Health e capabilities são conceitos distintos.

Painel administrativo pode mostrar storage, attachments, exports, imports, operations, AI/OCR usage, jobs e egress Neon.

Níveis de egress:

- NORMAL;
- NOTICE;
- WARNING;
- RESTRICTED;
- CRITICAL.

Restrições graduais bloqueiam somente operações grandes e não essenciais quando possível.

## 23. Frontend e rotas

TanStack Router com rotas tipadas e deep links para Profiles, documents, bills, imports, Assistant threads, OCR, duplicate cases, operations, exports, saved queries, settings e administração.

Estado relevante da página permanece na URL. Tokens, signed URLs, drafts e conteúdo sensível não entram na URL.

Bootstrap:

```text
session
+ public settings
+ capabilities
→ AppShell e rota
```

Rotas declaram permissions. O frontend filtra navegação e ações, mas o backend é sempre a barreira de segurança. Responses de detalhe podem incluir permissions efetivas do recurso.

URLs de detalhe são reais; Drawer serve para preview, não como único acesso.

Navegação atualiza título, breadcrumbs e foco principal.

## 24. Cliente API, Query Keys e cache

Frontend usa openapi-typescript + openapi-fetch.

Camadas:

```text
generated types
→ central typed client
→ feature query/mutation functions
→ components
```

Componentes não chamam fetch diretamente.

Cliente central cuida de cookies, CSRF, AbortSignal, idempotency headers e error envelope, sem lógica de negócio.

Retries automáticos apenas para reads transitórias limitadas; mutations não são repetidas indiscriminadamente.

Query keys são factories estruturadas por domínio e usam parâmetros normalizados/fingerprints.

Mutation response é fonte imediata. Atualizar detail/cache local e invalidar apenas listas/resultados potencialmente afetados. Nada de refetch global.

Optimistic update somente para ações simples e reversíveis, como notification read e preferências visuais. Nunca para merge, OCR apply, import execute ou settings críticos.

## 25. Formulários e erros

Produto usa TanStack Form + Valibot. Gymkhana-UI permanece independente.

Frontend valida experiência; backend valida definitivamente tipos, normalização, permissions, unicidade, relações, concorrência e duplicatas.

Field errors mapeiam paths estáveis. Form preserva valores e foca o primeiro erro. Dirty state protege contra perda real.

Formulários complexos usam rota dedicada. Dialog/Drawer apenas para tarefas curtas.

Categorias de erro:

- 401: sessão;
- 403/404: permissão/recurso;
- 422: campos;
- 409: decisão de negócio;
- 412: versão;
- 429: quota/rate limit;
- 500/503: infraestrutura com request ID.

Error boundaries por raiz, rota e áreas complexas. Erros esperados de API são estados normais.

## 26. Assistant e operações no frontend

Uma execução ativa por thread inicialmente. Threads diferentes podem executar conforme quota.

SSE partial text pode usar estado local/buffer e consolidar na mensagem final. Reconexão usa sequence/run status e nunca duplica run.

Assistant renderiza separadamente texto, interpretação, filters, references, table, warnings, operation e actions. Markdown é sanitizado e limitado.

Central de operações mostra operações próprias e administrativas autorizadas. Cada feature também apresenta sua operação contextual. Cancelamento só aparece quando `cancelable=true` e fica `CANCEL_REQUESTED` até confirmação do backend.

## 27. Segurança, retenção e observabilidade

- logs JSON com `slog` e request ID;
- dados pessoais, tokens, signed URLs, prompts, tool payloads e conteúdo de arquivo redigidos;
- audit log separado de operational log;
- health endpoints `/health/live` e `/health/ready`;
- rate limiting em edge, app e PostgreSQL para quotas globais quando necessário;
- secrets recuperáveis com AES-256-GCM e `key_version`;
- master key fora do banco e Git;
- nenhuma conversa privada acessível a outros usuários pela UI comum;
- OpenTelemetry adiado inicialmente.

Retenções principais:

- attachment trash: 7 dias;
- notifications: 7 dias;
- export file: 24 horas;
- import temporary details: 30 dias;
- AI run/tool details: 30 dias;
- OCR pending: 30 dias;
- OCR post-action details: no máximo 7 dias;
- duplicate resolved full details: 7 dias;
- gymkhana task pending details: 30 dias;
- gymkhana task post-action details: no máximo 7 dias;
- security audit: 90 dias;
- business audit essencial: conforme tipo.

Housekeeping centralizado.

## 28. Testes e critérios de qualidade

Frontend: Vitest, Testing Library, Playwright e axe-core.

Backend: stdlib testing, httptest, Testcontainers com PostgreSQL real, fuzz e race detector.

Contract/OpenAPI:

- lint;
- schemas/examples;
- operation IDs únicos;
- generated code sincronizado;
- responses reais compatíveis;
- breaking-change detection.

Authorization matrix cobre unauthenticated, MEMBER, ADMIN, SUPERADMIN, disabled/deleted user e expired session.

Concorrência cobre edits simultâneos, OCR review desatualizada, merge preview stale, import row durante execução e restore com unicidade.

Idempotência cobre same key/same body, different body, concurrent requests, timeout, retry e usuário diferente.

Integração cobre todos os módulos e egress.

E2E obrigatório:

1. login autorizado;
2. criar Profile e detectar duplicata;
3. adicionar documento/attachment;
4. OCR review/apply;
5. adicionar conta;
6. Search e AI Chat;
7. tarefa combinando pessoa + documento + conta;
8. import XLSX;
9. Forms sync;
10. duplicate merge;
11. export;
12. delete/restore;
13. administração.

Testes de linguagem natural usam famílias de paráfrases e comparam intenção/plano, não texto final idêntico.

Testes de payload/egress falham para SELECT *, listas sem paginação, Profile completo por pair, agregação no worker, joins cartesianos e refetch global desnecessário.

Acessibilidade é validada em login, AppShell, Profile, forms, Data Grid, import, OCR, duplicates, Assistant, operations e admin.

Quality gates incluem gofmt, vet, staticcheck, tests, race where relevant, govulncheck, OSV, dependency review, image scan, pinned Actions e exact tool versions.

## 29. Decisões adiadas

- multi-organização;
- Redis;
- OpenTelemetry;
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
- PostgreSQL RLS without demonstrated need.

## 30. Próxima etapa

**Etapa 9 — planejamento de implementação, milestones, dependências e ordem de entrega.**

Objetivos:

- transformar as decisões em workstreams implementáveis;
- definir milestones e critérios de aceite incrementais;
- ordenar fundações, domínio, API, frontend, workers e integrações;
- mapear dependências entre os três repositórios;
- definir estratégia de branches, PRs, releases e ambientes por milestone;
- definir datasets sintéticos, benchmarks e gates antes de produção;
- evitar big-bang e manter o produto utilizável ao fim de cada incremento.

Este documento deve ser atualizado novamente ao final da Etapa 9.
# Gymkhana Database — Documento de Orquestração

> **Planning version:** Stage 7  
> **Última sincronização:** 2026-07-12  
> **Etapa atual:** Etapa 7 concluída  
> **Fonte principal de verdade:** `Pherlsz/Gymkhana-Database/docs/ORCHESTRATION.md`  
> **Repositórios relacionados:** `Pherlsz/Gymkhana-UI` e `Pherlsz/Gymkhana-Core`

Este documento consolida as decisões aprovadas nas Etapas 1 a 7 do rebuild. Deve ser atualizado conscientemente ao final de cada etapa, sem sincronização automática entre repositórios.

## 1. Objetivo e princípios

Reconstruir o Gymkhana Database como aplicação privada, leve, extensível, segura e centrada em `Profile`, capaz de armazenar e consultar pessoas, documentos, contas, anexos, imports, Google Forms, duplicatas, campos e entidades customizadas, Search, OCR e AI Chat.

Princípios obrigatórios:

- instalação única; multi-organização foi removida completamente;
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
- processamento próximo ao banco para reduzir egress;
- filtros, paginação, sorting e grouping refletidos na URL;
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

CIN e RG permanecem distintos. Tipos são tabelas administráveis, não enums PostgreSQL. Tipos de sistema podem ser desativados, relabelados, reordenados e ampliados, mas não excluídos. Tipos customizados exigem desativação e confirmação de impacto antes da exclusão.

### 3.3 Contas

`bills` pertence a Profile, mas preserva titular e endereço impressos.

Campos incluem tipo, fornecedor, número de cliente/conta, competência, emissão, vencimento, valor, endereço impresso, notas e versão.

- dinheiro: `NUMERIC(14,2)`;
- competência: `YEAR_MONTH` e exibição `MM/AAAA`;
- tipos iniciais: água, energia e internet;
- bill types também são administráveis.

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

Upload direto por URL assinada, seguido de validação de MIME, assinatura, tamanho e SHA-256. Exclusão envia à lixeira por sete dias.

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

### 3.7 Imports e Google Forms

Imports XLSX e Forms usam staging, mapping, preview, validação, duplicatas e revisão.

Modos:

- `CREATE_ONLY`;
- `CREATE_AND_UPDATE`;
- `UPDATE_ONLY`.

Execução em batches, checkpoints e idempotência. XLSX é descartado após processamento; relatório permanece; detalhes temporários de erro/revisão ficam 30 dias.

Google Forms usa conexão administrativa separada do OAuth de login. Sem polling recorrente inicial: sync manual, final e reprocessamento. Exclusões externas não removem dados locais.

### 3.8 Exports

Somente XLSX inicialmente. Operação assíncrona, arquivo privado no R2 por 24 horas e histórico permanente da operação. Sem attachments, auditoria, usuários, versões ou notas internas por padrão.

### 3.9 Usuários e autorização

Google OAuth + allowlist.

Roles:

- `MEMBER`;
- `ADMIN`;
- `SUPERADMIN`.

Exatamente um SuperAdmin ativo. Usuários excluídos são anonimizados para preservar FKs.

Sessão opaca, hash SHA-256 no banco, cookie Secure/HttpOnly/SameSite=Lax/Path=/ por 24 horas, sem JWT ou localStorage. CSRF por token e `Origin`.

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
└── attachments e exports
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

### 8.1 Objetivo

Permitir consultas sobre qualquer dado estruturado atual ou futuro, incluindo campos nativos/customizados, documents, bills, custom entities, relações, padrões de caracteres, agrupamentos e combinações.

O Engine atende Search, Data Grid, filtros, AI Chat, duplicates, exports, bulk actions e relatórios.

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

### 8.2 Catálogo dinâmico

Catálogo runtime com entities, fields, relations, capabilities, operators e aliases autorizados.

Entidades iniciais:

- profile;
- document;
- bill;
- gymkhana_team;
- active_usage;
- custom entities.

Chaves técnicas estáveis em inglês. Labels podem mudar sem quebrar planos.

O catálogo possui fingerprint considerando entities, fields, types, relations, operators, custom schema e permission scope. Planos antigos são revalidados quando o catálogo muda.

### 8.3 Tipos e operadores

Tipos neutros:

- text, long_text e identifier;
- integer, decimal e money;
- boolean;
- civil_date, year_month e instant;
- enum e multi_enum;
- email, phone, URL, UUID e attachment reference.

`identifier` preserva zeros e pode conter letras.

Operadores incluem equality, contains, prefix/suffix, empty, comparisons, ranges, enum membership, normalized/fuzzy matching, pattern matching, character classes, length e relation quantifiers.

Nem todo campo expõe todos os operadores do tipo.

### 8.4 Field paths e relações

Field paths são estruturados em root entity, relation path e final field. Traversal arbitrário é proibido; cada relação precisa existir no catálogo.

Cardinalidades: one-to-one, many-to-one, one-to-many e many-to-many.

Quantificadores relacionais: any, all e none.

### 8.5 AST e planos

Filtro suporta `Predicate`, `RelationPredicate`, `And`, `Or` e `Not`.

`QueryPlan` inclui version, catalog fingerprint, root, projection, filter, sort, grouping, aggregations, distinct, pagination/limit e result mode.

Result modes:

- records;
- references;
- count;
- aggregation;
- groups;
- exists.

`ExecutionPlan` organiza múltiplas etapas em DAG acíclico:

- query;
- transform;
- set operation;
- combination;
- rank;
- summarize.

Etapas podem depender de resultados anteriores por referências tipadas, nunca interpolação textual.

Set operations: union, intersection, difference e symmetric difference. Granularidade (`ResultGrain`) distingue Profiles, documents, bills e outros owners para evitar contagens infladas por joins.

### 8.6 Padrões e combinações de caracteres

O Engine diferencia:

- sequência contígua;
- subsequência mantendo ordem;
- subconjunto sem ordem;
- permutação.

`CharacterSelection` define classes/literais, cardinalidade e modo. Cada posição é usada uma vez por padrão.

Transformações incluem binary-to-integer/byte, digits-to-integer, characters-to-text e counts. Predicados podem verificar letter, digit, alphanumeric, printable, equality, ranges e sets.

Exemplo obrigatório: CPF com oito dígitos binários em quaisquer posições, possivelmente reorganizados, capazes de formar letra ou número.

O executor deve usar matemática, pruning ou programação dinâmica antes de enumerar combinações. Consultas grandes passam ao worker com batches, checkpoints e orçamento.

### 8.7 Ranking e combination solver

Ranking usa critérios e pesos explicáveis, nunca score apresentado como probabilidade.

Combination plans suportam múltiplos records, distinctness, cardinalidade, constraints e objectives. Proibido executar scripts arbitrários; somente transformações tipadas.

### 8.8 Validação e orçamento

Camadas:

1. estrutural;
2. catálogo;
3. tipos;
4. permissão;
5. operação/custo.

Erros técnicos estáveis incluem unknown entity/field/relation, invalid value/path/aggregation, unsupported operator, permission denied, catalog changed, query too complex, timeout e result too large.

Budgets configuram relation depth, logical nodes, projection, groups, aggregations, character selections, combination width, candidate values, sync rows e execution time.

Planos válidos acima do orçamento síncrono tornam-se jobs, em vez de serem funcionalmente proibidos.

### 8.9 Execução e fingerprint

PostgreSQL executa filtros indexáveis, joins, sets, grouping e aggregations. Go pós-filtra apenas quando necessário, em streaming/batches e sem carregar tabelas inteiras.

Planos possuem JSON canônico, catalog fingerprint e permission scope para gerar query fingerprint. Usos: all-matching, auditoria, jobs, resultados anteriores, cache futuro e bulk actions.

Todo plano gera explicação operacional sem revelar SQL ou raciocínio interno.

## 9. AI Chat

### 9.1 Escopo

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

A interpretação nunca depende de o usuário escrever palavras específicas. Linguagem natural, sinônimos, ordem das frases, erros de digitação e diferentes maneiras de descrever o mesmo requisito devem ser resolvidos semanticamente. Keywords podem auxiliar recall, mas não podem ser a regra principal.

### 9.2 Catálogo e tools

A IA recebe catálogo semântico autorizado, não schema físico. Catálogo contextual reduz tokens e pode ser ampliado via descoberta controlada.

Tools iniciais:

- discover catalog;
- validate query plan;
- execute query plan;
- start query job;
- get operation status/result;
- get record details.

Schemas fechados, versionados, sem propriedades extras e sem execução parcial de calls inválidas.

Plano inválido pode ser corrigido automaticamente até duas vezes. Depois, resposta honesta sem inventar resultado.

### 9.3 Ambiguidade e execução

Interpretações dominantes e de baixo risco podem ser executadas, sempre explicando o plano quando útil. Ambiguidades materialmente diferentes exigem escolha, consulta abrangente segura ou interpretação explícita.

Consultas read-only comuns executam sem confirmação. Jobs pesados, export grande, análise combinatória extensa ou custo relevante exigem confirmação de escopo.

A IA recebe resumo, agregações, amostra controlada e referências, nunca milhares de rows completos. Total e número exibido são distinguidos. Sampling informa método.

### 9.4 Threads, mensagens e runs

Thread privada por usuário, status active/archived, renomeável. Sem compartilhamento inicial.

Mensagens: user, assistant e system_notice. Respostas persistem texto, explanation, references, tables, warnings, operation e declarative actions separadamente, não como Markdown monolítico.

Runs registram estados queued, interpreting, planning, validating, executing, synthesizing, waiting, completed, failed e cancelled. Retries são attempts separados.

Tool calls persistem nome, versão, status, duração, counts e erro sanitizado, nunca prompts, chain of thought, credenciais ou resultados completos.

Detalhes técnicos de runs/tool calls ficam 30 dias; mensagens finais permanecem.

### 9.5 Streaming e contexto

SSE via POST ou criação de run + endpoint de stream, conforme compatibilidade Vercel/Cloud Run. Sem WebSocket inicialmente.

Eventos incluem run status, plan, tool, operation, text delta, references, table, completed, failed, cancelled e heartbeat.

Sem uma gravação por token. Reconexão recupera status/resposta e não cria outro run automaticamente. Cancelamento é best effort e propaga context cancellation.

Contexto é construído pela aplicação com system policies, catálogo contextual, mensagens recentes, resumo estruturado, references e operation state. Sem memória global oculta.

Follow-ups como “desses” usam result set ID, query fingerprint, grain e/ou operation ID, não apenas interpretação textual.

### 9.6 Providers

Interface `AIProvider` neutra, com adapters OpenAI e Google e capabilities explícitas para streaming, structured output, tools, vision, cache e cancellation.

Modelos podem ser configurados por purpose: classification, planning, correction, synthesis, OCR e summarization. A aplicação escolhe o modelo, não a IA.

Fallback somente para erros técnicos transitórios e nunca para esconder plano inválido, permissão, ambiguidade ou resultado vazio.

### 9.7 Segurança, custos e auditoria

Prompt injection em mensagens, dados do banco, PDFs e imagens não altera tools, catálogo, permission scope ou policies. Dados recuperados são conteúdo não confiável, nunca instruções.

Projection mínima, revalidação em cada tool call, limites, auditoria de acesso sensível e exports separados evitam exfiltração.

Run budget controla model calls, tools, corrections, tokens, execution time, records for model e estimated cost. Quotas por usuário podem limitar runs, tokens, AI jobs e OCR.

Logs guardam IDs, status, duração, provider/model, error codes e counts; não guardam mensagens, respostas, CPF, documents, prompts ou payloads completos por padrão.

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

Entradas JPEG, PNG, WebP e PDF. Arquivos são validados por owner, permission, MIME, signature, size, hash, pages e corruption. Provider recebe bytes privados ou signed URL curta.

Schemas tipados por document/bill type e custom fields. Cada `SuggestedValue` possui status found/not_found/unclear/conflicting/invalid/not_applicable, raw snippet mínimo, warnings e evidências.

Evidência pode conter attachment, page, normalized bounding box e snippet. Sem inventar coordenadas. Confiança é apresentada como clear/review recommended/unclear/conflicting, nunca porcentagem objetiva.

Comparação diferencia empty current, same, different, invalid e source conflict.

Revisão por campo permite accept, reject, keep current e edit before accepting. Campos de Profile e documento são separados. Aplicação revalida permissions, values, version e duplicates em transaction.

OCR é job assíncrono com estados queued, preparing, classifying, extracting, validating, ready_for_review, applied, completed_without_changes, failed, cancelled e expired.

Retenção:

- `ready_for_review`: 30 dias;
- após aceite, rejeição, descarte ou aplicação: detalhes temporários por no máximo 7 dias, padrão 7;
- depois ficam somente operation, attachment, reviewer, action, affected field keys, date e result summary;
- temporários de imagem/PDF são removidos após processamento;
- original segue retenção do attachment.

Structured output é obrigatório quando suportado; saída inválida tem no máximo uma correção estrutural. Prompt/schema/model/attachments geram fingerprint idempotente.

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

Candidate pair é canônico por UUID e possui unique constraint.

Blocking usa CPF, official document, email, phone, name+birth, parent, address fragments e trigram candidates. Blocking apenas reduz candidatos.

Evidências possuem direção supports duplicate, supports distinct ou neutral e strength. Níveis visíveis:

- VERY_STRONG;
- PROBABLE;
- POSSIBLE.

Score interno pode ordenar, mas não é probabilidade e sempre exige reasons estruturadas.

Cases possuem origem, status, rule version, normalization version e evidence fingerprint. Decisões de pessoas diferentes permanecem minimamente para evitar recriação, mas podem reabrir por mudança material.

Ações: usar existente, merge, criar mesmo assim, confirmar diferentes, descartar candidato ou atualizar existente.

Merge escolhe destination/source, mostra impacto, resolve conflicts, bloqueia rows, valida versions e executa uma única transaction com rollback total. Sem undo completo inicial; auditoria reforçada obrigatória.

Casos abertos permanecem. Após resolução, detalhes completos ficam sete dias; depois permanece decisão mínima, pair, fingerprints, rule, user e date.

## 12. Duplicatas e orçamento de egress Neon

A otimização de egress é requisito arquitetural, pois o free tier disponível possui orçamento mensal pequeno.

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
- detalhes e relações carregados sob demanda;
- bulk actions enviam case IDs ou query fingerprint/exclusions;
- inspection incremental por profile fingerprint, rule version e normalization version;
- um global duplicate job por vez inicialmente;
- batches e keyset pagination;
- direct Neon connection para jobs que dependam de session/temp state; pooled endpoint para API comum;
- read replicas e pooling não são tratados como redução de egress;
- `pg_trgm`, `unaccent`, B-tree e partial indexes conforme query real;
- `online_advisor` pode ser usado em PG17 apenas como recomendação validada por `EXPLAIN (ANALYZE, BUFFERS)`;
- `pg_stat_statements` é obrigatório nos benchmarks;
- medir `data_transfer_bytes` antes/depois quando API do Neon estiver disponível;
- relatório mostra Profiles, pairs, rows recebidas, estimated bytes, observed delta e duração;
- checkpoints permitem interromper por orçamento e retomar.

Alertas de consumo: 50%, 70%, 85% e 95%. Inspeções não essenciais podem ser bloqueadas conforme consumo.

Critério inicial de benchmark com 20 mil candidate pairs:

- nenhum Profile completo por pair;
- nenhuma lista sem paginação;
- nenhuma aggregation no frontend/worker;
- no máximo um compact vector por pair enviado ao Core;
- detalhes apenas sob demanda;
- nenhuma multiplicação cartesiana;
- scan incremental;
- stats e delta de egress registrados;
- operação retomável;
- meta de dezenas de MB, não GB, por inspeção completa.

## 13. Tarefas complexas de gincana

O usuário pode colar tarefa em qualquer estilo e pedir análise. O parser é semântico e não exige palavras-chave específicas. Deve interpretar sinônimos, ordem livre, abreviações, linguagem coloquial e variações de escrita; palavras-chave são apenas sinais auxiliares.

Uma tarefa pode exigir simultaneamente:

- pessoas;
- documentos;
- contas;
- endereços;
- custom entities;
- campos customizados;
- padrões/transformações de caracteres;
- combinações entre múltiplos records;
- requisitos externos.

Não existe exclusividade entre categorias. Exemplo válido: duas pessoas distintas, uma CNH e uma conta de água do mesmo owner, mais um documento de outra pessoa que forme caractere binário.

O plano contém requirements, constraints, objectives, quantities, mandatory flags e warnings. Cada requirement possui binding explícito:

- same entity;
- different entities;
- same owner;
- distinct records;
- any member;
- whole combination.

Isso determina quando person, document e bill precisam pertencer ao mesmo Profile ou a pessoas diferentes.

Requisitos são classificados como consultáveis ou externos. O sistema nunca inventa o cumprimento de requisito externo.

Cardinalidade reconhece exact, min e max. Ordem de caracteres diferencia contiguous, ordered subsequence, unordered subset e permutation.

Dicionários auxiliares versionados podem representar santos, cidades, animais, cores, profissões, times e classes de caracteres. Não há web search automática na primeira versão do produto; conhecimento externo precisa estar cadastrado ou ser fornecido pelo usuário.

Cada requirement gera candidate sets; sets são combinados por bindings. O solver:

- começa pelo conjunto mais restrito;
- propaga constraints;
- elimina incompatíveis cedo;
- usa branch-and-bound, dynamic programming e limited memoization quando útil;
- não enumera combinações cegamente;
- mantém IDs e minimal evidence próximos ao banco;
- executa como job quando excede orçamento.

Soluções podem ser principal e alternativas materialmente diferentes. Ranking é explicável e considera mandatory coverage, optional coverage, evidence clarity, warnings, diversity e external dependencies. Nunca é probabilidade de sucesso.

Requirement status: satisfied, partially satisfied, not satisfied, not verifiable e ambiguous.

Cada solução traz references e evidências por requirement, incluindo positions/characters/transformations quando necessário, mas minimiza exposição de dados sensíveis.

Retenção segue o padrão de ação:

- enquanto pendente ou em acompanhamento: até 30 dias;
- após conclusão/consulta: detalhes temporários por no máximo 7 dias;
- mensagem final, critérios e referências essenciais podem permanecer na thread;
- candidate sets e temporary combination tables são removidos.

## 14. Segurança, retenção e observabilidade

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
- security audit: 90 dias.

Housekeeping centralizado.

## 15. Testes e critérios de qualidade

Frontend: Vitest, Testing Library, Playwright e axe-core.

Backend: stdlib testing, httptest, Testcontainers com PostgreSQL real, fuzz e race detector.

Query Engine/Core:

- AST, catalog, operators, paths, quantifiers;
- canonical serialization/fingerprints;
- character selection/permutation;
- sets, combinations, ranking e explanations;
- deterministic matching;
- Unicode and invalid-plan fuzzing.

Database:

- safe SQL and parameterization;
- permission filtering;
- grouping/aggregations;
- post-processing streaming;
- jobs, cancellation e checkpoints;
- SSE reconnect/cancel;
- prompt injection;
- OCR schemas, evidence, conflicts e review;
- duplicate candidate generation, merge rollback e re-open rules;
- 20k duplicate pair egress benchmark;
- complex tasks crossing people + documents + bills + custom entities;
- multiple natural-language phrasings for the same intent, ensuring no dependency on exact words.

Quality gates incluem gofmt, vet, staticcheck, tests, race where relevant, govulncheck, OSV, dependency review, image scan, pinned Actions e exact tool versions.

## 16. Decisões adiadas

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
- multiple saved Data Grid views;
- advanced visual nested AND/OR builder;
- default horizontal virtualization;
- automatic table-to-card conversion on mobile.

## 17. Próxima etapa

**Etapa 8 — APIs, permissões e fluxos completos frontend/backend.**

Objetivos:

- definir recursos e endpoints REST por módulo;
- fechar authorization matrix e permission checks;
- detalhar OpenAPI schemas e pagination envelopes;
- mapear mutations, idempotency e optimistic concurrency;
- definir flows de Profile, document, bill, custom fields, imports, Forms, duplicates, Search, Assistant, OCR, exports e admin;
- definir route tree, query keys, error handling e cache invalidation;
- fechar contract tests e integration boundaries.

Este documento deve ser atualizado novamente ao final da Etapa 8.
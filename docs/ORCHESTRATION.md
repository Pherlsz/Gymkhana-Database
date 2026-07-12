# Gymkhana Database — Documento de Orquestração

> **Planning version:** Stage 4  
> **Última sincronização:** 2026-07-12  
> **Etapa atual:** Etapa 4 concluída  
> **Fonte principal de verdade:** `Pherlsz/Gymkhana-Database`  
> **Repositórios relacionados:** `Pherlsz/Gymkhana-UI` e `Pherlsz/Gymkhana-Core`

Este documento registra as decisões aprovadas para o rebuild do Gymkhana Database. Ele deve ser atualizado ao final de cada etapa de planejamento antes do início da etapa seguinte.

## 1. Objetivo do rebuild

Reconstruir o Gymkhana Database como uma aplicação privada, leve, extensível e segura para centralizar pessoas, documentos, contas, anexos, imports, integrações com Google Forms, inspeção de duplicatas, Search, OCR e AI Chat.

O produto deve permitir consultar qualquer dado estruturado cadastrado, sem depender de listas rígidas de perguntas pré-configuradas. A IA atua como camada de interpretação e consulta, não como acesso direto ao banco.

Princípios aprovados:

- simplicidade antes de abstração preventiva;
- modelo centrado em `Profile`;
- nenhuma preparação inicial para multi-organização;
- sem microserviços na primeira versão;
- sem ORM;
- sem dependência de um provedor específico dentro do domínio;
- nenhuma atualização de versão apenas por ser mais nova;
- dependências somente quando trouxerem benefício concreto;
- nenhum release com vulnerabilidade conhecida e aplicável;
- URLs devem preservar filtros, paginação, ordenação, agrupamentos e estado navegável;
- performance deve ser resolvida com modelagem, SQL, índices, streaming e processamento em lotes.

## 2. Escopo funcional aprovado

### 2.1 Profiles

A entidade principal é uma pessoa física. Não haverá documentos, contas ou registros customizados órfãos.

Campos nativos previstos em `profiles`:

- `id` UUIDv7;
- nome completo e nome social;
- CPF opcional e globalmente único quando preenchido;
- nascimento e falecimento;
- nacionalidade e naturalidade;
- nome da mãe e do pai;
- estado civil;
- profissão;
- equipe de gincana atual;
- clube de futebol e categoria de associação;
- telefone fixo, celular e e-mail;
- endereço estruturado;
- observações;
- `version` para controle de concorrência;
- datas técnicas.

Regras:

- somente pessoas físicas na primeira versão;
- CPF armazenado apenas com dígitos e formatado no frontend;
- telefones normalizados como texto para preservar zeros e códigos internacionais;
- e-mail em minúsculas;
- endereço direto no Profile;
- pais armazenados como nomes opcionais, sem relacionamento entre Profiles;
- no máximo uma equipe de gincana atual por Profile, sem histórico;
- `death_date`, `notes`, `version`, `created_at` e `updated_at` não aparecem por padrão em tabelas nem exports.

### 2.2 Equipes e vínculos de futebol

`gymkhana_teams` contém equipes de gincana administráveis, com `name` e `active`.

Os campos antigos foram interpretados assim:

- `team`: equipe de gincana;
- `club_membership`: Internacional, Grêmio ou Outro;
- `membership_type`: Cartão, Sócio ou Outro.

Não haverá tabela de clubes de futebol inicialmente. Os valores controlados ficam diretamente no Profile, com campo complementar para `OTHER`.

### 2.3 Documentos

Estratégia híbrida:

- tabela comum `documents`;
- tabelas de detalhe somente para famílias com campos próprios;
- tipos customizados usam campos customizados tipados.

Campos comuns:

- `profile_id`;
- `document_type_id`;
- número canônico;
- órgão emissor;
- estado, país e local de emissão;
- emissão e validade;
- formato `PHYSICAL`, `DIGITAL` ou `NOT_INFORMED`;
- observações;
- versão e datas técnicas.

O número será uma única representação canônica em texto, preservando zeros à esquerda, removendo separadores visuais e usando letras maiúsculas quando aplicável. Máscaras ficam no frontend.

Tipos oficiais iniciais:

- CIN;
- RG;
- comprovante de CPF;
- CNH;
- CTPS;
- passaporte;
- título de eleitor;
- carteira estudantil;
- conselhos profissionais;
- cartão SUS;
- Cartão Cidadão;
- certidão de nascimento;
- certidão de casamento;
- Outro.

Detalhes específicos aprovados:

- CIN e RG são distintos;
- CIN normalmente deve corresponder ao CPF do Profile; divergência gera revisão;
- comprovante de CPF usa `documents.number` e verifica o CPF do Profile;
- CNH: categoria, primeira habilitação, local de emissão e campos de segurança opcionais;
- CTPS: modelo físico ou digital, série, UF e PIS/PASEP;
- título de eleitor: zona, seção, UF e município;
- passaporte: tipo, nacionalidade e local de nascimento;
- conselho profissional: conselho, UF, categoria ou especialidade;
- carteira estudantil: instituição, curso, nível, matrícula e período;
- SUS e Cartão Cidadão não precisam de tabela de detalhe;
- certidões compartilham `certificate_details`;
- tipo Outro usa `custom_label`.

Não haverá aviso automático de documento próximo do vencimento. Quando necessário, a aplicação apenas deriva válido ou vencido.

### 2.4 Tipos de documentos e contas

`document_types` e `bill_types` são tabelas administráveis.

Tipos de sistema:

- não podem ser excluídos;
- podem ser desativados, renomeados para exibição, reordenados e receber campos customizados.

Tipos customizados:

- podem ser criados pelo Admin;
- possuem chave técnica estável;
- podem ser desativados antes da exclusão definitiva;
- exclusão definitiva exige contagem de impacto e confirmação reforçada.

### 2.5 Contas

`bills` representa contas vinculadas a um Profile, mantendo separadamente o titular e endereço impressos na conta.

Campos principais:

- tipo;
- titular impresso;
- fornecedor;
- número da conta ou cliente;
- competência mensal;
- emissão e vencimento;
- valor em `NUMERIC(14,2)`;
- endereço impresso estruturado;
- observações;
- versão e datas técnicas.

`reference_month` representa `YEAR_MONTH`, persistido como o primeiro dia do mês, mas exibido como `MM/AAAA`. Esse formatador não pode ser aplicado a datas completas.

Tipos iniciais:

- energia;
- água;
- internet.

Detalhes:

- energia: número da fatura e roteiro de leitura; `account_number` representa a UC;
- água: número da fatura, leituras, categoria, hidrômetro, localização e código de arrecadação; `account_number` representa o código do imóvel;
- internet: código de faturamento; `account_number` representa o código do cliente.

### 2.6 Uso ativo

Não haverá histórico de utilização.

`active_usages` representa somente o uso atual de um documento ou conta:

- ao usar, cria-se o registro;
- ao devolver, o registro é excluído;
- exatamente um proprietário, documento ou conta;
- exatamente um uso ativo por item;
- o estado “em uso” é derivado da existência do registro.

### 2.7 Anexos

Anexos privados ficam no Cloudflare R2.

Podem pertencer a:

- documento;
- conta;
- valor de campo customizado do tipo anexo.

Não haverá anexo direto no Profile.

Categorias:

- `FRONT`;
- `BACK`;
- `FULL_DOCUMENT`;
- `DIGITAL_FILE`;
- `OCR_ORIGINAL`;
- `EXTRA`.

Limites iniciais:

- imagem: 15 MB;
- PDF: 30 MB;
- até 10 arquivos por item;
- até 100 MB totais por item.

A lixeira de anexos dura sete dias. O hash SHA-256 gera aviso de possível repetição, mas não bloqueia automaticamente.

O attachment guarda somente metadados operacionais mínimos de OCR. Não haverá tabela `ocr_extractions`, resposta bruta da IA, prompt, raciocínio ou base64 persistidos.

### 2.8 Campos e entidades customizadas

`custom_entity_types` define tipos relacionais vinculados a Profile, com cardinalidade `ONE_PER_PROFILE` ou `MANY_PER_PROFILE`.

`custom_entity_records` nunca fica órfão.

`custom_fields` pode se aplicar a:

- Profile;
- tipo de documento;
- tipo de conta;
- tipo de entidade customizada.

Tipos aprovados:

- `SHORT_TEXT`;
- `LONG_TEXT`;
- `NUMBER`;
- `MONEY`;
- `DATE`;
- `DATETIME`;
- `BOOLEAN`;
- `SINGLE_SELECT`;
- `MULTI_SELECT`;
- `EMAIL`;
- `PHONE`;
- `URL`;
- `ATTACHMENT`.

Não haverá fórmulas, scripts, JSON livre, relações arbitrárias ou campos calculados na primeira versão.

Valores são tipados em colunas próprias. Selects possuem opções com chave estável, label, estado ativo e ordem. Multiselect usa tabela de junção.

Campos nativos continuam sendo colunas reais. Eles podem ter apresentação configurável, mas não podem ter o tipo técnico alterado.

### 2.9 Imports e Google Forms

Imports suportam XLSX e Google Forms.

Regras:

- um arquivo por destino;
- modos `CREATE_ONLY`, `CREATE_AND_UPDATE` e `UPDATE_ONLY`;
- staging, mapeamento, preview, validação, duplicatas, revisão e aplicação em lotes;
- arquivo XLSX descartado após processamento;
- relatório final permanente;
- detalhes temporários de erro e revisão por 30 dias;
- dados crus de linhas bem-sucedidas removidos após conclusão;
- dados temporários do Forms removidos sete dias após finalização.

Google Forms:

- uma conexão Google administrativa central;
- login dos usuários não concede acesso automático aos Forms;
- integrações com status `ACTIVE`, `PAUSED`, `CLOSED` ou `CONNECTION_ERROR`;
- sem sincronização recorrente;
- sincronização manual, final e reprocessamento;
- fechamento só conclui após sincronização final bem-sucedida;
- edições externas posteriores geram revisão, nunca sobrescrita silenciosa;
- exclusão no Google não exclui dados locais;
- reabertura processa somente respostas novas ou alteradas.

### 2.10 Duplicatas

A fila persistente de revisão é somente para Profiles.

Níveis:

- `VERY_STRONG`;
- `PROBABLE`;
- `POSSIBLE`.

Nome nunca é o único critério. Não haverá porcentagem exibida.

Origens:

- criação manual;
- import;
- Google Forms;
- OCR;
- edição de Profile;
- inspeção manual.

Resoluções:

- usar existente;
- mesclar;
- criar mesmo assim;
- descartar sugestão;
- dados atualizados.

Merge transfere relacionamentos e exclui a origem em transação. Não haverá undo nem snapshot completo.

Duplicatas de documentos e contas são verificadas inline, sem fila separada.

Inspeção geral só ocorre manualmente via job. Não haverá cron periódico de duplicatas.

### 2.11 Search

Search consulta dados estruturados diretamente no PostgreSQL.

Não busca:

- conteúdo bruto de OCR;
- conteúdo de arquivos;
- anexos;
- respostas brutas de IA.

Tecnologias:

- B-tree para igualdade, filtros, datas e FKs;
- `pg_trgm` para buscas parciais e aproximadas;
- `unaccent` e normalização de caixa/espaços;
- consultas próprias por domínio retornando um formato comum.

Não haverá inicialmente:

- Elasticsearch;
- Meilisearch;
- Typesense;
- tabela universal duplicada de Search;
- embeddings para busca comum;
- Neon Search sem benchmark que demonstre benefício.

Consultas com um ou dois caracteres continuam permitidas, com limites e scans controlados quando necessário.

### 2.12 AI Chat

O AI Chat é um consultor privado e somente leitura.

Ele deve aceitar perguntas simples, moderadas e complexas sobre qualquer dado estruturado do sistema.

Fluxo:

1. interpretar intenção;
2. gerar plano de consulta tipado;
3. validar plano e permissões;
4. converter para consultas seguras;
5. executar pelo Query Engine;
6. sintetizar resposta;
7. retornar referências navegáveis para registros ou tabelas.

A IA nunca terá:

- conexão direta ao banco;
- credenciais do PostgreSQL;
- ferramenta genérica de SQL arbitrário;
- acesso direto aos repositories.

Threads são privadas por usuário. Grandes resultados não são copiados para as mensagens; ficam critérios, contagem, resumo e links para os dados atuais.

Runs técnicos detalhados permanecem por 30 dias. Métricas agregadas podem permanecer sem conteúdo pessoal.

### 2.13 OCR

OCR é opcional e baseado inicialmente em visão multimodal.

Regras:

- revisão humana obrigatória;
- nenhum campo é alterado automaticamente;
- comparação entre valor atual e sugerido;
- resultado validado contra schema do tipo;
- `store: false` quando o provedor permitir;
- sem retenção de resposta bruta ou raciocínio;
- salvar apenas provedor, modelo, status, duração e erro operacional.

### 2.14 Exports

Somente XLSX.

Tipos principais:

- Profile completo;
- tabela de documentos;
- tabela de contas;
- tabela de entidade customizada.

O export completo de Profiles contém abas para Profiles, documentos, contas e tipos customizados, relacionadas por UUID.

Não inclui:

- anexos;
- observações internas;
- datas e versões técnicas;
- usuários;
- auditoria.

Arquivo privado no R2, URL assinada e validade inicial de 24 horas. O histórico da operação permanece depois da remoção do arquivo.

### 2.15 Usuários, autenticação e permissões

Acesso por Google OAuth e allowlist de e-mail.

Roles:

- `MEMBER`;
- `ADMIN`;
- `SUPERADMIN`.

Exatamente um SuperAdmin ativo. A interface não permite desativar, excluir ou rebaixar o único SuperAdmin.

Estados de usuário:

- `INVITED`;
- `ACTIVE`;
- `DEACTIVATED`;
- `DELETED`.

Exclusão de usuário é lógica e anonimizada para preservar FKs:

- e-mail, subject e avatar são removidos;
- nome passa a “Usuário excluído”;
- a linha permanece.

Sessões:

- token opaco aleatório;
- somente hash SHA-256 no banco;
- cookie `Secure`, `HttpOnly` e `SameSite=Lax`;
- validade de 24 horas;
- nenhuma sessão em `localStorage`;
- revogação imediata ao desativar usuário.

### 2.16 Preferências, flags e notificações

Preferências do usuário:

- aparência;
- densidade de tabelas;
- linhas por página;
- visibilidade, ordem, largura e fixação de colunas.

Filtros, paginação, ordenação e agrupamentos permanecem na URL, não nas preferências.

Feature flags são globais para a instalação e liberadas por roles, nunca por usuário individual.

Notificações:

- uma linha por destinatário;
- leitura independente;
- `target_url` em vez de várias FKs opcionais;
- expiração padrão de sete dias;
- expirar uma notificação não remove o relatório relacionado.

### 2.17 Auditoria e retenções

Auditoria de segurança separada dos logs técnicos, com retenção inicial de 90 dias.

Retenções principais:

- anexos excluídos: sete dias;
- notificações: sete dias;
- arquivo de export: 24 horas;
- revisões de duplicatas resolvidas: sete dias;
- dados temporários de Forms: sete dias após finalização;
- detalhes temporários de import: 30 dias;
- runs detalhados de IA: 30 dias.

Um processo central de housekeeping executa as limpezas.

## 3. Decisões de domínio e banco

### 3.1 Instalação única

Foi removida toda preparação para multi-organização:

- sem tabela `organizations`;
- sem `organization_id`;
- sem FKs compostas por tenant;
- sem escopo de queries por organização;
- sem configuração multi-tenant.

Uma futura migração deverá ser explícita e intencional.

### 3.2 PostgreSQL

Banco principal: Neon PostgreSQL.

Convenções:

- nomes em inglês e `snake_case`;
- UUIDv7 para entidades principais;
- IDs gerados pelo backend Go, com default do banco apenas como proteção;
- `timestamptz` para instantes;
- `date` para datas civis;
- `NUMERIC(14,2)` para dinheiro;
- texto para CPF, telefone e números de documento;
- `text` como padrão para conteúdo textual, usando `varchar` somente com limite técnico real;
- `version INTEGER DEFAULT 1` para concorrência otimista.

Migrations:

- SQL versionado no Git;
- Tern v2;
- aplicadas primeiro em staging;
- migrations já aplicadas nunca são alteradas;
- rollback apenas quando realmente seguro;
- produção prefere correção para frente;
- seeds separados.

Seeds iniciais:

- tipos oficiais de documentos;
- tipos de contas;
- estados civis;
- clubes e categorias aprovadas;
- roles;
- feature flags;
- configuração padrão da aplicação.

A lista de equipes de gincana será adicionada após revisão dos dados atuais.

### 3.3 Exclusões

Não haverá `deleted_at` indiscriminado.

- Profile, documento, conta e conversa: exclusão física;
- anexos: lixeira de sete dias;
- usuários: anonimização;
- tipos e campos customizados: desativação antes de exclusão definitiva;
- temporários: housekeeping.

### 3.4 Constraints críticas

O PostgreSQL deve garantir, além do backend:

- CPF único quando preenchido;
- e-mail e `google_subject` únicos quando preenchidos;
- um único SuperAdmin ativo;
- exatamente um owner por attachment;
- exatamente um owner por custom field value;
- no máximo um uso ativo por documento ou conta;
- resposta externa única por integração do Forms;
- coerência dos valores tipados;
- dinheiro nunca como float.

## 4. Arquitetura aprovada

### 4.1 Monólito modular

Unidades executáveis:

- `web`;
- `api`;
- `worker`.

API e worker são produzidos pelo mesmo projeto Go e compartilham domínio, casos de uso, banco, integrações e configurações.

Não haverá inicialmente:

- microserviços;
- serviço separado de Search;
- serviço separado de IA;
- serviço separado de imports;
- Redis;
- RabbitMQ;
- GraphQL;
- backend Node intermediário.

### 4.2 Frontend

Stack aprovada:

- React 19;
- TypeScript 6 na inicialização, com migração planejada para TypeScript 7.1+ quando a API programática e o ecossistema estiverem maduros e sem necessidade de manter dois compiladores;
- Vite 8;
- Node.js 24 LTS;
- pnpm 11;
- TanStack Router v1;
- TanStack Query v5;
- TanStack Table v8;
- TanStack Virtual v3;
- TanStack Form v1;
- Valibot somente na aplicação;
- Oxlint;
- Oxfmt;
- Vitest;
- Testing Library;
- Playwright;
- axe-core no Playwright, após os gates de segurança.

Aplicação SPA, sem SSR, React Server Components ou TanStack Start.

Divisão de estado:

- URL: Router;
- estado da API: Query;
- formulários: Form;
- visual local: React;
- preferências persistidas: backend.

Sem Redux ou Zustand inicialmente.

React Compiler fica desativado inicialmente. Ele só será avaliado com benchmark real e integração madura.

### 4.3 Gymkhana-UI

Todos os componentes visuais serão próprios.

Não usar:

- Radix;
- Base UI;
- React Aria Components;
- shadcn/ui;
- Material UI;
- Chakra;
- Mantine;
- Ant Design;
- outros kits de componentes.

Base:

- React;
- HTML semântico;
- CSS Modules;
- CSS custom properties;
- APIs nativas do navegador;
- ARIA somente quando necessária.

Phosphor Icons será a biblioteca de ícones inicial. A API pública dos componentes recebe `ReactNode` e não expõe tipos específicos da biblioteca.

Componentes de layout incluem `AppShell`, compound components `Page.*`, `Stack`, `Inline`, `Cluster`, `Grid`, `Split`, `Container`, `Section`, `Divider`, `ScrollArea` e `ResizablePanel`.

`Page` usa composição, não um objeto gigante de configuração.

Datas usam `Intl` e lógica própria, separando `DATE`, `YEAR_MONTH` e `DATETIME`.

O repositório UI terá playground Vite em vez de Storybook inicialmente.

### 4.4 Backend Go

Stack:

- Go 1.26;
- `net/http`;
- `pgx/v5` nativo;
- `pgxpool`;
- sqlc;
- sem ORM;
- Tern v2;
- River OSS;
- OpenAPI 3.1;
- `oapi-codegen` com servidor strict sobre `net/http`;
- `openapi-typescript` e `openapi-fetch` no frontend.

Organização por domínio:

```text
cmd/
├── api/
└── worker/

internal/
├── auth/
├── profiles/
├── documents/
├── bills/
├── customfields/
├── imports/
├── forms/
├── duplicates/
├── search/
├── assistant/
├── attachments/
├── exports/
├── jobs/
├── admin/
├── platform/
└── database/
```

Interfaces só existem quando há substituição real, limite entre domínio e infraestrutura ou necessidade concreta de teste.

### 4.5 REST e OpenAPI

REST + JSON em `/api/v1`.

OpenAPI 3.1 é a fonte de verdade do contrato.

Código gerado é versionado e nunca editado manualmente. O CI regenera Go e TypeScript e falha se houver divergência.

Erro padrão:

```json
{
  "error": {
    "code": "PROFILE_NOT_FOUND",
    "message": "Profile não encontrado.",
    "request_id": "uuid",
    "field_errors": []
  }
}
```

### 4.6 Jobs

River OSS é infraestrutura interna.

O domínio não conhece:

- River;
- IDs internos do River;
- tabelas internas;
- estados específicos da biblioteca.

Não haverá `river_job_id` em entidades de negócio.

Cada operação possui seu próprio ID, e o adaptador recebe apenas esse ID.

Nenhuma tabela genérica `jobs` duplicará o estado técnico do River. Cada módulo mantém somente status, progresso e relatório úteis à interface.

Jobs obrigatórios para operações longas:

- imports;
- exports;
- OCR em lote;
- Forms;
- inspeção de duplicatas;
- mass delete;
- atualização em massa;
- housekeeping.

### 4.7 Storage e XLSX

Cloudflare R2 por adapter compatível com S3.

O domínio conhece somente `ObjectStore` e chaves opacas.

Uploads diretos por URL assinada, confirmação posterior e validação de:

- tamanho;
- extensão;
- MIME declarado;
- assinatura real;
- existência no R2;
- hash.

Formatos iniciais: JPEG, PNG, WebP e PDF. SVG, HTML, executáveis, compactados e tipos genéricos não são aceitos.

XLSX usa Excelize 2.11 pela correção de vulnerabilidades, panics e melhorias de memória.

- leitura por iterador de linhas;
- export por `StreamWriter`;
- `pgx.CopyFrom` para cargas em lote;
- processamento em batches e checkpoints;
- sem transação única gigante;
- sem carregar planilha inteira na memória.

### 4.8 IA

Provedores iniciais:

- OpenAI;
- Google.

SDKs oficiais isolados em adapters. Nenhum tipo do provedor cruza o limite do adapter.

Não usar inicialmente:

- LangChain;
- LangGraph;
- CrewAI;
- Semantic Kernel;
- Vercel AI SDK;
- framework genérico de agentes.

Orquestração própria, pequena e tipada.

AI Chat usa SSE sobre `fetch`, não WebSocket inicialmente.

### 4.9 Segurança

Autenticação Google no backend Go.

- OAuth 2.0;
- `state` de uso único;
- PKCE S256;
- validação de issuer, audience, assinatura e expiração;
- tokens Google nunca enviados ao frontend;
- conexão de Forms separada do login.

CSRF:

- token associado à sessão;
- validação de `Origin`;
- `SameSite=Lax` como camada complementar;
- nenhum efeito colateral em GET, HEAD e OPTIONS.

Rate limiting:

- borda;
- limites locais no Go;
- PostgreSQL para quotas globais quando necessárias;
- sem Redis inicialmente.

Segredos recuperáveis usam AES-256-GCM da biblioteca padrão, com `nonce` e `key_version`. A chave mestra fica fora do banco e do Git.

Logs devem redigir tokens, cookies, CPF, documentos, chaves, anexos e conteúdo integral de IA.

### 4.10 Logs e saúde

Logs estruturados com `log/slog`.

- JSON em produção;
- legível localmente;
- `request_id` em todas as requisições;
- sem bodies completos ou SQL com dados pessoais;
- auditoria separada.

OpenTelemetry não entra na primeira versão.

Health checks:

- `/health/live`;
- `/health/ready`;
- saúde separada do worker.

Falhas de R2, Google ou IA não derrubam a API inteira.

### 4.11 Cache

Sem Redis.

- TanStack Query no frontend;
- CDN da Vercel para assets;
- índices PostgreSQL;
- `pgxpool`;
- ETag para metadados estáveis;
- cache em memória somente para configurações pequenas e globais.

Dados privados usam normalmente `Cache-Control: private, no-store`.

### 4.12 Testes e segurança de dependências

Frontend:

- Vitest;
- Testing Library;
- Playwright;
- axe-core.

Backend:

- `testing`;
- `httptest`;
- Testcontainers com PostgreSQL real;
- fuzzing nativo;
- race detector.

Não usar SQLite como substituto do PostgreSQL.

Pipeline Go:

- `gofmt`;
- `go vet`;
- `staticcheck`;
- `go test`;
- `go test -race` periodicamente;
- `govulncheck`.

Supply chain:

- OSV-Scanner;
- Dependency Review quando disponível;
- Dependabot Alerts;
- scan da imagem final;
- GitHub Actions fixadas por SHA;
- versões exatas e lockfiles versionados;
- sem merge automático de updates.

Não existe garantia contra vulnerabilidade ainda desconhecida. O gate prático é: nenhum release com vulnerabilidade conhecida, aplicável e sem mitigação aprovada.

### 4.13 Deploy inicial

Arquitetura inicial sem custo fixo planejado:

```text
Vercel Hobby
├── React SPA
├── assets
├── preview deployments
└── proxy /api

Google Cloud Run Service
└── gymkhana api

Google Cloud Run Job
├── gymkhana worker --drain
├── imports
├── exports
├── OCR
├── duplicatas
└── Forms

Cloud Scheduler
└── recuperação + housekeeping

Neon
└── PostgreSQL + River

Cloudflare R2
└── anexos e exports temporários
```

O worker não fica permanentemente ligado. A API registra a operação, enfileira no River e solicita uma execução do Cloud Run Job. O worker processa até a fila ficar ociosa e termina.

Um Scheduler inicia recuperação e housekeeping. A arquitetura deve permitir futura migração integral para GCP sem alterar domínio, contratos ou entidades.

Frontend e API aparecem na mesma origem pública:

- `/` para SPA;
- `/api/v1/*` encaminhado ao Cloud Run.

O frontend usa caminhos relativos. A URL interna do provedor não entra no código da aplicação.

## 5. Responsabilidade dos repositórios

### 5.1 Gymkhana-Database

Produto e orquestrador:

- web React;
- API Go;
- worker;
- migrations;
- OpenAPI;
- SQL;
- domínio do produto;
- integrações;
- configuração e deploy.

Consome versões fixadas de Gymkhana-UI e Gymkhana-Core.

### 5.2 Gymkhana-UI

Biblioteca React visual:

- componentes próprios;
- AppShell;
- Page;
- DataGrid visual;
- tokens;
- CSS Modules;
- Phosphor Icons;
- playground;
- acessibilidade e testes.

Não contém regras de Profile, chamadas de API ou permissões do produto.

### 5.3 Gymkhana-Core

Módulo Go reutilizável, sem React e sem acesso direto ao banco:

- normalização;
- operadores e AST de consulta;
- matching genérico;
- pontuação de duplicidade;
- contratos neutros de IA;
- orquestração neutra de ferramentas;
- formatos de resultado.

Não contém repositories PostgreSQL, handlers HTTP, River, SDKs de provedores ou regras visuais.

Go e TypeScript compartilham contratos por OpenAPI, não por um pacote multi-linguagem.

## 6. Política de versões

Comparar sempre:

- a linha mais madura e comprovada;
- a linha estável mais atual.

Uma versão mais nova só entra quando trouxer pelo menos um benefício concreto:

- correção de segurança;
- correção de bug aplicável;
- performance mensurável;
- compatibilidade necessária;
- feature usada pelo projeto;
- redução clara de complexidade.

Betas e RCs somente em branches de pesquisa ou staging atrás de feature flag. Produção usa versões estáveis.

O patch exato será fixado na inicialização de cada projeto após verificação de compatibilidade, segurança e changelog.

## 7. Decisões explicitamente adiadas

- multi-organização;
- Redis;
- OpenTelemetry;
- React Compiler;
- TypeScript 7 antes da maturidade do ecossistema necessária ao projeto;
- Neon Search sem benchmark;
- full-text search indiscriminado;
- Storybook;
- bibliotecas de componentes;
- SSR;
- microserviços;
- histórico de uso de documentos e contas;
- anexos diretos em Profile;
- engines tradicionais de OCR;
- frameworks genéricos de agentes;
- execução de SQL arbitrário pela IA.

## 8. Próxima etapa

**Etapa 5 — divisão física dos repositórios e packages.**

Objetivos:

- definir estrutura definitiva de diretórios;
- estratégia de versionamento e releases;
- consumo entre os três repositórios;
- CI de cada repositório;
- publicação do UI e Core;
- contratos e ownership;
- fluxo de desenvolvimento sem commits ou builds desnecessários.

Este documento deve ser atualizado novamente ao final da Etapa 5 antes do início da Etapa 6.

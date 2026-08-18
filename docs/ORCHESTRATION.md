# Gymkhana Database — Regras de Produto, Domínio e Arquitetura

> **Escopo deste documento:** decisões permanentes aprovadas para o rebuild do Gymkhana Database.  
> **Inclui:** regras de negócio, domínio, arquitetura, stack escolhida, responsabilidades técnicas, alternativas rejeitadas e os motivos das decisões.  
> **Não inclui:** progresso de desenvolvimento, próxima ação, branches, PRs, versões publicadas, releases ou checklists de execução.  
> **Acompanhamento operacional único:** [issue mestre #31](https://github.com/Pherlsz/Gymkhana-Database/issues/31).  
> **Repositórios relacionados:** `Pherlsz/Gymkhana-Core` e o legado `Gymkhana-Database-Vercel`.

Este arquivo é a fonte permanente das regras de produto e das invariantes arquiteturais. Ele só deve mudar quando uma decisão aprovada de negócio, domínio, segurança, experiência, stack ou arquitetura for alterada.

Nenhum documento adicional deve ser criado para acompanhar andamento ou próxima ação. Documentação técnica específica pode existir quando necessária para operar uma funcionalidade real, mas não substitui a issue mestre como tracker e não deve duplicar o estado do projeto.

## Arquitetura escolhida e racional

Esta seção registra não apenas **o que** foi escolhido, mas também **por que** foi escolhido. Esses motivos fazem parte da arquitetura e devem ser preservados para impedir que uma implementação futura reintroduza complexidade já rejeitada.

### Topologia da aplicação

```text
React SPA
   │
   ▼
cmd/api ───────────────► PostgreSQL / Neon
   │                           ▲
   │                           │
   ├──────────────► Cloudflare R2
   │
   └──────────────► providers externos aprovados

cmd/worker ─────────────► PostgreSQL / River / providers

cmd/migrate ────────────► PostgreSQL
```

Executáveis e responsabilidades:

- `React SPA`: interface privada, rotas, formulários, grids, Search, AI Assistente e administração;
- `cmd/api`: HTTP, autenticação, autorização, contratos, composição de módulos e operações síncronas;
- `cmd/worker`: jobs reais, imports, matching, OCR, processamento pesado e housekeeping quando esses módulos existirem;
- `cmd/migrate`: aplicação e inspeção explícita de migrations, separado do lifecycle do servidor;
- nenhum executável contém regras de domínio em `cmd`; `cmd` apenas configura e compõe dependências.

**Motivo da separação:** API, worker e migrations possuem lifecycle, escala e permissões diferentes. Separá-los evita que deploy web execute migrations automaticamente, permite que jobs escalem independentemente e mantém o backend como um único produto modular, sem virar microserviços.

### Baseline tecnológica aprovada

As versões abaixo representam a baseline arquitetural definida para o rebuild. Atualizações compatíveis podem ocorrer nos manifests e PRs sem transformar o Orchestration em tracker de versões, desde que não alterem os contratos e motivos registrados aqui.

#### Backend e dados

- Go 1.26;
- biblioteca padrão `net/http`;
- pgx v5;
- sqlc;
- PostgreSQL 18;
- Tern v2;
- River OSS quando existirem jobs reais;
- OpenAPI 3.1;
- oapi-codegen em modo strict server;
- SQL explícito, parametrizado e versionado;
- executáveis compilados como binários independentes.

#### Frontend

- React 19;
- TypeScript 6 como baseline inicial;
- Vite 8;
- Node.js 24 LTS;
- pnpm 11;
- TanStack Router;
- TanStack Query;
- TanStack Form;
- Ant Design como base visual (Table, Form, Select, Pagination, overlays);
- Lucide (`lucide-react`) para ícones de chrome do produto;
- TanStack Table apenas onde a grade do produto ainda não usa a Table do Ant Design (DataGrid de Search/Operations até unificação);
- Valibot;
- openapi-typescript e openapi-fetch;

#### Infraestrutura

- PostgreSQL gerenciado no Neon;
- Vercel para a SPA;
- Cloud Run Service para a API;
- Cloud Run Job para o worker;
- Cloud Scheduler para recovery e housekeeping;
- Cloudflare R2 para arquivos privados;
- Cloudflare Access apenas como camada externa opcional.

#### Qualidade e geração

- gofmt, `go vet`, race tests, Staticcheck e govulncheck no backend;
- Oxlint, Oxfmt, Vitest, typecheck e build de produção no frontend;
- OSV-Scanner e dependency review para dependências;
- OpenAPI e sqlc gerados deterministicamente e versionados;
- Actions pinadas e workflows separados por responsabilidade.

### Por que monólito modular

Foi escolhido um **monólito modular**, não microserviços, porque:

- o produto é privado, com grupo fechado e volume operacional moderado;
- módulos possuem muitas transações e relações compartilhadas;
- deployment, debugging, migrations e consistência são mais simples em uma aplicação;
- limites de domínio podem ser preservados por packages e contratos sem custo de rede;
- microserviços adicionariam observabilidade distribuída, retries, versionamento entre serviços e operação sem ganho proporcional;
- a arquitetura ainda permite separar API e worker por processo sem fragmentar o domínio em serviços independentes.

Microserviços só podem ser reconsiderados quando houver evidência concreta de escala, isolamento operacional ou ownership que justifique a complexidade.

### Por que Go no backend

Go foi escolhido para reduzir complexidade e produzir um backend previsível:

- tipagem estática e toolchain simples;
- binários pequenos e fáceis de executar em containers/Cloud Run;
- baixo consumo de memória e inicialização rápida;
- concorrência adequada para HTTP, imports, streaming e jobs;
- excelente suporte a PostgreSQL e geração de contratos;
- menor superfície de dependências que frameworks backend maiores;
- facilita separar domínio puro de adapters de banco, HTTP e providers.

Não haverá backend Node paralelo. Manter uma única linguagem no servidor evita duplicação de modelos, regras, ferramentas e runtime.

### Por que `net/http` e não um framework web obrigatório

A biblioteca padrão foi escolhida porque:

- cobre o volume e as necessidades do produto;
- reduz lock-in e dependências transitivas;
- mantém middleware, timeouts, body limits e lifecycle explícitos;
- integra diretamente com handlers gerados pelo OpenAPI;
- evita abstrações de framework que escondam contratos HTTP ou dificultem testes.

Uma biblioteca auxiliar pequena pode ser adotada quando resolver um problema real, mas não deve substituir o contrato explícito do HTTP nem criar uma arquitetura baseada no framework.

### Por que PostgreSQL e Neon

PostgreSQL foi escolhido em vez de MongoDB, Firebase ou um armazenamento documental como base principal porque o produto depende de:

- relações fortes entre Profiles, documentos, contas, usos, custom data e anexos;
- constraints, transações e concorrência otimista;
- joins e filtros cruzados;
- agregações, patterns, sets e combinações;
- candidate generation de duplicatas;
- Query Engine e consultas ad hoc controladas;
- integridade referencial e migrations explícitas.

Neon foi escolhido como PostgreSQL gerenciado porque reduz operação inicial, possui boa integração com ambientes serverless e permite começar com custo baixo. O desenho deve manter processamento próximo ao banco e evitar transferência desnecessária para respeitar limites de egress.

Firebase/MongoDB poderiam simplificar CRUD isolado, mas tornariam mais complexas as consultas relacionais e combinatórias que são o objetivo central do produto.

### Por que pgx + sqlc + SQL explícito, sem ORM

O produto é orientado a dados e consultas complexas. SQL explícito foi escolhido porque:

- permite controlar joins, índices, locks, paginação e planos de execução;
- evita queries ocultas, N+1 e comportamento implícito de ORM;
- aproveita recursos reais do PostgreSQL;
- mantém performance e egress previsíveis;
- permite que o Query Engine tenha uma fronteira clara de compilação;
- sqlc gera tipos Go a partir do SQL aprovado, reduzindo boilerplate sem retirar controle;
- pgx oferece integração PostgreSQL direta e eficiente.

Não introduzir ORM como segunda camada de persistência. Isso criaria dois modelos de acesso e tornaria mais difícil entender qual SQL realmente é executado.

### Por que Tern para migrations

Tern foi escolhido para migrations SQL ordenadas e explícitas porque:

- combina com pgx/sqlc e com a decisão de usar SQL diretamente;
- é leve e previsível;
- permite validar banco vazio, upgrade e rollback onde aplicável;
- não acopla schema a um ORM;
- mantém migrations revisáveis como parte do produto.

Migrations não são executadas automaticamente pelo container do banco nem silenciosamente no startup da API.

### Por que REST JSON + OpenAPI, sem GraphQL

REST com OpenAPI foi escolhido porque:

- as operações do produto possuem contratos claros e autorizáveis;
- geração determinística mantém Go e TypeScript sincronizados;
- facilita validação, documentação, testes e error envelopes estáveis;
- torna caching, idempotência e semântica HTTP explícitos;
- reduz a superfície de autorização dinâmica e complexidade de GraphQL;
- combina com ferramentas tipadas do Query Engine sem expor o schema físico.

GraphQL não é necessário para permitir consultas flexíveis. Flexibilidade pertence ao QueryPlan/Query Engine tipado, não a uma API que exponha livremente o grafo de dados.

### Por que React SPA + Vite, sem SSR

A aplicação é privada e autenticada; SEO e renderização pública não são requisitos. SPA com Vite foi escolhida porque:

- reduz infraestrutura e lifecycle de servidor frontend;
- possui desenvolvimento e build rápidos;
- pode ser entregue como assets estáticos pela Vercel;
- mantém a API como única autoridade de dados e segurança;
- simplifica deploy, preview e rollback;
- evita duplicar lógica entre renderização servidor/cliente.

SSR só deve ser reconsiderado se surgir um requisito real de conteúdo público indexável ou uma limitação comprovada da SPA.

### Por que TanStack e divisão explícita de estado

Cada categoria de estado possui um responsável:

- TanStack Router: URL, filtros, paginação, sorting, grouping e tabs navegáveis;
- TanStack Query: cache e estado vindo do servidor;
- TanStack Form: lifecycle dos formulários;
- Ant Design Table (`virtual`): grades do produto (`/tables/*`);
- React: estado visual transitório;
- backend: preferências persistentes.

Redux/Zustand foram rejeitados inicialmente porque criariam outra fonte global de estado, duplicando Router e Query sem necessidade comprovada. Podem ser reconsiderados apenas para um problema que não seja resolvido por essa divisão.

### Por que Valibot

Valibot foi escolhido para validação frontend porque:

- é TypeScript-first;
- possui composição explícita;
- mantém bundles e dependências enxutos;
- integra bem com TanStack Form;
- serve para validar a borda da UI sem tentar substituir as validações autoritativas do backend.

### Por que Ant Design

Ant Design é a base visual do produto. A UI própria from-scratch foi dropada: o cadastro é grade-denso e mobile-first, e reconstruir Table, Form, Select, Pagination e overlays no repositório duplicaria o que a biblioteca já cobre.

- Material UI, shadcn e um design system próprio não são a base;
- componentes visuais permanecem neste repositório (wrappers e composição); não há pacote UI compartilhado;
- ícones de chrome do produto usam Lucide (`lucide-react`); glifos internos do Ant (Select, DatePicker, Pagination) permanecem no Ant;
- a Table do Ant Design, com `virtual` e `scroll.x`/`scroll.y` numéricos, é o motor da grade `/tables/*`. TanStack Table não é o sheet.

### Por que extrair hooks, componentes e módulos pequenos

Página monolítica (milhares de linhas misturando URL, query, colunas, inspector e markup) impede revisão, teste e mudança local. O frontend segue o mesmo recorte do monólito modular: muito comportamento atrás de uma interface pequena, não um arquivo único.

- Estado React isolável vira hook (`useX`). Hook só existe quando usa estado, efeito, contexto ou outro hook; função pura continua função.
- Markup nomeável vira componente. O mesmo vale para helpers, painéis e módulos equivalentes no backend: o que tem dono e interface próprios não fica embutido no arquivo da página ou do handler.
- Reuso real (dois ou mais módulos, ou o shell da aplicação) mora em pasta global. Código de um só módulo mora junto desse módulo.
- Não promover para global “por precaução”. Não criar pacote npm de UI. Não fatiar em arquivos rasos só para reduzir contagem de linhas se o recorte não tiver interface própria.

Detalhe operacional: §23.5.

### Por que Gymkhana-Core separado

Gymkhana-Core concentra lógica Go pura e independente de infraestrutura, como:

- normalização;
- datas civis;
- canonicalização;
- fingerprints;
- algoritmos de matching/solver quando comprovadamente reutilizáveis.

A separação evita copiar lógica entre aplicações, mas não deve virar um repositório genérico de abstrações prematuras. O Database continua dono das regras de produto e dos adapters.

### Por que River e não Redis/RabbitMQ

River foi escolhido para jobs quando eles realmente existirem porque:

- usa PostgreSQL já presente na arquitetura;
- reduz um serviço operacional adicional;
- permite coordenação transacional com dados do produto;
- é suficiente para imports, matching, OCR, retries e housekeeping previstos;
- preserva observabilidade e idempotência sem uma fila fictícia permanente.

Redis e RabbitMQ não entram inicialmente porque o produto ainda não possui necessidade de throughput ou fan-out que justifique outra infraestrutura.

### Por que Vercel, Cloud Run, R2 e Cloud Scheduler

- **Vercel para SPA:** entrega de assets e previews simples, alinhada ao frontend Vite; pushes devem ser consolidados para evitar builds desnecessários.
- **Cloud Run Service para API:** executa o binário Go em container, suporta scale-to-zero e mantém caminho coerente para infraestrutura GCP futura.
- **Cloud Run Job para worker:** jobs possuem lifecycle diferente da API e não devem ocupar processo HTTP permanente.
- **Cloud Scheduler:** dispara recovery/housekeeping sem criar servidor de cron próprio.
- **Cloudflare R2:** separa blobs privados do PostgreSQL, suporta URLs assinadas e reduz dependência de filesystem local.

Railway foi rejeitado porque adicionaria outra plataforma/custo sem necessidade, enquanto Vercel + Neon + Cloud Run cobrem as responsabilidades definidas e mantêm uma rota de evolução mais clara.

### Por que Cloudflare Access é opcional

Cloudflare Access pode filtrar acesso antes da aplicação, mas não substitui:

- Google OAuth;
- sessão da aplicação;
- roles e permissions;
- auditoria;
- constraints de domínio.

Isso evita que a segurança interna dependa exclusivamente de uma configuração externa e permite executar localmente sem reproduzir toda a camada Cloudflare.

### Por que não usar OpenTelemetry inicialmente

OpenTelemetry não faz parte da baseline porque:

- o produto começa como monólito modular e grupo fechado;
- logs estruturados, request IDs, auditoria e métricas da plataforma cobrem o diagnóstico inicial;
- instrumentação distribuída adicionaria dependências, configuração e custo operacional antes de existir sistema distribuído;
- pode ser reconsiderado quando houver uma necessidade observável que as ferramentas atuais não resolvam.

### Por que não usar agent framework como base do AI Assistente

O Assistente e as tarefas complexas usam ferramentas tipadas e Query Engine próprio porque:

- o domínio exige controle exato sobre catálogo, permissões e planos;
- frameworks de agentes podem esconder loops, estado e custos;
- a segurança depende de impedir SQL e ações arbitrárias;
- result sets, referências e explicações são contratos do produto;
- evita lock-in em CrewAI, LangGraph ou framework equivalente para uma função que pode ser implementada com primitives explícitas.

Frameworks podem ser avaliados como implementação interna futura, mas não podem substituir os contratos do produto ou se tornar requisito arquitetural obrigatório.

### Alternativas rejeitadas na baseline

Sem nova decisão fundamentada, não introduzir como arquitetura principal:

- microserviços;
- backend Node;
- SSR;
- GraphQL;
- ORM;
- MongoDB/Firebase como banco canônico;
- Redis ou RabbitMQ;
- Redux/Zustand;
- Material UI, shadcn ou UI própria from-scratch como base da UI;
- Railway;
- OpenTelemetry por padrão;
- agent framework obrigatório;
- MinIO local sem uso real;
- qualquer serviço criado apenas para representar arquitetura futura;
- páginas ou handlers monolíticos (arquivo único com milhares de linhas) no lugar de hooks, componentes e módulos com interface própria.

## 1. Objetivo do produto

Reconstruir o Gymkhana Database como uma aplicação privada, leve, extensível e segura para centralizar dados usados em gincanas e permitir consultas simples, relacionais e combinatórias sobre qualquer informação autorizada.

O produto deve armazenar e relacionar:

- pessoas físicas;
- documentos;
- contas e comprovantes;
- uso atual de documentos e contas;
- equipes de gincana e time de futebol (sócio clube) como campos da pessoa;
- campos e entidades customizadas;
- anexos;
- importações XLSX;
- integrações com Google Forms;
- resultados de busca e consultas estruturadas;
- candidatos a duplicidade e decisões humanas de merge;
- conversas e resultados do AI Assistente;
- sugestões multimodais/OCR;
- interpretações e soluções de tarefas complexas de gincana.

A capacidade central do sistema é consultar qualquer dado permitido, em qualquer campo ou relação, sem limitar o usuário a perguntas, filtros ou relatórios previamente cadastrados.

## 2. Princípios obrigatórios

- A aplicação é privada e destinada a um grupo fechado de usuários conhecidos.
- Existe uma única instalação do produto.
- Multi-organização e tenancy foram removidos completamente.
- Não devem existir `organizations`, `organization_id` ou abstrações equivalentes.
- `Profile` é o centro do domínio e substitui o antigo modelo genérico de pessoa/`Record`.
- Somente pessoas físicas fazem parte do escopo inicial.
- Documents, bills e registros customizados pertencentes a pessoa nunca existem órfãos.
- A IA é consultora e somente leitura; não cria, edita, exclui ou mescla dados diretamente.
- Nenhum SQL, código arbitrário, nome físico de tabela ou nome físico de coluna é exposto ao usuário ou fornecido livremente à IA.
- A cobertura semântica das consultas tem prioridade sobre atalhos baseados em perguntas pré-configuradas.
- Limites de custo, tempo, paginação e quota podem proteger o serviço, mas não podem reduzir artificialmente os tipos de pergunta ou os campos consultáveis.
- Processamento, filtros, candidate generation e agregações devem ocorrer próximos ao PostgreSQL para reduzir egress.
- A interface inicial é em `pt-BR` e deve permanecer preparada para i18n.
- Código, commits, PRs, workflows e documentação técnica de código permanecem em inglês.
- Datas técnicas são persistidas em UTC; datas civis são tratadas sem conversões indevidas de fuso.
- Filtros, paginação, ordenação, agrupamentos e abas relevantes são refletidos na URL.
- Acessibilidade, responsividade, segurança e desempenho são requisitos do produto, não etapas opcionais posteriores.
- No frontend, o que pode virar hook vira hook; hook ou componente reutilizável mora em pasta global; o restante fica no módulo dono. Páginas orquestram e não acumulam milhares de linhas. Detalhe: §23.5.
- Não há suporte offline-first.
- O produto não envia e-mails como funcionalidade inicial.
- O produto não possui módulo de notícias.

## 3. Usuários, autenticação e autorização

### 3.1 Autenticação

- O login da aplicação usa Google OAuth com allowlist de e-mails explícita.
- A identidade da conta é o e-mail allowlisted, como no legado. O subject do Google é metadado da identidade e não a chave da conta.
- Não existe senha local.
- Cloudflare Access pode ser adicionado como camada externa complementar, sem substituir autenticação, sessão ou autorização da aplicação.
- O primeiro login autorizado configurado como SUPERADMIN cria a conta privilegiada inicial.
- Demais logins autorizados podem ser criados como EXTERNAL.
- Um usuário inativo continua bloqueado mesmo que permaneça na allowlist de e-mails.

### 3.2 Sessões

- Sessões são opacas, revogáveis e mantidas no servidor.
- A duração aprovada é de 24 horas.
- Apenas o hash SHA-256 do token de sessão é persistido.
- Cookies são HttpOnly, host-only, SameSite=Lax e Secure fora de ambientes local/test.
- Logout revoga a sessão no servidor.
- Mudança efetiva de role ou estado ativo revoga todas as sessões do usuário afetado.
- Operações sem alteração real não criam gravações ou revogações desnecessárias.

### 3.3 Roles e permissões

- Roles da aplicação: `EXTERNAL`, `ADMIN` e `SUPERADMIN`.
- Deve existir exatamente um SUPERADMIN ativo.
- Autorizações são centralizadas por permissions/capabilities.
- Handlers e componentes não devem replicar regras de role de forma independente.
- SUPERADMIN não ignora constraints de domínio, privacidade ou integridade.
- ADMIN não pode alterar o próprio role ou estado de acesso pela administração genérica.
- ADMIN não pode alterar o SUPERADMIN pela administração genérica.
- Feature flags e capacidades são derivadas da role/permissão, sem `secure mode` paralelo.
- Cada admin visualiza e administra apenas os próprios formulários/conexões do Google Forms, salvo permissão superior explicitamente definida.

#### Sistema de capabilities

- Usuários EXTERNAL requerem grants explícitos de capabilities para acessar funcionalidades específicas.
- ADMIN e SUPERADMIN bypassam verificações de capability do produto (Profiles, tabelas, Search, Query, Assistente, OCR, operações, matching, Forms, anexos).
- ADMIN não acessa feature flags nem logs de segurança. Essas duas superfícies são exclusivas do SUPERADMIN.
- Somente EXTERNAL tem acesso variável por grant.
- Capabilities são gerenciadas via endpoints admin e armazenadas em `app_user_capabilities`.
- Capabilities disponíveis: `PROFILES`, `DATA_TABLES`, `SEARCH`, `OCR`, `OPERATIONS`, `MATCHING`, `GOOGLE_FORMS`, `ATTACHMENTS`, `CHAT`, `QUERY` e `CUSTOM_DATA`.
- Middleware HTTP verifica capabilities em todas as rotas protegidas.
- Grants e revogações são auditados.

### 3.4 Auditoria

- Auditoria é simples, essencial e separada dos logs técnicos.
- Devem ser auditados, quando aplicável: login, logout, falhas de acesso, administração de usuários, mudança de role/estado, revogação de sessão, criação, edição, duplicação, exclusão, importação, merge e operações destrutivas.
- Eventos são correlacionados com `request_id` e ator quando disponível.
- Falhas de persistência da auditoria devem ficar observáveis.
- Auditoria não armazena secrets, tokens, URLs assinadas, SQL, stack traces ou payloads sensíveis de providers.

### 3.5 Superfícies de configuração

Existem duas telas distintas. Não misturar gestão do sistema com preferência da pessoa logada.

**Administração** (ADMIN e SUPERADMIN; feature flags e logs de segurança só SUPERADMIN):

- gestão de users (EXTERNAL e, conforme regras de §3.3, ADMIN);
- exportação XLSX;
- integrações e o módulo `/forms`;
- chave de modelo compartilhada do Assistente e do OCR por modelo;
- definições de campo extra;
- demais ferramentas de operação do sistema.

No frontend atual, `/admin` e `/settings` permanecem como placeholders **Em desenvolvimento** até o redesenho dessas superfícies. As responsabilidades listadas aqui definem a implementação futura e não autorizam a exposição de conteúdo provisório.

**Preferências do usuário** (cada conta, inclusive EXTERNAL):

- nome de exibição da conta (`userName` do user da aplicação, não o Profile);
- tema claro/escuro;
- outras preferências futuras que sejam só da conta.

A chave de modelo é uma só, gerida na Administração, criptografada em repouso, e compartilhada por SUPERADMIN, ADMIN e EXTERNAL com permissão de Assistente (capability `CHAT`). EXTERNAL nunca vê nem edita o segredo. Sem chave configurada, Assistente e OCR-por-modelo ficam indisponíveis para todos; cadastro, tabelas, filtros e Search continuam.

Preferência de colunas visíveis da grade (vista) pode ser só no dispositivo ou na conta; isso ainda não está decidido.

## 4. Modelo central: Profile

### 4.1 Chave e estrutura

- Profile representa uma pessoa física.
- A chave é UUID; UUIDv7 é a preferência aprovada para novas entidades quando o suporte utilizado permitir.
- Profile possui timestamps e `version` para concorrência otimista.
- Não existe payload JSON genérico como modelo principal de Profile.
- Exclusão de Profile é definitiva, com confirmação explícita e auditoria.

### 4.2 Campos canônicos

- Nome completo é obrigatório.
- Nome social é opcional.
- CPF não é coluna de Profile. O número (só os dígitos/valor) vive em `document_presences` do tipo `cpf`. Validade, data e demais atributos ficam no exemplar (`documents`).
- O número de CPF, quando informado, é normalizado preservando zeros à esquerda.
- CPF não possui unicidade rígida global, pois possíveis duplicatas devem ser revisadas pelo fluxo de matching.
- Existe no máximo um e-mail por Profile.
- E-mail é normalizado para minúsculas.
- Existe no máximo um celular por Profile.
- Existe no máximo um telefone fixo/outro por Profile.
- Telefones são persistidos em formato normalizado; máscaras pertencem à borda da UI.
- Existe no máximo um endereço estruturado por Profile.
- O endereço pode conter logradouro, número, complemento, bairro, cidade, UF e CEP.
- Observações pessoais são opcionais.
- Equipe de gincana e time de futebol (sócio clube) são campos distintos da pessoa (`Profile`). Nenhum dos dois pertence ao user da aplicação.
- Naturalidade (`place_of_origin`) é campo próprio, distinto de cidade de nascimento. Não clonar um no outro.
- País de nascimento (`birth_country`) é distinto de nacionalidade.
- Data de casamento dos pais (`parents_wedding_date`) é distinta da data de casamento da pessoa.
- Clube de supermercado, animal de estimação e países de viagem são campos da pessoa.
- Cartão de crédito na pessoa guarda só bandeira (`card_brand`) e banco (`card_bank`). Não existe coluna de PAN, número do cartão, CVV ou validade.

### 4.3 Comportamento

- Valores são normalizados e validados na fronteira do domínio.
- Criação, leitura, edição, duplicação e exclusão são operações explícitas.
- Duplicar cria um novo Profile independente e deve abri-lo para revisão.
- Duplicação não significa merge e não herda IDs de entidades dependentes automaticamente.
- Atualizações usam `version`; conflito concorrente não pode sobrescrever silenciosamente outra alteração.
- Documents, bills, attachments e custom data aparecem como seções relacionadas, não como campos livres embutidos no Profile.

## 5. Equipe de gincana e time de futebol

Nem equipe de gincana nem time de futebol são módulo, entidade própria ou atributo do user da aplicação (`EXTERNAL` / `ADMIN` / `SUPERADMIN`). São dois campos distintos da pessoa (`Profile`), graváveis no cadastro.

### 5.1 Equipe de gincana

- Campo técnico `team` (rótulo “Equipe”).
- Identifica a equipe da gincana da pessoa (lista configurável: nomes como TNC, Força Tarefa, etc.).
- Não fica preso a uma enumeração irreversível; valores novos podem ser cadastrados.
- Histórico completo de equipes de gincana não entra automaticamente; precisa de decisão específica.

### 5.2 Time de futebol (sócio clube)

- Campo técnico `club_membership` (rótulo “Sócio clube”). Não é o mesmo campo que a equipe de gincana.
- Valores iniciais: `Internacional`, `Grêmio` e `Outros`. `Outros` não impede valores adicionais depois.
- Categoria de sócio (`membership_type`: Cartão, Sócio, Outro) é campo companheiro do cadastro, não um módulo.

## 6. Documentos

### 6.1 Propriedade e modelo

- Todo documento pertence a um Profile.
- Não existem documentos órfãos.
- O modelo é híbrido: tabela comum, detalhes específicos por família/tipo e custom fields tipados.
- Tipos de documento são administráveis.
- Cada tipo possui chave técnica estável que não muda com o rótulo exibido.
- Não deve existir JSON livre como fonte principal dos dados do documento.

### 6.2 Unicidade e validação

Cada tipo define uma política de unicidade, avaliada junto com o meio do exemplar:

- `NONE` — nenhuma restrição de unicidade;
- `PER_PROFILE` — único dentro do Profile para o mesmo meio;
- `GLOBAL_BY_TYPE` — único globalmente para aquele tipo e meio.

Além disso:

- tipos podem definir regex e validações controladas;
- validações devem produzir erros por campo, não mensagens opacas;
- valores devem preservar zeros à esquerda quando semanticamente relevantes;
- sequências alfanuméricas devem ser armazenadas sem perda de informação;
- não há leitura ou armazenamento obrigatório de código de barras no escopo aprovado.

### 6.3 Exemplar e meio

- Uma linha de `documents` é um exemplar cadastrado (original físico ou cópia digital), não o número informado e não a indicação/ausência.
- Cada exemplar tem um meio: `PHYSICAL` (original) ou `DIGITAL` (arquivo/scan).
- A mesma pessoa pode ter o mesmo identificador em um exemplar físico e um digital.
- Documento pode possuir observações e data associada quando o tipo exigir.
- Só o exemplar físico tem uso atual e guarda (`idle_custody`). Digital não se empresta: envia-se cópia.
- `idle_custody` do físico: `ORGANIZATION` (descansa na organização; típico de vencidos deixados conosco) ou `OWNER` (descansa com o dono). Devolver ao dono **não** apaga a linha: o original continua registrado, marcado com o dono, fora do inventário em mãos.
- O histórico completo de uso não faz parte do modelo inicial.

### 6.4 Presença do tipo na pessoa

Além do exemplar, cada par (Profile, document type) pode registrar presença. Identificadores técnicos em inglês: `absence`, `indication`, `informed_number`, `physical`, `physical_with_owner`, `digital`, e os badges derivados `physical_digital` e `physical_with_owner_digital`. Não criar colunas `possui_*` nem uma coluna de número por tipo em `profiles`. A célula da grade mostra a sigla do tipo (RG, CPF), nunca o nome do estado.

Estados persistidos em `document_presences` (esparso: só existe linha se alguém declarou ou informou número):

- `absence` — a pessoa afirma não ter aquele tipo. **Não** aparece como badge.
- `indication` — afirma ter, sem número e sem exemplar.
- `informed_number` — o valor (dígitos/identificador) está na pessoa. Validade e demais dados **não** entram aqui.

Exemplares em `documents`, `medium` `PHYSICAL` ou `DIGITAL`. No máximo um físico e um digital por (pessoa, tipo). O número do físico atualiza `informed_number`. Silêncio (nunca falou) não gera linha — na grade é indistinguível de ausência; filtro e ficha separam `unspecified` de `absence`.

Na grade, badge só no **positivo**. Chip compacto de densidade de tabela (cabe vários tipos na célula, com quebra de linha se preciso — não inflar a altura da linha). O chip em si é neutro (cinza do produto). A atenção vai para as marcas saturadas, no idioma da paleta do site (cinza + âmbar `--md-primary` / `--amber-9`). Ícones Lucide, não letras `F` / `D` / `(i)`: a forma carrega o significado quando a cor falha.

- `indication` — contorno tracejado, só a sigla;
- `informed_number` — marca `nº` em ardósia (não compete com inventário);
- `physical` — ícone de folha (`File`) verde saturado (original na organização);
- `digital` — ícone de leitura/scan (`ScanLine`) azul saturado (arquivo);
- ambos — os dois ícones no mesmo chip, cada um na sua cor;
- `physical_with_owner` — o mesmo ícone físico, mais o ícone de pessoa (`UserRound`) em disco **âmbar primário** (o amarelo de foco do produto), traço contrastante. Hover e `aria-label`: **«Com o dono»**. O âmbar chama atenção porque o original **não** está na gaveta. Não usar `Fd` nem chamar a pessoa de user.

O ícone de pessoa não aparece quando o original está na organização. Cor sozinha não basta: folha, scan, pessoa e `nº` permanecem como formas distintas. Ausência não mostra tag. Coluna de CPF/RG na grade de pessoas, se existir, mostra só o identificador; validade fica na ficha/tabela de documentos.

Transições (não inventar ausência):

- perder o último exemplar **mantém** o número (`informed_number`);
- apagar o número, se a pessoa ainda afirma ter, volta a `indication`;
- `absence` só quando alguém declara que não tem.

### 6.5 Estrutura persistida

Duas tabelas, não uma bagagem com `medium` nulo fazendo as vezes de indicação:

- `document_presences` — `(profile_id, document_type_id)` único; `claim` em `absence` | `indication` | `informed_number`; `identifier_value` só quando `informed_number`; dígitos canônicos em coluna gerada armazenada (`identifier_digits`) para busca/matching, indexada com `document_type_id`.
- `documents` — só exemplar; `presence_id`; `medium` obrigatório `PHYSICAL` | `DIGITAL`; unique `(presence_id, medium)`; data, validade, notas e anexos aqui. Físico tem `idle_custody` (`ORGANIZATION` | `OWNER`).
- `document_current_uses` permanece no exemplar físico (portador agora). Não confundir portador com dono nem com guarda.
- Lista paginada de pessoas: buscar a página de Profiles e, em seguida, presenças/exemplares de `WHERE profile_id = ANY($ids)` — sem N+1 e sem JSON como verdade.
- Contadores de inventário em mãos: exemplares físicos com `idle_custody = ORGANIZATION` ou com uso atual. Físico com o dono conta como cadastrado, não como item na gaveta.

Não usar `profiles.cpf`. Não materializar idade/soma. Não pré-criar presença para todos os tipos.

## 7. Contas e comprovantes

- Toda conta pertence a um Profile. Não existem contas órfãs.
- Ao cadastrar a conta, o operador informa o nome do dono. Correspondência exata do nome completo (trim, sem distinção de maiúsculas): zero pessoas → cria Profile com o nome e com os dados extras extraídos da conta, se houver (só nome → Profile só com nome); uma pessoa → vincula; várias → o operador escolhe, sem vínculo silencioso.
- A conta preserva os dados como aparecem no documento impresso/original (`printed_*`). Alterar dados impressos depois não altera implicitamente o Profile.
- Competência usa valor civil `YYYY-MM` quando aplicável.
- Valores monetários usam decimal, nunca ponto flutuante binário.
- Tipos de conta podem possuir campos canônicos e custom fields tipados.
- Uma linha de conta é um exemplar em poder da organização, com meio `físico` ou `digital`.
- Só o comprovante original físico tem uso atual e guarda (`idle_custody`), como em documentos.
- Exemplar digital não tem uso atual.
- O histórico completo de uso não faz parte do modelo inicial.

## 8. Uso atual de documentos e contas

- Uso atual existe somente no exemplar físico.
- O sistema mantém somente o uso atual no escopo inicial. Sem timeline histórica.
- Um original físico pode estar com no máximo um portador (`holder`) por vez.
- Dono (`presence` / Profile da linha), portador (uso atual) e guarda (`idle_custody`) são três papéis distintos.
- `idle_custody` é do exemplar físico: `ORGANIZATION` ou `OWNER`. Não se infere da validade; o operador informa. Vencido deixado na organização por opção do dono usa `ORGANIZATION`.
- Ao emprestar, grava-se o portador. O original continua no inventário enquanto o empréstimo dura.
- Ao encerrar o empréstimo, o destino padrão é a guarda. A linha do exemplar **permanece**:
  - `ORGANIZATION` — some o portador; o original fica **disponível** na organização (badge de folha).
  - `OWNER` — some o portador; `idle_custody` fica `OWNER`; o original continua cadastrado. Badge de folha com o ícone de pessoa «Com o dono». Número, validade e demais dados do exemplar não se apagam.
- Apagar o exemplar físico é outra operação (sumiu, rasgou, cadastro errado). Aí vale a transição de 6.4: some o físico, permanece o número informado.
- Na devolução o operador pode confirmar ou trocar o destino daquela vez, sem mudar a guarda gravada, salvo se editar a guarda explicitamente.
- Mudanças de uso e de guarda são transacionais, autorizadas e auditadas.

## 9. Custom data

### 9.1 Custom fields

Custom fields tipados podem ser definidos para:

- Profile;
- document type;
- bill type;
- custom entity type.

Regras:

- cada tipo de campo possui validação, normalização, persistência e renderização conhecidas;
- configuração não permite scripts arbitrários;
- configuração não permite fórmulas executáveis arbitrárias;
- JSON livre não é a fonte principal de verdade;
- alterações de definição devem preservar dados existentes ou exigir migração explícita;
- alterações administrativas são auditáveis.

### 9.2 Custom entities

- Custom entities pertencem ao Profile quando essa relação for configurada.
- Cardinalidades aprovadas: `ONE_PER_PROFILE` e `MANY_PER_PROFILE`.
- Custom entity não deve substituir módulos canônicos já definidos.
- Relações e campos devem permanecer disponíveis para Search e Query Engine conforme permissão.

## 10. Objetos

- Objetos formam um módulo separado futuro.
- Não devem ser escondidos dentro de Profile, Document, Bill ou Custom Data sem decisão explícita.
- Campos, propriedade, disponibilidade, uso e relações de objetos devem ser definidos antes da implementação do módulo.

## 11. Anexos e armazenamento

- Anexos são privados.
- O armazenamento aprovado é Cloudflare R2.
- Anexos podem pertencer a documento, conta ou valor customizado do tipo attachment.
- Upload usa URL assinada de curta duração e confirmação posterior no backend.
- O backend valida MIME declarado, assinatura real do arquivo, tamanho e hash.
- A confirmação só registra o anexo após verificar que o objeto esperado foi enviado.
- URLs assinadas, credenciais e chaves nunca aparecem em logs ou auditoria.
- Exclusão lógica envia o arquivo para lixeira por sete dias antes da remoção definitiva.
- Limpeza, retry e recovery devem ser idempotentes e observáveis.
- Anexos seguem as permissões da entidade proprietária.

## 12. Tabelas, formulários e experiência de CRUD

### 12.0 Rotas da SPA

Rotas aprovadas (inglês no path; UI em pt-BR):

- `/` — home;
- `/tables/people`, `/tables/documents`, `/tables/bills` — grades (pessoas, documentos, contas);
- `/tables` sem tipo redireciona para `/tables/people`. Tipo desconhecido não inventa uma quarta grade: redireciona para `/tables/people` ou responde ausência.
- `/search` — Search;
- `/admin` — Administração (users, chave de modelo, exportação XLSX, definições de campo extra, integrações; duplicatas só quando o matching for implementado);
- `/settings` — Preferências da conta;
- `/forms` — formulários. No início é a integração Google Forms neste path. Um módulo interno próprio, se existir, entra depois; Google passa a ser a segunda opção. Não existe `/google-forms`.

Estado transitório autorizado do rebuild: somente login, `/` e as três grades em `/tables/*` expõem conteúdo funcional. `/search`, `/admin`, `/settings` e `/forms` exibem apenas **Em desenvolvimento** até que suas experiências sejam redesenhadas e aprovadas pelo operador. O escopo funcional descrito nas seções próprias continua sendo o destino do produto, mas conteúdo provisório ou gerado automaticamente não deve ser exposto nessas rotas.

Cadastro de dados acontece na grade e no formulário da linha: criar/editar uma pessoa, documento ou conta, OCR de anexo (submódulo do cadastro, sem rota `/ocr`), e importação XLSX como cadastro em massa daquela tabela (como no legado). Não existe `/register`, `/ocr` nem `/operations` como destino.

Não existem como destino: `/chat`, `/query`, `/tasks`, `/ocr`, `/operations`, `/matching`, `/custom-data`, `/google-forms`, `/profiles`. Assistente, Query Engine e tarefas de gincana não têm rota. Exportação XLSX fica em `/admin`. Matching de duplicatas é submódulo de operações — último a implementar, complexo e caro — e não ganha rota agora. Campos extras não são tela: o valor aparece na grade; a definição fica em `/admin`.

### 12.1 Data Grid

- Tabelas suportam filtros por coluna, ordenação e paginação refletidos na URL.
- A vista atual é compartilhável por link (módulo, filtros, ordenação, paginação). Quem abre o link vê o mesmo recorte, sujeito à própria permissão. Isso não exige Assistente nem chave de modelo.
- Recorte produzido pelo Assistente entra no link como plano/recorte já compilado, não como linhas na URL e não como texto para a IA reinterpretar. Quem abre reexecuta o plano com a própria autorização de dados — mesmo sem permissão de Assistente e mesmo se a chave de modelo estiver ausente. Sem permissão sobre os dados, o recorte não aparece.
- O funil do cabeçalho é o filtro de coluna (§12.1.1). Coluna de nome sempre pinada fica para revisão posterior; não bloqueia o restante.
- Tamanho de página deve oferecer opções entre 100 e 1000 quando o módulo comportar esse volume.
- Desktop pode editar campos suportados no estilo planilha.
- Edição de célula salva no `blur`.
- Conflitos de `version` exibem feedback e recarregam/reconciliam a linha; nunca sobrescrevem silenciosamente.
- Não existe criação de nova linha diretamente no grid.
- Ação de duplicar pode existir no menu da linha quando suportada pelo módulo.
- Mobile não permite edição direta estilo planilha.
- Mobile mantém leitura e operações por formulários dedicados.
- Filtros/ordenação genéricos não devem ser duplicados de maneira incompatível entre módulos.
- Na grade de pessoas, documentos positivos aparecem como badges compactos do tipo (RG, CPF, …), segundo 6.4. Vários tipos na mesma célula; quebra de linha, sem crescer o chip. Chip cinza; folha (`File`) verde saturado, scan (`ScanLine`) azul saturado; pessoa (`UserRound`) no âmbar primário do produto quando o original está com o dono.

### 12.1.1 Funil do cabeçalho

Referência de comportamento: filtro de coluna de planilha (Google Sheets). Um menu no cabeçalho da coluna, com OK e Cancelar. Cópia da UI em pt-BR. Não é tela de QueryPlan nem SQL.

O menu junta, nesta ordem:

- ordenar A → Z;
- ordenar Z → A;
- filtrar por condição;
- filtrar por valores.

**Filtrar por condição** — um seletor; `Nenhum` desliga a condição. Operadores aprovados, agrupados como na planilha:

- vazio: está vazio, não está vazio;
- texto: contém, não contém, começa com, termina com, é exatamente;
- data: a data é, é anterior a, é posterior a;
- número: maior que, maior ou igual, menor que, menor ou igual, igual, diferente, está entre, não está entre.

Condição e valores combinam (as duas ativas restringem juntas). `Está entre` / `não está entre` pedem dois valores. O operador só aparece quando o tipo da coluna comporta (texto, data, número); vazio vale em qualquer coluna.

Fora do funil, mesmo que a planilha mostre:

- ordenar ou filtrar por cor — a grade não tem cor de célula como dado;
- dados validados / não validados — recurso da planilha, não do cadastro;
- fórmula personalizada — §9.1 e §14: sem fórmula executável do usuário no filtro. Coluna calculada do sistema é §12.4, outro caminho.

**Filtrar por valores** — lista com caixa de seleção de cada valor distinto da coluna no conjunto já recortado (servidor, não só a página visível):

- busca que estreita a lista;
- selecionar tudo e limpar;
- contagem do que a lista está exibindo;
- OK aplica; Cancelar descarta.

O funil é mecânico: funciona sem Assistente e sem chave de modelo.

### 12.2 Formulários e detalhe

- Criação e edição usam formulários explícitos.
- Máscaras e formatação pertencem à fronteira frontend; o backend recebe e persiste valores canônicos.
- Telas devem tratar loading, empty, validation error, permission error, conflict, failure e success.
- Exclusões definitivas e em massa exigem confirmação explícita.
- Para exclusão em massa, o texto de confirmação aprovado é `Confirmar`.
- A UI deve ser responsiva e acessível por teclado e leitores de tela.

### 12.3 Biblioteca visual

- Ant Design é a base visual. Material UI, shadcn e UI própria from-scratch não são.
- Componentes visuais permanecem neste repositório. Não há extração para um pacote UI compartilhado.
- Páginas e módulos compõem controles Ant Design (Table, Form, Select, Pagination, overlays). Ícones de chrome do produto usam Lucide (`lucide-react`).
- HTML cru permanece apropriado para estrutura semântica (`main`, `nav`) e decoração sem comportamento de componente.
- Composição, extração de hooks/componentes e tamanho de arquivo: §23.5. Não despejar a tela inteira num único `*Page.tsx`.

### 12.4 Colunas calculadas

- Idade, signo, soma de dígitos e equivalentes são colunas de apresentação da grade.
- O valor não é persistido. Alternar visibilidade ou o modo de cálculo (contagem de caracteres vs soma) não grava cadastro.
- A origem continua sendo o campo canônico (nascimento, número informado do tipo, nome, …).
- O usuário só vê o valor calculado da página carregada. A Table do Ant Design usa `virtual` na renderização da página atual; o cálculo cobre as linhas dessa página, não a base inteira nem um índice no banco.
- O Assistente pode calcular o mesmo conceito sobre o result set que já obteve (página, recorte da Query, etapa da tarefa). Não é necessário materializar idade/soma em SQL para 88 mil linhas só para a grade.

## 13. Search

No frontend atual, `/search` permanece como placeholder **Em desenvolvimento** até o redesenho da superfície. As regras abaixo continuam definindo a implementação futura da busca.

- Search deve consultar qualquer campo autorizado de qualquer módulo.
- Busca simples não depende de perguntas ou relatórios pré-configurados.
- Deve ser possível buscar nomes, partes de nomes, endereços, documentos, contas, custom fields e sequências arbitrárias.
- Busca pode combinar múltiplos parâmetros e relações.
- Permissões filtram módulos, campos, relações e resultados.
- A API não recebe nomes físicos livres de tabela/coluna.
- Resultados devem informar por que foram encontrados e em qual entidade/campo ocorreu a correspondência quando aplicável.
- Paginação, filtros e ordenação permanecem determinísticos.

## 14. Query Engine

### 14.1 Catálogo e planejamento

- O catálogo de entidades, campos, operadores e relações é gerado em runtime e filtrado por permission.
- Consultas são representadas por `QueryPlan` tipado e AST validada.
- O plano aprovado pode expressar projection, filters, relations, sorting, pagination, grouping, aggregation, sets, patterns e combinações.
- Essa superfície é um único motor versionado: v1 pode ser um subconjunto limitado; grouping, aggregation, sets, patterns e combinações permanecem aprovados e entram em versões posteriores do mesmo `QueryPlan`, não como API ou motor paralelo.
- A API trabalha com identificadores lógicos allowlisted, não com schema físico livre.
- O backend compila o plano validado para `ExecutionPlan` e SQL parametrizado dentro de fronteira confiável.
- Não existe tela para o usuário montar ou editar `QueryPlan`. O plano é tool do Assistente.

### 14.2 Execução e resultados

- Nenhum SQL fornecido pelo usuário ou modelo é executado diretamente.
- Consultas possuem limites de cardinalidade, custo, tempo, profundidade e paginação.
- Limites protegem o serviço sem transformar a ferramenta em catálogo de consultas fixas.
- Resultados podem ser materializados como result sets referenciáveis.
- Resultados preservam origem, entidades, campos e evidências necessárias para explicação.
- Operações próximas ao banco têm preferência para reduzir transferência de dados.

## 15. Duplicatas e merge

Matching de Profiles é o último fluxo a implementar: é complexo e computacionalmente caro. É submódulo de operações, sem rota própria (`/matching` fora do produto). Quando existir, a revisão humana entra na Administração ou na grade de pessoas — não como módulo paralelo no menu.

- O fluxo persistente inicial de duplicatas é exclusivo de Profiles.
- Análise ocorre sob demanda.
- Não existe inspeção periódica obrigatória de toda a base.
- Candidate generation e agregações devem ocorrer no PostgreSQL.
- Worker/Core podem calcular evidence vectors compactos quando necessário.
- Possíveis duplicatas exibem evidências de similaridade.
- Casos duvidosos exigem revisão humana.
- Não existe merge automático.
- Merge é transacional, autorizado e auditado.
- Regras de precedência de campos e movimentação de dependências devem ser explícitas antes do merge.
- O sistema não deve depender de palavras exatas digitadas pelo usuário para reconhecer a intenção de revisão.

## 16. Importação e exportação XLSX

### 16.1 Importação

- Importação XLSX é cadastro em massa da tabela aberta (pessoas, documentos ou contas), no mesmo fluxo de cadastro do legado — não é tela de Administração nem rota `/operations`.
- XLSX utiliza staging antes de alterar dados canônicos.
- O fluxo inclui upload temporário, seleção/mapeamento pré-preenchido pelo catálogo (§16.2), validação, decisões, preview, execução em batches e relatório.
- A execução é idempotente.
- Estados de negócio são simples; fases técnicas podem ser registradas separadamente em `stage`.
- Ambiguidades nunca são resolvidas silenciosamente.
- Decisões humanas são obrigatórias quando houver dúvida sobre criação, atualização, vínculo ou duplicidade.
- O fluxo inicial trabalha com uma única aba selecionada/controlada; múltiplas abas não viram múltiplos imports implícitos.
- O arquivo original não é guardado permanentemente após o período operacional necessário.
- Relatório informa inseridos, atualizados, ignorados, erros e decisões.
- Import não aplica merge automático de Profiles.

### 16.2 Mapeamento automático de colunas e valores

Importação em massa (XLSX e o mesmo pipeline de `/forms`) não pode exigir que o operador refaça, a cada arquivo, o mapeamento de cabeçalhos já conhecidos nem a interpretação de valores já catalogados. Isso vale tanto para a carga histórica do cadastro quanto para imports futuros de planilhas e formulários no mesmo molde.

O backend aplica um catálogo versionado, compartilhado por XLSX e Google Forms, com pelo menos:

- aliases de coluna: cabeçalho dobrado (sem acento, minúsculo) → campo canônico, tipo de documento, metadado de importação ou descarte;
- colunas sem título reconhecidas por posição/layout conhecido do mesmo molde (não por chute em arquivo inédito);
- valores fechados: `sim`/`não`, sócio clube, categoria de sócio, clube de supermercado, e equivalentes; grafias conhecidas colapsam no default; combinação vira vários valores quando o campo permitir;
- remap de documento pelo rótulo no valor (OAB/CREA/COREN/estudante numa coluna genérica; TRI/TEU como cartão de transporte; PIS distinto de CTPS);
- descarte explícito: idade, signo, somas de dígitos, “quem indicou”, horário de nascimento, peculiaridade operacional, link de anexo a tratar depois como documento digital, e colunas calculadas ou derivadas que a grade já produz.

Gymkhana-Core permanece dono da canonicalização reutilizável (fold, identificadores, validação de documento). Gymkhana-Database é dono do catálogo de produto (qual cabeçalho vira qual campo, qual grafia vira qual default, o que se descarta).

Mapeamento automático não é merge automático nem criação silenciosa de Profile. Cabeçalho ou valor fora do catálogo, checksum inválido, coluna tipada que não passa no validador daquele tipo, e dúvida de criar/atualizar/vincular continuam em staging com decisão humana. O importador não inventa alias, não promove número falho para outro tipo e não clona cidade de nascimento em naturalidade.

O operador ainda confirma o mapeamento proposto e o preview. O sistema pré-preenche o mapeamento a partir do catálogo; não começa em branco a cada arquivo do mesmo layout.

### 16.3 Exportação

- Exportação XLSX vive na Administração (`/admin`), não na grade de cadastro.
- Exporta a tabela escolhida: sem filtro ativo, todas as linhas desse módulo; com filtros ativos (inclusive recorte aplicado do Assistente), só o conjunto filtrado.
- Não limita o arquivo à página visível da grade virtualizada.
- Não cria dump cruzado de outros módulos nem subconjunto que não seja a tabela/filtros atuais.
- Formatação de apresentação não altera valores canônicos.
- Export respeita permissões e não inclui campos ocultos ao usuário.

## 17. Formulários

- A rota do produto é `/forms`. Não existe `/google-forms`.
- No frontend atual, `/forms` exibe apenas **Em desenvolvimento**; o conteúdo provisório anterior não faz parte da superfície aprovada.
- Quando a implementação da rota for retomada, `/forms` começa pela integração Google Forms: conexão, mapeamento, sync e cadastro via o mesmo pipeline de importação XLSX (staging, mapping automático de colunas e valores §16.2, validação, decisões, batches, idempotência, relatório).
- Cada admin gerencia apenas os próprios formulários/conexões, salvo permissão superior explícita.
- Um módulo interno de formulários, se for criado depois, usa a mesma rota. Google Forms permanece como segunda opção, não como tela paralela.
- Não deve existir pipeline paralelo incompatível com importação.
- IDs, tokens e credenciais do provider são armazenados com segurança.
- Payloads sensíveis do provider não aparecem em responses, logs ou auditoria.
- Integração não inclui envio de e-mails.

## 18. AI Assistente

Não existe página ou rota de Chat, Query ou Tarefas para o usuário montar plano, SQL ou prova. O produto é um Assistente integrado às tabelas, à Search e a um painel amplo para provas longas. Search, Query Engine e interpretação de tarefa são tools internas da IA. O motor HTTP do Assistente permanece um só (`/api/v1/chat`); a capability de uso é `CHAT`.

### 18.1 Papel e limites

- O Assistente é consultor privado e read-only.
- Não cria, altera, exclui, vincula, desvincula ou mescla dados.
- Usa tools tipadas sobre Search, Query Engine e a interpretação de tarefas de gincana (§20). Não existe módulo, menu ou rota de “Tarefas”, “Query” ou Chat para o usuário.
- Não recebe acesso irrestrito ao banco.
- Não executa SQL arbitrário. O modelo nunca monta SQL; o backend compila apenas `QueryPlan` validado ou Search tipado.
- Threads são privadas ao usuário conforme permissões administrativas aprovadas.
- Streaming pode usar SSE.
- Uso possui quotas e limites operacionais.
- A chave de modelo fica na Administração, compartilhada, criptografada em repouso. SUPERADMIN e ADMIN cadastram/rotacionam. EXTERNAL com `CHAT` usa o Assistente; sem `CHAT` não usa. Sem chave, ninguém pergunta à IA.
- Funil mecânico da tabela, colunas calculadas, Search e o link de recorte funcionam sem o Assistente e sem a chave.
- O desenvolvimento de Assistente, chave na Administração e das tools de Query/tarefa continua até cobrir §18.3 e §20.

### 18.2 Interpretação fundamentada

O Assistente interpreta qualquer dúvida ou solicitação autorizada, não só pedidos que recortam a tabela visível. Nem toda resposta aplica filtro na grade: pergunta pontual (“quantos?”, “o que significa?”, “este CPF serve?”) pode ser só explicação com evidência.

A geração é fundamentada no contexto recuperado e nas tools, no espírito de RAG — não um prompt genérico contra o banco:

- recupera do catálogo lógico (entidades, campos, operadores, relações) só o que for pertinente à pergunta, já filtrado por permissão;
- recupera result sets e evidências anteriores da conversa (“desses”) como memória de trabalho;
- pode recuperar glossário/regras de produto quando a dúvida for sobre o sistema, não sobre uma linha;
- consulta dados canônicos só via Search e Query Engine; não indexa o cadastro inteiro como chunks no lugar desses motores;
- toda afirmação sobre dados cita evidência das tools; sem evidência, declara que não sabe.

Padrões, conjuntos, dígitos binários e combinações são planos do Query Engine, não similaridade vetorial sobre texto de Profile.

Não deve existir um planner limitado a tipos fixos de pergunta ou `field hints` rígidos.

### 18.3 Cobertura funcional

O Assistente deve interpretar perguntas sobre qualquer campo ou relação autorizada, incluindo:

- pessoas cujos nomes contenham determinado texto;
- sequências de qualquer tipo de caractere em qualquer documento ou campo;
- documentos com padrões de letras, números, dígitos binários ou combinações;
- pessoas com determinado nome que morem em endereços com textos específicos;
- combinação de pessoa, endereço, documento, conta e custom data;
- agregações, agrupamentos, contagens, interseções e diferenças;
- tarefas de gincana: uma lista de requisitos, possivelmente em várias etapas, interpretada pela IA, com resultados da base para cada etapa.

### 18.4 Conversação, modos de resposta e resultados

- Escopo de cada pedido é explícito: esta tabela, o result set ativo, ou a base autorizada.
- Modos de resposta: só responder; aplicar recorte na tabela atual quando o resultado couber nela; devolver etapas quando for tarefa de gincana.
- Aplicar na tabela é ação explícita, não efeito colateral de qualquer pergunta.
- Uma tarefa de gincana colada no Assistente é uma lista de requisitos; a IA devolve resultados da base por etapa.
- Follow-ups como “desses”, “agora somente...” e “combine com...” refinam o contexto/result set ativo.
- Respostas podem combinar explicação e tabela genérica.
- Interpretação ambígua é apresentada antes de aplicar um recorte na grade.
- Cada resultado deve preservar referências e explicação suficiente.
- O modelo pode corrigir/refinar o plano quando uma tentativa falhar dentro dos limites seguros.
- Saved queries são executadas exclusivamente quando o usuário clicar nelas.
- Uma saved query nunca intercepta ou substitui automaticamente uma pergunta digitada manualmente.
- O sistema não cria saved queries automáticas.
- A caixa de busca/filtro da tabela não dispara o modelo sozinha.

## 19. OCR e sugestões multimodais

- OCR é submódulo do cadastro na grade e no formulário da linha. Não tem rota `/ocr`.
- OCR/multimodal produz sugestões, não alterações.
- Toda sugestão deve apresentar evidência e confiança quando disponível.
- Revisão humana é obrigatória antes de aplicar qualquer valor.
- Nenhuma informação extraída atualiza automaticamente Profile, documento, conta ou custom data.
- O sistema pode preservar região/origem da evidência e comparação entre valor sugerido e aceito.
- OCR respeita permissões, quotas, privacidade e retenção do anexo original.
- Extração com modelo (Gemini ou equivalente) só entra quando for o meio adequado; o fluxo de sugestão/revisão/aplicação permanece o contrato.
- Um toggle/feature flag pode exigir Gemini only. Nesse modo a extração só roda se a chave de modelo compartilhada estiver cadastrada na Administração; sem chave o OCR por modelo permanece desligado para todos.

## 20. Tarefas complexas de gincana

- Tarefa de gincana não é um módulo da aplicação e não tem workspace, menu ou rota para o usuário.
- É uma capacidade da IA: o Assistente deve esperar e interpretar listas de requisitos, inclusive uma prova completa copiada e colada.
- A IA interpreta a tarefa semanticamente; o usuário não precisa usar palavras específicas.
- A tarefa pode ter várias etapas. Cada etapa é um requisito (ou um conjunto de requisitos) e deve produzir o seu próprio resultado a partir da base autorizada.
- A IA usa apenas tools tipadas de Search e Query Engine. Não recebe SQL, schema físico nem permissão de mutação.
- Uma tarefa pode exigir simultaneamente pessoas, documentos, contas e custom data, inclusive restrições cruzadas entre entidades.
- Deve suportar sequências, padrões, caracteres permitidos, dígitos binários, letras, números e zeros à esquerda.
- Resultados são explicáveis: cada etapa indica quais requisitos atendeu, quais evidências usou e onde falhou.
- O resultado não pode ser apenas texto opaco sem rastreabilidade até os dados.

## 21. Normalização, validação e concorrência

- Whitespace, Unicode, formas de busca e valores canônicos usam contratos compartilhados do Gymkhana-Core quando reutilizáveis.
- E-mail é lowercase.
- CPF, CEP e identificadores numéricos preservam zeros à esquerda.
- Telefones são normalizados no backend e formatados na UI.
- Dinheiro usa decimal.
- Datas civis não são armazenadas como instantes quando não representam horário.
- Timestamps técnicos usam UTC.
- Erros de validação são estáveis e associados a campos.
- Entidades mutáveis usam `version` para concorrência otimista quando aplicável.
- Conflitos retornam resposta explícita e nunca fazem last-write-wins silencioso.

## 22. API, segurança e privacidade

### 22.1 Contrato HTTP

- Base versionada: `/api/v1`.
- Health endpoints: `/health/live` e `/health/ready`.
- Métodos HTTP são semânticos.
- GET não possui side effects.
- Listas são paginadas.
- Projection é allowlisted.
- Operações críticas usam `Idempotency-Key` quando necessário.
- Status de sucesso podem usar 200, 201, 202 e 204.
- Erros podem usar 400, 401, 403, 404, 409, 412, 422, 429, 500 e 503 conforme o caso.
- Error envelope contém `code`, mensagem em `pt-BR`, `request_id`, `field_errors` e `details` controlados.
- Contratos OpenAPI Go/TypeScript são gerados e não editados manualmente.

### 22.2 Segurança

- State-changing browser requests validam a origem exata aprovada.
- CORS credentialed é restrito à origem configurada da aplicação.
- Logs são estruturados e redigidos.
- Responses não expõem secret, token, signed URL, SQL, stack trace ou provider payload.
- Rate limits e quotas são aplicados onde Search, AI, OCR, upload e import puderem gerar abuso/custo.
- Usuários conhecidos não eliminam a necessidade de integridade, autorização, auditoria e proteção de secrets.

### 22.3 Exclusão e operações destrutivas

- Exclusão definitiva exige confirmação explícita.
- Exclusão em massa exige confirmação reforçada.
- Operações destrutivas são autorizadas e auditadas.
- Quando existir lixeira/retenção, a regra específica do módulo prevalece; anexos usam sete dias.

## 23. Arquitetura técnica permanente

Esta seção resume as invariantes. A seção “Arquitetura escolhida e racional” registra a stack detalhada e os motivos.

### 23.1 Aplicação

- Monólito modular.
- Backend em Go.
- Frontend em React e TypeScript.
- REST JSON e OpenAPI 3.1.
- `net/http`, pgx e sqlc.
- SQL explícito e parametrizado.
- Sem ORM.
- Migrations forward-first com Tern.
- SPA com Vite.
- TanStack Router, Query e Form conforme a necessidade real.
- Ant Design Table com `virtual` nas grades do produto; TanStack Table só onde a Table do Ant Design ainda não é o sheet.
- Valibot na validação frontend quando aplicável.
- Nenhum `fetch` direto em componente visual.

### 23.2 Estado frontend

- Router controla estado de URL.
- Query controla server state.
- Form controla estado de formulários.
- React controla estado visual local.
- Backend controla preferências persistentes.
- Não usar Redux ou Zustand sem nova decisão explícita.

### 23.3 Backend e worker

- Domínio é desacoplado de banco, HTTP, providers, filas e UI.
- `cmd/api`, `cmd/worker` e `cmd/migrate` contêm apenas bootstrap, configuração, lifecycle e composição.
- Features usam packages próprios quando isso representa o domínio; não existe obrigação de camada genérica controller/service/repository.
- River entra somente quando houver jobs reais.
- Não criar fila fictícia permanente.
- Sem Redis, RabbitMQ ou microserviços inicialmente.
- Sem GraphQL, SSR ou backend Node.
- Sem agent framework como dependência arquitetural obrigatória.
- OpenTelemetry não faz parte da arquitetura padrão aprovada.

### 23.4 Geração e banco

- sqlc e contratos OpenAPI gerados são versionados.
- Generated files nunca são editados manualmente.
- CI regenera e falha em caso de drift.
- Queries dinâmicas existem apenas nas fronteiras aprovadas de Search/Query Engine.
- Migrations são testadas em banco vazio e em upgrade.
- Seeds são separados de migrations.

### 23.5 Composição frontend (hooks, componentes, tamanho)

- O que pode virar hook vira hook. Função pura não se disfarça de hook.
- Markup nomeável vira componente. Helpers, painéis e o equivalente no backend seguem a mesma localidade: extrair quando o recorte tem interface própria.
- Reutilizável (dois ou mais call sites, ou o shell) vai para pasta global:
  - hooks: `apps/web/src/hooks/`
  - componentes: `apps/web/src/components/`
- Específico de um módulo fica no módulo dono, por exemplo `apps/web/src/lib/tables/` ou `apps/web/src/lib/home/`. Não promover para global sem o segundo uso.
- Página e rota só orquestram: URL, dados e composição. Não acumulam colunas, inspector, filtros e markup no mesmo arquivo.
- Arquivo de implementação com 1000+ linhas não é aceitável. Ao editar um arquivo já grande, extrair na mesma mudança até ficar revisável. Não fatiar em dezenas de arquivos rasos sem interface própria.
- Teste acompanha o módulo extraído, não a página-orquestradora, quando o comportamento saiu dela.
- Continua sem pacote UI compartilhado (§12.3) e sem camada genérica obrigatória no backend (§23.3).

## 24. Infraestrutura e ambientes

- PostgreSQL de produção utiliza Neon, major 18. Não há upgrade in-place de major no Neon: projeto novo e migração de dados.
- O desenho deve considerar o limite de egress do plano utilizado.
- SPA utiliza Vercel.
- API utiliza Cloud Run Service quando o deploy de produção for ativado.
- Worker utiliza Cloud Run Job quando jobs reais forem ativados.
- Cloud Scheduler pode executar recovery e housekeeping.
- Anexos utilizam Cloudflare R2.
- Railway não faz parte da infraestrutura aprovada.
- Local usa Docker Compose apenas para infraestrutura realmente necessária, inicialmente PostgreSQL.
- MinIO, Redis e serviços sem uso real não são adicionados por conveniência.
- Migrations não rodam automaticamente dentro do container do PostgreSQL.
- Local, staging e production são separados.
- Staging usa somente dados sintéticos.
- Dados de produção não são copiados para staging.
- Secrets ficam no secret manager/configuração da plataforma e nunca no repositório.
- Localmente, secrets entram no processo via lokeys (`lokeys run -p gymkhana --env dev`) e não via arquivo `.env`.
- Testes básicos não exigem credenciais externas reais.

## 25. Limites entre repositórios

### Gymkhana-Database

Responsável por:

- produto e regras de aplicação, inclusive o catálogo versionado de aliases de coluna e de valores de importação;
- persistência e migrations;
- API e autorização;
- workers e jobs;
- providers e integrações;
- rotas e composição da aplicação;
- módulos de negócio.

### Gymkhana-Core

Responsável por lógica Go reutilizável e independente de infraestrutura, como normalização, datas civis, canonicalização, fingerprints e algoritmos puros comprovadamente compartilhados.

### Gymkhana-Database-Vercel

Aplicação legado Next.js ainda em produção. Não é dependência do rebuild. Pode ser executada em paralelo no local apenas para comparação; lê `.env` / `.env.local` (Prisma `dotenv/config` e Next), usa Neon Dev e não o PostgreSQL Docker do Database. Não usar `lokeys run` nesse processo.

### Regras de dependência

- Database pode depender de Core.
- Core não depende do Database.
- Database consome versões exatas publicadas.
- Não existem dependências permanentes por branch, commit, `replace`, subtree, submodule ou cópia manual.
- Extração para Core ocorre somente após contrato ou reutilização comprovada.

## 26. Decisões deliberadamente fora do escopo

Sem nova decisão explícita, não adicionar:

- multi-organização ou tenancy;
- pessoas jurídicas no modelo canônico de Profile;
- senha local;
- edição de dados pela IA;
- SQL arbitrário;
- prompt genérico que gera SQL ou varre o banco sem Search/QueryPlan;
- página ou rota de Chat, Query, Tarefas, OCR, Operations, Matching, custom-data, google-forms ou profiles como destino;
- `/register` paralelo à grade para cadastro de dados;
- chave de modelo por usuário (a chave é compartilhada na Administração);
- RAG que substitui Search/Query por índice vetorial do cadastro inteiro;
- merge automático de duplicatas;
- aplicação automática de OCR;
- histórico completo de uso de documentos/contas;
- código de barras obrigatório;
- envio de e-mail;
- módulo de notícias;
- suporte offline-first;
- microserviços;
- ORM;
- GraphQL;
- SSR;
- Redis ou RabbitMQ;
- OpenTelemetry como padrão;
- Railway;
- Material UI, shadcn ou UI própria from-scratch como base visual;
- scripts ou fórmulas arbitrárias em custom fields;
- persistir colunas calculadas (idade, signo, soma de dígitos) como dado canônico;
- JSON livre como modelo principal de dados;
- saved queries executadas automaticamente a partir de texto manual;
- consultas limitadas a templates predefinidos;
- equipe como entidade própria ou como campo do user da aplicação;
- misturar equipe de gincana (`team`) com time de futebol / sócio clube (`club_membership`).

## 27. Governança deste documento

- Este arquivo contém regras permanentes de produto, domínio, segurança, experiência, stack, racional arquitetural e limites técnicos.
- Não registrar aqui progresso, checklist, próxima ação, branch, PR, release ou versão instalada no momento.
- A baseline e os motivos das tecnologias escolhidas devem permanecer registrados, mesmo após upgrades de versão.
- Não atualizar este arquivo ao concluir uma tarefa.
- Atualizar este arquivo somente quando o usuário aprovar mudança real de regra, stack ou arquitetura.
- A issue [#31](https://github.com/Pherlsz/Gymkhana-Database/issues/31) é o único checklist vivo e a única fonte de progresso/retomada.
- Issues específicas detalham execução; não redefinem silenciosamente as regras permanentes.
- Em caso de conflito, uma decisão mais recente e explicitamente aprovada pelo usuário deve primeiro atualizar este arquivo e depois refletir-se na checklist.
- Não criar novos documentos de planejamento ou tracking; usar a issue mestre e as issues executáveis.

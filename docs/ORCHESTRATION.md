# Gymkhana Database — Regras de Produto, Domínio e Arquitetura

> **Escopo deste documento:** decisões permanentes aprovadas para o rebuild do Gymkhana Database.  
> **Inclui:** regras de negócio, domínio, arquitetura, stack escolhida, responsabilidades técnicas, alternativas rejeitadas e os motivos das decisões.  
> **Não inclui:** progresso de desenvolvimento, milestone atual, próxima ação, branches, PRs, versões publicadas, releases ou checklists de execução.  
> **Acompanhamento operacional único:** [issue mestre #31](https://github.com/Pherlsz/Gymkhana-Database/issues/31).  
> **Repositórios relacionados:** `Pherlsz/Gymkhana-UI` e `Pherlsz/Gymkhana-Core`.

Este arquivo é a fonte permanente das regras de produto e das invariantes arquiteturais. Ele só deve mudar quando uma decisão aprovada de negócio, domínio, segurança, experiência, stack ou arquitetura for alterada.

Nenhum documento adicional deve ser criado para acompanhar andamento, aceite de milestone ou próxima ação. Documentação técnica específica pode existir quando necessária para operar uma funcionalidade real, mas não substitui a issue mestre como tracker e não deve duplicar o estado do projeto.

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

- `React SPA`: interface privada, rotas, formulários, grids, Search, AI Chat e administração;
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
- PostgreSQL;
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
- TanStack Table;
- TanStack Virtual;
- TanStack Form;
- Valibot;
- openapi-typescript e openapi-fetch;
- Gymkhana-UI como pacote privado;
- Lucide para ícones.

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
- TanStack Table/Virtual: grids e grandes listas;
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

### Por que uma UI própria e Gymkhana-UI

Ant Design, Material UI e shadcn não são a base visual do produto. A UI própria foi escolhida porque:

- a identidade visual deve permanecer controlada pelo projeto;
- os componentes precisam funcionar em grids densos, formulários e ferramentas internas específicas;
- reduz dependência de APIs e estilos de terceiros;
- permite reutilizar primitives nos demais projetos Gymkhana;
- evita carregar dezenas de componentes ou padrões que não serão usados;
- mantém temas, densidade e acessibilidade sob controle do projeto.

Essas bibliotecas podem ser usadas como referência de comportamento, não como dependência principal. Lucide foi escolhido por oferecer ícones consistentes, amplos e reutilizáveis sem acoplar o design a uma biblioteca de componentes.

Gymkhana-UI recebe apenas componentes com contrato ou reutilização comprovada. Componentes específicos permanecem no Database até justificar extração.

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

- GitHub OAuth;
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

### Por que não usar agent framework como base do AI Chat

AI Chat e tarefas complexas usam ferramentas tipadas e Query Engine próprio porque:

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
- Ant Design, Material UI ou shadcn como base da UI;
- Railway;
- OpenTelemetry por padrão;
- agent framework obrigatório;
- MinIO local sem uso real;
- qualquer serviço criado apenas para representar arquitetura futura.

## 1. Objetivo do produto

Reconstruir o Gymkhana Database como uma aplicação privada, leve, extensível e segura para centralizar dados usados em gincanas e permitir consultas simples, relacionais e combinatórias sobre qualquer informação autorizada.

O produto deve armazenar e relacionar:

- pessoas físicas;
- documentos;
- contas e comprovantes;
- uso atual de documentos e contas;
- equipes;
- campos e entidades customizadas;
- anexos;
- importações XLSX;
- integrações com Google Forms;
- resultados de busca e consultas estruturadas;
- candidatos a duplicidade e decisões humanas de merge;
- conversas e resultados do AI Chat;
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
- Não há suporte offline-first.
- O produto não envia e-mails como funcionalidade inicial.
- O produto não possui módulo de notícias.

## 3. Usuários, autenticação e autorização

### 3.1 Autenticação

- O login da aplicação usa Google OAuth com allowlist de e-mails explícita.
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
- ADMIN e SUPERADMIN bypassam verificações de capability e têm acesso total.
- Capabilities são gerenciadas via endpoints admin e armazenadas em `app_user_capabilities`.
- Capabilities disponíveis: `search`, `profiles`, `data_tables`, `attachments`, `ocr`, `operations`, `google_forms`, `query`, `matching`, `chat`, `tasks`.
- Middleware HTTP verifica capabilities em todas as rotas protegidas.
- Grants e revogações são auditados.

### 3.4 Auditoria

- Auditoria é simples, essencial e separada dos logs técnicos.
- Devem ser auditados, quando aplicável: login, logout, falhas de acesso, administração de usuários, mudança de role/estado, revogação de sessão, criação, edição, duplicação, exclusão, importação, merge e operações destrutivas.
- Eventos são correlacionados com `request_id` e ator quando disponível.
- Falhas de persistência da auditoria devem ficar observáveis.
- Auditoria não armazena secrets, tokens, URLs assinadas, SQL, stack traces ou payloads sensíveis de providers.

## 4. Modelo central: Profile

### 4.1 Identidade e estrutura

- Profile representa uma pessoa física.
- A chave é UUID; UUIDv7 é a preferência aprovada para novas entidades quando o suporte utilizado permitir.
- Profile possui timestamps e `version` para concorrência otimista.
- Não existe payload JSON genérico como modelo principal de Profile.
- Exclusão de Profile é definitiva, com confirmação explícita e auditoria.

### 4.2 Campos canônicos

- Nome completo é obrigatório.
- Nome social é opcional.
- CPF é opcional.
- CPF é normalizado para 11 dígitos quando informado.
- CPF não possui unicidade rígida global, pois possíveis duplicatas devem ser revisadas pelo fluxo de matching.
- Existe no máximo um e-mail por Profile.
- E-mail é normalizado para minúsculas.
- Existe no máximo um celular por Profile.
- Existe no máximo um telefone fixo/outro por Profile.
- Telefones são persistidos em formato normalizado; máscaras pertencem à borda da UI.
- Existe no máximo um endereço estruturado por Profile.
- O endereço pode conter logradouro, número, complemento, bairro, cidade, UF e CEP.
- Observações pessoais são opcionais.
- Equipe atual pode ser associada ao Profile quando o módulo correspondente estiver disponível.

### 4.3 Comportamento

- Valores são normalizados e validados na fronteira do domínio.
- Criação, leitura, edição, duplicação e exclusão são operações explícitas.
- Duplicar cria um novo Profile independente e deve abri-lo para revisão.
- Duplicação não significa merge e não herda IDs de entidades dependentes automaticamente.
- Atualizações usam `version`; conflito concorrente não pode sobrescrever silenciosamente outra alteração.
- Documents, bills, attachments e custom data aparecem como seções relacionadas, não como campos livres embutidos no Profile.

## 5. Equipes

- A associação principal é a equipe atual do Profile.
- Equipes de futebol devem suportar inicialmente `Internacional`, `Grêmio` e `Outros`.
- A opção `Outros` não pode impedir cadastro posterior de equipes adicionais.
- Equipes de gincana usam lista configurável futura.
- O domínio não deve ficar preso a uma enumeração irreversível.
- Histórico completo de equipes não entra automaticamente; precisa de decisão específica antes de ser criado.

## 6. Documentos

### 6.1 Propriedade e modelo

- Todo documento pertence a um Profile.
- Não existem documentos órfãos.
- O modelo é híbrido: tabela comum, detalhes específicos por família/tipo e custom fields tipados.
- Tipos de documento são administráveis.
- Cada tipo possui chave técnica estável que não muda com o rótulo exibido.
- Não deve existir JSON livre como fonte principal dos dados do documento.

### 6.2 Unicidade e validação

Cada tipo define uma política de unicidade:

- `NONE` — nenhuma restrição de unicidade;
- `PER_PROFILE` — único dentro do Profile;
- `GLOBAL_BY_TYPE` — único globalmente para aquele tipo.

Além disso:

- tipos podem definir regex e validações controladas;
- validações devem produzir erros por campo, não mensagens opacas;
- valores devem preservar zeros à esquerda quando semanticamente relevantes;
- sequências alfanuméricas devem ser armazenadas sem perda de informação;
- não há leitura ou armazenamento obrigatório de código de barras no escopo aprovado.

### 6.3 Ciclo de vida

- Documentos antigos, substituídos ou vencidos podem ser preservados.
- Documento pode possuir observações e data associada quando o tipo exigir.
- O status funcional inicial é `em uso` ou `disponível`.
- O histórico completo de uso não faz parte do modelo inicial.

## 7. Contas e comprovantes

- Toda conta pertence a um Profile.
- Não existem contas órfãs.
- A conta preserva os dados como aparecem no documento impresso/original.
- Alterar dados impressos da conta não altera implicitamente o Profile.
- Competência usa valor civil `YYYY-MM` quando aplicável.
- Valores monetários usam decimal, nunca ponto flutuante binário.
- Tipos de conta podem possuir campos canônicos e custom fields tipados.
- O status funcional inicial é `em uso` ou `disponível` quando a conta puder ser emprestada/utilizada.
- O histórico completo de uso não faz parte do modelo inicial.

## 8. Uso atual de documentos e contas

- O sistema mantém somente o uso atual no escopo inicial.
- Um documento ou conta pode estar vinculado a no máximo uma pessoa/uso atual.
- Ao devolver ou desvincular um item, o vínculo atual anterior é removido.
- Após a devolução, o item volta a ficar disponível.
- Não deve ser criada timeline histórica fictícia ou incompleta sem decisão posterior.
- Mudanças de uso são transacionais, autorizadas e auditadas.

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

### 12.1 Data Grid

- Tabelas suportam filtros por coluna, ordenação e paginação refletidos na URL.
- Tamanho de página deve oferecer opções entre 100 e 1000 quando o módulo comportar esse volume.
- Desktop pode editar campos suportados no estilo planilha.
- Edição de célula salva no `blur`.
- Conflitos de `version` exibem feedback e recarregam/reconciliam a linha; nunca sobrescrevem silenciosamente.
- Não existe criação de nova linha diretamente no grid.
- Ação de duplicar pode existir no menu da linha quando suportada pelo módulo.
- Mobile não permite edição direta estilo planilha.
- Mobile mantém leitura e operações por formulários dedicados.
- Filtros/ordenação genéricos não devem ser duplicados de maneira incompatível entre módulos.

### 12.2 Formulários e detalhe

- Criação e edição usam formulários explícitos.
- Máscaras e formatação pertencem à fronteira frontend; o backend recebe e persiste valores canônicos.
- Telas devem tratar loading, empty, validation error, permission error, conflict, failure e success.
- Exclusões definitivas e em massa exigem confirmação explícita.
- Para exclusão em massa, o texto de confirmação aprovado é `Confirmar`.
- A UI deve ser responsiva e acessível por teclado e leitores de tela.

### 12.3 Biblioteca visual

- Componentes reutilizáveis pertencem ao Gymkhana-UI somente após reutilização ou contrato comprovado.
- O produto não adota Ant Design, Material UI ou shadcn como dependência visual principal.
- Essas bibliotecas podem servir apenas como referência de comportamento/design.
- Ícones usam Lucide.
- Componentes de página devem compor as primitives próprias, incluindo `Page`, `AppShell`, feedback e overlays.

## 13. Search

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
- O plano pode expressar projection, filters, relations, sorting, pagination, grouping, aggregation, sets, patterns e combinações.
- A API trabalha com identificadores lógicos allowlisted, não com schema físico livre.
- O backend compila o plano validado para `ExecutionPlan` e SQL parametrizado dentro de fronteira confiável.

### 14.2 Execução e resultados

- Nenhum SQL fornecido pelo usuário ou modelo é executado diretamente.
- Consultas possuem limites de cardinalidade, custo, tempo, profundidade e paginação.
- Limites protegem o serviço sem transformar a ferramenta em catálogo de consultas fixas.
- Resultados podem ser materializados como result sets referenciáveis.
- Resultados preservam origem, entidades, campos e evidências necessárias para explicação.
- Operações próximas ao banco têm preferência para reduzir transferência de dados.

## 15. Duplicatas e merge

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

- XLSX utiliza staging antes de alterar dados canônicos.
- O fluxo inclui upload temporário, seleção/mapeamento, validação, decisões, preview, execução em batches e relatório.
- A execução é idempotente.
- Estados de negócio são simples; fases técnicas podem ser registradas separadamente em `stage`.
- Ambiguidades nunca são resolvidas silenciosamente.
- Decisões humanas são obrigatórias quando houver dúvida sobre criação, atualização, vínculo ou duplicidade.
- O fluxo inicial trabalha com uma única aba selecionada/controlada; múltiplas abas não viram múltiplos imports implícitos.
- O arquivo original não é guardado permanentemente após o período operacional necessário.
- Relatório informa inseridos, atualizados, ignorados, erros e decisões.
- Import não aplica merge automático de Profiles.

### 16.2 Exportação

- Exportação XLSX inicial exporta a tabela inteira do módulo solicitado.
- Não cria exportações arbitrárias de subconjuntos sem regra explícita.
- Formatação de apresentação não altera valores canônicos.
- Export respeita permissões e não inclui campos ocultos ao usuário.

## 17. Google Forms

- Google Forms reutiliza staging, mapping, validação, decisões, batches, idempotência e relatórios do XLSX.
- Não deve existir pipeline paralelo incompatível com importação.
- Cada admin gerencia apenas seus próprios formulários/conexões, salvo permissão superior explícita.
- IDs, tokens e credenciais do provider são armazenados com segurança.
- Payloads sensíveis do provider não aparecem em responses, logs ou auditoria.
- Integração não inclui envio de e-mails.

## 18. AI Chat

### 18.1 Papel e limites

- AI Chat é consultor privado e read-only.
- Não cria, altera, exclui, vincula, desvincula ou mescla dados.
- Usa tools tipadas sobre Search/Query Engine.
- Não recebe acesso irrestrito ao banco.
- Não executa SQL arbitrário.
- Threads são privadas ao usuário conforme permissões administrativas aprovadas.
- Streaming pode usar SSE.
- Uso possui quotas e limites operacionais.

### 18.2 Cobertura funcional

AI Chat deve interpretar perguntas sobre qualquer campo ou relação autorizada, incluindo:

- pessoas cujos nomes contenham determinado texto;
- sequências de qualquer tipo de caractere em qualquer documento ou campo;
- documentos com padrões de letras, números, dígitos binários ou combinações;
- pessoas com determinado nome que morem em endereços com textos específicos;
- combinação de pessoa, endereço, documento, conta e custom data;
- agregações, agrupamentos, contagens, interseções e diferenças;
- resultados necessários para tarefas completas de gincana.

Não deve existir um planner limitado a tipos fixos de pergunta ou `field hints` rígidos.

### 18.3 Conversação e resultados

- Follow-ups como “desses”, “agora somente...” e “combine com...” refinam o contexto/result set ativo.
- Respostas podem combinar explicação e tabela genérica.
- Cada resultado deve preservar referências e explicação suficiente.
- O modelo pode corrigir/refinar o plano quando uma tentativa falhar dentro dos limites seguros.
- Saved queries são executadas exclusivamente quando o usuário clicar nelas.
- Uma saved query nunca intercepta ou substitui automaticamente uma pergunta digitada manualmente.
- O sistema não cria saved queries automáticas.

## 19. OCR e sugestões multimodais

- OCR/multimodal produz sugestões, não alterações.
- Toda sugestão deve apresentar evidência e confiança quando disponível.
- Revisão humana é obrigatória antes de aplicar qualquer valor.
- Nenhuma informação extraída atualiza automaticamente Profile, documento, conta ou custom data.
- O sistema pode preservar região/origem da evidência e comparação entre valor sugerido e aceito.
- OCR respeita permissões, quotas, privacidade e retenção do anexo original.

## 20. Tarefas complexas de gincana

- O sistema deve aceitar texto livre, inclusive uma tarefa completa copiada e colada.
- A interpretação é semântica e não exige que o usuário use palavras específicas.
- Uma tarefa pode exigir simultaneamente pessoas, documentos, contas e custom data.
- Requisitos são convertidos em bindings explícitos para campos, relações e operadores.
- O sistema constrói candidate sets e combina candidatos com solver/pruning.
- Deve suportar sequências, padrões, caracteres permitidos, dígitos binários, letras, números e zeros à esquerda.
- Deve suportar restrições cruzadas entre entidades.
- Resultados são explicáveis: cada candidato/composição indica quais requisitos atende, quais evidências foram usadas e onde falhou.
- O resultado não pode ser apenas texto opaco sem rastreabilidade até os dados.
- Nenhuma resposta exige match exato de palavras do enunciado quando a intenção puder ser interpretada semanticamente.

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
- TanStack Router, Query, Table, Virtual e Form conforme a necessidade real.
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

## 24. Infraestrutura e ambientes

- PostgreSQL de produção utiliza Neon.
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
- Testes básicos não exigem credenciais externas reais.

## 25. Limites entre repositórios

### Gymkhana-Database

Responsável por:

- produto e regras de aplicação;
- persistência e migrations;
- API e autorização;
- workers e jobs;
- providers e integrações;
- rotas e composição da aplicação;
- módulos de negócio.

### Gymkhana-UI

Responsável por componentes e primitives visuais comprovadamente reutilizáveis.

### Gymkhana-Core

Responsável por lógica Go reutilizável e independente de infraestrutura, como normalização, datas civis, canonicalização, fingerprints e algoritmos puros comprovadamente compartilhados.

### Regras de dependência

- Database pode depender de UI e Core.
- UI e Core não dependem do Database.
- UI e Core não dependem um do outro.
- Database consome versões exatas publicadas.
- Não existem dependências permanentes por branch, commit, `replace`, subtree, submodule ou cópia manual.
- Extração para UI/Core ocorre somente após contrato ou reutilização comprovada.

## 26. Decisões deliberadamente fora do escopo

Sem nova decisão explícita, não adicionar:

- multi-organização ou tenancy;
- pessoas jurídicas no modelo canônico de Profile;
- senha local;
- edição de dados pela IA;
- SQL arbitrário;
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
- bibliotecas visuais externas como base principal da UI;
- scripts ou fórmulas arbitrárias em custom fields;
- JSON livre como modelo principal de dados;
- saved queries executadas automaticamente a partir de texto manual;
- consultas limitadas a templates predefinidos.

## 27. Governança deste documento

- Este arquivo contém regras permanentes de produto, domínio, segurança, experiência, stack, racional arquitetural e limites técnicos.
- Não registrar aqui milestone atual, progresso, checklist, próxima ação, branch, PR, release ou versão instalada no momento.
- A baseline e os motivos das tecnologias escolhidas devem permanecer registrados, mesmo após upgrades de versão.
- Não atualizar este arquivo ao concluir uma tarefa ou milestone.
- Atualizar este arquivo somente quando o usuário aprovar mudança real de regra, stack ou arquitetura.
- A issue [#31](https://github.com/Pherlsz/Gymkhana-Database/issues/31) é o único checklist vivo e a única fonte de progresso/retomada.
- Issues específicas detalham execução; não redefinem silenciosamente as regras permanentes.
- Em caso de conflito, uma decisão mais recente e explicitamente aprovada pelo usuário deve primeiro atualizar este arquivo e depois refletir-se na checklist.
- Não criar novos documentos de planejamento, tracking ou aceite de milestone; usar a issue mestre e as issues executáveis.

# Gymkhana Database — Regras de Produto, Domínio e Arquitetura

> **Escopo deste documento:** decisões permanentes aprovadas para o rebuild do Gymkhana Database.  
> **Não pertence a este documento:** progresso de desenvolvimento, milestone atual, próxima ação, branches, PRs, versões publicadas, releases ou checklists de execução.  
> **Acompanhamento operacional único:** [issue mestre #31](https://github.com/Pherlsz/Gymkhana-Database/issues/31).  
> **Repositórios relacionados:** `Pherlsz/Gymkhana-UI` e `Pherlsz/Gymkhana-Core`.

Este arquivo é a fonte permanente das regras de produto e das invariantes arquiteturais. Ele só deve mudar quando uma decisão aprovada de negócio, domínio, segurança, experiência ou arquitetura for alterada.

Nenhum documento adicional deve ser criado para acompanhar andamento, aceite de milestone ou próxima ação. Documentação técnica específica pode existir quando necessária para operar uma funcionalidade real, mas não substitui a issue mestre como tracker e não deve duplicar o estado do projeto.

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

- O login da aplicação usa GitHub OAuth com allowlist explícita.
- Não existe senha local.
- Cloudflare Access pode ser adicionado como camada externa complementar, sem substituir autenticação, sessão ou autorização da aplicação.
- O primeiro login autorizado configurado como SUPERADMIN cria a conta privilegiada inicial.
- Demais logins autorizados podem ser criados como MEMBER.
- Um usuário inativo continua bloqueado mesmo que permaneça na allowlist do GitHub.

### 3.2 Sessões

- Sessões são opacas, revogáveis e mantidas no servidor.
- A duração aprovada é de 24 horas.
- Apenas o hash SHA-256 do token de sessão é persistido.
- Cookies são HttpOnly, host-only, SameSite=Lax e Secure fora de ambientes local/test.
- Logout revoga a sessão no servidor.
- Mudança efetiva de role ou estado ativo revoga todas as sessões do usuário afetado.
- Operações sem alteração real não criam gravações ou revogações desnecessárias.

### 3.3 Roles e permissões

- Roles da aplicação: `MEMBER`, `ADMIN` e `SUPERADMIN`.
- Deve existir exatamente um SUPERADMIN ativo.
- Autorizações são centralizadas por permissions/capabilities.
- Handlers e componentes não devem replicar regras de role de forma independente.
- SUPERADMIN não ignora constraints de domínio, privacidade ou integridade.
- ADMIN não pode alterar o próprio role ou estado de acesso pela administração genérica.
- ADMIN não pode alterar o SUPERADMIN pela administração genérica.
- Feature flags e capacidades são derivadas da role/permissão, sem `secure mode` paralelo.
- Cada admin visualiza e administra apenas os próprios formulários/conexões do Google Forms, salvo permissão superior explicitamente definida.

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

- Este arquivo contém somente regras permanentes de produto, domínio, segurança, experiência e arquitetura.
- Não registrar aqui milestone atual, progresso, checklist, próxima ação, branch, PR, release ou versão instalada.
- Não atualizar este arquivo ao concluir uma tarefa ou milestone.
- Atualizar este arquivo somente quando o usuário aprovar mudança real de regra ou arquitetura.
- A issue [#31](https://github.com/Pherlsz/Gymkhana-Database/issues/31) é o único checklist vivo e a única fonte de progresso/retomada.
- Issues específicas detalham execução; não redefinem silenciosamente as regras permanentes.
- Em caso de conflito, uma decisão mais recente e explicitamente aprovada pelo usuário deve primeiro atualizar este arquivo e depois refletir-se na checklist.
- Não criar novos documentos de planejamento, tracking ou aceite de milestone; usar a issue mestre e as issues executáveis.

# Gymkhana Database

Cadastro de pessoas da gincana e do inventário de documentos e contas que a organização tem em mãos.

## Language

**Profile**:
A pessoa cadastrada.
_Avoid_: User, member, cadastro

**Número informado**:
O número do tipo na pessoa (CPF, RG, CNH, …), sem exemplar. Validade e demais dados do papel não pertencem a este conceito.
_Avoid_: Identidade, tipo 1, número conhecido, registro de CPF, documento, coluna cpf em Profile

**Exemplar**:
Original físico ou cópia digital cadastrados. O físico pode estar na organização ou com o dono; digital fica sempre conosco.
_Avoid_: Registro de identidade, cadastro do número, identidade da pessoa

**Indicação**:
A pessoa afirma ter aquele tipo, sem número e sem exemplar.
_Avoid_: tipo 0, posse declarada, possui_cnh, checkbox na Profile

**Ausência**:
A pessoa afirma não ter aquele tipo. Não aparece na grade.
_Avoid_: negativo na célula, badge riscado

**Físico**:
O original cadastrado. Guarda diz se descansa na organização ou com o dono.
_Avoid_: Papel, impresso, apagar a linha ao devolver ao dono

**Digital**:
Meio do exemplar quando a organização guarda um arquivo ou scan. Digital não se empresta.
_Avoid_: Cópia, PDF, anexo

**Uso atual**:
Quem está com o original físico agora (em uso com um portador). Só existe no exemplar físico. Digital não se empresta.
_Avoid_: record_state, CURRENT, REPLACED, EXPIRED, ARCHIVED, histórico de empréstimo

**Guarda**:
Onde o original descansa quando não está emprestado: `ORGANIZATION` (conosco) ou `OWNER` (com o dono). A linha do exemplar permanece nos dois casos. Vencido deixado conosco por opção do dono é guarda na organização.
_Avoid_: dono = portador, devolver sempre para a gaveta, apagar o físico ao devolver

**Assistente**:
Consultor de IA no fluxo (tabela, Search, painel). Interpreta a pergunta, usa Search/Query e às vezes só responde.
_Avoid_: Chat como página, copiloto, agente que grava cadastro

**Administração**:
Tela de gestão do sistema (users, integrações, chave compartilhada do modelo). Feature flags e logs de segurança só o SUPERADMIN.
_Avoid_: Preferências, settings do user

**Preferências**:
Tela da conta logada: nome de exibição, tema.
_Avoid_: Administração, Profile, chave de modelo

**Coluna calculada**:
Valor derivado na página atual da grade (idade, signo, soma de dígitos). Não é cadastro e não cobre a base inteira.
_Avoid_: custom field, coluna persistida, fórmula no banco, cálculo em 88 mil linhas

**Funil**:
Menu de filtro no cabeçalho da coluna, no molde de planilha: ordenar A–Z/Z–A, condição (vazio, texto, data, número) e lista de valores com busca e caixa de seleção. OK aplica; o recorte vai na URL.
_Avoid_: QueryPlan, SQL, fórmula personalizada, filtrar por cor, dados validados

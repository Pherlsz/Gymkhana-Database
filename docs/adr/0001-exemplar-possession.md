# Exemplar em posse, não identidade

O número no cadastro da pessoa não é um documento. O CPF informado vive em `document_presences` do tipo `cpf` (Orchestration §4 / §6.4); validade e o original ficam no exemplar (`documents`). Uma linha em documentos ou contas é um exemplar em nosso poder, físico ou digital. Só o original físico tem uso atual e guarda (`idle_custody`); digital existe sem esse status porque se envia cópia, não o original.

**Considered Options**: tratar CPF da pessoa como documento; manter `record_state` (CURRENT/REPLACED/EXPIRED/ARCHIVED) ao lado do uso atual; amarrar empréstimo ao tipo (`supports_current_use`).

**Consequences**: unicidade por tipo passa a incluir o meio, para a mesma pessoa poder ter o mesmo número em físico e digital. Colunas `record_state` e `supports_current_use` saem do schema.

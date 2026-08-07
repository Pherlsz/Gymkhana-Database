import type { CatalogV1 } from "./catalog";

export const ptBRV1 = {
  auth: {
    login: {
      title: "Gymkhana Database",
      subtitle: "Faça login para continuar",
      logoAlt: "Gymkhana",
      googleButton: "Entrar com Google",
      developmentButton: "Dev Login",
      googleStartError: "Não foi possível iniciar o login com Google.",
      developmentStartError: "Não foi possível iniciar a sessão de desenvolvimento.",
    },
  },
  shell: {
    productName: "Gymkhana Database",
    navigationLabel: "Navegação principal",
    navigation: {
      home: "Início",
      profiles: "Pessoas",
      search: "Buscar",
      query: "Consultar",
      tasks: "Tarefas",
      matching: "Duplicidades",
      chat: "Chat",
      ocr: "OCR",
      customData: "Dados personalizados",
      operations: "Operações",
      googleForms: "Google Forms",
    },
    roles: {
      external: "Membro",
      admin: "Admin",
      superadmin: "Superadmin",
    },
    signOut: "Sair",
    signingOut: "Saindo",
  },
  home: {
    eyebrow: "Aplicação privada",
    description: "Gerencie pessoas e permissões com sessões privadas e dados normalizados.",
    sessionActive: "Sessão ativa",
    userAdministrationTitle: "Administração de usuários",
    userAdministrationDescription:
      "Funções, acesso ativo e revogação de sessões são controlados pela aplicação.",
    foundationTitle: "Foundation status",
    foundationDescription:
      "A infraestrutura compartilhada continua consumida somente por versões exatas.",
  },
} satisfies CatalogV1;

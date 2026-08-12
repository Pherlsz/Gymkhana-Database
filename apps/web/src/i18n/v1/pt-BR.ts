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
} satisfies CatalogV1;

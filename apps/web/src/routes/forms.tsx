import { createFileRoute, redirect } from "@tanstack/react-router";
import { normalizeCadastroPageSearch } from "../lib/cadastro/cadastroSearch";

export const Route = createFileRoute("/forms")({
  validateSearch: (search: Record<string, unknown>) => ({
    tab: typeof search.tab === "string" ? search.tab : undefined,
    source: typeof search.source === "string" ? search.source : undefined,
    google_forms: typeof search.google_forms === "string" ? search.google_forms : undefined,
  }),
  beforeLoad: ({ search }) => {
    throw redirect({
      search: normalizeCadastroPageSearch({ ...search, mode: "forms" }),
      to: "/cadastro",
    });
  },
});

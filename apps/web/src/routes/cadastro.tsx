import { createFileRoute, stripSearchParams } from "@tanstack/react-router";
import { CadastroPage } from "../CadastroPage";
import {
  CADASTRO_SEARCH_DEFAULTS,
  normalizeCadastroPageSearch,
} from "../lib/cadastro/cadastroSearch";

export const Route = createFileRoute("/cadastro")({
  validateSearch: normalizeCadastroPageSearch,
  search: {
    middlewares: [stripSearchParams(CADASTRO_SEARCH_DEFAULTS)],
  },
  component: CadastroPage,
});

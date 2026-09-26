import { APIRequestError } from "../api/client";

export function searchErrorMessage(error: unknown): string {
  if (error instanceof APIRequestError) {
    switch (error.code) {
      case "rate_limited":
        return "O limite de buscas foi atingido. Aguarde um minuto e tente novamente.";
      case "query_too_costly":
        return "A busca ficou ampla demais. Reduza módulos, campos, termos ou a página.";
      case "result_set_too_large":
        return "A busca encontrou candidatos demais. Use módulos, campos ou termos mais específicos.";
      case "search_timeout":
        return "A busca excedeu o tempo seguro. Tente usar filtros mais específicos.";
      case "forbidden":
        return "Você não possui permissão para consultar este escopo.";
      default:
        if (error.fieldErrors.length > 0)
          return error.fieldErrors.map((field) => field.message).join(" · ");
        return error.message;
    }
  }
  return error instanceof Error ? error.message : "Erro inesperado.";
}

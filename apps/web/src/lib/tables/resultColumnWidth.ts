import { SHEET_COLUMN_WIDTH } from "./sheetDefaults";

/** Default pixel width for an assistant-result column (recorte does not reuse people-sheet prefs). */
export function resultColumnWidth(key: string, label: string): number {
  const token = `${key} ${label}`.toLowerCase();
  if (
    token.includes("full_name") ||
    token.includes("entity_label") ||
    token.includes("nome") ||
    /(^|[^a-z])name([^a-z]|$)/.test(token)
  ) {
    return SHEET_COLUMN_WIDTH.identity;
  }
  if (token.includes("found") || token.includes("match") || token.includes("formad")) {
    return 280;
  }
  if (token.includes("cpf")) {
    return 132;
  }
  if (token.includes("email")) {
    return 220;
  }
  if (token.includes("phone") || token.includes("celular") || token.includes("telefone")) {
    return 140;
  }
  return SHEET_COLUMN_WIDTH.default;
}

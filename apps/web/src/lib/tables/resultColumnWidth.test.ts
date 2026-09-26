import { describe, expect, it } from "vitest";
import { resultColumnWidth } from "./resultColumnWidth";
import { SHEET_COLUMN_WIDTH } from "./sheetDefaults";

describe("resultColumnWidth", () => {
  it("widens identity and found columns for assistant recortes", () => {
    expect(resultColumnWidth("profile.full_name", "Nome completo")).toBe(
      SHEET_COLUMN_WIDTH.identity,
    );
    expect(resultColumnWidth("found", "FOUND")).toBe(280);
    expect(resultColumnWidth("profile.cpf", "CPF")).toBe(132);
    expect(resultColumnWidth("profile.city", "Cidade")).toBe(SHEET_COLUMN_WIDTH.default);
  });
});

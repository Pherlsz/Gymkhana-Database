import { describe, expect, it } from "vitest";
import { evalFormula } from "./eval";

const columns = [
  { key: "full_name", label: "Nome completo" },
  { key: "cpf", label: "CPF" },
  { key: "number", label: "Número" },
  { key: "state", label: "UF" },
  { key: "city", label: "Cidade" },
];

describe("sheet formulas", () => {
  it("evaluates TOTAL.CARAC across columns", () => {
    expect(
      evalFormula("=TOTAL.CARAC([Nome completo]; [CPF])", { full_name: "Ana", cpf: "123" }, columns),
    ).toBe(6);
  });

  it("evaluates FAIXA and OU", () => {
    expect(evalFormula("=FAIXA([Número]; 10; 200)", { number: 120, state: "RS" }, columns)).toBe(
      "SIM",
    );
    expect(
      evalFormula(
        '=OU(E.IGUAL([UF]; "RS"); E.IGUAL([Cidade]; "Recife"))',
        { state: "RS", city: "" },
        columns,
      ),
    ).toBe("SIM");
  });

  it("evaluates SOMA.CARAC as letter sum", () => {
    expect(evalFormula('=SOMA.CARAC([Nome completo])', { full_name: "Ana" }, columns)).toBe(16);
    expect(
      evalFormula("=SOMA.CARAC([Nome completo])", { full_name: "Abbas Ahmad Bjaige" }, columns),
    ).toBeGreaterThan(0);
  });
});

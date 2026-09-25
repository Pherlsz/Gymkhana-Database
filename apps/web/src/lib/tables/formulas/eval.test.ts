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

  it("evaluates OU and E logic formulas", () => {
    expect(
      evalFormula(
        '=OU([UF]="RS"; [Cidade]="Recife")',
        { state: "RS", city: "" },
        columns,
      ),
    ).toBe("SIM");
    expect(
      evalFormula(
        '=E([UF]="RS"; [Número]>50)',
        { state: "RS", number: 120 },
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
  it("evaluates Brazilian identifier and extraction formulas", () => {
    const data = {
      cpf: "123.456.789-01",
      number: "01001-000",
      phone: "(11) 98765-4321",
      email: "contato@empresa.com.br",
    };
    const testCols = [
      ...columns,
      { key: "phone", label: "Celular" },
      { key: "email", label: "E-mail" },
    ];

    expect(evalFormula("=SO.NUMEROS([CPF])", data, testCols)).toBe("12345678901");
    expect(evalFormula("=SO.NUMEROS([Número])", data, testCols)).toBe("01001000");
    expect(evalFormula("=DDD([Celular])", data, testCols)).toBe("11");
    expect(evalFormula("=DOMINIO.EMAIL([E-mail])", data, testCols)).toBe("empresa.com.br");
    expect(evalFormula("=USUARIO.EMAIL([E-mail])", data, testCols)).toBe("contato");
  });

  it("evaluates math, financial and rounding formulas", () => {
    expect(evalFormula("=MEDIA([Número]; 200)", { number: 100 }, columns)).toBe(150);
    expect(evalFormula("=MINIMO([Número]; 50; 200)", { number: 100 }, columns)).toBe(50);
    expect(evalFormula("=MAXIMO([Número]; 50; 200)", { number: 100 }, columns)).toBe(200);
    expect(evalFormula("=SUBTRAIR([Número]; 30)", { number: 100 }, columns)).toBe(70);
    expect(evalFormula("=MULT([Número]; 3)", { number: 20 }, columns)).toBe(60);
    expect(evalFormula("=PORCENTAGEM([Número]; 15)", { number: 200 }, columns)).toBe(30);
    expect(evalFormula("=ARRED(123,456; 2)", {}, columns)).toBe(123.46);
    expect(evalFormula("=ARRED.PARA.CIMA(123,12; 0)", {}, columns)).toBe(124);
    expect(evalFormula("=ARRED.PARA.BAIXO(123,99; 0)", {}, columns)).toBe(123);
  });

  it("evaluates date and calendar formulas", () => {
    const dateData = { date: "1990-05-15", due: "2020-01-01" };
    const dateCols = [
      ...columns,
      { key: "date", label: "Data de nascimento" },
      { key: "due", label: "Vencimento" },
    ];

    expect(evalFormula("=ANO([Data de nascimento])", dateData, dateCols)).toBe(1990);
    expect(evalFormula("=MES([Data de nascimento])", dateData, dateCols)).toBe(5);
    expect(evalFormula("=MES.NOME([Data de nascimento])", dateData, dateCols)).toBe("Maio");
    expect(evalFormula("=DIA([Data de nascimento])", dateData, dateCols)).toBe(15);
    expect(evalFormula("=VENCIDO([Vencimento])", dateData, dateCols)).toBe("SIM");
  });

  it("evaluates text and logic formulas", () => {
    expect(evalFormula('=MAIUSCULA([Cidade])', { city: "porto alegre" }, columns)).toBe(
      "PORTO ALEGRE",
    );
    expect(evalFormula('=MINUSCULA([UF])', { state: "RS" }, columns)).toBe("rs");
    expect(evalFormula('=INICIAIS([Nome completo]; 3)', { full_name: "Ana Maria Silva" }, columns)).toBe(
      "AMS",
    );
    expect(evalFormula('=JUNTAR([Cidade]; " - "; [UF])', { city: "Porto Alegre", state: "RS" }, columns)).toBe(
      "Porto Alegre - RS",
    );
    expect(evalFormula('=REPETIR("A"; 3)', {}, columns)).toBe("AAA");
    expect(evalFormula('=SE([UF]="RS"; "Sul"; "Outro")', { state: "RS" }, columns)).toBe("Sul");
    expect(evalFormula('=NAO(FALSO)', {}, columns)).toBe("SIM");
    expect(evalFormula('=NAO(VERDADEIRO)', {}, columns)).toBe("NÃO");
  });
});

import { foldName } from "./text";
import type { FunnelKind } from "../columnPredicate";

export type FormulaSpec = {
  name: string;
  /** Template with [$col] / [$other]; extra args should already be example literals. */
  insert: string;
  /** One-line tip shown in the function picker and under the formula field. */
  hint: string;
  numeric?: boolean;
  also?: string[];
};

export const FORMULA_FUNCS: FormulaSpec[] = [
  { name: "IDADE", insert: "=IDADE([$col])", hint: "idade em anos a partir da data", numeric: true },
  { name: "ANO", insert: "=ANO([$col])", hint: "só o ano da data", numeric: true },
  { name: "MES", insert: "=MES([$col])", hint: "só o mês (1–12)", numeric: true, also: ["MÊS"] },
  { name: "DIA", insert: "=DIA([$col])", hint: "só o dia do mês", numeric: true },
  {
    name: "N.PRIMEIRO",
    insert: "=N.PRIMEIRO([$col]; 3)",
    hint: "primeiros N caracteres — troque o 3",
    also: ["ESQUERDA"],
  },
  {
    name: "N.ULTIMO",
    insert: "=N.ULTIMO([$col]; 2)",
    hint: "últimos N caracteres — troque o 2",
    also: ["DIREITA"],
  },
  {
    name: "TRECHO",
    insert: "=TRECHO([$col]; 1; 3)",
    hint: "pedaço do texto: início; tamanho",
  },
  { name: "MAIUSCULA", insert: "=MAIUSCULA([$col])", hint: "tudo em MAIÚSCULAS" },
  { name: "MINUSCULA", insert: "=MINUSCULA([$col])", hint: "tudo em minúsculas" },
  { name: "NOME.PROPRIO", insert: "=NOME.PROPRIO([$col])", hint: "primeira letra de cada palavra" },
  {
    name: "JUNTAR",
    insert: '=JUNTAR([$col]; " / "; [$other])',
    hint: "junta textos com o separador do meio",
  },
  {
    name: "TOTAL.CARAC",
    insert: "=TOTAL.CARAC([$col])",
    hint: "conta quantos caracteres tem",
    numeric: true,
  },
  {
    name: "SOMA.CARAC",
    insert: "=SOMA.CARAC([$col]; [$other])",
    hint: "soma o tamanho de vários textos",
    numeric: true,
  },
  {
    name: "TROCA",
    insert: '=TROCA([$col]; "a"; "A")',
    hint: 'substitui texto: TROCA([col]; "de"; "para")',
  },
  {
    name: "ARRUMAR",
    insert: "=ARRUMAR([$col])",
    hint: "tira espaços extras — só a coluna",
  },
  {
    name: "INICIAIS",
    insert: "=INICIAIS([$col]; 1)",
    hint: "iniciais de cada palavra — troque o 1",
  },
  {
    name: "SOMA.DIGITO",
    insert: "=SOMA.DIGITO([$col])",
    hint: "soma os dígitos do valor",
    numeric: true,
  },
  { name: "N.VALOR", insert: "=N.VALOR([$col])", hint: "converte texto em número", numeric: true },
  { name: "ABS", insert: "=ABS([$col])", hint: "valor absoluto (sem sinal)", numeric: true },
  {
    name: "SOMA",
    insert: "=SOMA([$col]; [$other])",
    hint: "soma números de colunas",
    numeric: true,
  },
  {
    name: "FAIXA",
    insert: "=FAIXA([$col]; 10; 200)",
    hint: "verdadeiro se estiver entre os dois valores",
    also: ["RANGE", "ENTRE"],
  },
  {
    name: "OU",
    insert: '=OU(E.IGUAL([$col]; "RS"); E.IGUAL([$other]; "SP"))',
    hint: "verdadeiro se qualquer condição for",
    also: ["OR"],
  },
  {
    name: "SE",
    insert: '=SE([$col]="RS"; "Sul"; "Outro")',
    hint: "SE(condição; se sim; se não)",
  },
  { name: "E.VAZIO", insert: "=E.VAZIO([$col])", hint: "verdadeiro se a célula estiver vazia" },
  {
    name: "SE.ERRO",
    insert: '=SE.ERRO(IDADE([$col]); "")',
    hint: "se der erro, usa o valor à direita",
  },
  {
    name: "E.IGUAL",
    insert: '=E.IGUAL([$col]; "RS")',
    hint: "compara com o valor de exemplo",
  },
  {
    name: "POSICAO",
    insert: '=POSICAO("Silva"; [$col])',
    hint: "onde o texto aparece na coluna",
    numeric: true,
  },
];

export function matchFormula(expression: string): FormulaSpec | undefined {
  const folded = firstName(expression);
  return FORMULA_FUNCS.find(
    (item) =>
      foldName(item.name) === folded || (item.also ?? []).some((alias) => foldName(alias) === folded),
  );
}

function firstName(expression: string): string {
  const match = String(expression ?? "")
    .replace(/^=/, "")
    .match(/^\s*([A-Za-zÀ-ÿÉé0-9._]+)/);
  return match ? foldName(match[1] ?? "") : "";
}

export function funcsForKind(kind: FunnelKind): FormulaSpec[] {
  const names =
    kind === "date"
      ? ["IDADE", "ANO", "MES", "DIA", "FAIXA", "OU"]
      : kind === "number"
        ? ["ABS", "N.VALOR", "SOMA", "FAIXA", "OU", "SOMA.DIGITO"]
        : kind === "select"
          ? ["SE", "E.IGUAL", "MAIUSCULA", "E.VAZIO", "JUNTAR", "OU"]
          : kind === "bool"
            ? ["SE", "E.IGUAL", "E.VAZIO", "OU"]
            : [
                "MAIUSCULA",
                "MINUSCULA",
                "N.PRIMEIRO",
                "INICIAIS",
                "TOTAL.CARAC",
                "SOMA.CARAC",
                "JUNTAR",
                "TROCA",
                "ARRUMAR",
                "OU",
              ];
  return FORMULA_FUNCS.filter((item) => names.includes(item.name));
}

export function insertFormula(
  spec: FormulaSpec,
  col: string,
  other: string,
  kind: FunnelKind,
): string {
  let insert = spec.insert;
  if (spec.name === "FAIXA") {
    insert =
      kind === "date" ? "=FAIXA([$col]; DATA(1990;1;1); DATA(2010;12;31))" : "=FAIXA([$col]; 10; 200)";
  }
  if (spec.name === "OU") {
    insert =
      kind === "number"
        ? "=OU([$col]>100; [$col]<10)"
        : kind === "date"
          ? "=OU([$col]<DATA(1990;1;1); [$col]>DATA(2010;12;31))"
          : '=OU(E.IGUAL([$col]; "RS"); E.IGUAL([$other]; "SP"))';
  }
  return insert.replaceAll("$col", col).replaceAll("$other", other);
}

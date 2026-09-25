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
  // --- Data e Tempo ---
  { name: "IDADE", insert: "=IDADE([$col])", hint: "idade em anos a partir da data", numeric: true },
  { name: "ANO", insert: "=ANO([$col])", hint: "só o ano da data (ex: 2024)", numeric: true },
  { name: "MES", insert: "=MES([$col])", hint: "só o mês numérico (1–12)", numeric: true, also: ["MÊS", "MONTH"] },
  { name: "MES.NOME", insert: "=MES.NOME([$col])", hint: "nome do mês por extenso (ex: Janeiro)", also: ["NOMEDOMES"] },
  { name: "DIA", insert: "=DIA([$col])", hint: "só o dia do mês (1–31)", numeric: true, also: ["DAY"] },
  { name: "DIA.SEMANA", insert: "=DIA.SEMANA([$col])", hint: "dia da semana (ex: Segunda-feira)", also: ["DIASEMANA", "WEEKDAY"] },
  { name: "DIAS", insert: "=DIAS([$col]; [$other])", hint: "quantidade de dias entre duas datas", numeric: true, also: ["DIASENTRE"] },
  { name: "DIAS.ATE.HOJE", insert: "=DIAS.ATE.HOJE([$col])", hint: "dias corridos até a data de hoje", numeric: true, also: ["DIASATEHOJE"] },
  { name: "HOJE", insert: "=HOJE()", hint: "data atual do sistema", also: ["DATA.HOJE"] },
  { name: "VENCIDO", insert: "=VENCIDO([$col])", hint: "verdadeiro se a data já expirou/venceu", also: ["EXPIRADO"] },

  // --- Matemática, Números e Finanças ---
  {
    name: "SOMA",
    insert: "=SOMA([$col]; [$other])",
    hint: "soma números de colunas",
    numeric: true,
    also: ["SUM"],
  },
  {
    name: "MEDIA",
    insert: "=MEDIA([$col]; [$other])",
    hint: "média aritmética dos valores",
    numeric: true,
    also: ["MÉDIA", "AVERAGE"],
  },
  {
    name: "MINIMO",
    insert: "=MINIMO([$col]; [$other])",
    hint: "menor valor entre as colunas",
    numeric: true,
    also: ["MÍNIMO", "MIN"],
  },
  {
    name: "MAXIMO",
    insert: "=MAXIMO([$col]; [$other])",
    hint: "maior valor entre as colunas",
    numeric: true,
    also: ["MÁXIMO", "MAX"],
  },
  {
    name: "SUBTRAIR",
    insert: "=SUBTRAIR([$col]; [$other])",
    hint: "diferença entre dois valores",
    numeric: true,
    also: ["SUB"],
  },
  {
    name: "MULT",
    insert: "=MULT([$col]; [$other])",
    hint: "multiplicação dos valores",
    numeric: true,
    also: ["MULTIPLICAR", "PRODUCT"],
  },
  {
    name: "PORCENTAGEM",
    insert: "=PORCENTAGEM([$col]; 10)",
    hint: "calcula percentual do valor (ex: 10%)",
    numeric: true,
    also: ["PERCENT"],
  },
  {
    name: "ARRED",
    insert: "=ARRED([$col]; 2)",
    hint: "arredonda para N casas decimais",
    numeric: true,
    also: ["ARREDONDAR", "ROUND"],
  },
  {
    name: "ARRED.PARA.CIMA",
    insert: "=ARRED.PARA.CIMA([$col]; 0)",
    hint: "arredonda para cima (teto)",
    numeric: true,
    also: ["TETO", "CEIL"],
  },
  {
    name: "ARRED.PARA.BAIXO",
    insert: "=ARRED.PARA.BAIXO([$col]; 0)",
    hint: "arredonda para baixo (piso)",
    numeric: true,
    also: ["PISO", "FLOOR"],
  },
  { name: "ABS", insert: "=ABS([$col])", hint: "valor absoluto (sem sinal negativo)", numeric: true },
  { name: "N.VALOR", insert: "=N.VALOR([$col])", hint: "converte texto em número", numeric: true, also: ["VALOR", "NUMERO"] },
  {
    name: "SOMA.DIGITO",
    insert: "=SOMA.DIGITO([$col])",
    hint: "soma todos os dígitos numéricos do valor",
    numeric: true,
    also: ["SOMADIGITOS"],
  },

  // --- Identificadores e Extração ---
  {
    name: "SO.NUMEROS",
    insert: "=SO.NUMEROS([$col])",
    hint: "extrai apenas os dígitos numéricos (remove formatação e pontuação)",
    also: ["SO.DIGITOS", "SODIGITOS", "SONUMEROS"],
  },
  {
    name: "DDD",
    insert: "=DDD([$col])",
    hint: "extrai o DDD de 2 dígitos do telefone",
  },
  {
    name: "DOMINIO.EMAIL",
    insert: "=DOMINIO.EMAIL([$col])",
    hint: "extrai o domínio do e-mail (após o @)",
    also: ["DOMINIOEMAIL", "DOMINIO"],
  },
  {
    name: "USUARIO.EMAIL",
    insert: "=USUARIO.EMAIL([$col])",
    hint: "extrai o usuário do e-mail (antes do @)",
    also: ["USUARIOEMAIL"],
  },

  // --- Manipulação de Texto ---
  { name: "MAIUSCULA", insert: "=MAIUSCULA([$col])", hint: "tudo em MAIÚSCULAS", also: ["MAIÚSCULA", "UPPER"] },
  { name: "MINUSCULA", insert: "=MINUSCULA([$col])", hint: "tudo em minúsculas", also: ["MINÚSCULA", "LOWER"] },
  {
    name: "TOTAL.CARAC",
    insert: "=TOTAL.CARAC([$col])",
    hint: "contagem total de caracteres",
    numeric: true,
    also: ["TOTALCARAC", "COMPRIMENTO", "LEN", "TAMANHO"],
  },
  {
    name: "SOMA.CARAC",
    insert: "=SOMA.CARAC([$col])",
    hint: "soma dos valores das letras (A=1, B=2...)",
    numeric: true,
    also: ["SOMACARAC", "VALORCARACT", "SOMA.LETRAS", "SOMALETRAS"],
  },
  {
    name: "INICIAIS",
    insert: "=INICIAIS([$col]; 1)",
    hint: "iniciais das palavras — troque o 1",
  },
  {
    name: "N.PRIMEIRO",
    insert: "=N.PRIMEIRO([$col]; 3)",
    hint: "primeiros N caracteres — troque o 3",
    also: ["ESQUERDA", "LEFT"],
  },
  {
    name: "N.ULTIMO",
    insert: "=N.ULTIMO([$col]; 2)",
    hint: "últimos N caracteres — troque o 2",
    also: ["DIREITA", "RIGHT"],
  },
  {
    name: "JUNTAR",
    insert: '=JUNTAR([$col]; " / "; [$other])',
    hint: "junta textos com o separador do meio",
    also: ["CONCAT", "CONCATENAR"],
  },
  {
    name: "POSICAO",
    insert: '=POSICAO("Silva"; [$col])',
    hint: "onde o texto aparece na coluna (número)",
    numeric: true,
    also: ["LOCALIZAR", "ONDE", "SEARCH"],
  },
  // --- Lógica Avançada e Condicionais ---
  {
    name: "SE",
    insert: '=SE([$col]="RS"; "Sul"; "Outro")',
    hint: "SE(condição; se sim; se não)",
    also: ["IF"],
  },
  {
    name: "E",
    insert: '=E([$col]="SP"; [$other]="Capital")',
    hint: "verdadeiro se todas as condições forem atendidas",
    also: ["AND"],
  },
  {
    name: "OU",
    insert: '=OU([$col]="RS"; [$other]="SP")',
    hint: "verdadeiro se qualquer uma das condições for atendida",
    also: ["OR"],
  },
  {
    name: "NAO",
    insert: "=NAO([$col])",
    hint: "inverte o valor lógico (não/negação)",
    also: ["NÃO", "NOT"],
  },
  {
    name: "SE.ERRO",
    insert: '=SE.ERRO(IDADE([$col]); "")',
    hint: "se der erro no cálculo, usa o valor alternativo",
    also: ["SEERRO", "IFERROR"],
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

export function funcsForKind(kind: FunnelKind, columnKey = ""): FormulaSpec[] {
  const key = columnKey.toLowerCase();

  // Documentos: CPF, RG, CNH, Passaporte, NIS, etc.
  if (/cpf|rg|doc|documento|cnh|passaporte|nis|pis/.test(key)) {
    const names = [
      "SO.NUMEROS",
      "SOMA.DIGITO",
      "TOTAL.CARAC",
      "SOMA.CARAC",
      "N.PRIMEIRO",
      "N.ULTIMO",
      "SE",
    ];
    return FORMULA_FUNCS.filter((item) => names.includes(item.name));
  }

  // Contato: Telefones, celulares, whatsapp
  if (/phone|mobile|celular|telefone|landline|contato|whatsapp/.test(key)) {
    const names = [
      "DDD",
      "SO.NUMEROS",
      "TOTAL.CARAC",
      "N.PRIMEIRO",
      "N.ULTIMO",
      "SE",
    ];
    return FORMULA_FUNCS.filter((item) => names.includes(item.name));
  }

  // CEP / Endereçamento postal
  if (/postal|cep/.test(key)) {
    const names = [
      "SO.NUMEROS",
      "TOTAL.CARAC",
      "N.PRIMEIRO",
      "N.ULTIMO",
      "SE",
    ];
    return FORMULA_FUNCS.filter((item) => names.includes(item.name));
  }

  // E-mail
  if (key.includes("email")) {
    const names = [
      "DOMINIO.EMAIL",
      "USUARIO.EMAIL",
      "MINUSCULA",
      "TOTAL.CARAC",
      "SE",
    ];
    return FORMULA_FUNCS.filter((item) => names.includes(item.name));
  }

  // Nomes de pessoas (completo, social, titular, filiação, etc.)
  if (/nome|name|titular|holder|social|pai|mae|mother|father|autor|responsavel/.test(key)) {
    const names = [
      "TOTAL.CARAC",
      "SOMA.CARAC",
      "MAIUSCULA",
      "MINUSCULA",
      "INICIAIS",
      "N.PRIMEIRO",
      "N.ULTIMO",
      "JUNTAR",
      "SE",
      "OU",
    ];
    return FORMULA_FUNCS.filter((item) => names.includes(item.name));
  }

  // Valores financeiros, preços, saldos, faturas
  if (/amount|valor|preco|custo|saldo|fatura|taxa|pagamento/.test(key)) {
    const names = [
      "SOMA",
      "MEDIA",
      "MINIMO",
      "MAXIMO",
      "SUBTRAIR",
      "MULT",
      "PORCENTAGEM",
      "ARRED",
      "ARRED.PARA.CIMA",
      "ARRED.PARA.BAIXO",
      "ABS",
      "SE",
    ];
    return FORMULA_FUNCS.filter((item) => names.includes(item.name));
  }

  // Vencimentos e prazos de validade
  if (/valid|vencimento|expira|validade/.test(key)) {
    const names = [
      "VENCIDO",
      "DIAS.ATE.HOJE",
      "DIAS",
      "ANO",
      "MES",
      "MES.NOME",
      "DIA",
      "DIA.SEMANA",
      "HOJE",
      "SE",
    ];
    return FORMULA_FUNCS.filter((item) => names.includes(item.name));
  }

  // Nascimento / Idade
  if (/nasc|birth|aniversario/.test(key)) {
    const names = [
      "IDADE",
      "ANO",
      "MES",
      "MES.NOME",
      "DIA",
      "DIA.SEMANA",
      "HOJE",
      "SE",
    ];
    return FORMULA_FUNCS.filter((item) => names.includes(item.name));
  }

  const names =
    kind === "date"
      ? [
          "IDADE",
          "ANO",
          "MES",
          "MES.NOME",
          "DIA",
          "DIA.SEMANA",
          "DIAS",
          "DIAS.ATE.HOJE",
          "HOJE",
          "VENCIDO",
          "SE",
          "OU",
        ]
      : kind === "number"
        ? [
            "SOMA",
            "MEDIA",
            "MINIMO",
            "MAXIMO",
            "SUBTRAIR",
            "MULT",
            "PORCENTAGEM",
            "ARRED",
            "ARRED.PARA.CIMA",
            "ARRED.PARA.BAIXO",
            "ABS",
            "N.VALOR",
            "SOMA.DIGITO",
            "SE",
            "OU",
          ]
        : kind === "select"
          ? [
              "MAIUSCULA",
              "MINUSCULA",
              "SE",
              "E",
              "OU",
            ]
          : kind === "bool"
            ? ["NAO", "SE", "E", "OU"]
            : [
                "TOTAL.CARAC",
                "SOMA.CARAC",
                "MAIUSCULA",
                "MINUSCULA",
                "INICIAIS",
                "N.PRIMEIRO",
                "N.ULTIMO",
                "JUNTAR",
                "POSICAO",
                "REPETIR",
                "SE",
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
  if (spec.name === "OU") {
    insert =
      kind === "number"
        ? "=OU([$col]>100; [$col]<10)"
        : kind === "date"
          ? "=OU([$col]<DATA(1990;1;1); [$col]>DATA(2010;12;31))"
          : '=OU([$col]="RS"; [$other]="SP")';
  }
  if (spec.name === "E") {
    insert =
      kind === "number"
        ? "=E([$col]>0; [$other]>0)"
        : '=E([$col]="SP"; [$other]="Capital")';
  }
  return insert.replaceAll("$col", col).replaceAll("$other", other);
}


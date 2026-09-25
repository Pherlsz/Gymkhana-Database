import { EMPTY_CELL } from "../tableRows";
import { FORMULA_FUNCS, matchFormula } from "./catalog";
import {
  ageFrom,
  asNumber,
  asText,
  digitSum,
  faixaKey,
  firstFnName,
  foldName,
  letterSum,
  splitTextsAndNumber,
  titleCase,
  yearsBetween,
} from "./text";
import { HyperFormula } from "hyperformula";

const TO_HF: Record<string, string> = {
  MES: "MONTH",
  NVALOR: "VALUE",
  NUMERO: "VALUE",
  VALOR: "VALUE",
  ABSOLUTO: "ABS",
  SEERRO: "IFERROR",
  POSICAO: "SEARCH",
  ONDE: "SEARCH",
  LOCALIZAR: "SEARCH",
  OR: "OR",
  OU: "OR",
  AND: "AND",
  E: "AND",
  SE: "IF",
  ANO: "YEAR",
  DIA: "DAY",
  DATA: "DATE",
  RANGE: "FAIXA",
  ENTRE: "FAIXA",
  EMFAIXA: "FAIXA",
  VERDADEIRO: "TRUE",
  FALSO: "FALSE",
  NPRIMEIRO: "NPRIMEIRO",
  /** Catalog dotted names → JS custom dispatch keys. */
  SOMACARAC: "VALORCARACT",
  VALORCARACT: "VALORCARACT",
  TOTALCARAC: "TOTALCARAC",
  SOMADIGITO: "SOMADIGITOS",
  SOMADIGITOS: "SOMADIGITOS",
  NOMEPROPRIO: "NOMEPROPRIO",
  EVAZIO: "EVAZIO",
  EIGUAL: "EIGUAL",
  MAIUSCULA: "MAIUSCULA",
  MINUSCULA: "MINUSCULA",
  IDADE: "IDADE",
  INICIAIS: "INICIAIS",
  TRECHO: "TRECHO",
  JUNTAR: "JUNTAR",
  ARRUMAR: "ARRUMAR",
  TROCA: "TROCA",
  FAIXA: "FAIXA",
  NULTIMO: "NULTIMO",
};

const PLUGIN = new Set([
  "IDADE",
  "INICIAIS",
  "TOTALCARAC",
  "VALORCARACT",
  "SOMADIGITOS",
  "EVAZIO",
  "EIGUAL",
  "MAIUSCULA",
  "MINUSCULA",
  "NOMEPROPRIO",
  "NPRIMEIRO",
  "NULTIMO",
  "TRECHO",
  "JUNTAR",
  "ARRUMAR",
  "TROCA",
  "FAIXA",
  "OR",
  "AND",
  "IF",
  "DATE",
]);

function resolveCustomName(raw: string): string {
  const folded = foldName(raw);
  return TO_HF[folded] ?? folded;
}

let engine: HyperFormula | null = null;
let sheetId = 0;

function gymEngine(): HyperFormula {
  if (engine) return engine;
  engine = HyperFormula.buildFromArray([[null]], {
    licenseKey: "gpl-v3",
    localeLang: "pt-BR",
    functionArgSeparator: ";",
    decimalSeparator: ",",
    thousandSeparator: " ",
    dateFormats: ["YYYY-MM-DD", "DD/MM/YYYY"],
  });
  sheetId = 0;
  return engine;
}

function rewriteFriendlyNames(src: string): string {
  let out = "";
  let inStr = false;
  let i = 0;
  while (i < src.length) {
    const ch = src[i] ?? "";
    if (inStr) {
      out += ch;
      if (ch === '"') inStr = false;
      i += 1;
      continue;
    }
    if (ch === '"') {
      inStr = true;
      out += ch;
      i += 1;
      continue;
    }
    if (ch === "[") {
      const close = src.indexOf("]", i);
      if (close < 0) {
        out += src.slice(i);
        break;
      }
      out += src.slice(i, close + 1);
      i = close + 1;
      continue;
    }
    if (/[A-Za-zÀ-ÿÉé]/.test(ch)) {
      const start = i;
      while (i < src.length && /[A-Za-zÀ-ÿÉé0-9._]/.test(src[i] ?? "")) i += 1;
      const raw = src.slice(start, i);
      out += resolveCustomName(raw);
      continue;
    }
    out += ch;
    i += 1;
  }
  return out;
}

function rewriteArgSeparators(src: string): string {
  let out = "";
  let inStr = false;
  for (let i = 0; i < src.length; i += 1) {
    const ch = src[i] ?? "";
    if (inStr) {
      out += ch;
      if (ch === '"') inStr = false;
      continue;
    }
    if (ch === '"') {
      inStr = true;
      out += ch;
      continue;
    }
    if (ch === ",") {
      const prev = out.trimEnd().slice(-1);
      const next = src[i + 1] ?? "";
      out += /\d/.test(prev) && /\d/.test(next) ? "," : ";";
      continue;
    }
    out += ch;
  }
  return out;
}

function toFormulaLiteral(value: unknown, preferNumber: boolean): string {
  if (value === true || value === "SIM") return "TRUE";
  if (value === false || value === "NÃO" || value === "NAO") return "FALSE";
  if (value === "" || value == null || value === EMPTY_CELL) return '""';
  const text = String(value);
  if (/^\d{4}-\d{2}-\d{2}$/.test(text)) {
    const [year, month, day] = text.split("-");
    return `DATE(${year};${Number(month)};${Number(day)})`;
  }
  if (/^\d{2}\/\d{2}\/\d{4}$/.test(text)) {
    const [day, month, year] = text.split("/");
    return `DATE(${year};${Number(month)};${Number(day)})`;
  }
  if (preferNumber) {
    const num = asNumber(value);
    if (num != null) return String(num).replace(".", ",");
  }
  if (typeof value === "number" && Number.isFinite(value)) return String(value).replace(".", ",");
  return `"${text.replaceAll('"', '""')}"`;
}

function wantsNumberArgs(expression: string): boolean {
  const name = firstFnName(expression);
  return name === "SOMA" || name === "SUM" || name === "ABS" || name === "NVALOR" || name === "VALUE" || name === "FAIXA";
}

export type FormulaColumn = { key: string; label: string };

function substColumns(
  expression: string,
  cells: Record<string, unknown>,
  columns: FormulaColumn[],
): string {
  const preferNumber = wantsNumberArgs(expression);
  return expression.replace(/\[([^\]]+)\]/g, (_, label: string) => {
    const needle = foldName(label);
    const col = columns.find(
      (item) => foldName(item.label) === needle || foldName(item.key) === needle,
    );
    if (!col) throw new Error(`coluna ${label}`);
    return toFormulaLiteral(cells[col.key], preferNumber);
  });
}

function splitArgs(inner: string): string[] {
  const parts: string[] = [];
  let depth = 0;
  let inStr = false;
  let current = "";
  for (let i = 0; i < inner.length; i += 1) {
    const ch = inner[i] ?? "";
    if (inStr) {
      current += ch;
      if (ch === '"') inStr = false;
      continue;
    }
    if (ch === '"') {
      inStr = true;
      current += ch;
      continue;
    }
    if (ch === "(") depth += 1;
    if (ch === ")") depth -= 1;
    if (ch === ";" && depth === 0) {
      parts.push(current.trim());
      current = "";
      continue;
    }
    current += ch;
  }
  if (current.trim()) parts.push(current.trim());
  return parts;
}

function parseLiteral(src: string): unknown {
  const text = src.trim();
  if (text === "TRUE") return true;
  if (text === "FALSE") return false;
  if (text.startsWith('"') && text.endsWith('"')) return text.slice(1, -1).replaceAll('""', '"');
  const num = Number(text.replace(",", "."));
  if (text !== "" && Number.isFinite(num)) return num;
  return text;
}

function truthy(value: unknown): boolean {
  if (value === true) return true;
  if (value === false || value == null || value === "") return false;
  return true;
}

function dispatch(name: string, args: unknown[]): unknown {
  switch (name) {
    case "IDADE":
      return args.length >= 2 ? yearsBetween(args[0], args[1]) : ageFrom(args[0]);
    case "INICIAIS": {
      const { spaced, count } = splitTextsAndNumber(args);
      const letters = spaced
        .split(/\s+/)
        .filter(Boolean)
        .map((word) => word[0])
        .join("")
        .toLocaleUpperCase("pt-BR");
      return count == null ? letters : letters.slice(0, Math.max(0, count));
    }
    case "TOTALCARAC":
      return args.reduce<number>((sum, value) => sum + asText(value).length, 0);
    case "VALORCARACT":
      return args.reduce<number>((sum, value) => sum + letterSum(value), 0);
    case "SOMADIGITOS":
      return args.reduce<number>((sum, value) => sum + (asText(value) ? digitSum(value) : 0), 0);
    case "EVAZIO":
      return args.every((value) => asText(value).trim() === "");
    case "EIGUAL":
      return args.length >= 2 && args.every((value) => asText(value) === asText(args[0]));
    case "MAIUSCULA":
      return args.map((value) => asText(value).toLocaleUpperCase("pt-BR")).join("");
    case "MINUSCULA":
      return args.map((value) => asText(value).toLocaleLowerCase("pt-BR")).join("");
    case "NOMEPROPRIO":
      return titleCase(args.map(asText).filter(Boolean).join(" "));
    case "NPRIMEIRO": {
      const { joined, count } = splitTextsAndNumber(args);
      return joined.slice(0, Math.max(0, count ?? 1));
    }
    case "NULTIMO": {
      const { joined, count } = splitTextsAndNumber(args);
      return joined.slice(-Math.max(0, count ?? 1));
    }
    case "TRECHO": {
      const nums = args.filter((arg) => typeof arg === "number") as number[];
      const texts = args.filter((arg) => typeof arg !== "number").map(asText);
      const start = Math.max(1, nums[0] ?? 1) - 1;
      return texts.join("").slice(start, start + Math.max(0, nums[1] ?? 0));
    }
    case "JUNTAR":
      return args.map(asText).join("");
    case "ARRUMAR":
      return args
        .map((value) => asText(value).replace(/\s+/g, " ").trim())
        .filter(Boolean)
        .join(" ");
    case "TROCA": {
      if (args.length < 3) return asText(args[0]);
      const to = asText(args[args.length - 1]);
      const from = asText(args[args.length - 2]);
      return args.slice(0, -2).map(asText).join("").split(from).join(to);
    }
    case "FAIXA": {
      if (args.length < 3) return false;
      const value = faixaKey(args[0]);
      if (value == null) return false;
      for (let i = 1; i + 1 < args.length; i += 2) {
        const from = faixaKey(args[i]);
        const to = faixaKey(args[i + 1]);
        if ((from == null || value >= from) && (to == null || value <= to)) return true;
      }
      return false;
    }
    case "OR":
      return args.some(truthy);
    case "AND":
      return args.every(truthy);
    case "IF":
      return truthy(args[0]) ? args[1] : args[2];
    case "DATE":
      return `${args[0]}-${String(args[1]).padStart(2, "0")}-${String(args[2]).padStart(2, "0")}`;
    default:
      return undefined;
  }
}

function evalNode(src: string): unknown {
  const text = src.trim();
  const open = text.indexOf("(");
  if (open > 0 && text.endsWith(")")) {
    const name = resolveCustomName(text.slice(0, open));
    const args = splitArgs(text.slice(open + 1, -1)).map(evalNode);
    if (PLUGIN.has(name)) {
      const custom = dispatch(name, args);
      if (custom !== undefined) return custom;
    }
    // Native / unknown call: let HyperFormula evaluate the full expression.
    throw new Error("hf");
  }
  return parseLiteral(text);
}

function unwrap(value: unknown): string | number {
  if (value === true) return "SIM";
  if (value === false) return "NÃO";
  if (typeof value === "number" && Number.isFinite(value)) {
    return Number.isInteger(value) ? value : Math.round(value * 1000) / 1000;
  }
  if (value && typeof value === "object") {
    const mark = (value as { value?: string }).value;
    if (typeof mark === "string" && mark.includes("#")) throw new Error(mark);
    throw new Error("expr");
  }
  return value == null ? "" : String(value);
}

export function evalFormula(
  expression: string,
  cells: Record<string, unknown>,
  columns: FormulaColumn[],
): string | number {
  let src = String(expression).trim();
  if (!src) throw new Error("vazia");
  if (!src.startsWith("=")) src = `=${src}`;
  src = substColumns(rewriteFriendlyNames(rewriteArgSeparators(src)), cells, columns);
  try {
    return unwrap(evalNode(src.replace(/^=/, "")));
  } catch {
    gymEngine().setSheetContent(sheetId, [[src]]);
    return unwrap(gymEngine().getCellValue({ sheet: sheetId, col: 0, row: 0 }));
  }
}

export function formulaIsNumeric(expression: string): boolean {
  return Boolean(
    matchFormula(expression)?.numeric ||
      FORMULA_FUNCS.find((item) => item.numeric && foldName(item.name) === firstFnName(expression)),
  );
}

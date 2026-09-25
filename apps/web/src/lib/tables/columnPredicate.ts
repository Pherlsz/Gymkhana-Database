import { EMPTY_CELL, cellText } from "./tableRows";

export type ColumnOp =
  | "eq"
  | "neq"
  | "contains"
  | "starts_with"
  | "gt"
  | "gte"
  | "lt"
  | "lte"
  | "between"
  | "in"
  | "is_null"
  | "not_null";

export type FunnelKind = "text" | "number" | "date" | "select" | "bool";

export type ColumnPredicate = {
  op: ColumnOp;
  values: string[];
};

export function defaultOp(kind: FunnelKind): ColumnOp {
  if (kind === "select" || kind === "bool" || kind === "date") return "eq";
  return "contains";
}

export const ALL_COLUMN_OPS: ColumnOp[] = [
  "contains",
  "eq",
  "neq",
  "starts_with",
  "gt",
  "gte",
  "lt",
  "lte",
  "between",
  "in",
  "is_null",
  "not_null",
];

export function opsForKind(_kind?: FunnelKind): ColumnOp[] {
  return ALL_COLUMN_OPS;
}

export function predicateActive(predicate: ColumnPredicate | undefined): boolean {
  if (!predicate) return false;
  if (predicate.op === "is_null" || predicate.op === "not_null") return true;
  if (predicate.op === "in") return predicate.values.some((value) => value.trim() !== "");
  if (predicate.op === "between") return predicate.values.some((value) => value.trim() !== "");
  return String(predicate.values[0] ?? "").trim() !== "";
}

export function isEmptyCell(value: unknown): boolean {
  const text = cellText(value).trim();
  return text === "" || text === EMPTY_CELL;
}

function comparable(value: unknown): string | number | null {
  if (isEmptyCell(value)) return null;
  if (typeof value === "number" && Number.isFinite(value)) return value;
  const text = cellText(value).trim();
  const iso = toIsoDate(text);
  if (iso) return iso;
  const cleanNum = text
    .replace(/^[R$\s]+/, "")
    .replace(/\s/g, "")
    .replace(/\.(?=\d{3})/g, "")
    .replace(",", ".");
  if (/^-?\d+(\.\d+)?$/.test(cleanNum)) {
    const num = Number(cleanNum);
    if (Number.isFinite(num)) return num;
  }
  const asN = Number(text.replace(",", "."));
  if (Number.isFinite(asN) && /^-?\d/.test(text)) return asN;
  return text.toLowerCase();
}

function toIsoDate(text: string): string | null {
  const ptBr = /^(\d{2})\/(\d{2})\/(\d{4})$/.exec(text);
  if (ptBr?.[1] && ptBr[2] && ptBr[3]) return `${ptBr[3]}-${ptBr[2]}-${ptBr[1]}`;
  const iso = /^(\d{4}-\d{2}-\d{2})/.exec(text);
  return iso?.[1] ?? null;
}

export function cellMatches(value: unknown, predicate: ColumnPredicate): boolean {
  if (!predicateActive(predicate)) return true;
  const text = cellText(value).trim();
  const lower = text.toLowerCase();
  if (predicate.op === "is_null") return isEmptyCell(value);
  if (predicate.op === "not_null") return !isEmptyCell(value);
  if (predicate.op === "in") {
    const digitsVal = text.replace(/\D/g, "");
    return predicate.values.some((item) => {
      if (!item) return false;
      const q = item.trim().toLowerCase();
      const qDigits = q.replace(/\D/g, "");
      return lower === q || (qDigits.length > 0 && digitsVal === qDigits);
    });
  }
  const q = String(predicate.values[0] ?? "").trim().toLowerCase();
  const qDigits = q.replace(/\D/g, "");
  const digits = text.replace(/\D/g, "");

  if (predicate.op === "contains") {
    return lower.includes(q) || (qDigits.length > 0 && digits.includes(qDigits));
  }
  if (predicate.op === "starts_with") {
    return lower.startsWith(q) || (qDigits.length > 0 && digits.startsWith(qDigits));
  }
  if (predicate.op === "eq") {
    return lower === q || (qDigits.length > 0 && digits === qDigits);
  }
  if (predicate.op === "neq") {
    return lower !== q && (qDigits.length === 0 || digits !== qDigits);
  }
  const left = comparable(value);
  if (predicate.op === "between") {
    const from = predicate.values[0]?.trim() ? comparable(predicate.values[0]) : null;
    const to = predicate.values[1]?.trim() ? comparable(predicate.values[1]) : null;
    if (left == null) return false;
    if (from != null && left < from) return false;
    if (to != null && left > to) return false;
    return true;
  }
  const right = comparable(predicate.values[0]);
  if (left == null || right == null) return false;
  if (predicate.op === "gt") return left > right;
  if (predicate.op === "gte") return left >= right;
  if (predicate.op === "lt") return left < right;
  if (predicate.op === "lte") return left <= right;
  return true;
}

export function funnelKindOf(fieldKind: "text" | "date" | "select", key: string): FunnelKind {
  if (fieldKind === "date") return "date";
  if (fieldKind === "select") return key.includes("donor") ? "bool" : "select";
  if (
    /amount|number|cpf|phone|mobile|postal|digit|year|plate/.test(key) ||
    key === "number"
  ) {
    return "number";
  }
  return "text";
}

export function formulaColumnKey(sourceKey: string): string {
  return `fx:${sourceKey}`;
}

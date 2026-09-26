export function foldName(name: string): string {
  return String(name)
    .normalize("NFD")
    .replace(/\p{M}/gu, "")
    .replace(/[._\s]/g, "")
    .toUpperCase();
}

export function asText(value: unknown): string {
  if (value == null || value === false) return "";
  if (value === true) return "SIM";
  return String(value);
}

export function asNumber(value: unknown): number | null {
  if (value === "" || value == null) return null;
  if (typeof value === "number" && Number.isFinite(value)) return value;
  const text = String(value).trim();
  if (!text) return null;
  const simple = text.replace(/\s/g, "").replace(",", ".");
  if (/^-?\d+(\.\d+)?$/.test(simple)) {
    const num = Number(simple);
    return Number.isFinite(num) ? num : null;
  }
  if (/^[\d.\-/]+$/.test(simple)) {
    const digits = simple.replace(/\D/g, "");
    if (!digits) return null;
    const num = Number(digits);
    return Number.isFinite(num) ? num : null;
  }
  return null;
}

export function digitSum(value: unknown): number {
  return [...String(value).replace(/\D/g, "")].reduce((sum, digit) => sum + Number(digit), 0);
}

export function letterSum(value: unknown): number {
  return [
    ...asText(value).normalize("NFD").replace(/\p{M}/gu, "").toLocaleUpperCase("pt-BR"),
  ].reduce((sum, ch) => {
    if (ch >= "A" && ch <= "Z") return sum + (ch.charCodeAt(0) - 64);
    if (ch >= "0" && ch <= "9") return sum + Number(ch);
    return sum;
  }, 0);
}

export function titleCase(text: string): string {
  return asText(text)
    .toLocaleLowerCase("pt-BR")
    .replace(/(^|\s)\S/g, (chunk) => chunk.toLocaleUpperCase("pt-BR"));
}

export function toDate(value: unknown): Date | null {
  if (!value && value !== 0) return null;
  if (value instanceof Date) return Number.isNaN(value.getTime()) ? null : value;
  if (typeof value === "number" && Number.isFinite(value)) {
    const date = new Date(1899, 11, 30);
    date.setDate(date.getDate() + Math.round(value));
    return date;
  }
  const text = String(value).trim();
  if (/^\d{4}-\d{2}-\d{2}$/.test(text)) return new Date(`${text}T00:00:00`);
  const month = /^(\d{4})-(\d{2})$/.exec(text);
  if (month) return new Date(`${month[1]}-${month[2]}-01T00:00:00`);
  const br = /^(\d{2})\/(\d{2})\/(\d{4})$/.exec(text);
  if (br) return new Date(`${br[3]}-${br[2]}-${br[1]}T00:00:00`);
  const date = new Date(text);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function yearsBetween(from: unknown, to: unknown): number | "" {
  const start = toDate(from);
  const end = toDate(to);
  if (!start || !end) return "";
  let years = end.getFullYear() - start.getFullYear();
  const month = end.getMonth() - start.getMonth();
  if (month < 0 || (month === 0 && end.getDate() < start.getDate())) years -= 1;
  return years;
}

export function ageFrom(value: unknown, today = new Date()): number | "" {
  const birth = toDate(value);
  if (!birth) return "";
  let years = today.getFullYear() - birth.getFullYear();
  const month = today.getMonth() - birth.getMonth();
  if (month < 0 || (month === 0 && today.getDate() < birth.getDate())) years -= 1;
  return years;
}

export function faixaKey(value: unknown): string | number | null {
  if (value == null || value === "") return null;
  if (typeof value === "number" && Number.isFinite(value)) return value;
  const iso = String(value);
  if (/^\d{4}-\d{2}-\d{2}$/.test(iso) || /^\d{4}-\d{2}$/.test(iso)) return iso;
  const num = asNumber(value);
  if (num != null) return num;
  const text = asText(value).trim();
  return text ? text.toLocaleLowerCase("pt-BR") : null;
}

export function splitTextsAndNumber(args: unknown[]): {
  joined: string;
  spaced: string;
  count: number | null;
} {
  const values = args.filter((arg) => arg !== undefined);
  if (!values.length) return { joined: "", spaced: "", count: null };
  const last = values[values.length - 1];
  const lastNum = typeof last === "number" ? last : null;
  const texts = (lastNum == null ? values : values.slice(0, -1)).map(asText);
  return {
    joined: texts.join(""),
    spaced: texts.filter((text) => text.trim()).join(" "),
    count: lastNum,
  };
}

export function firstFnName(expression: string): string {
  const match = String(expression ?? "")
    .replace(/^=/, "")
    .match(/^\s*([A-Za-zÀ-ÿÉé0-9._]+)/);
  return match ? foldName(match[1] ?? "") : "";
}

export function digitsOnly(value: unknown): string {
  return String(value ?? "").replace(/\D/g, "");
}

export function formatCPF(value: unknown): string {
  const d = digitsOnly(value);
  if (d.length !== 11) return asText(value);
  return `${d.slice(0, 3)}.${d.slice(3, 6)}.${d.slice(6, 9)}-${d.slice(9, 11)}`;
}

export function formatCEP(value: unknown): string {
  const d = digitsOnly(value);
  if (d.length !== 8) return asText(value);
  return `${d.slice(0, 5)}-${d.slice(5, 8)}`;
}

export function formatPhone(value: unknown): string {
  const d = digitsOnly(value);
  if (d.length === 11) return `(${d.slice(0, 2)}) ${d.slice(2, 7)}-${d.slice(7)}`;
  if (d.length === 10) return `(${d.slice(0, 2)}) ${d.slice(2, 6)}-${d.slice(6)}`;
  return asText(value);
}

export function extractDDD(value: unknown): string {
  const d = digitsOnly(value);
  return d.length >= 10 ? d.slice(0, 2) : "";
}

export function extractEmailDomain(value: unknown): string {
  const text = asText(value).trim();
  const idx = text.indexOf("@");
  return idx >= 0 ? text.slice(idx + 1).toLowerCase() : "";
}

export function extractEmailUser(value: unknown): string {
  const text = asText(value).trim();
  const idx = text.indexOf("@");
  return idx >= 0 ? text.slice(0, idx) : text;
}

const MONTH_NAMES = [
  "Janeiro",
  "Fevereiro",
  "Março",
  "Abril",
  "Maio",
  "Junho",
  "Julho",
  "Agosto",
  "Setembro",
  "Outubro",
  "Novembro",
  "Dezembro",
];

export function monthName(value: unknown): string {
  const d = toDate(value);
  if (d) return MONTH_NAMES[d.getMonth()] ?? "";
  const n = asNumber(value);
  if (n != null && n >= 1 && n <= 12) return MONTH_NAMES[Math.trunc(n) - 1] ?? "";
  return "";
}

const WEEKDAY_NAMES = [
  "Domingo",
  "Segunda-feira",
  "Terça-feira",
  "Quarta-feira",
  "Quinta-feira",
  "Sexta-feira",
  "Sábado",
];

export function weekdayName(value: unknown): string {
  const d = toDate(value);
  return d ? (WEEKDAY_NAMES[d.getDay()] ?? "") : "";
}

export function daysBetween(from: unknown, to: unknown): number | "" {
  const start = toDate(from);
  const end = toDate(to);
  if (!start || !end) return "";
  const diffMs = end.getTime() - start.getTime();
  return Math.round(diffMs / (1000 * 60 * 60 * 24));
}

export function daysFromToday(value: unknown, today = new Date()): number | "" {
  const d = toDate(value);
  if (!d) return "";
  today.setHours(0, 0, 0, 0);
  d.setHours(0, 0, 0, 0);
  const diffMs = d.getTime() - today.getTime();
  return Math.round(diffMs / (1000 * 60 * 60 * 24));
}

export function isExpired(value: unknown, today = new Date()): boolean {
  const d = toDate(value);
  if (!d) return false;
  today.setHours(0, 0, 0, 0);
  d.setHours(0, 0, 0, 0);
  return d.getTime() < today.getTime();
}

export function roundNum(val: unknown, decimals = 0): number | "" {
  const n = asNumber(val);
  if (n == null) return "";
  const factor = Math.pow(10, Math.max(0, decimals));
  return Math.round(n * factor) / factor;
}

export function ceilNum(val: unknown, decimals = 0): number | "" {
  const n = asNumber(val);
  if (n == null) return "";
  const factor = Math.pow(10, Math.max(0, decimals));
  return Math.ceil(n * factor) / factor;
}

export function floorNum(val: unknown, decimals = 0): number | "" {
  const n = asNumber(val);
  if (n == null) return "";
  const factor = Math.pow(10, Math.max(0, decimals));
  return Math.floor(n * factor) / factor;
}

export function averageNums(args: unknown[]): number | "" {
  const nums = args.map(asNumber).filter((n): n is number => n != null);
  if (!nums.length) return "";
  return nums.reduce((a, b) => a + b, 0) / nums.length;
}

export function minNums(args: unknown[]): number | "" {
  const nums = args.map(asNumber).filter((n): n is number => n != null);
  if (!nums.length) return "";
  return Math.min(...nums);
}

export function maxNums(args: unknown[]): number | "" {
  const nums = args.map(asNumber).filter((n): n is number => n != null);
  if (!nums.length) return "";
  return Math.max(...nums);
}

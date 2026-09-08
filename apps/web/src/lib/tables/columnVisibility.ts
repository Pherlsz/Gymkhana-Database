import type { SpreadsheetColumn } from "./SpreadsheetTable";

export type TableSheetScope = "profiles" | "documents" | "bills";
export type ColumnVisibilityOverrides = Record<string, boolean>;

const COLUMN_KEY = /^[a-z][a-z0-9_]*(?::[a-z0-9_]+)?$/i;

export const PEOPLE_LOCKED_KEYS = new Set(["full_name"]);
export const DOCUMENT_LOCKED_KEYS = new Set(["identifier"]);
export const BILL_LOCKED_KEYS = new Set(["reference"]);

export const PEOPLE_OTHER_DOC_KEYS = [
  "voter_id",
  "cnh",
  "ctps",
  "ctps_series",
  "pis",
  "crea",
  "oab",
  "student_id",
  "sus_card",
  "citizen_card",
  "passport",
] as const;

export const PEOPLE_DEFAULT_VISIBLE_KEYS = new Set([
  "full_name",
  "documents",
  "cpf",
  "rg",
  "street",
  "number",
  "city",
  "postal_code",
  "email",
  "mobile",
  "birth_date",
  "team",
  "sector",
  ...PEOPLE_OTHER_DOC_KEYS,
]);

export const DOCUMENT_DEFAULT_VISIBLE_KEYS = new Set([
  "identifier",
  "type",
  "owner",
  "status",
  "medium",
  "idle_custody",
  "valid_until",
]);

export const BILL_DEFAULT_VISIBLE_KEYS = new Set([
  "reference",
  "type",
  "owner",
  "competence",
  "amount",
  "status",
  "medium",
  "idle_custody",
]);

const LOCKED: Record<TableSheetScope, Set<string>> = {
  profiles: PEOPLE_LOCKED_KEYS,
  documents: DOCUMENT_LOCKED_KEYS,
  bills: BILL_LOCKED_KEYS,
};

const DEFAULT_VISIBLE: Record<TableSheetScope, Set<string>> = {
  profiles: PEOPLE_DEFAULT_VISIBLE_KEYS,
  documents: DOCUMENT_DEFAULT_VISIBLE_KEYS,
  bills: BILL_DEFAULT_VISIBLE_KEYS,
};

export type ColumnGroupKey =
  | "identity"
  | "documents"
  | "contact"
  | "address"
  | "family"
  | "record"
  | "ownership"
  | "custody"
  | "gymkhana"
  | "other"
  | "custom";

/**
 * Which heading a column sits under in the picker. People alone carries ~57
 * columns, so a flat list is unreadable; the group names are shared across the
 * three sheets wherever the same concept exists, because Orchestration 12.1
 * forbids per-module divergence in the table chrome.
 */
const COLUMN_GROUPS: Record<TableSheetScope, Record<string, ColumnGroupKey>> = {
  profiles: {
    full_name: "identity",
    social_name: "identity",
    gender: "identity",
    birth_date: "identity",
    blood_type: "identity",
    marital_status: "identity",
    nationality: "identity",
    birth_city: "identity",
    birth_country: "identity",
    place_of_origin: "identity",
    documents: "documents",
    cpf: "documents",
    rg: "documents",
    voter_id: "documents",
    cnh: "documents",
    ctps: "documents",
    ctps_series: "documents",
    pis: "documents",
    crea: "documents",
    oab: "documents",
    student_id: "documents",
    sus_card: "documents",
    citizen_card: "documents",
    passport: "documents",
    email: "contact",
    mobile: "contact",
    landline: "contact",
    street: "address",
    number: "address",
    complement: "address",
    neighborhood: "address",
    city: "address",
    state: "address",
    postal_code: "address",
    father_name: "family",
    father_birth_date: "family",
    mother_name: "family",
    mother_birth_date: "family",
    wedding_date: "family",
    parents_wedding_date: "family",
    team: "gymkhana",
    sector: "gymkhana",
    club_membership: "gymkhana",
    membership_type: "gymkhana",
    collections: "gymkhana",
  },
  documents: {
    identifier: "record",
    type: "record",
    date: "record",
    valid_until: "record",
    notes: "record",
    owner: "ownership",
    status: "custody",
    medium: "custody",
    idle_custody: "custody",
    current_holder: "custody",
  },
  bills: {
    reference: "record",
    type: "record",
    competence: "record",
    amount: "record",
    currency: "record",
    notes: "record",
    owner: "ownership",
    printed_holder_name: "ownership",
    printed_address: "ownership",
    status: "custody",
    medium: "custody",
    idle_custody: "custody",
  },
};

export function columnGroup(key: string, scope: TableSheetScope): ColumnGroupKey {
  if (key.startsWith("custom:")) return "custom";
  return COLUMN_GROUPS[scope][key] ?? "other";
}

export function columnLabel<T>(column: SpreadsheetColumn<T>): string {
  if (column.label) return column.label;
  return typeof column.title === "string" ? column.title : column.key;
}

export function isLockedColumn(key: string, scope: TableSheetScope): boolean {
  return LOCKED[scope].has(key);
}

export function defaultColumnVisible(
  column: Pick<SpreadsheetColumn<unknown>, "key">,
  scope: TableSheetScope,
): boolean {
  if (LOCKED[scope].has(column.key)) return true;
  if (DEFAULT_VISIBLE[scope].has(column.key)) return true;
  return false;
}

export function withColumnLayout<T>(
  column: SpreadsheetColumn<T>,
  scope: TableSheetScope,
): SpreadsheetColumn<T> {
  return {
    ...column,
    label: columnLabel(column),
    locked: isLockedColumn(column.key, scope),
    defaultVisible: defaultColumnVisible(column, scope),
  };
}

export function isColumnVisible<T>(
  column: SpreadsheetColumn<T>,
  overrides: ColumnVisibilityOverrides,
): boolean {
  if (column.locked) return true;
  const override = overrides[column.key];
  if (typeof override === "boolean") return override;
  return column.defaultVisible !== false;
}

export function setColumnVisible(
  column: Pick<SpreadsheetColumn<unknown>, "key" | "locked" | "defaultVisible">,
  visible: boolean,
  overrides: ColumnVisibilityOverrides,
): ColumnVisibilityOverrides {
  if (column.locked) return overrides;
  const next = { ...overrides };
  const fallsBackToDefault = visible === (column.defaultVisible !== false);
  if (fallsBackToDefault) {
    delete next[column.key];
    return next;
  }
  next[column.key] = visible;
  return next;
}

export function showAllColumns<T>(
  columns: SpreadsheetColumn<T>[],
  overrides: ColumnVisibilityOverrides,
): ColumnVisibilityOverrides {
  let next = overrides;
  for (const column of columns) {
    next = setColumnVisible(column, true, next);
  }
  return next;
}

export function parseColumnCols(value: unknown): ColumnVisibilityOverrides {
  if (typeof value !== "string" || !value.trim()) return {};
  const locked = new Set([...PEOPLE_LOCKED_KEYS, ...DOCUMENT_LOCKED_KEYS, ...BILL_LOCKED_KEYS]);
  const overrides: ColumnVisibilityOverrides = {};
  for (const raw of value.split(",")) {
    const token = raw.trim();
    if (!token) continue;
    const hidden = token.startsWith("-");
    const key = hidden ? token.slice(1) : token;
    if (!COLUMN_KEY.test(key)) continue;
    if (hidden && locked.has(key)) continue;
    overrides[key] = !hidden;
  }
  return overrides;
}

export function formatColumnCols(overrides: ColumnVisibilityOverrides): string {
  return Object.keys(overrides)
    .sort()
    .filter((key) => typeof overrides[key] === "boolean" && COLUMN_KEY.test(key))
    .map((key) => (overrides[key] ? key : `-${key}`))
    .join(",");
}

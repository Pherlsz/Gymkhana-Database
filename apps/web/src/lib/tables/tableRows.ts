import type { BillRecord, CustomField, DocumentRecord, Profile } from "../api/client";
import { formatAmount, formatCPF, formatDate } from "../formatters";
import { t } from "../../i18n";

export type TableRow = {
  id: string;
  cells: Record<string, unknown>;
};

export const EMPTY_CELL = "—";

export const DETAILS_FIELD_KEYS = [
  "birth_date",
  "gender",
  "blood_type",
  "nationality",
  "birth_city",
  "marital_status",
  "wedding_date",
  "father_name",
  "father_birth_date",
  "mother_name",
  "mother_birth_date",
  "health_plan",
  "blood_donor",
  "organ_donor",
  "team",
  "sector",
  "collections",
  "vehicle_model",
  "vehicle_color",
  "vehicle_plate",
  "vehicle_year",
  "club_membership",
  "membership_type",
  "place_of_origin",
  "birth_country",
  "parents_wedding_date",
  "supermarket_club",
  "pet",
  "travel_countries",
  "card_brand",
  "card_bank",
] as const;

export const PEOPLE_SHEET_DOC_KEYS = [
  "rg",
  "voter_id",
  "cnh",
  "ctps",
  "pis",
  "crea",
  "oab",
  "student_id",
  "sus_card",
  "citizen_card",
  "passport",
] as const;

const SKIP_DOC_IDENTIFIER_KEYS = new Set([
  "cpf",
  "identidade",
  "identity",
  "crea_oab",
  "crea-oab",
  "crm",
  "cro",
  "coren",
  "birth_certificate",
  "marriage_certificate",
]);

const CANONICAL_KEYS = new Set([
  "full_name",
  "social_name",
  "cpf",
  "cpf_digit_sum",
  "email",
  "mobile",
  "landline",
  "street",
  "number",
  "complement",
  "neighborhood",
  "city",
  "state",
  "postal_code",
  "notes",
  "document_badges",
  "ctps_series",
  "type",
  "identifier",
  "owner",
  "owner_id",
  "status",
  "status_key",
  "medium",
  "medium_key",
  "idle_custody",
  "valid_until",
  "date",
  "current_holder",
  "current_holder_id",
  "reference",
  "competence",
  "amount",
  "currency",
  "printed_holder_name",
  "printed_address",
  ...DETAILS_FIELD_KEYS,
  ...PEOPLE_SHEET_DOC_KEYS,
]);

export function mergeExtraCells(
  extras: Record<string, string> | undefined,
  cells: Record<string, unknown>,
): Record<string, unknown> {
  const merged: Record<string, unknown> = { ...extras };
  for (const [key, value] of Object.entries(cells)) merged[key] = value;
  return merged;
}

function personalCells(profile: Profile): Record<string, string> {
  const cells: Record<string, string> = {};
  const record = profile as unknown as Record<string, unknown>;
  for (const key of DETAILS_FIELD_KEYS) {
    const value = record[key];
    if (value === undefined || value === null || value === "") continue;
    cells[key] = String(value);
  }
  return cells;
}

const PEOPLE_DATE_KEYS = new Set([
  "birth_date",
  "wedding_date",
  "father_birth_date",
  "mother_birth_date",
  "parents_wedding_date",
]);

const PEOPLE_BOOLEAN_KEYS = new Set(["blood_donor", "organ_donor"]);

export type SheetBooleanCopy = { yes: string; no: string };

export type RecordSheetLabels = {
  boolean: SheetBooleanCopy;
  status: { AVAILABLE: string; IN_USE: string };
  medium: { PHYSICAL: string; DIGITAL: string };
  idleCustody: { ORGANIZATION: string; OWNER: string };
};

export function personRow(
  profile: Profile,
  extraFields: CustomField[] = [],
  booleanCopy: SheetBooleanCopy = { yes: "Sim", no: "Não" },
): TableRow {
  const identifiers: Record<string, string> = {};
  for (const [key, value] of Object.entries(profile.document_identifiers ?? {})) {
    if (SKIP_DOC_IDENTIFIER_KEYS.has(key)) continue;
    identifiers[key] = value;
  }
  const cells = mergeExtraCells(
    { ...profile.custom_values, ...personalCells(profile), ...identifiers },
    {
      full_name: profile.full_name,
      social_name: profile.social_name,
      cpf: profile.cpf,
      cpf_digit_sum: profile.cpf_digit_sum ?? null,
      email: profile.email,
      city: profile.address.city,
      state: profile.address.state,
      street: profile.address.street,
      number: profile.address.number,
      complement: profile.address.complement,
      neighborhood: profile.address.neighborhood,
      postal_code: profile.address.postal_code,
      mobile: profile.mobile_phone,
      landline: profile.landline_phone,
      document_badges: positiveBadges(profile.document_badges),
    },
  );
  return { id: profile.id, cells: formatPeopleCells(cells, extraFields, booleanCopy) };
}

export function documentRow(
  record: DocumentRecord,
  labels: RecordSheetLabels,
  extraFields: CustomField[] = [],
): TableRow {
  const cells = mergeExtraCells(record.custom_values, {
    type: record.type.label,
    type_id: record.document_type_id,
    type_key: record.type.technical_key,
    identifier: record.identifier_value,
    owner: record.owner_full_name,
    owner_id: record.owner_profile_id,
    status: labeled(record.status, labels.status),
    status_key: record.status,
    medium: labeled(record.medium, labels.medium),
    medium_key: record.medium,
    idle_custody: labeled(record.idle_custody, labels.idleCustody),
    valid_until: formatDate(record.valid_until ?? ""),
    date: formatDate(record.document_date),
    notes: record.notes,
    current_holder: record.current_use?.holder_full_name ?? "",
    current_holder_id: record.current_use?.holder_profile_id ?? "",
  });
  return {
    id: record.id,
    cells: formatExtraCells(displayStringCells(cells), extraFields, labels.boolean),
  };
}

export function billRow(
  record: BillRecord,
  labels: RecordSheetLabels,
  extraFields: CustomField[] = [],
): TableRow {
  const cells = mergeExtraCells(record.custom_values, {
    type: record.type.label,
    type_id: record.bill_type_id,
    type_key: record.type.technical_key,
    reference: record.reference_value,
    owner: record.owner_full_name,
    owner_id: record.owner_profile_id,
    competence: record.competence,
    amount: formatAmount(record.amount, record.currency),
    currency: record.currency,
    status: labeled(record.status, labels.status),
    status_key: record.status,
    medium: labeled(record.medium, labels.medium),
    medium_key: record.medium,
    idle_custody: labeled(record.idle_custody, labels.idleCustody),
    notes: record.notes,
    printed_holder_name: record.printed_holder_name,
    printed_address: record.printed_address,
    current_holder: record.current_use?.holder_full_name ?? "",
    current_holder_id: record.current_use?.holder_profile_id ?? "",
  });
  return {
    id: record.id,
    cells: formatExtraCells(displayStringCells(cells), extraFields, labels.boolean),
  };
}

function displayStringCells(cells: Record<string, unknown>): Record<string, unknown> {
  const next: Record<string, unknown> = { ...cells };
  for (const [key, value] of Object.entries(next)) {
    if (
      key === "document_badges" ||
      key.endsWith("_id") ||
      key === "status_key" ||
      key === "medium_key"
    )
      continue;
    next[key] = displayCell(value);
  }
  return next;
}

function formatPeopleCells(
  cells: Record<string, unknown>,
  extraFields: CustomField[],
  booleanCopy: SheetBooleanCopy,
): Record<string, unknown> {
  const next: Record<string, unknown> = { ...cells };
  for (const [key, value] of Object.entries(next)) {
    if (key === "document_badges" || key === "owner_id" || key === "current_holder_id") continue;
    if (PEOPLE_DATE_KEYS.has(key)) {
      next[key] = displayCell(formatDate(cellText(value)));
      continue;
    }
    if (PEOPLE_BOOLEAN_KEYS.has(key)) {
      next[key] = booleanCell(cellText(value), booleanCopy);
      continue;
    }
    next[key] = displayCell(value);
  }
  return formatExtraCells(next, extraFields, booleanCopy);
}

function formatExtraCells(
  cells: Record<string, unknown>,
  extraFields: CustomField[],
  booleanCopy: SheetBooleanCopy,
): Record<string, unknown> {
  if (extraFields.length === 0) return cells;
  const next = { ...cells };
  for (const field of extraFields) {
    const formatted = formatCustomValue(next[field.technical_key], field.field_kind);
    next[field.technical_key] =
      field.field_kind === "BOOLEAN" ? booleanCell(formatted, booleanCopy) : displayCell(formatted);
  }
  return next;
}

function labeled(value: string | undefined, copy: Record<string, string>): string {
  if (!value) return EMPTY_CELL;
  return copy[value] ?? value;
}

function booleanCell(value: string, booleanCopy: SheetBooleanCopy): string {
  if (value === "true") return booleanCopy.yes;
  if (value === "false") return booleanCopy.no;
  return displayCell(value);
}

function positiveBadges(
  badges: Profile["document_badges"],
): NonNullable<Profile["document_badges"]> {
  return (badges ?? []).filter((badge) => Boolean(badge.badge));
}

export function uniqueCustomFields(fields: CustomField[]): CustomField[] {
  const seen = new Set<string>();
  const result: CustomField[] = [];
  for (const field of fields) {
    if (!field.active || seen.has(field.technical_key) || CANONICAL_KEYS.has(field.technical_key)) {
      continue;
    }
    seen.add(field.technical_key);
    result.push(field);
  }
  return result;
}

export { formatCPF, formatDate, formatAmount };

export function formatCustomValue(value: unknown, kind: CustomField["field_kind"]): string {
  const text = value == null ? "" : String(value);
  if (!text) return "";
  if (kind === "CIVIL_DATE") return formatDate(text);
  return text;
}

export function cellText(value: unknown): string {
  if (value && typeof value === "object" && "ok" in value) return "";
  if (value == null) return "";
  return String(value);
}

export function displayCell(value: unknown): string {
  const text = cellText(value).trim();
  return text === "" ? EMPTY_CELL : text;
}

export function paginationRange(page: number, pageSize: number, total: number, template: string) {
  if (total === 0) {
    return t(template, { from: 0, to: 0, total: 0 });
  }
  const from = (page - 1) * pageSize + 1;
  const to = Math.min(page * pageSize, total);
  return t(template, { from, to, total });
}

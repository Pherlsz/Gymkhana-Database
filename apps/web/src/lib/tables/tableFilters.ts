import type { CustomField } from "../api/client";
import { SHEET_DISTINCT_VALUE_LIMIT } from "./sheetDefaults";
import type { ToolbarFilterField } from "./TablesToolbar";
import { cellText, type TableRow } from "./tableRows";

export const BRAZIL_STATES = [
  "AC",
  "AL",
  "AP",
  "AM",
  "BA",
  "CE",
  "DF",
  "ES",
  "GO",
  "MA",
  "MT",
  "MS",
  "MG",
  "PA",
  "PB",
  "PR",
  "PE",
  "PI",
  "RJ",
  "RN",
  "RS",
  "RO",
  "RR",
  "SC",
  "SP",
  "SE",
  "TO",
] as const;

const CNH_CATEGORIES = [
  "A",
  "B",
  "AB",
  "C",
  "D",
  "E",
  "ACC",
  "AC",
  "AD",
  "AE",
  "BC",
  "BD",
  "BE",
  "CD",
  "CE",
  "DE",
];

const BRAZIL_STATE_SET = new Set<string>(BRAZIL_STATES);
const UF_KEYS = new Set(["state", "uf", "estado", "issuing_state"]);
const CATEGORY_KEYS = new Set(["category", "categoria"]);
const ENUM_KEYS = new Set([
  ...UF_KEYS,
  ...CATEGORY_KEYS,
  "sexo",
  "genero",
  "gender",
  "status",
  "suporte",
  "tipo",
  "type",
  "medium",
  "idle_custody",
]);

export function brazilStateOptions() {
  return BRAZIL_STATES.map((value) => ({ value, label: value }));
}

export function cellKeyForFilter(field: ToolbarFilterField) {
  if (field.key.startsWith("custom:")) return field.key.slice("custom:".length);
  if (field.key.startsWith("doc:")) return field.key.slice("doc:".length);
  return field.key;
}

/** Normalizes a display date (pt-BR `dd/mm/yyyy` or ISO `yyyy-mm-dd`) to ISO. */
function toIsoDate(text: string): string | null {
  const ptBr = /^(\d{2})\/(\d{2})\/(\d{4})$/.exec(text);
  if (ptBr && ptBr[1] && ptBr[2] && ptBr[3]) return `${ptBr[3]}-${ptBr[2]}-${ptBr[1]}`;
  const iso = /^(\d{4}-\d{2}-\d{2})/.exec(text);
  return iso && iso[1] ? iso[1] : null;
}

/**
 * Date filters commit ISO values from `<input type="date">`, but cells display
 * as pt-BR (`dd/mm/yyyy`). Compare both sides canonically instead of doing a
 * substring match, which would never hit.
 */
export function localDateMatches(cellValue: string, needle: string): boolean {
  const cellIso = toIsoDate(cellValue.trim());
  const needleIso = toIsoDate(needle.trim());
  if (cellIso == null || needleIso == null) {
    return cellValue.trim().toLowerCase().includes(needle.trim().toLowerCase());
  }
  return cellIso === needleIso;
}

export function distinctValues(
  rows: TableRow[],
  key: string,
  limit = SHEET_DISTINCT_VALUE_LIMIT,
): string[] {
  const values = new Set<string>();
  for (const row of rows) {
    const text = cellText(row.cells[key]).trim();
    if (text) values.add(text);
    if (values.size > limit) break;
  }
  return [...values].toSorted((a, b) => a.localeCompare(b, "pt-BR"));
}

export function textFilter(
  key: string,
  label: string,
  value: string,
  onChange: (value: string) => void,
  local = false,
): ToolbarFilterField {
  if (local) return { key, kind: "text", label, value, local: true, onChange };
  return { key, kind: "text", label, value, onChange };
}

export function dateFilter(
  key: string,
  label: string,
  value: string,
  onChange: (value: string) => void,
  local = false,
): ToolbarFilterField {
  if (local) return { key, kind: "date", label, value, local: true, onChange };
  return { key, kind: "date", label, value, onChange };
}

export function selectFilter(
  key: string,
  label: string,
  value: string,
  options: { value: string; label: string }[],
  allLabel: string,
  onChange: (value: string) => void,
  groups?: { label: string; options: { value: string; label: string }[] }[],
  local = false,
): ToolbarFilterField {
  if (groups?.length) {
    return local
      ? { key, kind: "select", label, value, options, allLabel, groups, local: true, onChange }
      : { key, kind: "select", label, value, options, allLabel, groups, onChange };
  }
  return local
    ? { key, kind: "select", label, value, options, allLabel, local: true, onChange }
    : { key, kind: "select", label, value, options, allLabel, onChange };
}

export function customFieldFilter(
  field: CustomField,
  value: string,
  onChange: (value: string) => void,
  loadedOptions: { value: string; label: string }[],
  distinct: string[],
  copy: { all: string; boolean: { yes: string; no: string } },
): ToolbarFilterField {
  const key = `custom:${field.technical_key}`;
  if (field.field_kind === "BOOLEAN") {
    return selectFilter(
      key,
      field.label,
      value,
      [
        { value: "true", label: copy.boolean.yes },
        { value: "false", label: copy.boolean.no },
      ],
      copy.all,
      onChange,
      undefined,
      true,
    );
  }
  if (field.field_kind === "CIVIL_DATE") {
    return dateFilter(key, field.label, value, onChange, true);
  }

  const preset = presetOptions(field.technical_key, distinct);
  const fromKind =
    field.field_kind === "SINGLE_SELECT" ||
    field.field_kind === "MULTI_SELECT" ||
    ENUM_KEYS.has(field.technical_key);
  const options =
    loadedOptions.length > 0
      ? loadedOptions
      : preset.length > 0
        ? preset
        : fromKind
          ? distinct.map((item) => ({ value: item, label: item }))
          : [];
  if (options.length > 0) {
    return selectFilter(key, field.label, value, options, copy.all, onChange, undefined, true);
  }
  return textFilter(key, field.label, value, onChange, true);
}

function presetOptions(
  technicalKey: string,
  distinct: string[],
): { value: string; label: string }[] {
  if (UF_KEYS.has(technicalKey)) {
    const extra = distinct
      .filter((value) => !BRAZIL_STATE_SET.has(value.toUpperCase()))
      .map((value) => ({ value, label: value }));
    return extra.length > 0 ? [...brazilStateOptions(), ...extra] : brazilStateOptions();
  }
  if (CATEGORY_KEYS.has(technicalKey)) {
    const seen = new Set(CNH_CATEGORIES);
    const options = CNH_CATEGORIES.map((value) => ({ value, label: value }));
    for (const value of distinct) {
      if (!seen.has(value)) {
        seen.add(value);
        options.push({ value, label: value });
      }
    }
    return options;
  }
  return [];
}

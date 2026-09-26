import type { ProfileListSearch } from "../api/client";
import {
  BILL_DEFAULT_VISIBLE_KEYS,
  DOCUMENT_DEFAULT_VISIBLE_KEYS,
  PEOPLE_DEFAULT_VISIBLE_KEYS,
  formatColumnCols,
  parseColumnCols,
} from "../tables/columnVisibility";
import { clearFilters } from "../tables/sheetQuery";

export type AssistantRecorte = {
  table: "people" | "documents" | "bills";
  filters: Record<string, string>;
  columns: string[];
  sort?: string;
  order?: "asc" | "desc";
};

const PEOPLE_FILTERS = ["full_name", "cpf", "email", "city", "state"] as const;
const DOCUMENT_FILTERS = ["document_identifier", "document_medium"] as const;
const BILL_FILTERS = ["bill_reference", "bill_competence", "bill_medium"] as const;
const PEOPLE_SORT = [
  "full_name",
  "cpf",
  "email",
  "address_city",
  "address_street",
  "address_neighborhood",
  "mobile_phone",
  "birth_date",
  "created_at",
  "updated_at",
] as const;
const DOCUMENT_SORT = [
  "identifier_value",
  "type_label",
  "document_date",
  "created_at",
  "updated_at",
] as const;
const BILL_SORT = [
  "reference_value",
  "type_label",
  "competence",
  "amount",
  "created_at",
  "updated_at",
] as const;

function sectionOf(table: AssistantRecorte["table"]): ProfileListSearch["section"] {
  if (table === "documents") return "documents";
  if (table === "bills") return "bills";
  return "profile";
}

function pick(
  filters: Record<string, string>,
  keys: readonly string[],
): Partial<ProfileListSearch> {
  const patch: Partial<ProfileListSearch> = {};
  for (const key of keys) {
    const value = filters[key];
    if (value) (patch as Record<string, string>)[key] = value;
  }
  return patch;
}

function extraColumns(section: ProfileListSearch["section"], cols: string, columns: string[]) {
  const defaults =
    section === "documents"
      ? DOCUMENT_DEFAULT_VISIBLE_KEYS
      : section === "bills"
        ? BILL_DEFAULT_VISIBLE_KEYS
        : PEOPLE_DEFAULT_VISIBLE_KEYS;
  const overrides = parseColumnCols(cols);
  const added: string[] = [];
  for (const key of columns) {
    if (!/^[a-z][a-z0-9_]*$/i.test(key) || defaults.has(key)) continue;
    overrides[key] = true;
    added.push(key);
  }
  return { cols: formatColumnCols(overrides), marker: added.join(",") };
}

export function applyRecortePatch(
  current: ProfileListSearch,
  recorte: AssistantRecorte,
): Partial<ProfileListSearch> {
  const section = sectionOf(recorte.table);
  const filters =
    section === "documents"
      ? pick(recorte.filters, DOCUMENT_FILTERS)
      : section === "bills"
        ? pick(recorte.filters, BILL_FILTERS)
        : pick(recorte.filters, PEOPLE_FILTERS);
  const { cols, marker } = extraColumns(
    section,
    current.section === section ? current.cols : "",
    recorte.columns,
  );
  const sort = sortPatch(section, recorte.sort, recorte.order);
  return {
    section,
    ...clearFilters(section),
    ...filters,
    ...sort,
    cols,
    recorte: marker || "on",
  };
}

function sortPatch(
  section: ProfileListSearch["section"],
  sort: string | undefined,
  order: AssistantRecorte["order"],
): Partial<ProfileListSearch> {
  if (!sort || (order !== "asc" && order !== "desc")) return {};
  if (section === "documents" && (DOCUMENT_SORT as readonly string[]).includes(sort)) {
    return { document_sort: sort as ProfileListSearch["document_sort"], document_order: order };
  }
  if (section === "bills" && (BILL_SORT as readonly string[]).includes(sort)) {
    return { bill_sort: sort as ProfileListSearch["bill_sort"], bill_order: order };
  }
  if (section === "profile" && (PEOPLE_SORT as readonly string[]).includes(sort)) {
    return { sort: sort as ProfileListSearch["sort"], order };
  }
  return {};
}

export function clearRecortePatch(search: ProfileListSearch): Partial<ProfileListSearch> {
  const overrides = parseColumnCols(search.cols);
  if (search.recorte && search.recorte !== "on") {
    for (const key of search.recorte.split(",")) delete overrides[key];
  }
  return { ...clearFilters(search.section), cols: formatColumnCols(overrides), recorte: "" };
}

export function recorteFilterParts(search: ProfileListSearch): { key: string; value: string }[] {
  const keys =
    search.section === "documents"
      ? DOCUMENT_FILTERS
      : search.section === "bills"
        ? BILL_FILTERS
        : PEOPLE_FILTERS;
  return keys.flatMap((key) => {
    const value = search[key];
    return value ? [{ key, value }] : [];
  });
}

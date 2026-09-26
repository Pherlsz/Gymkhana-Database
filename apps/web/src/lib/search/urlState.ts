import type { GlobalSearchState, SearchModule } from "./types";
import { SEARCH_MODULE_VALUES } from "./types";

export function normalizeGlobalSearch(search: Record<string, unknown>): GlobalSearchState {
  const requestedLimit = Number(search.limit);
  const limit = requestedLimit === 25 || requestedLimit === 100 ? requestedLimit : 50;
  const preview = typeof search.preview === "string" ? search.preview.trim() : "";
  return {
    q: typeof search.q === "string" ? search.q : "",
    modules: normalizeListParameter(search.modules, (value) =>
      SEARCH_MODULE_VALUES.includes(value as SearchModule),
    ),
    fields: normalizeListParameter(search.fields, (value) => value.length > 0),
    page: positiveInteger(search.page, 1),
    limit,
    sort: search.sort === "updated_at" ? "updated_at" : "relevance",
    order: search.order === "asc" ? "asc" : "desc",
    preview,
  };
}

export const GLOBAL_SEARCH_DEFAULTS = normalizeGlobalSearch({});

export function termsFromSearch(value: string): string[] {
  return value
    .split("\n")
    .map((term) => term.trim())
    .filter(Boolean);
}

export function parseList(value: string): string[] {
  return [
    ...new Set(
      value
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  ];
}

export function serializeList(values: readonly string[]): string {
  return [...new Set(values)].toSorted().join(",");
}

function normalizeListParameter(value: unknown, allowed: (value: string) => boolean): string {
  if (typeof value !== "string") return "";
  return serializeList(parseList(value).filter(allowed));
}

function positiveInteger(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.trunc(parsed) : fallback;
}

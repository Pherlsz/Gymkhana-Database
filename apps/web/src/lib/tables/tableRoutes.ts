import { stripSearchParams } from "@tanstack/react-router";
import type { ProfileListSearch } from "../api/client";
import { normalizeProfileSearch } from "../../ProfilePanel";
import { formatColumnCols, parseColumnCols } from "./columnVisibility";

export const TABLE_KINDS = ["people", "documents", "bills"] as const;
export type TableKind = (typeof TABLE_KINDS)[number];
export type TableSearch = Omit<ProfileListSearch, "section">;

export function isTableKind(value: string): value is TableKind {
  return (TABLE_KINDS as readonly string[]).includes(value);
}

export function sectionFromTable(kind: TableKind): ProfileListSearch["section"] {
  if (kind === "documents") return "documents";
  if (kind === "bills") return "bills";
  return "profile";
}

export function tableFromSection(section: ProfileListSearch["section"] | undefined): TableKind {
  if (section === "documents") return "documents";
  if (section === "bills") return "bills";
  return "people";
}

export function normalizeTableSearch(search: Record<string, unknown>): TableSearch {
  const { section: _section, ...rest } = normalizeProfileSearch(search);
  return { ...rest, cols: formatColumnCols(parseColumnCols(rest.cols)) };
}

export const TABLE_SEARCH_DEFAULTS = normalizeTableSearch({});

export function compactTableSearch(search: TableSearch): Partial<TableSearch> {
  const next: Partial<TableSearch> = {};
  for (const key of Object.keys(search) as (keyof TableSearch)[]) {
    const value = search[key];
    if (value === undefined || value === TABLE_SEARCH_DEFAULTS[key]) continue;
    (next as Record<string, unknown>)[key] = value;
  }
  return next;
}

export const tableSearchMiddlewares = [stripSearchParams(TABLE_SEARCH_DEFAULTS)] as const;

export function tableLinkProps(input: Record<string, unknown> | ProfileListSearch) {
  const normalized =
    "page" in input && "section" in input
      ? (input as ProfileListSearch)
      : normalizeProfileSearch(input);
  const { section, ...search } = normalized;
  return {
    to: "/tables/$table" as const,
    params: { table: tableFromSection(section) },
    search: compactTableSearch(search) as TableSearch,
  };
}

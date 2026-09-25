import { cellText, type TableRow } from "../tables/tableRows";

/** Matches the chat result page ceiling so one request loads the recorte for local sort/filter. */
export const RECORTE_GRID_LIMIT = 500;

export function recorteCellMatches(value: unknown, needle: string): boolean {
  const text = needle.trim().toLowerCase();
  if (!text) return true;
  return cellText(value).toLowerCase().includes(text);
}

export function filterRecorteRows(
  rows: TableRow[],
  filters: Record<string, string>,
  query: string,
): TableRow[] {
  const q = query.trim().toLowerCase();
  return rows.filter((row) => {
    for (const [key, value] of Object.entries(filters)) {
      if (value && !recorteCellMatches(row.cells[key], value)) return false;
    }
    if (!q) return true;
    return Object.entries(row.cells).some(([key, value]) => {
      if (key === "entityKind" || key === "entityId") return false;
      return recorteCellMatches(value, q);
    });
  });
}

export function compareRecorteCells(left: unknown, right: unknown): number {
  const a = cellText(left).trim();
  const b = cellText(right).trim();
  if (a === b) return 0;
  if (!a) return 1;
  if (!b) return -1;
  return a.localeCompare(b, "pt-BR", { numeric: true, sensitivity: "base" });
}

export function sortRecorteRows(
  rows: TableRow[],
  field: string,
  order: "asc" | "desc",
): TableRow[] {
  if (!field) return rows;
  const sign = order === "desc" ? -1 : 1;
  return [...rows].sort(
    (left, right) => sign * compareRecorteCells(left.cells[field], right.cells[field]),
  );
}

export function pageRecorteRows(rows: TableRow[], page: number, pageSize: number): TableRow[] {
  const size = Math.max(1, pageSize);
  const start = Math.max(0, (Math.max(1, page) - 1) * size);
  return rows.slice(start, start + size);
}

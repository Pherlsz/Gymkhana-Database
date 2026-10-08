import { useCallback, useEffect, useState } from "react";
import {
  formatColumnCols,
  parseColumnCols,
  type ColumnVisibilityOverrides,
} from "./columnVisibility";
import type { TableKind } from "./tableRoutes";

export const SHEET_COLUMN_VISIBILITY_PREFIX = "gymkhana.sheet.cols.v1.";

export function sheetColumnVisibilityKey(table: TableKind): string {
  return SHEET_COLUMN_VISIBILITY_PREFIX + table;
}

export function readSheetColumnVisibility(table: TableKind): ColumnVisibilityOverrides {
  try {
    return parseColumnCols(localStorage.getItem(sheetColumnVisibilityKey(table)) ?? "");
  } catch {
    return {};
  }
}

export function writeSheetColumnVisibility(
  table: TableKind,
  overrides: ColumnVisibilityOverrides,
) {
  const key = sheetColumnVisibilityKey(table);
  const token = formatColumnCols(parseColumnCols(formatColumnCols(overrides)));
  if (!token) localStorage.removeItem(key);
  else localStorage.setItem(key, token);
}


function sameOverrides(left: ColumnVisibilityOverrides, right: ColumnVisibilityOverrides) {
  const leftKeys = Object.keys(left);
  const rightKeys = Object.keys(right);
  if (leftKeys.length !== rightKeys.length) return false;
  return leftKeys.every((key) => left[key] === right[key]);
}

export function useSheetColumnVisibility(
  table: TableKind,
  urlCols: string,
  clearUrlCols: () => void,
) {
  const [overrides, setOverrides] = useState<ColumnVisibilityOverrides>(() =>
    urlCols.trim() ? parseColumnCols(urlCols) : readSheetColumnVisibility(table),
  );

  useEffect(() => {
    const next = urlCols.trim() ? parseColumnCols(urlCols) : readSheetColumnVisibility(table);
    if (urlCols.trim()) {
      writeSheetColumnVisibility(table, next);
      clearUrlCols();
    }
    setOverrides((current) => (sameOverrides(current, next) ? current : next));
  }, [clearUrlCols, table, urlCols]);

  const replace = useCallback(
    (next: ColumnVisibilityOverrides) => {
      writeSheetColumnVisibility(table, next);
      setOverrides(next);
    },
    [table],
  );

  return { overrides, replace };
}

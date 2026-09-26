import { SHEET_COLUMN_WIDTH, SHEET_MAX_PAGE_SIZE, SHEET_PAGE_SIZES } from "./sheetDefaults";
import { resolveSheetPreferences, type SheetPreferenceOverrides } from "./sheetPreferences";

export const SPREADSHEET_PAGE_SIZES = SHEET_PAGE_SIZES;
export const SPREADSHEET_MAX_PAGE_SIZE = SHEET_MAX_PAGE_SIZE;
export const SPREADSHEET_DEFAULT_COLUMN_WIDTH = SHEET_COLUMN_WIDTH.default;

export function clampSpreadsheetPageSize(
  value: number,
  overrides?: SheetPreferenceOverrides | null,
) {
  const prefs = resolveSheetPreferences(overrides);
  if (!Number.isFinite(value)) return prefs.defaultPageSize;
  return Math.min(prefs.maxPageSize, Math.max(prefs.minPageSize, Math.trunc(value)));
}

export function spreadsheetPageSizeOptions(
  current: number,
  overrides?: SheetPreferenceOverrides | null,
) {
  const prefs = resolveSheetPreferences(overrides);
  const sizes = new Set<number>(prefs.pageSizes);
  sizes.add(clampSpreadsheetPageSize(current, overrides));
  return [...sizes].sort((left, right) => left - right);
}

import {
  SHEET_COLUMN_WIDTH,
  SHEET_DEFAULT_PAGE_SIZE,
  SHEET_DISTINCT_VALUE_LIMIT,
  SHEET_FALLBACK_BODY_HEIGHT_PX,
  SHEET_HEADER_FALLBACK_HEIGHT_PX,
  SHEET_INSPECTOR_MAX_WIDTH_PX,
  SHEET_INSPECTOR_SHEET_SIZE,
  SHEET_MAX_PAGE_SIZE,
  SHEET_MIN_BODY_HEIGHT_PX,
  SHEET_MIN_PAGE_SIZE,
  SHEET_PAGE_SIZES,
  SHEET_ROW_HEIGHT_PX,
  SHEET_SEARCH_DEBOUNCE_MS,
  sheetInspectorMediaQuery,
} from "./sheetDefaults";

/**
 * Optional account overlay for sheet chrome. Persistent values belong on the
 * backend (`/settings`); this type is the merge surface until that API exists.
 */
export type SheetPreferenceOverrides = {
  pageSize?: number;
  pageSizes?: readonly number[];
  minPageSize?: number;
  maxPageSize?: number;
  columnWidth?: number;
  searchDebounceMs?: number;
  inspectorMaxWidthPx?: number;
  inspectorSheetSize?: string;
  minBodyHeightPx?: number;
  fallbackBodyHeightPx?: number;
  headerFallbackHeightPx?: number;
  rowHeightPx?: number;
  distinctValueLimit?: number;
};

export type ResolvedSheetPreferences = {
  pageSizes: readonly number[];
  defaultPageSize: number;
  minPageSize: number;
  maxPageSize: number;
  columnWidth: number;
  searchDebounceMs: number;
  inspectorMaxWidthPx: number;
  inspectorMediaQuery: string;
  inspectorSheetSize: string;
  minBodyHeightPx: number;
  fallbackBodyHeightPx: number;
  headerFallbackHeightPx: number;
  rowHeightPx: number;
  distinctValueLimit: number;
};

function clampInt(value: number, min: number, max: number, fallback: number) {
  if (!Number.isFinite(value)) return fallback;
  return Math.min(max, Math.max(min, Math.trunc(value)));
}

export function resolveSheetPreferences(
  overrides?: SheetPreferenceOverrides | null,
): ResolvedSheetPreferences {
  const minPageSize = overrides?.minPageSize ?? SHEET_MIN_PAGE_SIZE;
  const maxPageSize = overrides?.maxPageSize ?? SHEET_MAX_PAGE_SIZE;
  const inspectorMaxWidthPx = overrides?.inspectorMaxWidthPx ?? SHEET_INSPECTOR_MAX_WIDTH_PX;
  return {
    pageSizes: overrides?.pageSizes ?? SHEET_PAGE_SIZES,
    defaultPageSize: clampInt(
      overrides?.pageSize ?? SHEET_DEFAULT_PAGE_SIZE,
      minPageSize,
      maxPageSize,
      SHEET_DEFAULT_PAGE_SIZE,
    ),
    minPageSize,
    maxPageSize,
    columnWidth: overrides?.columnWidth ?? SHEET_COLUMN_WIDTH.default,
    searchDebounceMs: overrides?.searchDebounceMs ?? SHEET_SEARCH_DEBOUNCE_MS,
    inspectorMaxWidthPx,
    inspectorMediaQuery: sheetInspectorMediaQuery(inspectorMaxWidthPx),
    inspectorSheetSize: overrides?.inspectorSheetSize ?? SHEET_INSPECTOR_SHEET_SIZE,
    minBodyHeightPx: overrides?.minBodyHeightPx ?? SHEET_MIN_BODY_HEIGHT_PX,
    fallbackBodyHeightPx: overrides?.fallbackBodyHeightPx ?? SHEET_FALLBACK_BODY_HEIGHT_PX,
    headerFallbackHeightPx: overrides?.headerFallbackHeightPx ?? SHEET_HEADER_FALLBACK_HEIGHT_PX,
    rowHeightPx: overrides?.rowHeightPx ?? SHEET_ROW_HEIGHT_PX,
    distinctValueLimit: overrides?.distinctValueLimit ?? SHEET_DISTINCT_VALUE_LIMIT,
  };
}

export const DEFAULT_SHEET_PREFERENCES = resolveSheetPreferences();

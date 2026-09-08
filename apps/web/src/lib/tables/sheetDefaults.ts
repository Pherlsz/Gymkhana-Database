/** Product defaults for gymkhana sheets. Account overlays go through `resolveSheetPreferences`. */

import { BREAKPOINT } from "../breakpoints";

export const SHEET_PAGE_SIZES = [50, 100, 250, 500] as const;
export const SHEET_MIN_PAGE_SIZE = 50;
export const SHEET_MAX_PAGE_SIZE = 500;
export const SHEET_DEFAULT_PAGE_SIZE = 100;

export const SHEET_COLUMN_WIDTH = {
  default: 140,
  identity: 220,
  documents: 240,
} as const;

export const SHEET_SEARCH_DEBOUNCE_MS = 450;
/** Derived from the breakpoint ladder so CSS and JS cannot disagree. */
export const SHEET_INSPECTOR_MAX_WIDTH_PX = BREAKPOINT.md - 1;
export const SHEET_INSPECTOR_SHEET_SIZE = "min(60dvh, 36rem)";
export const SHEET_MIN_BODY_HEIGHT_PX = 160;
export const SHEET_FALLBACK_BODY_HEIGHT_PX = 480;
export const SHEET_HEADER_FALLBACK_HEIGHT_PX = 40;
export const SHEET_ROW_HEIGHT_PX = 44;
export const SHEET_DISTINCT_VALUE_LIMIT = 48;

export function sheetInspectorMediaQuery(maxWidthPx = SHEET_INSPECTOR_MAX_WIDTH_PX) {
  return `(max-width: ${maxWidthPx}px)`;
}

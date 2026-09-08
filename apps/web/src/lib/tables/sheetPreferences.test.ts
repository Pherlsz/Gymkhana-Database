import { describe, expect, it } from "vitest";
import {
  SHEET_DEFAULT_PAGE_SIZE,
  SHEET_INSPECTOR_MAX_WIDTH_PX,
  SHEET_PAGE_SIZES,
  SHEET_SEARCH_DEBOUNCE_MS,
  sheetInspectorMediaQuery,
} from "./sheetDefaults";
import { DEFAULT_SHEET_PREFERENCES, resolveSheetPreferences } from "./sheetPreferences";

describe("resolveSheetPreferences", () => {
  it("returns product defaults when no overlay is given", () => {
    expect(resolveSheetPreferences()).toEqual(DEFAULT_SHEET_PREFERENCES);
    expect(DEFAULT_SHEET_PREFERENCES.pageSizes).toEqual(SHEET_PAGE_SIZES);
    expect(DEFAULT_SHEET_PREFERENCES.defaultPageSize).toBe(SHEET_DEFAULT_PAGE_SIZE);
    expect(DEFAULT_SHEET_PREFERENCES.searchDebounceMs).toBe(SHEET_SEARCH_DEBOUNCE_MS);
    expect(DEFAULT_SHEET_PREFERENCES.inspectorMediaQuery).toBe(
      sheetInspectorMediaQuery(SHEET_INSPECTOR_MAX_WIDTH_PX),
    );
  });

  it("overlays account values and clamps page size to the allowed window", () => {
    const prefs = resolveSheetPreferences({
      pageSize: 2000,
      searchDebounceMs: 200,
      inspectorMaxWidthPx: 640,
      columnWidth: 160,
    });
    expect(prefs.defaultPageSize).toBe(prefs.maxPageSize);
    expect(prefs.searchDebounceMs).toBe(200);
    expect(prefs.columnWidth).toBe(160);
    expect(prefs.inspectorMediaQuery).toBe(sheetInspectorMediaQuery(640));
  });
});

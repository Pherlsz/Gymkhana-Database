import { describe, expect, it } from "vitest";
import {
  columnHasResizeHandle,
  dragSheetDivider,
  fillSheetColumnWidths,
  SHEET_DATA_COLUMN_MIN_WIDTH,
} from "./sheetColumnLayout";

describe("fillSheetColumnWidths", () => {
  it("gives a single narrow column the rest of the sheet", () => {
    expect(fillSheetColumnWidths([220], 800)).toEqual([800]);
  });

  it("keeps the actions column fixed and widens the only data column", () => {
    expect(fillSheetColumnWidths([48, 220], 800)).toEqual([48, 752]);
  });

  it("leaves earlier columns untouched when a small set does not fill the sheet", () => {
    expect(fillSheetColumnWidths([48, 220, 140], 800)).toEqual([48, 220, 532]);
  });

  it("does not shrink or redistribute columns that already overflow", () => {
    expect(fillSheetColumnWidths([48, 500, 400], 800)).toEqual([48, 500, 400]);
  });

  it("returns preferred widths until the viewport is known", () => {
    expect(fillSheetColumnWidths([220], 0)).toEqual([220]);
    expect(fillSheetColumnWidths([], 800)).toEqual([]);
  });

  it("does not fill past the visible sheet", () => {
    const fitted = fillSheetColumnWidths([48, 220, 140], 800);
    expect(fitted.reduce((sum, width) => sum + width, 0)).toBe(800);
    expect(fillSheetColumnWidths([220], 800.9)).toEqual([800]);
  });
});

describe("dragSheetDivider", () => {
  it("gives Nome growth to CPF without growing the sheet", () => {
    const start = [220, 580];
    const next = dragSheetDivider(start, 0, 400);
    expect(next).toEqual([400, 400]);
    expect(next.reduce((sum, width) => sum + width, 0)).toBe(800);
  });

  it("stops before the next data column disappears", () => {
    const start = [220, 580];
    const next = dragSheetDivider(start, 0, 5000);
    expect(next[1]).toBe(SHEET_DATA_COLUMN_MIN_WIDTH);
    expect(next[0]).toBe(800 - SHEET_DATA_COLUMN_MIN_WIDTH);
    expect(next.reduce((sum, width) => sum + width, 0)).toBe(800);
  });

  it("keeps the dragged column at the same minimum", () => {
    const next = dragSheetDivider([220, 580], 0, 10);
    expect(next[0]).toBe(SHEET_DATA_COLUMN_MIN_WIDTH);
    expect(next[1]).toBe(800 - SHEET_DATA_COLUMN_MIN_WIDTH);
  });
});

describe("columnHasResizeHandle", () => {
  it("has no handle when a single column edges are both outer", () => {
    expect(columnHasResizeHandle(0, 1)).toBe(false);
  });

  it("only keeps the divider between two columns", () => {
    expect(columnHasResizeHandle(0, 2)).toBe(true);
    expect(columnHasResizeHandle(1, 2)).toBe(false);
  });

  it("drops the right edge and keeps every internal divider", () => {
    expect([0, 1, 2].map((index) => columnHasResizeHandle(index, 3))).toEqual([
      true,
      true,
      false,
    ]);
  });
});

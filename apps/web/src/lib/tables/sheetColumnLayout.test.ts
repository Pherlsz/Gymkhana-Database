import { describe, expect, it } from "vitest";
import { columnHasResizeHandle, fillSheetColumnWidths } from "./sheetColumnLayout";

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
});

describe("columnHasResizeHandle", () => {
  it("has no handle when a single column's edges are both outer", () => {
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

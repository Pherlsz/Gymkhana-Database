import { describe, expect, it } from "vitest";
import { fillSheetColumnWidths } from "./sheetColumnLayout";

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

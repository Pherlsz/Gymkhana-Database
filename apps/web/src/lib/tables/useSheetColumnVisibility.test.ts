import { describe, expect, it } from "vitest";
import {
  readSheetColumnVisibility,
  sheetColumnVisibilityKey,
  writeSheetColumnVisibility,
} from "./useSheetColumnVisibility";

describe("sheet column visibility storage", () => {
  it("keeps a separate token per table and drops a locked hide", () => {
    expect(sheetColumnVisibilityKey("people")).toBe("gymkhana.sheet.cols.v1.people");
    expect(sheetColumnVisibilityKey("documents")).toBe("gymkhana.sheet.cols.v1.documents");
    expect(sheetColumnVisibilityKey("bills")).toBe("gymkhana.sheet.cols.v1.bills");
    writeSheetColumnVisibility("people", { city: false, full_name: false });
    expect(localStorage.getItem("gymkhana.sheet.cols.v1.people")).toBe("-city");
    expect(readSheetColumnVisibility("people")).toEqual({ city: false });
    expect(localStorage.getItem("gymkhana.sheet.cols.v1.documents")).toBeNull();
    writeSheetColumnVisibility("documents", {});
    expect(localStorage.getItem("gymkhana.sheet.cols.v1.documents")).toBeNull();
  });
});

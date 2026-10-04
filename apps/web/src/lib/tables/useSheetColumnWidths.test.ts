import { describe, expect, it } from "vitest";
import { clampColumnWidth } from "./useSheetColumnWidths";

describe("clampColumnWidth", () => {
  it("follows a drag that starts from a filled column wider than 640", () => {
    expect(clampColumnWidth(1100 + 40)).toBe(1140);
    expect(clampColumnWidth(1100 - 80)).toBe(1020);
    expect(clampColumnWidth(20)).toBe(72);
  });
});

import { describe, expect, it } from "vitest";
import { SEARCH_RESULT_CAP, searchResultWindow } from "./searchWindow";

describe("searchResultWindow", () => {
  it("keeps a small result set", () => {
    expect(searchResultWindow(12, 50)).toEqual({ shown: 12, truncated: false });
  });

  it("warns when the API window cannot show every hit", () => {
    const windowed = searchResultWindow(SEARCH_RESULT_CAP + 25, 50);
    expect(windowed.truncated).toBe(true);
    expect(windowed.shown).toBeLessThanOrEqual(SEARCH_RESULT_CAP + 50);
    expect(windowed.shown).toBeGreaterThan(0);
  });
});

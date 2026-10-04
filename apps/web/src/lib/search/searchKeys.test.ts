import { describe, expect, it } from "vitest";
import { searchResultKey } from "./searchKeys";

describe("searchResultKey", () => {
  it("moves through results and opens the active one", () => {
    expect(searchResultKey("ArrowDown", 0, 3, false)).toBe(1);
    expect(searchResultKey("ArrowDown", 2, 3, false)).toBe(2);
    expect(searchResultKey("ArrowUp", 0, 3, false)).toBe(0);
    expect(searchResultKey("Enter", 1, 3, false)).toBe("toggle");
  });

  it("focuses search on slash only when the user is not typing", () => {
    expect(searchResultKey("/", 0, 3, false)).toBe("focus");
    expect(searchResultKey("/", 0, 3, true)).toBe(null);
    expect(searchResultKey("ArrowDown", 0, 3, true)).toBe(null);
  });
});

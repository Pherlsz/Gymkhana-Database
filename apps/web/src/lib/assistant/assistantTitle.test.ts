import { describe, expect, it } from "vitest";
import { DEFAULT_THREAD_TITLE, THREAD_TITLE_RUNES, threadTitle } from "./assistantTitle";

describe("threadTitle", () => {
  it("uses the first non-empty line", () => {
    expect(threadTitle("\n  Pessoas com inicial O  \nresto")).toBe("Pessoas com inicial O");
  });

  it("clips to 50 runes with an ellipsis", () => {
    const title = threadTitle("A".repeat(80));
    expect(Array.from(title)).toHaveLength(THREAD_TITLE_RUNES);
    expect(title.endsWith("…")).toBe(true);
    expect(title).toBe(`${"A".repeat(THREAD_TITLE_RUNES - 1)}…`);
  });

  it("keeps blank input as the default session name", () => {
    expect(threadTitle("   \n")).toBe(DEFAULT_THREAD_TITLE);
  });
});

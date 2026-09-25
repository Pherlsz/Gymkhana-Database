import { describe, expect, it } from "vitest";
import { cellMatches, predicateActive } from "./columnPredicate";

describe("columnPredicate", () => {
  it("treats em dash as empty", () => {
    expect(cellMatches("—", { op: "is_null", values: [] })).toBe(true);
    expect(cellMatches("Ana", { op: "not_null", values: [] })).toBe(true);
  });

  it("matches contains and between", () => {
    expect(cellMatches("Ana da Silva", { op: "contains", values: ["ana"] })).toBe(true);
    expect(cellMatches("120", { op: "between", values: ["10", "200"] })).toBe(true);
    expect(cellMatches("210", { op: "between", values: ["10", "200"] })).toBe(false);
  });

  it("ignores inactive predicates", () => {
    expect(predicateActive({ op: "contains", values: [""] })).toBe(false);
    expect(cellMatches("x", { op: "contains", values: [""] })).toBe(true);
  });
});

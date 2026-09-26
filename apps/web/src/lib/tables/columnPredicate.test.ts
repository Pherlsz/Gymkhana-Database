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

  it("matches contains, starts_with, eq with formatted document numbers (CPF/Phone)", () => {
    const formattedCpf1 = "100.051.299-15";
    const formattedCpf2 = "092.348.100-15";

    // starts_with: 100
    expect(cellMatches(formattedCpf1, { op: "starts_with", values: ["100"] })).toBe(true);
    expect(cellMatches(formattedCpf2, { op: "starts_with", values: ["100"] })).toBe(false);

    // starts_with with raw digits without dot: 100051
    expect(cellMatches(formattedCpf1, { op: "starts_with", values: ["100051"] })).toBe(true);
    expect(cellMatches(formattedCpf2, { op: "starts_with", values: ["100051"] })).toBe(false);

    // contains: 100
    expect(cellMatches(formattedCpf1, { op: "contains", values: ["100"] })).toBe(true);
    expect(cellMatches(formattedCpf2, { op: "contains", values: ["100"] })).toBe(true);

    // eq with unmasked digits
    expect(cellMatches(formattedCpf1, { op: "eq", values: ["10005129915"] })).toBe(true);
    expect(cellMatches(formattedCpf2, { op: "eq", values: ["10005129915"] })).toBe(false);

    // neq with unmasked digits
    expect(cellMatches(formattedCpf1, { op: "neq", values: ["10005129915"] })).toBe(false);
    expect(cellMatches(formattedCpf2, { op: "neq", values: ["10005129915"] })).toBe(true);
  });

  it("ignores inactive predicates", () => {
    expect(predicateActive({ op: "contains", values: [""] })).toBe(false);
    expect(cellMatches("x", { op: "contains", values: [""] })).toBe(true);
  });
});

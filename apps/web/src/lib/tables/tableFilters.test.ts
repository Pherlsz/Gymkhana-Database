import { describe, expect, it, vi } from "vitest";
import {
  brazilStateOptions,
  distinctValues,
  localDateMatches,
  selectFilter,
  textFilter,
} from "./tableFilters";
import type { TableRow } from "./tableRows";

const rows: TableRow[] = [
  { id: "1", cells: { digit_sum_cpf: 55, full_name: "Ana", state: "RS", amount: "10" } },
  { id: "2", cells: { digit_sum_cpf: 11, full_name: "Bruno", state: "SP", amount: "20" } },
];

describe("tableFilters helpers", () => {
  it("lists distinct values from the loaded page", () => {
    expect(distinctValues(rows, "state")).toEqual(["RS", "SP"]);
    expect(distinctValues(rows, "digit_sum_cpf")).toEqual(["11", "55"]);
  });

  it("builds toolbar text and select filters", () => {
    const text = textFilter("full_name", "Nome", "", vi.fn());
    expect(text.kind).toBe("text");
    const select = selectFilter("state", "UF", "", brazilStateOptions(), "Todos", vi.fn());
    expect(select.kind).toBe("select");
    if (select.kind !== "select") return;
    expect(select.options.some((option) => option.value === "RS")).toBe(true);
  });
});

describe("localDateMatches", () => {
  it("matches an ISO needle against a pt-BR cell", () => {
    expect(localDateMatches("08/04/2009", "2009-04-08")).toBe(true);
    expect(localDateMatches("08/04/2009", "2009-04-09")).toBe(false);
  });

  it("matches a pt-BR needle against a pt-BR cell", () => {
    expect(localDateMatches("08/04/2009", "08/04/2009")).toBe(true);
  });

  it("matches an ISO needle against an ISO cell", () => {
    expect(localDateMatches("2009-04-08", "2009-04-08")).toBe(true);
  });

  it("falls back to substring match when neither side parses as a date", () => {
    expect(localDateMatches("—", "2009-04-08")).toBe(false);
    expect(localDateMatches("some text", "text")).toBe(true);
  });
});

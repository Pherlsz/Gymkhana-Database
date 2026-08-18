import { describe, expect, it, vi } from "vitest";
import { brazilStateOptions, distinctValues, selectFilter, textFilter } from "./tableFilters";
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

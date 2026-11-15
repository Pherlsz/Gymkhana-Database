import { describe, expect, it } from "vitest";
import type { ToolbarFilterField } from "./FilterControl";
import { activeFilterChips } from "./sheetFilters";

const noop = () => undefined;

describe("activeFilterChips", () => {
  const status: ToolbarFilterField = {
    key: "status",
    label: "Status",
    kind: "select",
    value: "IN_USE",
    options: [
      { value: "AVAILABLE", label: "Disponível" },
      { value: "IN_USE", label: "Em uso" },
    ],
    allLabel: "Todos",
    onChange: noop,
  };

  it("shows the option label and keeps server filters unmarked", () => {
    const [chip] = activeFilterChips([status]);
    expect(chip).toMatchObject({ field: "Status", value: "Em uso", local: false });
  });

  it("marks filters that only narrow the loaded page and names the operator", () => {
    const [chip] = activeFilterChips(
      [
        {
          key: "full_name",
          label: "Nome",
          kind: "text",
          value: "",
          predicate: { op: "neq", values: ["Ana"] },
          onChange: noop,
        },
      ],
      { opLabels: { neq: "≠", contains: "contém" } },
    );
    expect(chip).toMatchObject({ value: "≠ Ana", local: true });
  });

  it("does not mark local scope inside an assistant result", () => {
    const [chip] = activeFilterChips(
      [
        {
          key: "nome",
          label: "Nome",
          kind: "text",
          value: "",
          local: true,
          predicate: { op: "contains", values: ["a"] },
          onChange: noop,
        },
      ],
      { markLocal: false },
    );
    expect(chip?.local).toBe(false);
  });
});

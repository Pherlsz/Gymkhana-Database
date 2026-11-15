import { describe, expect, it, vi } from "vitest";
import {
  bindPredicates,
  clientPredicate,
  decodeFieldValue,
  encodePredicateValue,
  serverAppliesPredicate,
} from "./bindPredicates";
import type { ToolbarFilterField } from "./FilterControl";

describe("bindPredicates", () => {
  it("encodes and decodes prefixes for text operators", () => {
    expect(encodePredicateValue({ op: "starts_with", values: ["100"] }, "text")).toBe("^100");
    expect(encodePredicateValue({ op: "eq", values: ["100"] }, "text")).toBe("=100");
    expect(encodePredicateValue({ op: "contains", values: ["100"] }, "text")).toBe("100");

    expect(decodeFieldValue("^100", "contains")).toEqual({ op: "starts_with", values: ["100"] });
    expect(decodeFieldValue("=100", "contains")).toEqual({ op: "eq", values: ["100"] });
    expect(decodeFieldValue("100", "contains")).toEqual({ op: "contains", values: ["100"] });
    expect(decodeFieldValue("", "contains")).toEqual({ op: "contains", values: [] });
  });

  it("passes search query to field.onChange for contains, starts_with, and eq", () => {
    let extraState: Record<string, any> = {};
    const setExtra = (updater: any) => {
      extraState = typeof updater === "function" ? updater(extraState) : updater;
    };

    const onChangeSpy = vi.fn();
    const fields: ToolbarFilterField[] = [
      {
        key: "cpf",
        label: "CPF",
        kind: "text",
        value: "",
        onChange: onChangeSpy,
      },
    ];

    const bound = bindPredicates(fields, extraState, setExtra);
    const cpfField = bound[0]!;

    // Case 1: starts_with '100'
    cpfField.onPredicate?.({ op: "starts_with", values: ["100"] });
    expect(extraState.cpf).toEqual({ op: "starts_with", values: ["100"] });
    expect(onChangeSpy).toHaveBeenLastCalledWith("^100");

    // Case 2: contains '100'
    cpfField.onPredicate?.({ op: "contains", values: ["100"] });
    expect(extraState.cpf).toEqual({ op: "contains", values: ["100"] });
    expect(onChangeSpy).toHaveBeenLastCalledWith("100");

    // Case 3: eq '100'
    cpfField.onPredicate?.({ op: "eq", values: ["100"] });
    expect(extraState.cpf).toEqual({ op: "eq", values: ["100"] });
    expect(onChangeSpy).toHaveBeenLastCalledWith("=100");

    // Case 4: is_null
    cpfField.onPredicate?.({ op: "is_null", values: [] });
    expect(extraState.cpf).toEqual({ op: "is_null", values: [] });
    expect(onChangeSpy).toHaveBeenLastCalledWith("");

    // Case 5: cleared / empty
    cpfField.onPredicate?.({ op: "contains", values: [""] });
    expect(extraState.cpf).toBeUndefined();
    expect(onChangeSpy).toHaveBeenLastCalledWith("");
  });

  it("keeps operators the list API cannot apply out of the server param", () => {
    let extraState: Record<string, any> = {};
    const setExtra = (updater: any) => {
      extraState = updater(extraState);
    };
    const onChange = vi.fn();
    const [field] = bindPredicates(
      [{ key: "full_name", label: "Nome", kind: "text", value: "", onChange }],
      extraState,
      setExtra,
    );

    field!.onPredicate?.({ op: "neq", values: ["Ana"] });
    expect(extraState.full_name).toEqual({ op: "neq", values: ["Ana"] });
    expect(onChange).toHaveBeenLastCalledWith("");

    const [local] = bindPredicates(
      [{ key: "notes", label: "Notas", kind: "text", value: "", local: true, onChange }],
      {},
      setExtra,
    );
    local!.onPredicate?.({ op: "neq", values: ["x"] });
    expect(onChange).toHaveBeenLastCalledWith("x");
  });
});

describe("serverAppliesPredicate", () => {
  const text: ToolbarFilterField = {
    key: "cpf",
    label: "CPF",
    kind: "text",
    value: "",
    onChange: () => undefined,
  };
  const select: ToolbarFilterField = {
    key: "state",
    label: "UF",
    kind: "select",
    value: "",
    options: [{ value: "RS", label: "RS" }],
    allLabel: "Todos",
    onChange: () => undefined,
  };

  it("lets the server apply contains, starts with and equals on text params", () => {
    for (const op of ["contains", "starts_with", "eq"] as const) {
      expect(serverAppliesPredicate(text, { op, values: ["1"] })).toBe(true);
    }
    for (const op of ["neq", "gt", "between", "in", "is_null", "not_null"] as const) {
      expect(serverAppliesPredicate(text, { op, values: ["1", "2"] })).toBe(false);
    }
  });

  it("lets the server apply only an exact value on select params", () => {
    expect(serverAppliesPredicate(select, { op: "eq", values: ["RS"] })).toBe(true);
    expect(serverAppliesPredicate(select, { op: "in", values: ["RS", "SC"] })).toBe(false);
    expect(serverAppliesPredicate(select, { op: "contains", values: ["R"] })).toBe(false);
  });

  it("never treats page-only fields as server filtered", () => {
    expect(
      serverAppliesPredicate({ ...text, local: true }, { op: "contains", values: ["1"] }),
    ).toBe(false);
  });
});

describe("clientPredicate", () => {
  it("compares select values by the label the cell shows", () => {
    const field: ToolbarFilterField = {
      key: "custom:doador",
      label: "Doador",
      kind: "select",
      value: "true",
      local: true,
      options: [
        { value: "true", label: "Sim" },
        { value: "false", label: "Não" },
      ],
      allLabel: "Todos",
      onChange: () => undefined,
    };
    expect(clientPredicate(field)).toEqual({ op: "eq", values: ["Sim"] });
  });
});

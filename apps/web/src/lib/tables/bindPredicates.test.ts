import { describe, expect, it, vi } from "vitest";
import {
  bindPredicates,
  decodeFieldValue,
  encodePredicateValue,
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
});


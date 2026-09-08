import { describe, expect, it } from "vitest";
import { ptBRV1 } from "../../i18n/v1/pt-BR";
import { groupedTypeFilterOptions } from "./typeFilterOptions";

describe("groupedTypeFilterOptions", () => {
  it("groups real document types and skips empty canonical placeholders", () => {
    const groups = groupedTypeFilterOptions(
      [
        { id: "type-rg", label: "Registro Geral", technicalKey: "rg" },
        { id: "type-cnh", label: "CNH", technicalKey: "cnh" },
      ],
      "document",
      ptBRV1.home.tables,
    );
    const identity = groups.find((group) => group.key === "identity");
    expect(identity?.label).toBe("Documentos civis");
    expect(identity?.options.map((option) => option.value)).toEqual(["type-rg", "type-cnh"]);
    expect(groups.every((group) => group.options.length > 0)).toBe(true);
  });
});

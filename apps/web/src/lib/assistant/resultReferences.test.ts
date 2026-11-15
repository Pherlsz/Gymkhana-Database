import { describe, expect, it } from "vitest";
import { gridReferences, preferredGridReference, referenceOptionLabel } from "./resultReferences";

const copy = {
  entities: { profiles: "Pessoas", documents: "Documentos", bills: "Contas" },
  rowsOne: "{count} linha",
  rowsOther: "{count} linhas",
};

describe("resultReferences", () => {
  it("prefers the last reference the server offers for the grid", () => {
    const message = {
      result_references: [
        { id: "a", kind: "SEARCH" as const, label: "Busca: Gravataí", row_count: 34 },
        { id: "b", kind: "SEARCH" as const, label: "Busca: primo", row_count: 27 },
      ],
    };
    expect(preferredGridReference(message)).toBe("b");
    expect(preferredGridReference({ result_references: [] })).toBe("");
    expect(gridReferences({ result_references: [] })).toEqual([]);
  });

  it("names query roots in Portuguese and counts rows", () => {
    expect(
      referenceOptionLabel(
        { id: "a", kind: "QUERY", label: "Consulta: profiles", row_count: 1 },
        copy,
      ),
    ).toBe("Consulta: Pessoas · 1 linha");
    expect(
      referenceOptionLabel({ id: "b", kind: "SEARCH", label: "Busca: primo", row_count: 27 }, copy),
    ).toBe("Busca: primo · 27 linhas");
  });
});

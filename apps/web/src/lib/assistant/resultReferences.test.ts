import { describe, expect, it } from "vitest";
import { gridReferences, preferredGridReference, referenceOptionLabel } from "./resultReferences";

const copy = {
  entities: { profiles: "Pessoas", documents: "Documentos", bills: "Contas" },
  rowsOne: "{count} linha",
  rowsOther: "{count} linhas",
  rowsMore: "{count}+ linhas",
};

describe("resultReferences", () => {
  it("prefers the last reference the server offers for the grid", () => {
    const message = {
      result_references: [
        {
          id: "a",
          kind: "SEARCH" as const,
          label: "Busca: Gravataí",
          row_count: 34,
          more_rows: false,
        },
        {
          id: "b",
          kind: "SEARCH" as const,
          label: "Busca: primo",
          row_count: 27,
          more_rows: false,
        },
      ],
    };
    expect(preferredGridReference(message)).toBe("b");
    expect(preferredGridReference({ result_references: [] })).toBe("");
    expect(gridReferences({ result_references: [] })).toEqual([]);
  });

  it("names query roots in Portuguese and counts rows", () => {
    expect(
      referenceOptionLabel(
        { id: "a", kind: "QUERY", label: "Consulta: profiles", row_count: 1, more_rows: false },
        copy,
      ),
    ).toBe("Consulta: Pessoas · 1 linha");
    expect(
      referenceOptionLabel(
        { id: "b", kind: "SEARCH", label: "Busca: primo", row_count: 27, more_rows: false },
        copy,
      ),
    ).toBe("Busca: primo · 27 linhas");
    expect(
      referenceOptionLabel(
        { id: "c", kind: "SEARCH", label: "Busca: Gravataí", row_count: 50, more_rows: true },
        copy,
      ),
    ).toBe("Busca: Gravataí · 50+ linhas");
  });
});

import { describe, expect, it } from "vitest";
import { highlightNeedles, highlightParts } from "./highlightTerm";

describe("highlightParts", () => {
  it("marks the matched term without treating the rest as markup", () => {
    expect(highlightParts("Pedro <b>Lopes</b>", "Pedro")).toEqual([
      { text: "Pedro", match: true },
      { text: " <b>Lopes</b>", match: false },
    ]);
  });

  it("matches ignoring case and keeps the original letters", () => {
    expect(highlightParts("Notas de pedro", "PEDRO")).toEqual([
      { text: "Notas de ", match: false },
      { text: "pedro", match: true },
    ]);
  });

  it("uses the value of a field operator", () => {
    expect(highlightNeedles("tipo:rg Pedro")).toEqual(["rg", "Pedro"]);
    expect(highlightParts("RG 123", "tipo:rg")).toEqual([
      { text: "RG", match: true },
      { text: " 123", match: false },
    ]);
  });
});

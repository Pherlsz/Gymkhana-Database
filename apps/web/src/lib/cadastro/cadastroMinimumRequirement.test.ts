import { describe, expect, it } from "vitest";
import { OFFICIAL_DOCUMENT_TYPE_KEYS } from "./cadastroMinimumRequirement";

describe("cadastroMinimumRequirement", () => {
  it("keeps the official type keys stable (cpf/rg/cnh/birth certificate)", () => {
    expect(OFFICIAL_DOCUMENT_TYPE_KEYS).toEqual(["cpf", "rg", "cnh", "birth_certificate"]);
  });
});

import { describe, expect, it } from "vitest";
import { documentTypeAcronym } from "./documentBadges";

describe("documentTypeAcronym", () => {
  it("uses compact type labels and known keys", () => {
    expect(documentTypeAcronym("rg", "RG")).toBe("RG");
    expect(documentTypeAcronym("cpf", "Cadastro de Pessoa Física")).toBe("CPF");
    expect(documentTypeAcronym("voter_id", "Título de Eleitor")).toBe("TE");
    expect(documentTypeAcronym("birth_certificate", "Certidão de Nascimento")).toBe("CN");
    expect(documentTypeAcronym("custom_pass", "ABC")).toBe("ABC");
  });
});

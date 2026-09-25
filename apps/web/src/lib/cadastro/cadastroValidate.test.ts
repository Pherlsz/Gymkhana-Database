import { describe, expect, it } from "vitest";
import {
  canonicalBillAmount,
  cpfDigits,
  isCompetenceMonth,
  isCompleteCpf,
  matchesValidationRegex,
  phoneInputDigits,
} from "./cadastroValidate";

describe("cadastroValidate", () => {
  it("keeps the national number when a stored +55 phone is edited", () => {
    expect(phoneInputDigits("+55 51 99860-6105")).toBe("51998606105");
    expect(phoneInputDigits("51998606105")).toBe("51998606105");
  });

  it("extracts CPF digits and recognizes a complete number", () => {
    expect(cpfDigits("123.456.789-00")).toBe("12345678900");
    expect(isCompleteCpf("123.456.789-00")).toBe(true);
    expect(isCompleteCpf("123.456")).toBe(false);
  });

  it("accepts identifier against validation_regex, including digit-only fallback", () => {
    expect(matchesValidationRegex("12.345.678-9", "^[0-9]{9,11}$")).toBe(true);
    expect(matchesValidationRegex("ABC-12", "^[A-Z0-9-]+$")).toBe(true);
    expect(matchesValidationRegex("???", "^[0-9]+$")).toBe(false);
    expect(matchesValidationRegex("x", "")).toBe(true);
  });

  it("canonicalizes Brazilian bill amounts and competence months", () => {
    expect(canonicalBillAmount("R$ 1.234,50")).toBe("1234.50");
    expect(canonicalBillAmount("142,50")).toBe("142.50");
    expect(canonicalBillAmount("99.5")).toBe("99.5");
    expect(isCompetenceMonth("2026-09")).toBe(true);
    expect(isCompetenceMonth("2026-13")).toBe(false);
    expect(isCompetenceMonth("set/2026")).toBe(false);
  });
});

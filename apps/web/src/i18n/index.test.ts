import { describe, expect, it } from "vitest";
import { DEFAULT_LOCALE, I18N_CATALOG_VERSION, resolveAppLocale } from "./index";

describe("i18n", () => {
  it("keeps the initial catalog contract explicitly versioned", () => {
    expect(I18N_CATALOG_VERSION).toBe(1);
    expect(DEFAULT_LOCALE).toBe("pt-BR");
  });

  it("resolves Portuguese and safely falls back to the default locale", () => {
    expect(resolveAppLocale("pt-BR")).toBe("pt-BR");
    expect(resolveAppLocale("pt")).toBe("pt-BR");
    expect(resolveAppLocale("en-US")).toBe("pt-BR");
  });
});

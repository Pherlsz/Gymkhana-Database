import { describe, expect, it } from "vitest";
import {
  cadastroHref,
  cadastroReviewHref,
  cadastroWorkMode,
  googleFormsReturnPath,
  isCadastroMode,
  moduleFromTable,
  normalizeCadastroPageSearch,
} from "./cadastroSearch";

describe("cadastroSearch", () => {
  it("normalizes table, mode and import id", () => {
    expect(normalizeCadastroPageSearch({})).toEqual({ table: "people" });
    expect(
      normalizeCadastroPageSearch({ table: "bills", mode: "xlsx", import: "not-a-uuid" }),
    ).toEqual({
      table: "bills",
      mode: "xlsx",
    });
    expect(
      normalizeCadastroPageSearch({
        table: "documents",
        mode: "manual",
        import: "019bf789-4400-7f12-9abc-123456789abc",
      }),
    ).toEqual({
      table: "documents",
      mode: "manual",
      import: "019bf789-4400-7f12-9abc-123456789abc",
    });
  });

  it("maps table to operations module", () => {
    expect(moduleFromTable("people")).toBe("PROFILES");
    expect(moduleFromTable("documents")).toBe("DOCUMENTS");
    expect(cadastroReviewHref("PROFILES", "019bf789-4400-7f12-9abc-123456789abc")).toBe(
      "/cadastro?table=people&mode=xlsx&import=019bf789-4400-7f12-9abc-123456789abc",
    );
  });

  it("drops Google Forms entry mode while the product entry is parked", () => {
    expect(
      normalizeCadastroPageSearch({
        mode: "forms",
        tab: "history",
        source: "019bf789-4400-7f12-9abc-123456789abc",
        google_forms: "connected",
      }),
    ).toEqual({
      table: "people",
    });
    expect(normalizeCadastroPageSearch({ mode: "xlsx", tab: "history" })).toEqual({
      table: "people",
      mode: "xlsx",
    });
    expect(
      cadastroHref({
        table: "people",
        mode: "forms",
        tab: "history",
      }),
    ).toBe("/cadastro?mode=forms&tab=history");
    expect(googleFormsReturnPath("https://app.test/forms?tab=history")).toBe(
      "/cadastro?mode=forms&tab=history",
    );
  });

  it("opens people, documents and bills as the form", () => {
    expect(cadastroWorkMode({ table: "people" })).toBe("manual");
    expect(cadastroWorkMode({ table: "documents" })).toBe("manual");
    expect(
      cadastroWorkMode({
        table: "documents",
        type: "019bf789-4400-7f12-9abc-123456789abe",
      }),
    ).toBe("manual");
    expect(cadastroWorkMode({ table: "people", mode: "xlsx" })).toBe("xlsx");
  });

  it("guards cadastro modes", () => {
    expect(isCadastroMode("xlsx")).toBe(true);
    expect(isCadastroMode("ocr")).toBe(false);
    expect(isCadastroMode("pick")).toBe(false);
    expect(isCadastroMode("bulk")).toBe(false);
  });

  it("ignores the retired OCR mode and keeps document type on the form URL", () => {
    expect(
      normalizeCadastroPageSearch({
        table: "documents",
        mode: "ocr",
        owner: "019bf789-4400-7f12-9abc-123456789abc",
        record: "019bf789-4400-7f12-9abc-123456789abd",
        type: "019bf789-4400-7f12-9abc-123456789abe",
      }),
    ).toEqual({
      table: "documents",
      owner: "019bf789-4400-7f12-9abc-123456789abc",
      record: "019bf789-4400-7f12-9abc-123456789abd",
      type: "019bf789-4400-7f12-9abc-123456789abe",
    });
    expect(
      normalizeCadastroPageSearch({
        table: "people",
        type: "019bf789-4400-7f12-9abc-123456789abe",
      }),
    ).toEqual({
      table: "people",
    });
    expect(
      cadastroHref({
        table: "bills",
        mode: "xlsx",
        owner: "019bf789-4400-7f12-9abc-123456789abc",
      }),
    ).toBe("/cadastro?table=bills&mode=xlsx&owner=019bf789-4400-7f12-9abc-123456789abc");
  });
});

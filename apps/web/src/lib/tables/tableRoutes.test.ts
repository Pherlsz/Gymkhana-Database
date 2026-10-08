import { describe, expect, it } from "vitest";
import { compactTableSearch, normalizeTableSearch, tableLinkProps } from "./tableRoutes";

describe("table search URLs", () => {
  it("omits defaults and empty filters from a people view", () => {
    expect(compactTableSearch(normalizeTableSearch({}))).toEqual({});
    expect(
      compactTableSearch(normalizeTableSearch({ page: 1, limit: 100, sort: "full_name" })),
    ).toEqual({});
  });

  it("keeps only the recorte that differs from the default view", () => {
    expect(
      compactTableSearch(
        normalizeTableSearch({
          sort: "cpf",
          order: "desc",
          page: 2,
          city: "Porto Alegre",
          cols: "-city,father_name",
          document_identifier: "",
          bill_reference: "",
        }),
      ),
    ).toEqual({
      sort: "cpf",
      order: "desc",
      page: 2,
      city: "Porto Alegre",
    });
  });

  it("builds table links without dumping the unused modules into search", () => {
    expect(tableLinkProps({ sort: "cpf" })).toEqual({
      to: "/tables/$table",
      params: { table: "people" },
      search: { sort: "cpf" },
    });
    expect(tableLinkProps({ section: "documents", document_identifier: "RG-1" })).toEqual({
      to: "/tables/$table",
      params: { table: "documents" },
      search: { document_identifier: "RG-1" },
    });
  });

  it("keeps an assistant result id on the link", () => {
    const id = "11111111-1111-4111-8111-111111111111";
    expect(compactTableSearch(normalizeTableSearch({ result: id }))).toEqual({ result: id });
    expect(normalizeTableSearch({ result: "nao-e-id" }).result).toBe("");
  });
});

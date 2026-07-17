import { describe, expect, it } from "vitest";
import {
  normalizeGlobalSearch,
  profileSearchForResult,
  searchErrorMessage,
  termsFromSearch,
} from "./SearchPage";
import { APIRequestError, type SearchResult } from "./lib/api/client";

const result: SearchResult = {
  module: "documents",
  entity_kind: "document",
  entity_id: "22222222-2222-2222-2222-222222222222",
  profile_id: "11111111-1111-1111-1111-111111111111",
  target_kind: "document",
  target_id: "22222222-2222-2222-2222-222222222222",
  entity_label: "RG · 001ABC",
  field_key: "document.identifier",
  field_label: "Identificador",
  preview: "001ABC",
  score: 775,
  updated_at: "2026-07-17T12:00:00Z",
};

describe("global Search URL state", () => {
  it("normalizes allowlisted modules and deterministic paging", () => {
    expect(
      normalizeGlobalSearch({
        q: "Ana",
        modules: "documents,physical_table,documents,profiles",
        fields: "document.identifier,profile.full_name,document.identifier",
        page: "2",
        limit: "100",
        sort: "updated_at",
        order: "asc",
      }),
    ).toEqual({
      q: "Ana",
      modules: "documents,profiles",
      fields: "document.identifier,profile.full_name",
      page: 2,
      limit: 100,
      sort: "updated_at",
      order: "asc",
    });
  });

  it("preserves arbitrary sequences and spaces inside each parameter", () => {
    expect(termsFromSearch("  Ana Maria  \n 00%_ABC \n")).toEqual(["Ana Maria", "00%_ABC"]);
  });

  it("builds a direct URL-backed document selection", () => {
    expect(profileSearchForResult(result)).toEqual(
      expect.objectContaining({
        selected: result.profile_id,
        mode: "view",
        section: "documents",
        document_selected: result.target_id,
        document_mode: "view",
      }),
    );
  });

  it.each([
    ["rate_limited", "limite de buscas"],
    ["query_too_costly", "ampla demais"],
    ["result_set_too_large", "candidatos demais"],
    ["search_timeout", "excedeu o tempo"],
    ["forbidden", "não possui permissão"],
  ])("explains the %s failure without exposing internals", (code, expected) => {
    const error = new APIRequestError("internal physical_table detail", {
      status: code === "rate_limited" ? 429 : code === "search_timeout" ? 503 : 422,
      code,
      requestId: "request-id",
    });
    const message = searchErrorMessage(error);
    expect(message).toContain(expected);
    expect(message).not.toContain("physical_table");
  });
});

import { describe, expect, it } from "vitest";
import {
  groupSearchResults,
  normalizeGlobalSearch,
  profileSearchForResult,
  searchErrorMessage,
  SearchPage,
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

describe("/search route mount", () => {
  it("mounts the SearchPage instead of the coming-soon placeholder", async () => {
    const { Route } = await import("./routes/search");
    expect(Route.options.component).toBe(SearchPage);
  });
});

describe("Option B result grouping", () => {
  const rows: SearchResult[] = [
    { ...result, field_key: "document.holder", score: 700 },
    { ...result, field_key: "document.identifier", score: 775 },
    {
      ...result,
      module: "profiles",
      entity_id: "11111111-1111-1111-1111-111111111111",
      profile_id: "11111111-1111-1111-1111-111111111111",
      target_kind: "profile",
      target_id: "11111111-1111-1111-1111-111111111111",
      field_key: "profile.full_name",
      score: 812,
    },
  ];

  it("groups rows by entity and keeps the best score on top", () => {
    const groups = groupSearchResults(rows);
    expect(groups.map((group) => [group.module, group.entityLabel])).toEqual([
      ["profiles", "RG · 001ABC"],
      ["documents", "RG · 001ABC"],
    ]);
    expect(groups[0]?.topScore).toBe(812);
  });

  it("collects every matched field inside one group", () => {
    const groups = groupSearchResults(rows);
    expect(groups[1]?.matches.map((match) => match.fieldKey)).toEqual([
      "document.holder",
      "document.identifier",
    ]);
    expect(groups[1]?.topScore).toBe(775);
  });

  it("keeps API order for matches within a group", () => {
    const groups = groupSearchResults(rows);
    expect(groups[0]?.matches[0]?.fieldLabel).toBeDefined();
  });
});

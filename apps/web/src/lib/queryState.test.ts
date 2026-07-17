import { describe, expect, it } from "vitest";
import type { QueryCatalog, QueryPlan } from "./api/query";
import {
  decodeQueryURLPlan,
  encodeQueryURLPlan,
  maximumQueryURLPlanCharacters,
  normalizeQuerySearch,
  recoverQueryURLPlan,
} from "./queryState";

const version = "a".repeat(64);

function plan(): QueryPlan {
  return {
    version: "v1",
    catalog_version: version,
    root_entity: "profiles",
    projections: ["profile.full_name"],
    filter: {
      kind: "predicate",
      field: "profile.full_name",
      operator: "contains",
      values: ["Ana"],
    },
    sort: [{ field: "profile.full_name", direction: "asc" }],
    maximum_rows: 100,
  };
}

function catalog(): QueryCatalog {
  return {
    version: "b".repeat(64),
    entities: [
      {
        key: "profiles",
        label: "Pessoas",
        kind: "profile",
        navigable: true,
        default_sort: "profile.full_name",
      },
    ],
    fields: [
      {
        key: "profile.full_name",
        entity: "profiles",
        label: "Nome completo",
        kind: "text",
        nullable: false,
        projectable: true,
        filterable: true,
        sortable: true,
        operators: ["contains", "eq"],
      },
    ],
    relations: [],
    operators: [
      { key: "eq", label: "é igual a", minimum_values: 1, maximum_values: 1 },
      { key: "contains", label: "contém", minimum_values: 1, maximum_values: 1 },
    ],
    limits: {
      maximum_projections: 20,
      maximum_filter_nodes: 40,
      maximum_filter_depth: 6,
      maximum_relation_depth: 3,
      maximum_predicate_values: 25,
      maximum_sort_fields: 3,
      maximum_rows: 500,
      maximum_page_size: 100,
    },
  };
}

describe("Query URL state", () => {
  it("round-trips a bounded strict QueryPlan", () => {
    const encoded = encodeQueryURLPlan(plan());
    expect(encoded).toBeDefined();
    expect(decodeQueryURLPlan(encoded)).toEqual(plan());
    expect(normalizeQuerySearch({ plan: encoded, result_page: "2" })).toEqual({
      plan: encoded,
      result_page: 2,
    });
  });

  it("rejects unknown properties, unsupported versions, and oversized input", () => {
    expect(decodeQueryURLPlan(JSON.stringify({ ...plan(), sql: "SELECT secret" }))).toBeUndefined();
    expect(decodeQueryURLPlan(JSON.stringify({ ...plan(), version: "v14" }))).toBeUndefined();
    expect(decodeQueryURLPlan("x".repeat(maximumQueryURLPlanCharacters + 1))).toBeUndefined();
    expect(normalizeQuerySearch({ plan: "forged", result_page: 99 })).toEqual({
      result_page: 1,
    });
  });

  it("preserves permitted nodes and removes stale forged catalog entries", () => {
    const stale = plan();
    stale.projections = ["profile.full_name", "profile.secret"];
    stale.filter = {
      kind: "group",
      conjunction: "AND",
      children: [
        stale.filter!,
        { kind: "predicate", field: "profile.secret", operator: "eq", values: ["x"] },
      ],
    };
    const recovered = recoverQueryURLPlan(stale, catalog());
    expect(recovered?.catalogChanged).toBe(true);
    expect(recovered?.plan.projections).toEqual(["profile.full_name"]);
    expect(recovered?.plan.filter).toEqual({
      kind: "group",
      conjunction: "AND",
      children: [plan().filter],
    });
    expect(recovered?.issues).toEqual(expect.arrayContaining(["colunas", "filtro.2"]));
    expect(JSON.stringify(recovered)).not.toContain("profile.secret");
  });
});

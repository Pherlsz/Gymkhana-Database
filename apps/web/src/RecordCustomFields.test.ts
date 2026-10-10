import { describe, expect, it } from "vitest";
import type { CustomField } from "./lib/api/customdata";
import { customInputsFromDraft, sortCustomFieldsByContext } from "./RecordCustomFields";

function field(technical_key: string, label: string): CustomField {
  return {
    id: technical_key,
    target_kind: "BILL_TYPE",
    technical_key,
    label,
    field_kind: "TEXT",
    required: false,
    active: true,
    version: 1,
    created_at: "",
    updated_at: "",
  };
}

describe("sortCustomFieldsByContext", () => {
  it("keeps emissão next to vencimento in a 2-col row", () => {
    const ordered = sortCustomFieldsByContext([
      field("utility_company", "Concessionária"),
      field("issue_month_year", "Emissão (mês/ano)"),
      field("nf", "NF"),
      field("reading_route", "Roteiro leitura"),
      field("uc", "UC"),
      field("due_month_year", "Vencimento (mês/ano)"),
    ]);
    expect(ordered.map((item) => item.technical_key)).toEqual([
      "utility_company",
      "uc",
      "issue_month_year",
      "due_month_year",
      "reading_route",
      "nf",
    ]);
  });
});

describe("customInputsFromDraft", () => {
  it("omits invalid integers instead of sending them as text", () => {
    const integerField: CustomField = {
      ...field("seats", "Lugares"),
      field_kind: "INTEGER",
    };
    expect(customInputsFromDraft([integerField], { seats: "12" })).toEqual([
      { field_definition_id: "seats", field_kind: "INTEGER", integer: 12 },
    ]);
    expect(customInputsFromDraft([integerField], { seats: "doze" })).toEqual([]);
  });
});

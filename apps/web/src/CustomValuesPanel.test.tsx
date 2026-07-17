import { describe, expect, it } from "vitest";
import { customInputsFromDraft, draftFromStoredValues } from "./CustomValuesPanel";
import type { CustomField, CustomValueSet } from "./lib/api/customdata";

function field(id: string, fieldKind: CustomField["field_kind"]): CustomField {
  return {
    id,
    target_kind: "PROFILE",
    technical_key: id,
    label: id,
    field_kind: fieldKind,
    required: false,
    active: true,
    version: 1,
    created_at: "2026-07-16T00:00:00Z",
    updated_at: "2026-07-16T00:00:00Z",
  };
}

describe("custom value draft conversion", () => {
  it("preserves zero, decimal text, false, civil values and selected options", () => {
    const values: CustomValueSet["values"] = [
      {
        id: "value-integer",
        field_definition_id: "integer",
        field_kind: "INTEGER",
        integer: 0,
        version: 1,
        created_at: "2026-07-16T00:00:00Z",
        updated_at: "2026-07-16T00:00:00Z",
      },
      {
        id: "value-decimal",
        field_definition_id: "decimal",
        field_kind: "DECIMAL",
        decimal: "0001.2300",
        version: 1,
        created_at: "2026-07-16T00:00:00Z",
        updated_at: "2026-07-16T00:00:00Z",
      },
      {
        id: "value-boolean",
        field_definition_id: "boolean",
        field_kind: "BOOLEAN",
        boolean: false,
        version: 1,
        created_at: "2026-07-16T00:00:00Z",
        updated_at: "2026-07-16T00:00:00Z",
      },
      {
        id: "value-date",
        field_definition_id: "date",
        field_kind: "CIVIL_DATE",
        civil_date: "2026-07-16",
        version: 1,
        created_at: "2026-07-16T00:00:00Z",
        updated_at: "2026-07-16T00:00:00Z",
      },
      {
        id: "value-select",
        field_definition_id: "select",
        field_kind: "MULTI_SELECT",
        option_ids: ["option-a", "option-b"],
        version: 1,
        created_at: "2026-07-16T00:00:00Z",
        updated_at: "2026-07-16T00:00:00Z",
      },
    ];
    expect(draftFromStoredValues(values)).toEqual({
      integer: "0",
      decimal: "0001.2300",
      boolean: false,
      date: "2026-07-16",
      select: ["option-a", "option-b"],
    });
  });

  it("creates typed inputs without dropping valid false or zero values", () => {
    expect(
      customInputsFromDraft(
        [
          field("integer", "INTEGER"),
          field("decimal", "DECIMAL"),
          field("boolean", "BOOLEAN"),
          field("date", "CIVIL_DATE"),
          field("select", "MULTI_SELECT"),
          field("empty", "TEXT"),
        ],
        {
          integer: "0",
          decimal: "0001.2300",
          boolean: false,
          date: "2026-07-16",
          select: ["option-a", "option-b"],
          empty: "   ",
        },
      ),
    ).toEqual([
      { field_definition_id: "integer", field_kind: "INTEGER", integer: 0 },
      { field_definition_id: "decimal", field_kind: "DECIMAL", decimal: "0001.2300" },
      { field_definition_id: "boolean", field_kind: "BOOLEAN", boolean: false },
      { field_definition_id: "date", field_kind: "CIVIL_DATE", civil_date: "2026-07-16" },
      {
        field_definition_id: "select",
        field_kind: "MULTI_SELECT",
        option_ids: ["option-a", "option-b"],
      },
    ]);
  });
});

import { describe, expect, it } from "vitest";
import type { DocumentRecord } from "../api/client";
import { documentRow } from "./tableRows";

const labels = {
  boolean: { yes: "Sim", no: "Não" },
  status: { AVAILABLE: "Disponível", IN_USE: "Em uso" },
  medium: { PHYSICAL: "Físico", DIGITAL: "Digital" },
  idleCustody: { ORGANIZATION: "Organização", OWNER: "Com o dono" },
};

describe("tableRows", () => {
  it("keeps internal status and medium keys off the display cells", () => {
    const record = {
      id: "doc-1",
      owner_profile_id: "p-1",
      owner_full_name: "Ana",
      document_type_id: "type-rg",
      identifier_value: "123",
      document_date: "2020-01-15",
      notes: "",
      medium: "PHYSICAL",
      idle_custody: "ORGANIZATION",
      valid_until: "",
      status: "AVAILABLE",
      type: {
        id: "type-rg",
        technical_key: "rg",
        label: "RG",
        active: true,
        uniqueness_policy: "PER_PROFILE",
        validation_regex: "",
        date_required: false,
        count: 1,
        version: 1,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      },
      current_use: {
        holder_profile_id: "p-1",
        holder_full_name: "Ana",
        assigned_at: "2026-07-01T00:00:00Z",
      },
      custom_values: {},
      version: 1,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    } as DocumentRecord;
    const row = documentRow(record, labels);
    expect(row.cells.status).toBe("Disponível");
    expect(row.cells.status_key).toBe("AVAILABLE");
    expect(row.cells.medium).toBe("Físico");
    expect(row.cells.medium_key).toBe("PHYSICAL");
  });
});

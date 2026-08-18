import { describe, expect, it } from "vitest";
import type { BillRecord, DocumentRecord, Profile } from "../api/client";
import { billRow, documentRow, formatAmount, personRow, type RecordSheetLabels } from "./tableRows";

const labels: RecordSheetLabels = {
  boolean: { yes: "Sim", no: "Não" },
  status: { AVAILABLE: "Disponível", IN_USE: "Em uso" },
  medium: { PHYSICAL: "Físico", DIGITAL: "Digital" },
  idleCustody: { ORGANIZATION: "Organização", OWNER: "Dono" },
};

function profile(overrides: Partial<Profile> = {}): Profile {
  return {
    id: "profile-1",
    full_name: "Ana da Silva",
    social_name: "",
    cpf: "12345678901",
    email: "ana@example.com",
    mobile_phone: "",
    landline_phone: "",
    address: {
      street: "Rua A",
      number: "1",
      complement: "",
      neighborhood: "",
      city: "Porto Alegre",
      state: "RS",
      postal_code: "",
    },
    notes: "",
    birth_date: "1990-08-14",
    blood_donor: "true",
    document_badges: [
      {
        document_type_id: "type-rg",
        technical_key: "rg",
        label: "RG",
        claim: "informed_number",
        badge: "physical",
        has_physical: true,
        has_digital: false,
        in_hands: true,
      },
      {
        document_type_id: "type-empty",
        technical_key: "empty",
        label: "Empty",
        claim: "absence",
        badge: "",
        has_physical: false,
        has_digital: false,
        in_hands: false,
      },
    ],
    version: 1,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  } as Profile;
}

describe("tableRows", () => {
  it("formats dates, booleans, empty cells and positive badges once when mapping a person", () => {
    const row = personRow(profile(), [], labels.boolean);
    expect(row.cells.birth_date).toBe("14/08/1990");
    expect(row.cells.blood_donor).toBe("Sim");
    expect(row.cells.social_name).toBe("—");
    expect(row.cells.document_badges).toEqual([
      expect.objectContaining({ technical_key: "rg", badge: "physical" }),
    ]);
  });

  it("uses owner_full_name from the document list item instead of a profile map", () => {
    const row = documentRow(documentRecord(), labels);
    expect(row.cells.owner).toBe("Ana da Silva");
    expect(row.cells.status).toBe("Disponível");
    expect(row.cells.date).toBe("15/01/2020");
    expect(row.cells.medium).toBe("Físico");
    expect(row.cells.idle_custody).toBe("Organização");
  });

  it("uses owner_full_name from the bill list item", () => {
    const row = billRow(
      {
        id: "bill-1",
        owner_profile_id: "profile-1",
        owner_full_name: "Ana da Silva",
        bill_type_id: "type-luz",
        printed_holder_name: "Ana da Silva",
        printed_address: "Rua A, 1",
        reference_value: "UC-100",
        competence: "2026-07",
        amount: "123.45",
        currency: "BRL",
        notes: "",
        medium: "PHYSICAL",
        idle_custody: "ORGANIZATION",
        status: "AVAILABLE",
        type: {
          id: "type-luz",
          technical_key: "luz",
          label: "Luz",
          active: true,
          count: 1,
          version: 1,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
        current_use: {
          holder_profile_id: "profile-2",
          holder_full_name: "Bruno Costa",
          assigned_at: "2026-07-01T00:00:00Z",
        },
        version: 1,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      } as BillRecord,
      labels,
    );
    expect(row.cells.owner).toBe("Ana da Silva");
    expect(String(row.cells.amount)).toMatch(/123,45/);
    expect(row.cells.current_holder).toBe("Bruno Costa");
    expect(row.cells.status).toBe("Disponível");
  });

  it("formats canonical and Brazilian amounts without shifting the decimal", () => {
    expect(formatAmount("123.45", "BRL")).toMatch(/123,45/);
    expect(formatAmount("1.234,56", "BRL")).toMatch(/1\.234,56/);
    expect(formatAmount("", "BRL")).toBe("");
  });
});

function documentRecord(): DocumentRecord {
  return {
    id: "doc-1",
    owner_profile_id: "profile-1",
    owner_full_name: "Ana da Silva",
    document_type_id: "type-rg",
    identifier_value: "123",
    document_date: "2020-01-15",
    notes: "",
    medium: "PHYSICAL",
    idle_custody: "ORGANIZATION",
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
      holder_profile_id: "profile-1",
      holder_full_name: "Ana da Silva",
      assigned_at: "2026-07-01T00:00:00Z",
    },
    version: 1,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

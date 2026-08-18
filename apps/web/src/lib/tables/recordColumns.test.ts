import { describe, expect, it } from "vitest";
import type { CustomField } from "../api/client";
import { buildBillColumns, buildDocumentColumns } from "./recordColumns";

const copy = {
  columns: {
    identifier: "Identificador",
    reference: "Referência",
    type: "Tipo",
    owner: "Dono",
    status: "Status",
    medium: "Suporte",
    idleCustody: "Guarda",
    validUntil: "Validade",
    date: "Data",
    currentHolder: "Portador",
    notes: "Notas",
    competence: "Competência",
    amount: "Valor",
    printedHolder: "Titular impresso",
    printedAddress: "Endereço impresso",
    currency: "Moeda",
  },
  status: { AVAILABLE: "Disponível", IN_USE: "Em uso" },
  medium: { PHYSICAL: "Físico", DIGITAL: "Digital" },
  idleCustody: { ORGANIZATION: "Organização", OWNER: "Com o dono" },
  boolean: { yes: "Sim", no: "Não" },
};

const extra: CustomField[] = [
  {
    id: "field-1",
    target_kind: "DOCUMENT_TYPE",
    target_id: "type-1",
    label: "Cor",
    technical_key: "color",
    field_kind: "TEXT",
    required: false,
    active: true,
    version: 1,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
];

describe("record columns", () => {
  it("builds document and bill sheets from the same extra-field seam", () => {
    const documents = buildDocumentColumns(copy, extra);
    const bills = buildBillColumns(copy, extra);
    expect(documents.map((column) => column.key)).toEqual([
      "identifier",
      "type",
      "owner",
      "status",
      "medium",
      "idle_custody",
      "valid_until",
      "date",
      "current_holder",
      "notes",
      "custom:color",
    ]);
    expect(bills.map((column) => column.key)).toContain("reference");
    expect(bills.map((column) => column.key)).toContain("custom:color");
    expect(bills.find((column) => column.key === "amount")).toBeTruthy();
  });
});

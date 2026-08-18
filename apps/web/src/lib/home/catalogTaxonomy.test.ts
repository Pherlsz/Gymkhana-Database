import { describe, expect, it } from "vitest";
import { groupHomeCatalog, normalizeCatalogToken, type HomeCatalogItem } from "./catalogTaxonomy";
import { ptBRV1 } from "../../i18n/v1/pt-BR";

function item(
  id: string,
  label: string,
  technicalKey: string,
  kind: HomeCatalogItem["kind"] = "document",
): HomeCatalogItem {
  return { id, label, technicalKey, kind, count: 0, loading: false };
}

const labels = ptBRV1.home.tables.contexts;

describe("groupHomeCatalog", () => {
  it("always lists the legacy subcategories, including empty contas", () => {
    const groups = groupHomeCatalog({
      people: item("people", "Dados pessoais", "dados-pessoais", "people"),
      bills: [],
      documents: [item("rg", "RG", "rg")],
      labels,
    });
    expect(groups.map((group) => group.key)).toEqual([
      "personal",
      "bills",
      "identity",
      "work",
      "socialHealth",
      "education",
      "certificates",
    ]);
    expect(groups.find((group) => group.key === "bills")?.items.map((row) => row.label)).toEqual([
      "Conta de luz",
      "Conta de água",
      "Internet",
    ]);
    expect(groups.find((group) => group.key === "identity")?.items.map((row) => row.label)).toEqual(
      ["RG", "CNH", "Passaporte"],
    );
    expect(groups.find((group) => group.key === "work")?.items.map((row) => row.label)).toEqual([
      "CTPS",
      "PIS",
      "CREA",
      "OAB",
      "CRM",
      "CRO",
      "COREN",
    ]);
  });

  it("overlays matching API types onto the canonical rows", () => {
    const groups = groupHomeCatalog({
      people: item("people", "Dados pessoais", "dados-pessoais", "people"),
      bills: [item("luz", "Energia", "energia", "bill")],
      documents: [item("rg-id", "Registro Geral", "rg")],
      labels,
    });
    expect(groups.find((group) => group.key === "bills")?.items[0]).toMatchObject({
      id: "luz",
      label: "Conta de luz",
    });
    expect(groups.find((group) => group.key === "identity")?.items[0]).toMatchObject({
      id: "rg-id",
      label: "RG",
    });
  });

  it("keeps identity as RG/CNH/Passaporte and work without CREA/OAB combined", () => {
    const groups = groupHomeCatalog({
      people: item("people", "Dados pessoais", "dados-pessoais", "people"),
      bills: [
        item("bill-luz", "Conta de luz", "energia", "bill"),
        item("bill-agua", "Conta de água", "agua", "bill"),
        item("bill-net", "Internet", "internet", "bill"),
      ],
      documents: [item("rg-id", "RG", "rg"), item("cnh-id", "CNH", "cnh")],
      labels,
    });
    expect(groups.find((group) => group.key === "bills")?.items.map((row) => row.id)).toEqual([
      "bill-luz",
      "bill-agua",
      "bill-net",
    ]);
    expect(groups.find((group) => group.key === "identity")?.items.map((row) => row.id)).toEqual([
      "rg-id",
      "cnh-id",
      "doc-passaporte",
    ]);
    expect(
      groups.find((group) => group.key === "work")?.items.map((row) => row.label),
    ).not.toContain("CREA / OAB");
    expect(
      groups.find((group) => group.key === "identity")?.items.map((row) => row.label),
    ).not.toContain("Identidade");
    expect(
      groups.find((group) => group.key === "identity")?.items.map((row) => row.label),
    ).not.toContain("CPF");
  });

  it("drops discontinued Identidade, CPF-documento and CREA/OAB from extras", () => {
    const groups = groupHomeCatalog({
      people: item("people", "Dados pessoais", "dados-pessoais", "people"),
      bills: [],
      documents: [
        item("id-identidade", "Identidade", "identidade"),
        item("id-cpf", "CPF", "cpf"),
        item("id-crea-oab", "CREA / OAB", "crea_oab"),
      ],
      labels,
    });
    const extras = groups.find((group) => group.key === "otherDocuments")?.items ?? [];
    expect(extras.map((row) => row.label)).toEqual([]);
    expect(
      groups.find((group) => group.key === "identity")?.items.map((row) => row.label),
    ).not.toContain("Identidade");
    expect(
      groups.find((group) => group.key === "work")?.items.map((row) => row.label),
    ).not.toContain("CREA / OAB");
  });
});

describe("normalizeCatalogToken", () => {
  it("strips accents and punctuation", () => {
    expect(normalizeCatalogToken("Título de Eleitor")).toBe("titulo-de-eleitor");
  });
});

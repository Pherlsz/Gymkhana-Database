import { describe, expect, it } from "vitest";
import {
  buildFieldFilterGroups,
  searchModuleChips,
} from "./fieldFilterGroups";

const moduleLabels = new Map([
  ["profiles", "Pessoas"],
  ["documents", "Documentos"],
  ["bills", "Contas e comprovantes"],
  ["custom_data", "Dados personalizados"],
  ["attachments", "Anexos"],
]);

describe("fieldFilterGroups", () => {
  it("hides custom_data from module chips", () => {
    expect(
      searchModuleChips([
        { key: "profiles", label: "Pessoas" },
        { key: "custom_data", label: "Dados personalizados" },
        { key: "bills", label: "Contas" },
      ]).map((module) => module.key),
    ).toEqual(["profiles", "bills"]);
  });

  it("splits type-scoped customs by context, not into Dados personalizados", () => {
    const groups = buildFieldFilterGroups(
      [
        {
          key: "document.identifier",
          module: "documents",
          group: "documents",
          label: "Identificador",
          kind: "identifier",
        },
        {
          key: "custom.11111111-1111-1111-1111-111111111111",
          module: "custom_data",
          group: "documents",
          label: "Livro · Certidão de Casamento",
          kind: "text",
        },
        {
          key: "custom.22222222-2222-2222-2222-222222222222",
          module: "custom_data",
          group: "bills",
          label: "Hidrômetro · Conta de água",
          kind: "text",
        },
        {
          key: "custom.33333333-3333-3333-3333-333333333333",
          module: "custom_data",
          group: "custom_data",
          label: "Placa · Veículo",
          kind: "text",
        },
      ],
      moduleLabels,
      [],
      ["profiles", "documents", "bills", "custom_data", "attachments"],
    );

    expect(groups.map((group) => group.label)).toEqual([
      "Documentos",
      "Certidão de Casamento",
      "Conta de água",
      "Veículo",
    ]);
    expect(groups.some((group) => group.label === "Dados personalizados")).toBe(false);
  });
});

import { describe, expect, it } from "vitest";
import { buildPeopleColumns } from "./peopleColumns";
import { buildBillColumns, buildDocumentColumns, type RecordSheetCopy } from "./recordColumns";
import {
  SPREADSHEET_MAX_PAGE_SIZE,
  SPREADSHEET_PAGE_SIZES,
  clampSpreadsheetPageSize,
  spreadsheetPageSizeOptions,
} from "./spreadsheetViewport";

const copy = {
  fullName: "Nome",
  digitSumName: "Soma nome",
  socialName: "Nome social",
  gender: "Sexo",
  birthDate: "Data de nascimento",
  age: "Idade",
  zodiacSign: "Signo",
  bloodType: "Tipo sanguíneo",
  rg: "RG",
  digitSumRg: "Soma RG",
  cpf: "CPF",
  digitSumCpf: "Soma CPF",
  voterId: "Título",
  cnh: "CNH",
  digitSumCnh: "Soma CNH",
  ctps: "CTPS",
  ctpsSeries: "Série",
  pis: "PIS",
  crea: "CREA",
  oab: "OAB",
  studentId: "Estudante",
  susCard: "SUS",
  citizenCard: "Cidadão",
  passport: "Passaporte",
  street: "Rua",
  number: "Número",
  complement: "Complemento",
  neighborhood: "Bairro",
  city: "Cidade",
  state: "UF",
  postalCode: "CEP",
  email: "E-mail",
  mobile: "Celular",
  digitSumPhone: "Soma telefone",
  landline: "Fixo",
  maritalStatus: "Estado civil",
  nationality: "Nacionalidade",
  birthCity: "Cidade nascimento",
  weddingDate: "Casamento",
  fatherName: "Pai",
  fatherBirthDate: "Nascimento pai",
  motherName: "Mãe",
  motherBirthDate: "Nascimento mãe",
  vehicleModel: "Modelo",
  vehicleColor: "Cor",
  vehiclePlate: "Placa",
  vehicleYear: "Ano",
  healthPlan: "Plano",
  bloodDonor: "Sangue",
  organDonor: "Órgãos",
  team: "Equipe",
  sector: "Setor",
  collections: "Coleções",
  clubMembership: "Sócio",
  membershipType: "Categoria",
  placeOfOrigin: "Naturalidade",
  birthCountry: "País nascimento",
  parentsWeddingDate: "Casamento pais",
  supermarketClub: "Supermercado",
  pet: "Animal",
  travelCountries: "Viagem",
  cardBrand: "Bandeira",
  cardBank: "Banco cartão",
  documents: "Documentos",
  withOwner: "Com o dono",
};

describe("spreadsheet viewport vs legacy", () => {
  it("clamps page size to the legacy 50–500 window", () => {
    expect(clampSpreadsheetPageSize(1000)).toBe(SPREADSHEET_MAX_PAGE_SIZE);
    expect(clampSpreadsheetPageSize(12)).toBe(50);
    expect(clampSpreadsheetPageSize(250)).toBe(250);
    expect(SPREADSHEET_PAGE_SIZES).toEqual([50, 100, 250, 500]);
    expect(spreadsheetPageSizeOptions(200)).toEqual([50, 100, 200, 250, 500]);
  });

  it("clamps page size using preference overlays", () => {
    expect(clampSpreadsheetPageSize(1000, { maxPageSize: 250 })).toBe(250);
    expect(spreadsheetPageSizeOptions(80, { pageSizes: [80, 160] })).toEqual([80, 160]);
    expect(spreadsheetPageSizeOptions(200, { pageSizes: [80, 160] })).toEqual([80, 160, 200]);
  });

  it("people sheet stays in the ~50-column range of the legacy sheet", () => {
    const columns = buildPeopleColumns(
      copy,
      {
        number: { glyph: "nº", label: "Número informado" },
        physical: { glyph: "F", label: "Exemplar físico" },
        digital: { glyph: "D", label: "Exemplar digital" },
      },
      [],
    );
    expect(columns.length).toBeGreaterThanOrEqual(50);
    expect(columns.length).toBeLessThan(70);
    expect(columns.filter((column) => column.render).map((column) => column.key)).toEqual([
      "documents",
    ]);
  });

  it("document and bill sheets paint text from preformatted row cells", () => {
    const recordCopy: RecordSheetCopy = {
      columns: {
        identifier: "Identificador",
        reference: "Referência",
        type: "Tipo",
        owner: "Pessoa",
        status: "Status",
        medium: "Suporte",
        idleCustody: "Guarda",
        validUntil: "Validade",
        date: "Data",
        currentHolder: "Em uso por",
        notes: "Notas",
        competence: "Competência",
        amount: "Valor",
        printedHolder: "Titular impresso",
        printedAddress: "Endereço impresso",
        currency: "Moeda",
      },
      status: { AVAILABLE: "Disponível", IN_USE: "Em uso" },
      medium: { PHYSICAL: "Físico", DIGITAL: "Digital" },
      idleCustody: { ORGANIZATION: "Organização", OWNER: "Dono" },
      boolean: { yes: "Sim", no: "Não" },
    };
    expect(buildDocumentColumns(recordCopy, []).every((column) => !column.render)).toBe(true);
    expect(buildBillColumns(recordCopy, []).every((column) => !column.render)).toBe(true);
  });
});

describe("spreadsheet card containment", () => {
  it("does not use layout containment that shrinks the sheet to the first columns", async () => {
    const { readFileSync } = await import("node:fs");
    const { dirname, join } = await import("node:path");
    const { fileURLToPath } = await import("node:url");
    const css = readFileSync(
      join(dirname(fileURLToPath(import.meta.url)), "../../tables.css"),
      "utf8",
    );
    const start = css.indexOf(".spreadsheet-table-card.ant-card");
    const end = css.indexOf(".spreadsheet-table-card .ant-card-body");
    const cardBlock = css.slice(start, end);
    expect(cardBlock).toContain("width: 100%");
    expect(cardBlock).not.toMatch(/contain:\s*layout/);
    expect(css).toContain(".tables-page__workspace:has(> .profile-panel)");
    expect(css).not.toContain("tables-inspector-slot");
  });
});

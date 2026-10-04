import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { Modal } from "antd";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "content-type": "application/json" },
  });
}

function session() {
  return {
    authenticated: true,
    user: { login: "member", display_name: "Member Name", role: "EXTERNAL" },
  };
}

function anaProfile() {
  return {
    id: "019bf789-4400-7f12-9abc-123456789abc",
    full_name: "Ana da Silva",
    social_name: "Ana",
    cpf: "529.982.247-25",
    email: "ana@example.com",
    mobile_phone: "+5551999998888",
    landline_phone: "",
    address: {
      street: "Rua A",
      number: "1",
      complement: "",
      neighborhood: "Centro",
      city: "Porto Alegre",
      state: "RS",
      postal_code: "90000000",
    },
    notes: "",
    custom_values: {},
    birth_date: "1990-08-14",
    gender: "F",
    blood_type: "O+",
    document_identifiers: { rg: "1122334455", cnh: "999888777" },
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
    ],
    cpf_digit_sum: 55,
    version: 1,
    created_at: "2026-07-15T12:00:00Z",
    updated_at: "2026-07-15T12:00:00Z",
  };
}

function profilePage() {
  return {
    profiles: [anaProfile()],
    page: { total: 1, limit: 100, offset: 0, sort_field: "full_name", sort_order: "asc" },
  };
}

function headerTexts() {
  return [...document.querySelectorAll(".spreadsheet-table thead th")].map((cell) => {
    const label = cell.querySelector(".spreadsheet-table__head-label");
    return (label?.textContent ?? cell.textContent ?? "").trim();
  });
}

async function applyColumnSelect(columnLabel: string, optionLabel: string) {
  fireEvent.click(await screen.findByRole("button", { name: `Filtrar ${columnLabel}` }));
  const select = await screen.findByRole("combobox", { name: "Valor" });
  fireEvent.mouseDown(select);
  const option = await screen.findByText(optionLabel, {
    selector: ".ant-select-item-option-content",
  });
  fireEvent.click(option);
}

function cnhType() {
  return {
    id: "type-cnh",
    technical_key: "cnh",
    label: "CNH",
    active: true,
    uniqueness_policy: "PER_PROFILE",
    validation_regex: "",
    date_required: false,
    count: 1,
    version: 1,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function documentRecord() {
  return {
    id: "doc-1",
    owner_profile_id: "019bf789-4400-7f12-9abc-123456789abc",
    owner_full_name: "Ana da Silva",
    document_type_id: "type-rg",
    identifier_value: "1234567890",
    document_date: "2020-01-15",
    notes: "",
    medium: "PHYSICAL",
    idle_custody: "ORGANIZATION",
    valid_until: "2028-01-15",
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
      holder_profile_id: "019bf789-4400-7f12-9abc-123456789abc",
      holder_full_name: "Ana da Silva",
      assigned_at: "2026-07-01T00:00:00Z",
    },
    custom_values: {},
    version: 1,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function cnhRecord() {
  return {
    ...documentRecord(),
    id: "doc-cnh",
    document_type_id: "type-cnh",
    identifier_value: "00123456789",
    type: cnhType(),
    custom_values: { category: "AB" },
  };
}

function inUseDocumentRecord() {
  return {
    ...documentRecord(),
    id: "doc-in-use",
    identifier_value: "9988776655",
    status: "IN_USE",
    owner_full_name: "Bruno Carvalho",
    idle_custody: "OWNER",
  };
}

function billRecord() {
  return {
    id: "bill-1",
    owner_profile_id: "019bf789-4400-7f12-9abc-123456789abc",
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
      label: "Conta de luz",
      active: true,
      count: 1,
      version: 1,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    current_use: null,
    custom_values: { uc: "98765" },
    version: 1,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function emptyCustomFields() {
  return { fields: [], page: { total: 0, limit: 1000, offset: 0 } };
}

function apiResponse(url: string): Response | undefined {
  const params = new URL(url, "http://tables.test").searchParams;
  if (url.endsWith("/api/auth/session")) return jsonResponse(session());
  if (url.includes("/api/v1/profiles?")) {
    let profiles = [anaProfile()];
    const city = params.get("city");
    const cpf = params.get("cpf");
    const email = params.get("email");
    const fullName = params.get("full_name");
    const state = params.get("state");
    if (city) {
      profiles = profiles.filter((profile) =>
        profile.address.city.toLowerCase().includes(city.toLowerCase()),
      );
    }
    if (cpf) {
      profiles = profiles.filter((profile) => profile.cpf.includes(cpf));
    }
    if (email) {
      profiles = profiles.filter((profile) =>
        profile.email.toLowerCase().includes(email.toLowerCase()),
      );
    }
    if (fullName) {
      profiles = profiles.filter((profile) =>
        profile.full_name.toLowerCase().includes(fullName.toLowerCase()),
      );
    }
    if (state) {
      profiles = profiles.filter((profile) => profile.address.state === state);
    }
    return jsonResponse({
      profiles,
      page: {
        total: profiles.length,
        limit: 100,
        offset: 0,
        sort_field: "full_name",
        sort_order: "asc",
      },
    });
  }
  if (url.includes("/api/v1/profiles/")) return jsonResponse(profilePage().profiles[0]);
  if (url.includes("/api/v1/document-types")) {
    return jsonResponse({
      types: [documentRecord().type, cnhType()],
      page: { total: 2, limit: 1000, offset: 0 },
    });
  }
  if (url.includes("/api/v1/bill-types")) {
    return jsonResponse({
      types: [billRecord().type],
      page: { total: 1, limit: 1000, offset: 0 },
    });
  }
  if (url.includes("/api/v1/custom-fields?")) {
    if (url.includes("target_id=type-cnh")) {
      return jsonResponse({
        fields: [
          {
            id: "field-cnh-category",
            target_kind: "DOCUMENT_TYPE",
            target_id: "type-cnh",
            technical_key: "category",
            label: "Categoria",
            field_kind: "TEXT",
            required: false,
            active: true,
            version: 1,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        ],
        page: { total: 1, limit: 1000, offset: 0 },
      });
    }
    return jsonResponse(emptyCustomFields());
  }
  if (url.includes("/api/v1/documents?")) {
    let documents = [documentRecord(), inUseDocumentRecord(), cnhRecord()];
    const typeId = params.get("document_type_id");
    const status = params.get("status");
    const medium = params.get("medium");
    const identifier = params.get("identifier");
    if (typeId) documents = documents.filter((document) => document.document_type_id === typeId);
    if (status) documents = documents.filter((document) => document.status === status);
    if (medium) documents = documents.filter((document) => document.medium === medium);
    if (identifier) {
      documents = documents.filter((document) => document.identifier_value.includes(identifier));
    }
    return jsonResponse({
      documents,
      page: {
        total: documents.length,
        limit: 100,
        offset: 0,
        sort_field: "identifier_value",
        sort_order: "asc",
      },
    });
  }
  if (url.includes("/api/v1/bills?")) {
    let bills = [billRecord()];
    const typeId = params.get("bill_type_id");
    const status = params.get("status");
    const medium = params.get("medium");
    const reference = params.get("reference");
    const competence = params.get("competence");
    if (typeId) bills = bills.filter((bill) => bill.bill_type_id === typeId);
    if (status) bills = bills.filter((bill) => bill.status === status);
    if (medium) bills = bills.filter((bill) => bill.medium === medium);
    if (reference) bills = bills.filter((bill) => bill.reference_value.includes(reference));
    if (competence) bills = bills.filter((bill) => bill.competence.includes(competence));
    return jsonResponse({
      bills,
      page: {
        total: bills.length,
        limit: 100,
        offset: 0,
        sort_field: "reference_value",
        sort_order: "asc",
      },
    });
  }
  if (url.includes("/api/v1/attachments")) {
    return jsonResponse({ attachments: [] });
  }
  if (url.includes("/api/v1/ocr/capability")) {
    return jsonResponse({ enabled: false });
  }
  if (url.includes("/api/v1/chat/capability")) {
    return jsonResponse({
      enabled: false,
      maximum_tool_calls: 8,
      maximum_rows: 100,
      maximum_result_bytes: 256,
      maximum_usage: 1,
      maximum_duration_seconds: 1,
      maximum_message_runes: 1,
    });
  }
  if (url.includes("/api/v1/chat/result-references/") && url.includes("/page")) {
    return jsonResponse({
      columns: [
        { key: "full_name", label: "Nome" },
        { key: "city", label: "Cidade" },
      ],
      rows: [
        {
          id: "r1",
          entity_kind: "profile",
          entity_id: "019bf789-4400-7f12-9abc-123456789abc",
          entity_label: "Ana da Silva",
          cells: { full_name: "Ana da Silva", city: "Porto Alegre" },
        },
        {
          id: "r2",
          entity_kind: "profile",
          entity_id: "019bf789-4400-7f12-9abc-123456789abd",
          entity_label: "Zélia Costa",
          cells: { full_name: "Zélia Costa", city: "Canoas" },
        },
      ],
      total: 2,
      limit: 500,
      offset: 0,
      summary: "2 pessoas",
    });
  }
  if (url.includes("/api/v1/custom-entity-types")) {
    return jsonResponse({ types: [], page: { total: 0, limit: 1000, offset: 0 } });
  }
  if (url.includes("/api/v1/custom-entities?")) {
    return jsonResponse({ entities: [], page: { total: 0, limit: 1, offset: 0 } });
  }
  return jsonResponse({ status: "ok" });
}

describe("TablesPage", () => {
  afterEach(() => {
    cleanup();
    Modal.destroyAll();
    vi.unstubAllGlobals();
    window.localStorage.clear();
    window.history.replaceState(null, "", "/");
  });

  beforeEach(() => {
    window.localStorage.clear();
  });

  it("marks an applied column and clears it from the chip", async () => {
    window.history.replaceState(null, "", "/tables/people?city=Porto+Alegre");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    await waitFor(() =>
      expect(document.querySelector(".tables-toolbar__chip")?.textContent).toBe(
        "Cidade: Porto Alegre",
      ),
    );
    expect(screen.getByRole("button", { name: "Limpar filtros" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Filtros" })).not.toBeInTheDocument();
    expect(document.querySelector(".column-funnel__trigger.is-active")).toBeTruthy();

    const chipClose = document.querySelector<HTMLElement>(
      ".tables-toolbar__chip .ant-tag-close-icon",
    );
    fireEvent.click(chipClose as HTMLElement);
    await waitFor(() => expect(window.location.search).not.toContain("city="));
  });

  it("commits text filters only after Enter in the column funnel", async () => {
    window.history.replaceState(null, "", "/tables/people");
    const fetchMock = vi
      .fn()
      .mockImplementation((input: RequestInfo | URL) =>
        Promise.resolve(apiResponse(String(input))),
      );
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Filtrar Cidade" }));
    const city = await screen.findByRole("textbox", { name: "Valor" });
    const initialProfileCalls = fetchMock.mock.calls.filter((call) =>
      String(call[0]).includes("/api/v1/profiles?"),
    ).length;

    fireEvent.change(city, { target: { value: "Porto Alegre" } });
    expect(window.location.search).not.toContain("city=Porto");
    expect(
      fetchMock.mock.calls.filter((call) => String(call[0]).includes("/api/v1/profiles?")),
    ).toHaveLength(initialProfileCalls);

    fireEvent.keyDown(city, { key: "Enter" });
    await waitFor(() => expect(window.location.search).toContain("city=Porto+Alegre"));
    await waitFor(() =>
      expect(
        fetchMock.mock.calls
          .map((call) => String(call[0]))
          .filter((url) => url.includes("/api/v1/profiles?"))
          .some((url) => url.includes("city=Porto+Alegre")),
      ).toBe(true),
    );
  });

  it(
    "lists people with priority columns first and hides the long tail",
    { timeout: 15_000 },
    async () => {
      window.history.replaceState(null, "", "/tables/people");
      vi.stubGlobal(
        "fetch",
        vi
          .fn()
          .mockImplementation((input: RequestInfo | URL) =>
            Promise.resolve(apiResponse(String(input))),
          ),
      );
      render(<App />);
      expect(await screen.findByText("Ana da Silva")).toBeInTheDocument();
      expect(await screen.findByRole("button", { name: "Filtrar Nome" })).toBeInTheDocument();
      expect(document.querySelector(".tables-inspector-slot")).toBeNull();
      expect(document.querySelector(".tables-page__workspace > .profile-panel")).toBeNull();
      const headers = headerTexts();
      expect(headers.slice(0, 13)).toEqual([
        "Ações",
        "Nome",
        "Documentos",
        "CPF",
        "RG",
        "Rua",
        "Número",
        "Cidade",
        "CEP",
        "E-mail",
        "Celular",
        "Data de nascimento",
        "Título eleitoral",
      ]);
      expect(headers).toContain("CNH");
      expect(headers).toContain("Equipe");
      expect(headers.indexOf("Equipe")).toBeGreaterThan(headers.indexOf("CNH"));
      expect(screen.getByRole("button", { name: "Filtrar Equipe" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Filtrar Setor" })).toBeInTheDocument();
      expect(headers).not.toContain("Idade");
      expect(headers).not.toContain("Soma nome");
      expect(headers).not.toContain("Signo");
      expect(headers).not.toContain("Nome do pai");
      expect(headers.some((text) => text === "Notas")).toBe(false);
      expect(screen.getByText("529.982.247-25")).toBeInTheDocument();
      expect(screen.getByText("1122334455")).toBeInTheDocument();
      expect(document.querySelector(".document-badge__acronym")?.textContent).toBe("RG");
      expect(document.querySelector(".document-badge__mark--physical")?.getAttribute("title")).toBe(
        "Exemplar físico",
      );
      expect(document.querySelector(".document-badge__mark--physical svg")).toBeTruthy();
      expect(screen.queryByText("Identidade")).not.toBeInTheDocument();
      const footer = document.querySelector(".spreadsheet-table__footer");
      expect(footer).toBeTruthy();
      expect(document.querySelector(".spreadsheet-table")?.contains(footer)).toBe(false);
      expect(screen.getByLabelText("Linhas por página")).toBeInTheDocument();
    },
  );

  it("applies compact column overrides from the people URL", async () => {
    window.history.replaceState(null, "", "/tables/people?cols=-city,father_name");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    expect(await screen.findByText("Ana da Silva")).toBeInTheDocument();
    expect(headerTexts()).toContain("Nome do pai");
    expect(headerTexts()).not.toContain("Cidade");
    expect(headerTexts()).toContain("Nome");
  });

  it("filters documents from the column funnel without a toolbar filter panel", async () => {
    window.history.replaceState(null, "", "/tables/documents?document_status=IN_USE");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    expect(await screen.findByRole("heading", { name: "Documentos" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Filtros" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Adicionar filtro" })).not.toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Filtrar Status" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Filtrar Tipo" })).toBeInTheDocument();

    await applyColumnSelect("Tipo", "RG");
    await waitFor(() => expect(window.location.search).toContain("document_type=type-rg"));
    expect(window.location.search).toContain("document_status=IN_USE");
  });

  it("sends the document type filter from the URL and lists mixed types without tabs", async () => {
    window.history.replaceState(null, "", "/tables/documents?document_type=type-rg");
    const fetchMock = vi
      .fn()
      .mockImplementation((input: RequestInfo | URL) =>
        Promise.resolve(apiResponse(String(input))),
      );
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    expect(await screen.findByRole("heading", { name: "Documentos" })).toBeInTheDocument();
    expect(await screen.findByText("1234567890")).toBeInTheDocument();
    expect(screen.getByText("Ana da Silva")).toBeInTheDocument();
    expect(
      fetchMock.mock.calls
        .map((call) => String(call[0]))
        .some((url) => url.includes("/api/v1/profiles?limit=1000")),
    ).toBe(false);
    expect(headerTexts().includes("Guarda")).toBe(true);
    expect(headerTexts().includes("Validade")).toBe(true);
    expect(headerTexts().includes("Soma identificador")).toBe(false);
    expect(screen.getByText("Organização")).toBeInTheDocument();
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    const documentCalls = fetchMock.mock.calls
      .map((call) => String(call[0]))
      .filter((url) => url.includes("/api/v1/documents?"));
    expect(documentCalls.some((url) => url.includes("document_type_id=type-rg"))).toBe(true);
    expect(documentCalls.every((url) => !url.includes("owner_profile_id="))).toBe(true);
  });

  it("sends the in-use status filter and hides other statuses", async () => {
    window.history.replaceState(null, "", "/tables/documents?document_status=IN_USE");
    const fetchMock = vi
      .fn()
      .mockImplementation((input: RequestInfo | URL) =>
        Promise.resolve(apiResponse(String(input))),
      );
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    expect(await screen.findByText("9988776655")).toBeInTheDocument();
    expect(screen.getAllByText("Em uso").length).toBeGreaterThan(0);
    expect(screen.queryByText("1234567890")).not.toBeInTheDocument();
    expect(screen.queryByText("Disponível")).not.toBeInTheDocument();
    const documentCalls = fetchMock.mock.calls
      .map((call) => String(call[0]))
      .filter((url) => url.includes("/api/v1/documents?"));
    expect(documentCalls.some((url) => url.includes("status=IN_USE"))).toBe(true);
    expect(documentCalls.every((url) => !url.includes("status=AVAILABLE"))).toBe(true);
  });

  it("shows the CNH category column when that type is filtered", async () => {
    window.history.replaceState(null, "", "/tables/documents?document_type=type-cnh");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    expect(await screen.findByText("00123456789")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Colunas" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Categoria" }));
    await waitFor(() => {
      expect(headerTexts()).toContain("Categoria");
    });
    expect(screen.getByText("AB")).toBeInTheDocument();
  });

  it("shows street by default and keeps neighborhood behind the column picker", async () => {
    window.history.replaceState(null, "", "/tables/people");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    expect(await screen.findByText("Ana da Silva")).toBeInTheDocument();
    expect(screen.getByText("Rua A")).toBeInTheDocument();
    expect(screen.getByText("90000000")).toBeInTheDocument();
    expect(screen.getByText("1122334455")).toBeInTheDocument();
    expect(screen.getByText("999888777")).toBeInTheDocument();
    expect(screen.queryByText("Centro")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("Colunas"));
    fireEvent.click(await screen.findByText("Mostrar todas"));
    expect(await screen.findByText("Centro")).toBeInTheDocument();
    expect(screen.queryByText("Soma CPF")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("Ocultar todas"));
    await waitFor(() => expect(screen.queryByText("Centro")).not.toBeInTheDocument());
    expect(screen.queryByText("Rua A")).not.toBeInTheDocument();
    expect(screen.getByText("Ana da Silva")).toBeInTheDocument();
    fireEvent.change(screen.getByPlaceholderText("Buscar coluna\u2026"), {
      target: { value: "Bairro" },
    });
    fireEvent.click(screen.getByText("Mostrar todas"));
    expect(await screen.findByText("Centro")).toBeInTheDocument();
    expect(screen.queryByText("Rua A")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("Ocultar todas"));
    await waitFor(() => expect(screen.queryByText("Centro")).not.toBeInTheDocument());
    expect(screen.getByText("Ana da Silva")).toBeInTheDocument();
  });

  it("does not mount stored formula columns on the people sheet", async () => {
    window.localStorage.setItem(
      "gymkhana-table-formulas:profiles",
      JSON.stringify([{ id: "formula_test", name: "Maiúsculo", expression: "MAIÚSCULO([nome])" }]),
    );
    window.history.replaceState(null, "", "/tables/people");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    expect(await screen.findByText("Ana da Silva")).toBeInTheDocument();
    expect(screen.queryByText("Maiúsculo")).not.toBeInTheDocument();
    expect(screen.queryByText("ANA DA SILVA")).not.toBeInTheDocument();
  });

  it("opens the bills spreadsheet with a stored formula and no type tabs", async () => {
    window.localStorage.setItem(
      "gymkhana-table-formulas:bills",
      JSON.stringify([
        { id: "formula_ref", name: "Ref maiúscula", expression: 'CONCATENAR("x", [referencia])' },
      ]),
    );
    window.history.replaceState(null, "", "/tables/bills");
    const fetchMock = vi
      .fn()
      .mockImplementation((input: RequestInfo | URL) =>
        Promise.resolve(apiResponse(String(input))),
      );
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    expect(await screen.findByRole("heading", { name: "Contas" })).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Buscar em todos os campos…")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Filtros" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Nova conta" })).not.toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Filtrar Tipo" })).toBeInTheDocument();
    expect(await screen.findByText("UC-100")).toBeInTheDocument();
    expect(headerTexts().includes("Soma referência")).toBe(false);
    expect(screen.queryByText("Ref maiúscula")).not.toBeInTheDocument();
    expect(screen.queryByText("xUC-100")).not.toBeInTheDocument();
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    const billCalls = fetchMock.mock.calls
      .map((call) => String(call[0]))
      .filter((url) => url.includes("/api/v1/bills?"));
    expect(billCalls.every((url) => !url.includes("owner_profile_id="))).toBe(true);
  });

  it("opens a docked inspector from a people row and closes it with Fechar", async () => {
    window.history.replaceState(null, "", "/tables/people");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    expect(await screen.findByText("Ana da Silva")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Ana da Silva"));
    const inspector = await screen.findByRole("complementary", { name: "Detalhes da pessoa" });
    expect(within(inspector).getByRole("button", { name: "Fechar" })).toBeInTheDocument();
    expect(within(inspector).getByRole("button", { name: "Editar" })).toBeInTheDocument();
    expect(within(inspector).queryByRole("button", { name: "Duplicar" })).not.toBeInTheDocument();
    const holderName = within(inspector).getByLabelText(/Nome do titular/i);
    expect(holderName).toBeDisabled();
    expect(
      within(inspector).queryByRole("button", { name: "Documentos desta pessoa" }),
    ).not.toBeInTheDocument();
    expect(
      within(inspector).queryByRole("button", { name: "Contas desta pessoa" }),
    ).not.toBeInTheDocument();
    expect(
      within(inspector).getByRole("heading", { name: "Identidade & contato" }),
    ).toBeInTheDocument();
    expect(within(inspector).getByRole("heading", { name: "Documentos" })).toBeInTheDocument();
    expect(
      within(inspector).getByRole("heading", { name: "Contas de consumo" }),
    ).toBeInTheDocument();
    expect(await within(inspector).findByText("nº 1234567890")).toBeInTheDocument();
    expect(inspector.querySelector(".document-presence__list")).toBeNull();
    expect(
      within(inspector).queryByRole("button", { name: "Adicionar documento" }),
    ).not.toBeInTheDocument();
    expect(within(inspector).getByDisplayValue("14/08/1990")).toBeInTheDocument();
    expect(within(inspector).getByDisplayValue("O+")).toBeInTheDocument();
    expect(within(inspector).queryByText("Exclusão permanente")).not.toBeInTheDocument();
    expect(document.querySelector(".tables-page__workspace > .profile-panel")).toBeTruthy();
    expect(document.querySelector(".tables-inspector-sheet")).toBeNull();
    expect(document.querySelector(".spreadsheet-table")).toBeTruthy();
    expect(document.querySelector(".profiles-cards")).toBeNull();
    fireEvent.click(within(inspector).getByRole("button", { name: "Editar" }));
    expect(await within(inspector).findByLabelText(/Nome do titular/i)).toBeEnabled();
    fireEvent.click(within(inspector).getByRole("button", { name: "Adicionar documento" }));
    expect(
      await within(inspector).findByText(/Preenchimento inteligente via OCR/i),
    ).toBeInTheDocument();
    expect(within(inspector).getByLabelText(/Número do documento/i)).toBeInTheDocument();
    expect(inspector.querySelector(".document-presence__list")).toBeNull();
    fireEvent.click(inspector.querySelector(".inline-actions button") as HTMLButtonElement);
    fireEvent.click(within(inspector).getByRole("button", { name: "Cancelar" }));
    await waitFor(() => {
      expect(within(inspector).getByLabelText(/Nome do titular/i)).toBeDisabled();
    });
    fireEvent.click(within(inspector).getByRole("button", { name: "Fechar" }));
    await waitFor(() => {
      expect(
        screen.queryByRole("complementary", { name: "Detalhes da pessoa" }),
      ).not.toBeInTheDocument();
    });
    expect(document.querySelector(".tables-inspector-slot")).toBeNull();
  });

  it("reveals permanent deletion only after Excluir", async () => {
    window.history.replaceState(null, "", "/tables/people");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/api/auth/session")) {
          return Promise.resolve(
            jsonResponse({
              authenticated: true,
              user: { login: "admin", display_name: "Admin", role: "ADMIN" },
            }),
          );
        }
        return Promise.resolve(apiResponse(url) ?? jsonResponse({ status: "ok" }));
      }),
    );
    render(<App />);
    fireEvent.click(await screen.findByText("Ana da Silva"));
    const inspector = await screen.findByRole("complementary", { name: "Detalhes da pessoa" });
    expect(within(inspector).getByRole("button", { name: /^Excluir$/ })).toBeInTheDocument();
    expect(screen.queryByText("Exclusão permanente")).not.toBeInTheDocument();
    fireEvent.click(within(inspector).getByRole("button", { name: /^Excluir$/ }));
    expect(screen.getByText("Exclusão permanente")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Confirmar")).toBeInTheDocument();
  });

  it("opens a listed document from the person inspector without changing tables", async () => {
    window.history.replaceState(null, "", "/tables/people");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    fireEvent.click(await screen.findByText("Ana da Silva"));
    const inspector = await screen.findByRole("complementary", { name: "Detalhes da pessoa" });
    fireEvent.click(await within(inspector).findByRole("button", { name: /nº 1234567890/ }));
    const card = await screen.findByRole("complementary", { name: "Detalhes do documento" });
    expect(screen.getByRole("heading", { name: "Pessoas" })).toBeInTheDocument();
    expect(window.location.pathname).toBe("/tables/people");
    expect(within(card).getByText("Dono")).toBeInTheDocument();
    fireEvent.click(within(card).getByRole("button", { name: /Ver pessoa/ }));
    expect(
      await screen.findByRole("complementary", { name: "Detalhes da pessoa" }),
    ).toBeInTheDocument();
    expect(window.location.pathname).toBe("/tables/people");
  });

  it("asks before discarding an unsaved edit when another row is clicked", async () => {
    const bruno = {
      ...anaProfile(),
      id: "019bf789-4400-7f12-9abc-bruno0000001",
      full_name: "Bruno Souza",
      social_name: "Bruno",
      email: "bruno@example.com",
    };
    window.history.replaceState(null, "", "/tables/people");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes("/api/v1/profiles?")) {
          return Promise.resolve(
            jsonResponse({
              profiles: [anaProfile(), bruno],
              page: { total: 2, limit: 100, offset: 0, sort_field: "full_name", sort_order: "asc" },
            }),
          );
        }
        if (url.includes(`/api/v1/profiles/${bruno.id}`))
          return Promise.resolve(jsonResponse(bruno));
        return Promise.resolve(apiResponse(url) ?? jsonResponse({ status: "ok" }));
      }),
    );
    render(<App />);
    fireEvent.click(await screen.findByText("Ana da Silva"));
    const inspector = await screen.findByRole("complementary", { name: "Detalhes da pessoa" });
    fireEvent.click(within(inspector).getByRole("button", { name: "Editar" }));
    expect(await within(inspector).findByLabelText(/Nome do titular/i)).toBeEnabled();
    fireEvent.click(screen.getByText("Bruno Souza"));
    const discard = await screen.findByRole("dialog");
    fireEvent.click(within(discard).getByRole("button", { name: "Descartar e trocar" }));
    await waitFor(() => {
      expect(
        within(screen.getByRole("complementary", { name: "Detalhes da pessoa" })).getByRole(
          "heading",
          { name: "Bruno Souza" },
        ),
      ).toBeInTheDocument();
    });
  });

  it("opens the document's own card, not the owner's, and links to the owner", async () => {
    window.history.replaceState(null, "", "/tables/documents");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    fireEvent.click(await screen.findByText("1234567890"));

    const card = await screen.findByRole("complementary", { name: "Detalhes do documento" });
    expect(screen.queryByRole("complementary", { name: "Detalhes da pessoa" })).toBeNull();
    expect(within(card).getByRole("heading", { name: "1234567890" })).toBeInTheDocument();
    // The record's own data, and the owner offered as a link rather than
    // substituted for the record.
    expect(within(card).getByText("Dono")).toBeInTheDocument();
    expect(card.querySelector(".record-panel__owner strong")?.textContent).toBe("Ana da Silva");
    // Missing values are omitted, not drawn as the grid's em dash.
    expect(within(card).queryByText("—")).toBeNull();
    expect(within(card).queryByText(/Preenchimento inteligente via OCR/i)).toBeNull();

    fireEvent.click(within(card).getByRole("button", { name: "Editar" }));
    expect(await within(card).findByText(/Preenchimento inteligente via OCR/i)).toBeInTheDocument();

    fireEvent.click(within(card).getByRole("button", { name: /Ver pessoa/ }));
    await waitFor(() =>
      expect(screen.getByRole("complementary", { name: "Detalhes da pessoa" })).toBeInTheDocument(),
    );
    expect(window.location.pathname).toBe("/tables/people");
  });

  it("opens the bill's own card from the bills table", async () => {
    window.history.replaceState(null, "", "/tables/bills");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    fireEvent.click(await screen.findByText("UC-100"));

    const card = await screen.findByRole("complementary", { name: "Detalhes da conta" });
    expect(screen.queryByRole("complementary", { name: "Detalhes da pessoa" })).toBeNull();
    expect(within(card).getByRole("heading", { name: "UC-100" })).toBeInTheDocument();

    fireEvent.click(within(card).getByRole("button", { name: "Fechar" }));
    await waitFor(() =>
      expect(screen.queryByRole("complementary", { name: "Detalhes da conta" })).toBeNull(),
    );
  });

  it("shows a loading state in the inspector while the person is fetched", async () => {
    // Deep link to a person who is not in the loaded page, which is what makes
    // the inspector fetch. A bill row no longer opens the owner.
    window.history.replaceState(null, "", `/tables/people?selected=${anaProfile().id}&mode=view`);
    let release: ((value: Response) => void) | undefined;
    const held = new Promise<Response>((resolve) => {
      release = resolve;
    });
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes(`/api/v1/profiles/${anaProfile().id}`) && !url.includes("?")) {
          return held;
        }
        if (url.includes("/api/v1/profiles?")) {
          return Promise.resolve(jsonResponse({ profiles: [], page: { total: 0 } }));
        }
        return Promise.resolve(apiResponse(url) ?? jsonResponse({ status: "ok" }));
      }),
    );
    render(<App />);
    const inspector = await screen.findByRole("complementary", { name: "Detalhes da pessoa" });
    expect(inspector).toHaveAttribute("aria-busy", "true");
    expect(within(inspector).getByRole("status")).toHaveTextContent("Carregando perfil");
    expect(within(inspector).queryByText("Pessoa não encontrada")).not.toBeInTheDocument();
    release!(jsonResponse(anaProfile()));
    const loaded = await screen.findByRole("heading", { name: "Ana da Silva" });
    expect(loaded).toBeInTheDocument();
    expect(screen.getByRole("complementary", { name: "Detalhes da pessoa" })).not.toHaveAttribute(
      "aria-busy",
    );
  });

  it("does not open the inspector when the row actions button is clicked", async () => {
    window.history.replaceState(null, "", "/tables/people");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    expect(await screen.findByText("Ana da Silva")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Ações de Ana da Silva"));
    expect(await screen.findByRole("menuitem", { name: "Editar" })).toBeInTheDocument();
    expect(
      screen.queryByRole("complementary", { name: "Detalhes da pessoa" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Marcar em uso" })).not.toBeInTheDocument();
  });

  it("opens a person in edit mode from the row actions menu", async () => {
    window.history.replaceState(null, "", "/tables/people");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    fireEvent.click(await screen.findByLabelText("Ações de Ana da Silva"));
    fireEvent.click(await screen.findByRole("menuitem", { name: "Editar" }));
    const inspector = await screen.findByRole("complementary", { name: "Detalhes da pessoa" });
    expect(await within(inspector).findByLabelText(/Nome do titular/i)).toBeEnabled();
  });

  it("offers current-use actions on physical documents and hides them on digital ones", async () => {
    window.history.replaceState(null, "", "/tables/documents");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    render(<App />);
    expect(await screen.findByText("1234567890")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Ações de 1234567890"));
    expect(await screen.findByRole("menuitem", { name: "Editar" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Marcar em uso" })).toBeInTheDocument();
    fireEvent.click(document.body);

    fireEvent.click(screen.getByLabelText("Ações de 9988776655"));
    expect(await screen.findByRole("menuitem", { name: "Substituir pessoa" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Devolver" })).toBeInTheDocument();
  });

  it("does not offer current-use actions for a digital document", async () => {
    window.history.replaceState(null, "", "/tables/documents");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes("/api/v1/documents?")) {
          return Promise.resolve(
            jsonResponse({
              documents: [
                {
                  ...documentRecord(),
                  id: "doc-digital",
                  identifier_value: "DIG-1",
                  medium: "DIGITAL",
                  current_use: null,
                },
              ],
              page: {
                total: 1,
                limit: 100,
                offset: 0,
                sort_field: "identifier_value",
                sort_order: "asc",
              },
            }),
          );
        }
        return Promise.resolve(apiResponse(url) ?? jsonResponse({ status: "ok" }));
      }),
    );
    render(<App />);
    fireEvent.click(await screen.findByLabelText("Ações de DIG-1"));
    expect(await screen.findByRole("menuitem", { name: "Editar" })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Marcar em uso" })).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Devolver" })).not.toBeInTheDocument();
  });

  it("lets the recorte be filtered and sorted without a row-actions menu", async () => {
    window.history.replaceState(null, "", "/tables/people");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL) =>
          Promise.resolve(apiResponse(String(input))),
        ),
    );
    const { unmount } = render(<App />);
    expect(await screen.findByRole("heading", { name: "Pessoas" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Exportar recorte" })).not.toBeInTheDocument();
    unmount();

    window.history.replaceState(
      null,
      "",
      "/tables/people?result=11111111-1111-4111-8111-111111111111",
    );
    render(<App />);
    expect(await screen.findByRole("button", { name: "Exportar recorte" })).toBeInTheDocument();
    expect(screen.getByText(/A grade está mostrando o assistente/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Ações de/ })).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText("Filtrar o recorte…")).toBeInTheDocument();
    expect(await screen.findByText("Zélia Costa")).toBeInTheDocument();
    expect(screen.getByText("Ana da Silva")).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText("Filtrar o recorte…"), {
      target: { value: "Zélia" },
    });
    await waitFor(() => {
      expect(screen.queryByText("Ana da Silva")).not.toBeInTheDocument();
    });
    expect(screen.getByText("Zélia Costa")).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText("Filtrar o recorte…"), {
      target: { value: "" },
    });
    await waitFor(() => expect(screen.getByText("Ana da Silva")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("columnheader", { name: /Nome/ }));
    await waitFor(() =>
      expect(screen.getByRole("columnheader", { name: /Nome/ })).toHaveAttribute(
        "aria-sort",
        "ascending",
      ),
    );
    fireEvent.click(screen.getByRole("columnheader", { name: /Nome/ }));
    await waitFor(() =>
      expect(screen.getByRole("columnheader", { name: /Nome/ })).toHaveAttribute(
        "aria-sort",
        "descending",
      ),
    );
    const sheet = document.querySelector(".spreadsheet-table")?.textContent ?? "";
    expect(sheet.indexOf("Zélia Costa")).toBeGreaterThan(-1);
    expect(sheet.indexOf("Zélia Costa")).toBeLessThan(sheet.indexOf("Ana da Silva"));
  });
});

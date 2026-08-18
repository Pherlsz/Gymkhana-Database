import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { sheetInspectorMediaQuery } from "./lib/tables/sheetDefaults";

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
    cpf: "***.***.***-25",
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
  return [...document.querySelectorAll(".spreadsheet-table thead th")].map(
    (cell) => cell.textContent ?? "",
  );
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
  if (url.endsWith("/api/auth/session")) return jsonResponse(session());
  if (url.includes("/api/v1/profiles?")) return jsonResponse(profilePage());
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
    const documents = url.includes("document_type_id=type-cnh")
      ? [cnhRecord()]
      : [documentRecord()];
    return jsonResponse({
      documents,
      page: { total: 1, limit: 100, offset: 0, sort_field: "identifier_value", sort_order: "asc" },
    });
  }
  if (url.includes("/api/v1/bills?")) {
    return jsonResponse({
      bills: [billRecord()],
      page: { total: 1, limit: 100, offset: 0, sort_field: "reference_value", sort_order: "asc" },
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
    vi.unstubAllGlobals();
    window.localStorage.clear();
    window.history.replaceState(null, "", "/");
  });

  beforeEach(() => {
    window.localStorage.clear();
  });

  it("keeps toolbar field filters and has no header funnel", async () => {
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
    expect(await screen.findByRole("button", { name: "Filtros" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Filtros por campo" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Mais filtros" })).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Soma (A=1…Z=26 / dígitos)" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Adicionar coluna" })).not.toBeInTheDocument();
    expect(
      document.querySelectorAll(".spreadsheet-table thead .ant-table-filter-trigger").length,
    ).toBe(0);
  });

  it("opens the toolbar surfaces as overlays that never displace the grid", async () => {
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
    const filters = await screen.findByRole("button", { name: "Filtros" });
    const columns = screen.getByRole("button", { name: "Colunas" });
    expect(filters).toHaveAttribute("aria-haspopup", "dialog");
    expect(filters).toHaveAttribute("aria-expanded", "false");
    expect(columns).toHaveAttribute("aria-haspopup", "dialog");

    fireEvent.click(filters);
    expect(filters).toHaveAttribute("aria-expanded", "true");
    const surface = await screen.findByRole("dialog", { name: "Filtros" });
    // The trigger points at the surface it controls, and the surface is an
    // overlay rather than a sibling panel inside the toolbar.
    expect(filters.getAttribute("aria-controls")).toBe(surface.id);
    expect(document.querySelector(".tables-toolbar")?.contains(surface)).toBe(false);

    fireEvent.keyDown(surface, { key: "Escape" });
    await waitFor(() => expect(filters).toHaveAttribute("aria-expanded", "false"));
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

    fireEvent.click(screen.getByRole("button", { name: "Filtros" }));
    const city = await screen.findByRole("button", { name: /^Cidade/ });
    expect(city).toHaveAttribute("aria-pressed", "true");

    const chipClose = document.querySelector<HTMLElement>(
      ".tables-toolbar__chip .ant-tag-close-icon",
    );
    fireEvent.click(chipClose as HTMLElement);
    await waitFor(() => expect(window.location.search).not.toContain("city="));
  });

  it("commits text filters only after blur", async () => {
    window.history.replaceState(null, "", "/tables/people");
    const fetchMock = vi
      .fn()
      .mockImplementation((input: RequestInfo | URL) =>
        Promise.resolve(apiResponse(String(input))),
      );
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Filtros" }));
    // Filtering is column-first now: choose the column, then narrow it.
    fireEvent.click(screen.getByRole("button", { name: "Cidade" }));
    const city = screen.getByRole("textbox", { name: "Cidade" });
    const initialProfileCalls = fetchMock.mock.calls.filter((call) =>
      String(call[0]).includes("/api/v1/profiles?"),
    ).length;

    fireEvent.change(city, { target: { value: "Porto Alegre" } });
    expect(window.location.search).not.toContain("city=Porto");
    expect(
      fetchMock.mock.calls.filter((call) => String(call[0]).includes("/api/v1/profiles?")),
    ).toHaveLength(initialProfileCalls);

    fireEvent.blur(city);
    await waitFor(() => expect(window.location.search).toContain("city=Porto+Alegre"));
    await waitFor(() =>
      expect(
        fetchMock.mock.calls.filter((call) => String(call[0]).includes("/api/v1/profiles?")),
      ).toHaveLength(initialProfileCalls + 1),
    );
  });

  it("lists people with priority columns first and hides the long tail", async () => {
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
    expect(document.querySelector(".tables-inspector-slot")).toBeNull();
    expect(document.querySelector(".tables-page__workspace > .profile-panel")).toBeNull();
    const headers = headerTexts();
    expect(headers.slice(0, 12)).toEqual([
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
    expect(headers).not.toContain("Idade");
    expect(headers).not.toContain("Soma nome");
    expect(headers).not.toContain("Signo");
    expect(headers).not.toContain("Nome do pai");
    expect(headers.some((text) => text === "Notas")).toBe(false);
    expect(screen.getByText("***.***.***-25")).toBeInTheDocument();
    expect(screen.getByText("1122334455")).toBeInTheDocument();
    expect(document.querySelector(".document-badge__acronym")?.textContent).toBe("RG");
    expect(document.querySelector(".document-badge__mark--physical")?.textContent).toBe("F");
    expect(screen.queryByText("Identidade")).not.toBeInTheDocument();
    const footer = document.querySelector(".spreadsheet-table__footer");
    expect(footer).toBeTruthy();
    expect(document.querySelector(".spreadsheet-table")?.contains(footer)).toBe(false);
    expect(screen.getByLabelText("Linhas por página")).toBeInTheDocument();
  });

  it("lets the operator reveal a hidden people column and keeps nome locked", async () => {
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
    fireEvent.click(screen.getByRole("button", { name: "Colunas" }));
    const father = screen.getByRole("checkbox", { name: "Nome do pai" });
    expect(father).not.toBeChecked();
    fireEvent.click(father);
    await waitFor(() => {
      expect(headerTexts()).toContain("Nome do pai");
      expect(decodeURIComponent(window.location.search)).toContain("cols=father_name");
    });
    const city = screen.getByRole("checkbox", { name: "Cidade" });
    expect(city).not.toBeDisabled();
    fireEvent.click(city);
    await waitFor(() => {
      expect(headerTexts()).not.toContain("Cidade");
      expect(decodeURIComponent(window.location.search)).toContain("-city");
    });
    expect(screen.getByRole("checkbox", { name: "Nome (Sempre visível)" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Restaurar padrão" }));
    await waitFor(() => {
      expect(headerTexts()).not.toContain("Nome do pai");
      expect(headerTexts()).toContain("Cidade");
      expect(window.location.search.includes("cols=")).toBe(false);
    });
  });

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

  it("filters by column from one searchable list, with Tipo promoted first", async () => {
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
    expect(await screen.findByRole("heading", { name: "Documentos" })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Contagem de caracteres" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Soma (A=1…Z=26 / dígitos)" }),
    ).not.toBeInTheDocument();
    // Closed, the surface contributes nothing to the bar; the grid keeps its height.
    expect(screen.queryByRole("textbox", { name: "Buscar campo…" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Filtros" }));
    // One list, no quick/advanced split and no "Adicionar filtro" step.
    expect(screen.getByRole("textbox", { name: "Buscar campo…" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Adicionar filtro" })).not.toBeInTheDocument();
    const fields = [...document.querySelectorAll(".filter-surface__name")].map(
      (node) => node.textContent,
    );
    expect(fields[0]).toBe("Tipo");
    expect(screen.getByRole("button", { name: "Status" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Suporte" })).toBeInTheDocument();
    expect(screen.queryByPlaceholderText("Categoria")).not.toBeInTheDocument();
    fireEvent.click(await screen.findByRole("button", { name: "Categoria" }));
    expect(document.querySelector(".filter-surface__control .ant-select")).not.toBeNull();
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
    fireEvent.click(screen.getByRole("button", { name: "Colunas" }));
    fireEvent.click(screen.getByRole("button", { name: "Mostrar todas" }));
    await waitFor(() => {
      expect(screen.getByText("Centro")).toBeInTheDocument();
    });
    expect(screen.queryByText("Soma CPF")).not.toBeInTheDocument();
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
    // Tipo is no longer a Select pinned to the bills bar; it is the promoted
    // first column filter, identical to documents.
    expect(screen.queryByRole("combobox", { name: "Tipo" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Nova conta" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Filtros" }));
    fireEvent.click(screen.getByRole("button", { name: "Tipo" }));
    expect(screen.getByRole("combobox", { name: "Tipo" })).toBeInTheDocument();
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
    expect(within(inspector).queryByRole("textbox")).not.toBeInTheDocument();
    expect(
      within(inspector).getByRole("button", { name: "Documentos desta pessoa" }),
    ).toBeInTheDocument();
    expect(
      within(inspector).getByRole("button", { name: "Contas desta pessoa" }),
    ).toBeInTheDocument();
    expect(within(inspector).getByRole("heading", { name: "Identificação" })).toBeInTheDocument();
    expect(
      within(inspector).queryByRole("heading", { name: "Contato e endereço" }),
    ).not.toBeInTheDocument();
    expect(within(inspector).getByText("Documentos")).toBeInTheDocument();
    expect(within(inspector).getByText("14/08/1990")).toBeInTheDocument();
    expect(within(inspector).getByText("O+")).toBeInTheDocument();
    fireEvent.click(within(inspector).getByRole("button", { name: "Ver mais dados" }));
    expect(
      within(inspector).getByRole("heading", { name: "Contato e endereço" }),
    ).toBeInTheDocument();
    expect(within(inspector).getByRole("button", { name: "Ver menos dados" })).toHaveAttribute(
      "aria-expanded",
      "true",
    );
    expect(within(inspector).queryByText("Exclusão permanente")).not.toBeInTheDocument();
    expect(document.querySelector(".tables-page__workspace > .profile-panel")).toBeTruthy();
    expect(document.querySelector(".tables-inspector-sheet")).toBeNull();
    expect(document.querySelector(".spreadsheet-table")).toBeTruthy();
    expect(document.querySelector(".profiles-cards")).toBeNull();
    fireEvent.click(within(inspector).getByRole("button", { name: "Editar" }));
    expect(await within(inspector).findByLabelText("Nome completo")).toBeInTheDocument();
    fireEvent.click(within(inspector).getByRole("button", { name: "Cancelar" }));
    await waitFor(() => {
      expect(within(inspector).queryByRole("textbox")).not.toBeInTheDocument();
    });
    fireEvent.click(within(inspector).getByRole("button", { name: "Fechar" }));
    await waitFor(() => {
      expect(
        screen.queryByRole("complementary", { name: "Detalhes da pessoa" }),
      ).not.toBeInTheDocument();
    });
    expect(document.querySelector(".tables-inspector-slot")).toBeNull();
  });

  it("opens the inspector in a standard bottom sheet under 840px without replacing the spreadsheet", async () => {
    vi.stubGlobal(
      "matchMedia",
      (query: string) =>
        ({
          matches: query.includes(sheetInspectorMediaQuery()),
          media: query,
          onchange: null,
          addListener: () => {},
          removeListener: () => {},
          addEventListener: () => {},
          removeEventListener: () => {},
          dispatchEvent: () => false,
        }) as MediaQueryList,
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
    fireEvent.click(screen.getByText("Ana da Silva"));
    const inspector = await screen.findByRole("complementary", { name: "Detalhes da pessoa" });
    expect(inspector.closest(".tables-inspector-sheet")).toBeTruthy();
    expect(document.querySelector(".tables-page__workspace > .profile-panel")).toBeNull();
    expect(document.querySelector(".spreadsheet-table")).toBeTruthy();
    expect(document.querySelector(".profiles-cards")).toBeNull();
    expect(within(inspector).getByRole("button", { name: "Fechar" })).toBeInTheDocument();
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
    expect(await screen.findByRole("button", { name: /^Excluir$/ })).toBeInTheDocument();
    expect(screen.queryByText("Exclusão permanente")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^Excluir$/ }));
    expect(screen.getByText("Exclusão permanente")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Confirmar")).toBeInTheDocument();
  });

  it("filters documents from the inspector without leaving the person", async () => {
    window.history.replaceState(null, "", "/tables/people");
    const fetchMock = vi
      .fn()
      .mockImplementation((input: RequestInfo | URL) =>
        Promise.resolve(apiResponse(String(input))),
      );
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    fireEvent.click(await screen.findByText("Ana da Silva"));
    const inspector = await screen.findByRole("complementary", { name: "Detalhes da pessoa" });
    fireEvent.click(within(inspector).getByRole("button", { name: "Documentos desta pessoa" }));
    expect(await screen.findByRole("heading", { name: "Documentos" })).toBeInTheDocument();
    expect(screen.getByRole("complementary", { name: "Detalhes da pessoa" })).toBeInTheDocument();
    // The chip names its field: the value alone used to be all a screen reader got.
    await waitFor(() =>
      expect(document.querySelector(".tables-toolbar__chip")?.textContent).toBe(
        "Pessoa: Ana da Silva",
      ),
    );
    await waitFor(() => {
      const documentCalls = fetchMock.mock.calls
        .map((call) => String(call[0]))
        .filter((url) => url.includes("/api/v1/documents?"));
      expect(documentCalls.some((url) => url.includes(`owner_profile_id=${anaProfile().id}`))).toBe(
        true,
      );
    });
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
    expect(await within(inspector).findByLabelText("Nome completo")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Bruno Souza"));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Descartar e trocar" }));
    await waitFor(() => {
      expect(
        within(screen.getByRole("complementary", { name: "Detalhes da pessoa" })).getByRole(
          "heading",
          { name: "Bruno Souza" },
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows a loading state in the inspector while the person is fetched", async () => {
    window.history.replaceState(null, "", "/tables/bills");
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
        return Promise.resolve(apiResponse(url) ?? jsonResponse({ status: "ok" }));
      }),
    );
    render(<App />);
    fireEvent.click(await screen.findByText("UC-100"));
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
});

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "content-type": "application/json" },
  });
}

function authenticatedSession() {
  return {
    authenticated: true,
    user: { login: "member", display_name: "Member Name", role: "EXTERNAL" },
  };
}

function profilePage() {
  return {
    profiles: [
      {
        id: "019bf789-4400-7f12-9abc-123456789abc",
        full_name: "Ana da Silva",
        social_name: "Ana",
        cpf: "52998224725",
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
        version: 1,
        created_at: "2026-07-15T12:00:00Z",
        updated_at: "2026-07-15T12:00:00Z",
      },
    ],
    page: { total: 1, limit: 100, offset: 0, sort_field: "full_name", sort_order: "asc" },
  };
}

function emptyResourcePage() {
  return {
    documents: [],
    bills: [],
    page: { total: 0, limit: 1, offset: 0, sort_field: "updated_at", sort_order: "desc" },
  };
}

function documentType(id: string, label: string, technicalKey: string, count = 0) {
  return {
    id,
    technical_key: technicalKey,
    label,
    active: true,
    uniqueness_policy: "PER_PROFILE",
    validation_regex: "",
    date_required: false,
    count,
    version: 1,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function billType(id: string, label: string, technicalKey: string) {
  return {
    id,
    technical_key: technicalKey,
    label,
    active: true,
    count: 0,
    version: 1,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function documentInUse(id: string, typeLabel: string) {
  return {
    id,
    identifier_value: `ident-${id}`,
    document_type_id: `dt-${typeLabel}`,
    type: { id: `dt-${typeLabel}`, label: typeLabel },
    owner_profile_id: "019bf789-4400-7f12-9abc-123456789abc",
    notes: "",
    status: "IN_USE",
    current_use: {
      holder_profile_id: "019bf789-4400-7f12-9abc-123456789abc",
      assigned_at: "2026-08-01T12:00:00Z",
    },
    version: 1,
    created_at: "2026-08-01T12:00:00Z",
    updated_at: "2026-08-01T12:00:00Z",
  };
}

function billInUse(id: string, typeLabel: string) {
  return {
    id,
    reference_value: `ref-${id}`,
    bill_type_id: `bt-${typeLabel}`,
    type: { id: `bt-${typeLabel}`, label: typeLabel },
    owner_profile_id: "019bf789-4400-7f12-9abc-123456789abc",
    notes: "",
    status: "IN_USE",
    current_use: {
      holder_profile_id: "019bf789-4400-7f12-9abc-123456789abc",
      assigned_at: "2026-08-01T12:00:00Z",
    },
    version: 1,
    created_at: "2026-08-01T12:00:00Z",
    updated_at: "2026-08-01T12:00:00Z",
  };
}

function homeOverviewResponse(
  url: string,
  options?: { rgCount?: number; inUse?: { documents: unknown[]; bills: unknown[] } },
): Response | undefined {
  if (url.includes("/api/v1/profiles?")) {
    return jsonResponse(profilePage());
  }
  if (url.includes("status=IN_USE") && url.includes("/api/v1/documents?")) {
    const inUse = options?.inUse;
    if (!inUse) return jsonResponse(emptyResourcePage());
    return jsonResponse({
      documents: inUse.documents,
      page: { total: inUse.documents.length, limit: 50, offset: 0 },
    });
  }
  if (url.includes("status=IN_USE") && url.includes("/api/v1/bills?")) {
    const inUse = options?.inUse;
    if (!inUse) return jsonResponse(emptyResourcePage());
    return jsonResponse({
      bills: inUse.bills,
      page: { total: inUse.bills.length, limit: 50, offset: 0 },
    });
  }
  if (url.includes("/api/v1/document-types")) {
    return jsonResponse({
      types: [documentType("type-rg", "RG", "rg", options?.rgCount ?? 0)],
      page: { total: 1, limit: 1000, offset: 0 },
    });
  }
  if (url.includes("/api/v1/bill-types")) {
    return jsonResponse({
      types: [billType("type-energy", "Energia", "energia")],
      page: { total: 1, limit: 1000, offset: 0 },
    });
  }
  if (url.includes("/api/v1/documents?")) {
    return jsonResponse(emptyResourcePage());
  }
  if (url.includes("/api/v1/bills?")) {
    return jsonResponse(emptyResourcePage());
  }
  if (url.includes("/api/v1/custom-fields?")) {
    return jsonResponse({ fields: [], page: { total: 0, limit: 1000, offset: 0 } });
  }
  if (url.includes("/api/v1/custom-entity-types")) {
    return jsonResponse({ types: [], page: { total: 0, limit: 1000, offset: 0 } });
  }
  if (url.includes("/api/v1/custom-entities?")) {
    return jsonResponse({ entities: [], page: { total: 0, limit: 1, offset: 0 } });
  }
  return undefined;
}

describe("App", () => {
  beforeEach(() => window.history.replaceState(null, "", "/"));
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    window.localStorage.removeItem("gymkhana-nav-collapsed");
    window.localStorage.removeItem("gymkhana-theme");
    document.documentElement.classList.remove("dark");
  });

  it("shows the authenticated dashboard shell", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(authenticatedSession()));
      const overview = homeOverviewResponse(url);
      if (overview) return Promise.resolve(overview);
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    expect(
      await screen.findByRole("heading", { name: "Bem-vindo de volta, Member Name" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("Nenhum item em uso")).toBeInTheDocument();
    expect(screen.getByText("Tabelas · em posse")).toBeInTheDocument();
    expect(screen.queryByText("ver todas")).not.toBeInTheDocument();
    expect(await screen.findByRole("link", { name: /Abrir pessoas/ })).toBeInTheDocument();
    expect(screen.queryByText("Dados pessoais")).not.toBeInTheDocument();
    expect(screen.getByText("Documentos civis")).toBeInTheDocument();
    expect(screen.getByText("Trabalho e profissional")).toBeInTheDocument();
    expect(screen.queryByText("Identidade")).not.toBeInTheDocument();
    expect(screen.queryByText("CREA / OAB")).not.toBeInTheDocument();
    expect(screen.getByText("CTPS")).toBeInTheDocument();
    expect(screen.getByText("Conta de luz")).toBeInTheDocument();
    expect(screen.getByText("Conta de água")).toBeInTheDocument();
    expect(screen.getByText("Internet")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Documentos" })).toBeInTheDocument();
    expect(document.querySelectorAll(".home-catalog__row-count").length).toBe(0);
    expect((await screen.findAllByText("cadastros")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("RG").length).toBeGreaterThan(0);
    expect(screen.queryByText("56.401")).not.toBeInTheDocument();
    expect(screen.queryByText("Nenhum registro")).not.toBeInTheDocument();
    expect(screen.queryByText("Nova tabela")).not.toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /Início/ }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole("link", { name: /Pessoas/ }).length).toBeGreaterThan(0);
    const navSearch = screen.getAllByRole("searchbox", { name: "Buscar" })[0];
    if (!navSearch) throw new Error("expected navbar search");
    fireEvent.change(navSearch, { target: { value: "tipo:" } });
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(
      fetchMock.mock.calls.some((call) => String(call[0]).includes("/api/v1/search/catalog")),
    ).toBe(false);
    expect(screen.getAllByRole("button", { name: /Tabelas/ }).length).toBeGreaterThan(0);
    expect(screen.queryByRole("link", { name: "Consultar" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Tarefas" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "OCR" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Operações" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Duplicidades" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Chat IA" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Administração" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Chat IA" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Criar formulário/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Abrir menu da conta" })).toBeInTheDocument();
    expect(screen.getByText("Member Name")).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Sair" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Abrir menu da conta" }));
    expect(await screen.findByText("Configurações")).toBeInTheDocument();
    expect(screen.getByText("Aparência")).toBeInTheDocument();
    expect(screen.getByText("Sair")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Recolher menu" })).toBeInTheDocument();
    expect(screen.getByText("Gymkhana Database")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Novo cadastro/ })).toBeInTheDocument();
    expect(screen.getByText("Itens em uso")).toBeInTheDocument();
  });

  it("splits in-use headline by kind and expands chips locally", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(authenticatedSession()));
      const overview = homeOverviewResponse(url, {
        inUse: {
          documents: [
            documentInUse("doc-1", "RG"),
            documentInUse("doc-2", "RG"),
            documentInUse("doc-3", "CNH"),
            documentInUse("doc-4", "CTPS"),
            documentInUse("doc-5", "Título Eleitoral"),
            documentInUse("doc-6", "Cartão SUS"),
            documentInUse("doc-7", "Passaporte"),
            documentInUse("doc-8", "PIS"),
            documentInUse("doc-9", "CRM"),
          ],
          bills: [billInUse("bill-1", "Energia")],
        },
      });
      if (overview) return Promise.resolve(overview);
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    expect(await screen.findByText("Itens em uso")).toBeInTheDocument();
    expect(await screen.findByText("10")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Abrir documentos" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Abrir contas" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "+1 tipos" }));
    expect(screen.getByRole("button", { name: "Mostrar menos" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Mostrar menos" }));
    expect(screen.getByRole("button", { name: "+1 tipos" })).toBeInTheDocument();
  });

  it("shows exemplar counts on Home when types have physical or digital stock", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(authenticatedSession()));
      const overview = homeOverviewResponse(url, { rgCount: 56401 });
      if (overview) return Promise.resolve(overview);
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    expect(await screen.findByText("56.401")).toBeInTheDocument();
    expect(screen.getByText("56.401")).toHaveClass("home-catalog__row-count");
    expect(document.querySelectorAll(".home-catalog__row-count--empty").length).toBe(0);
    expect(screen.queryByText("Nenhum registro")).not.toBeInTheDocument();
  });

  it("shows Google login when the protected session returns unauthorized", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        if (String(input).endsWith("/api/auth/session"))
          return Promise.resolve(
            jsonResponse(
              { error: { code: "unauthorized", message: "Authentication is required" } },
              401,
            ),
          );
        return Promise.resolve(jsonResponse({ status: "ok" }));
      }),
    );
    render(<App />);
    expect(await screen.findByRole("button", { name: /Entrar com Google/ })).toBeInTheDocument();
  });

  it("opens the people spreadsheet", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(authenticatedSession()));
      const overview = homeOverviewResponse(url);
      if (overview) return Promise.resolve(overview);
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    await screen.findByRole("heading", { name: "Bem-vindo de volta, Member Name" });
    const peopleLinks = screen.getAllByRole("link", { name: /Pessoas/ });
    const peopleLink = peopleLinks[0];
    if (!peopleLink) throw new Error("expected a Pessoas link");
    fireEvent.click(peopleLink);
    expect(await screen.findByRole("heading", { name: "Pessoas" })).toBeInTheDocument();
    expect(await screen.findByText("Ana da Silva")).toBeInTheDocument();
    expect(screen.getAllByText("Documentos").length).toBeGreaterThan(1);
    expect(screen.getAllByText("CPF").length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Filtros" })).toBeInTheDocument();
    expect(screen.queryByText("A ser desenvolvido")).not.toBeInTheDocument();
  });

  it("renders the global search page instead of the placeholder", async () => {
    window.history.replaceState(null, "", "/search?q=Ana");
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(authenticatedSession()));
      if (url.includes("/api/v1/search/catalog")) {
        return Promise.resolve(
          jsonResponse({
            modules: [
              { key: "profiles", label: "Pessoas" },
              { key: "documents", label: "Documentos" },
            ],
            fields: [
              { key: "profile.full_name", module: "profiles", label: "Nome completo", kind: "text" },
            ],
            operators: [],
            limits: { maximum_terms: 5 },
          }),
        );
      }
      if (url.includes("/api/v1/search")) {
        return Promise.resolve(
          jsonResponse({
            results: [],
            page: { total: 0, limit: 50, offset: 0 },
          }),
        );
      }
      const overview = homeOverviewResponse(url);
      if (overview) return Promise.resolve(overview);
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    expect(
      await screen.findByRole("heading", { name: "Buscar dados autorizados" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Em desenvolvimento")).not.toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Pessoas" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Documentos" })).toBeInTheDocument();
    expect(screen.getByText("Campos específicos")).toBeInTheDocument();

    const searchPosts = () =>
      fetchMock.mock.calls.filter(([input, init]) => {
        const url = String(input);
        return (
          url.includes("/api/v1/search") &&
          !url.includes("catalog") &&
          (init as RequestInit | undefined)?.method === "POST"
        );
      });
    const input = screen.getByRole("searchbox", { name: "Buscar dados autorizados" });
    await waitFor(() => expect(searchPosts().length).toBeGreaterThan(0));
    const postedBeforeTyping = searchPosts().length;
    fireEvent.change(input, { target: { value: "Ana Maria" } });
    expect(searchPosts()).toHaveLength(postedBeforeTyping);

    fireEvent.blur(input);
    await waitFor(() => expect(window.location.search).toContain("q=Ana+Maria"));

    fireEvent.click(screen.getByRole("button", { name: "Documentos" }));
    await waitFor(() => {
      const bodies = searchPosts().map(([, init]) =>
        JSON.parse(String((init as RequestInit).body)),
      );
      expect(bodies.some((body: { modules?: string[] }) => body.modules?.includes("profiles"))).toBe(
        true,
      );
    });
  });

  it("does not let an admin create tables from home", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session")) {
        return Promise.resolve(
          jsonResponse({
            authenticated: true,
            user: { login: "admin", display_name: "Admin Name", role: "ADMIN" },
          }),
        );
      }
      const overview = homeOverviewResponse(url);
      if (overview) return Promise.resolve(overview);
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    expect(
      await screen.findByRole("heading", { name: "Bem-vindo de volta, Admin Name" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Nova tabela")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Criar formulário/ })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Abrir menu da conta" }));
    const accountMenu = await screen.findByRole("menu");
    expect(within(accountMenu).queryByText("Administração")).not.toBeInTheDocument();
  });

  it("toggles html.dark from the account appearance switch without double-toggling", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(authenticatedSession()));
      const overview = homeOverviewResponse(url);
      if (overview) return Promise.resolve(overview);
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    await screen.findByRole("button", { name: "Abrir menu da conta" });
    expect(document.documentElement).not.toHaveClass("dark");

    fireEvent.click(screen.getByRole("button", { name: "Abrir menu da conta" }));
    const appearanceSwitch = await screen.findByRole("switch");
    fireEvent.click(appearanceSwitch);
    await waitFor(() => expect(document.documentElement).toHaveClass("dark"));
    expect(window.localStorage.getItem("gymkhana-theme")).toBe("dark");
    expect(screen.getByText("Aparência")).toBeInTheDocument();
    expect(appearanceSwitch).toBeChecked();

    fireEvent.click(appearanceSwitch);
    await waitFor(() => expect(document.documentElement).not.toHaveClass("dark"));
    expect(window.localStorage.getItem("gymkhana-theme")).toBe("light");
  });

  it("opens the people spreadsheet from the tables URL", async () => {
    window.history.replaceState(null, "", "/tables/people");
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(authenticatedSession()));
      const overview = homeOverviewResponse(url);
      if (overview) return Promise.resolve(overview);
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    expect(await screen.findByRole("heading", { name: "Pessoas" })).toBeInTheDocument();
    expect(await screen.findByText("Ana da Silva")).toBeInTheDocument();
  });
});

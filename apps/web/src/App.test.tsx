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

function homeOverviewResponse(url: string, options?: { rgCount?: number }): Response | undefined {
  if (url.includes("/api/v1/profiles?")) {
    return jsonResponse(profilePage());
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
    expect(await screen.findByText("Nenhum documento em uso")).toBeInTheDocument();
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
    expect((await screen.findAllByText("em posse")).length).toBeGreaterThan(0);
    expect((await screen.findAllByText("cadastros")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("RG").length).toBeGreaterThan(0);
    expect(screen.queryByText("56.401")).not.toBeInTheDocument();
    expect(screen.queryByText("Nenhum registro")).not.toBeInTheDocument();
    expect(screen.queryByText("Nova tabela")).not.toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /Início/ }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole("link", { name: /Pessoas/ }).length).toBeGreaterThan(0);
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
    expect(screen.getByText("Documentos em uso")).toBeInTheDocument();
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
    expect(screen.getAllByText("em posse").length).toBeGreaterThan(0);
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

  it("shows the search placeholder", async () => {
    window.history.replaceState(null, "", "/search?q=Ana");
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(authenticatedSession()));
      if (url.includes("/api/v1/search/catalog")) {
        return Promise.resolve(
          jsonResponse({
            modules: [{ key: "profiles", label: "Pessoas" }],
            fields: [],
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
    expect(await screen.findByRole("heading", { name: "Em desenvolvimento" })).toBeInTheDocument();
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
});

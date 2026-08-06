import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
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

describe("App", () => {
  beforeEach(() => window.history.replaceState(null, "", "/"));
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("shows the authenticated application shell and signs out", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/logout") && init?.method === "POST")
        return Promise.resolve(new Response(null, { status: 204 }));
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(authenticatedSession()));
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    expect(await screen.findByText("Member Name")).toBeInTheDocument();
    expect(screen.getByText("@member · Membro")).toBeInTheDocument();
    expect(screen.getByText("Sessão ativa")).toBeInTheDocument();
    expect(screen.getByText("v0.2.1")).toBeInTheDocument();
    expect(screen.getByText("v0.3.0")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Sair" }));
    expect(await screen.findByText("Faça login para continuar")).toBeInTheDocument();
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
    expect(await screen.findByText("Faça login para continuar")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Entrar com Google/ })).toBeInTheDocument();
  });

  it("navigates to Profiles, keeps list state in the URL, and saves an inline edit", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(authenticatedSession()));
      if (url.includes("/api/v1/profiles?") && (!init?.method || init.method === "GET"))
        return Promise.resolve(jsonResponse(profilePage()));
      if (url.includes("/api/v1/profiles/") && init?.method === "PUT")
        return Promise.resolve(
          jsonResponse({ ...profilePage().profiles[0], full_name: "Ana Souza", version: 2 }),
        );
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    await screen.findByText("Member Name");
    fireEvent.click(screen.getByRole("link", { name: "Pessoas" }));
    expect(await screen.findAllByText("Ana da Silva")).not.toHaveLength(0);
    const nameFilter = screen.getByLabelText("Nome");
    fireEvent.change(nameFilter, { target: { value: "Ana" } });
    await waitFor(() => expect(window.location.search).toContain("full_name=Ana"));
    const editor = await screen.findByLabelText("full_name de Ana da Silva");
    fireEvent.change(editor, { target: { value: "Ana Souza" } });
    fireEvent.blur(editor);
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/profiles/019bf789-4400-7f12-9abc-123456789abc"),
        expect.objectContaining({ method: "PUT" }),
      ),
    );
  });

  it("executes global Search with catalog filters and URL-backed terms", async () => {
    window.history.replaceState(
      null,
      "",
      "/search?q=Ana&page=1&limit=50&sort=relevance&order=desc",
    );
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(authenticatedSession()));
      if (url.endsWith("/api/v1/search/catalog"))
        return Promise.resolve(
          jsonResponse({
            modules: [{ key: "profiles", label: "Pessoas" }],
            fields: [
              {
                key: "profile.full_name",
                module: "profiles",
                label: "Nome completo",
                kind: "text",
              },
            ],
            limits: {
              maximum_terms: 5,
              maximum_term_length: 128,
              maximum_fields: 40,
              maximum_page_size: 100,
              maximum_offset: 10000,
            },
          }),
        );
      if (url.endsWith("/api/v1/search") && init?.method === "POST")
        return Promise.resolve(
          jsonResponse({
            results: [
              {
                module: "profiles",
                entity_kind: "profile",
                entity_id: "019bf789-4400-7f12-9abc-123456789abc",
                profile_id: "019bf789-4400-7f12-9abc-123456789abc",
                target_kind: "profile",
                target_id: "019bf789-4400-7f12-9abc-123456789abc",
                entity_label: "Ana da Silva",
                field_key: "profile.full_name",
                field_label: "Nome completo",
                preview: "Ana da Silva",
                score: 1080,
                updated_at: "2026-07-17T12:00:00Z",
              },
            ],
            page: { total: 1, limit: 50, offset: 0, sort: "relevance", sort_order: "desc" },
          }),
        );
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "Buscar dados autorizados" }),
    ).toBeInTheDocument();
    expect(await screen.findAllByText("Ana da Silva")).not.toHaveLength(0);
    expect(screen.getByRole("table", { name: "Resultados da busca global" })).toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: "Documentos" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: /physical/i })).not.toBeInTheDocument();
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/search"),
        expect.objectContaining({ method: "POST" }),
      ),
    );

    fireEvent.change(screen.getByLabelText("Termos — um por linha"), {
      target: { value: "Ana Maria\n001" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Buscar" }));
    await waitFor(() =>
      expect(new URLSearchParams(window.location.search).get("q")).toBe("Ana Maria\n001"),
    );
  });
});

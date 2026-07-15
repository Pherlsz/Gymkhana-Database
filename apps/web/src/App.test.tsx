import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "content-type": "application/json" },
  });
}

describe("App", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("shows the authenticated application shell and signs out", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/logout") && init?.method === "POST") {
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      if (url.endsWith("/api/auth/session")) {
        return Promise.resolve(
          jsonResponse({
            authenticated: true,
            user: { login: "member", display_name: "Member Name", role: "MEMBER" },
            capabilities: { manage_users: false },
          }),
        );
      }
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);

    expect(await screen.findByText("Member Name")).toBeInTheDocument();
    expect(screen.getByText("@member · Membro")).toBeInTheDocument();
    expect(screen.getByText("Sessão ativa")).toBeInTheDocument();
    expect(screen.queryByText("Administração de usuários")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Sair" }));
    expect(await screen.findByText("Autenticação necessária")).toBeInTheDocument();
  });

  it("shows GitHub login when the protected session returns unauthorized", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        if (String(input).endsWith("/api/auth/session")) {
          return Promise.resolve(
            jsonResponse(
              {
                error: { code: "unauthorized", message: "Authentication is required" },
              },
              401,
            ),
          );
        }
        return Promise.resolve(jsonResponse({ status: "ok" }));
      }),
    );

    render(<App />);

    expect(await screen.findByText("Autenticação necessária")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Entrar com GitHub" })).toBeInTheDocument();
  });

  it("loads and updates users when the session grants user management", async () => {
    const user = {
      id: "11111111-1111-4111-8111-111111111111",
      login: "member",
      display_name: "Member Name",
      role: "MEMBER",
      active: true,
      version: 1,
      protected: false,
    };
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session")) {
        return Promise.resolve(
          jsonResponse({
            authenticated: true,
            user: { login: "admin", display_name: "Admin Name", role: "ADMIN" },
            capabilities: { manage_users: true },
          }),
        );
      }
      if (url.endsWith("/api/admin/users") && !init?.method) {
        return Promise.resolve(jsonResponse({ users: [user] }));
      }
      if (url.endsWith(`/api/admin/users/${user.id}/access`) && init?.method === "PATCH") {
        return Promise.resolve(jsonResponse({ ...user, role: "ADMIN", version: 2 }));
      }
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);

    expect(await screen.findByText("Administração de usuários")).toBeInTheDocument();
    expect(await screen.findByText("@member")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Papel"), { target: { value: "ADMIN" } });
    fireEvent.click(screen.getByRole("button", { name: "Salvar acesso" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        `/api/admin/users/${user.id}/access`,
        expect.objectContaining({ method: "PATCH", credentials: "include" }),
      );
      expect(screen.getByLabelText("Papel")).toHaveValue("ADMIN");
    });
  });
});

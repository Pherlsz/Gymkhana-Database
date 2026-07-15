import { cleanup, fireEvent, render, screen } from "@testing-library/react";
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
    expect(screen.getByText("v0.2.1")).toBeInTheDocument();
    expect(screen.getByText("v0.3.0")).toBeInTheDocument();

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
});

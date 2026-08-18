import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "./i18n";
import { LoginScreen } from "./LoginScreen";
import { ThemeProvider } from "./theme";

function renderLogin(onLogin = () => undefined) {
  return render(
    <I18nProvider locale="pt-BR">
      <ThemeProvider>
        <LoginScreen onLogin={onLogin} />
      </ThemeProvider>
    </I18nProvider>,
  );
}

describe("LoginScreen", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    localStorage.removeItem("gymkhana-theme");
    document.documentElement.classList.remove("dark");
  });

  it("renders the GPT-Staging login layout and starts Google login", () => {
    const onLogin = vi.fn();
    const { container } = renderLogin(onLogin);

    expect(screen.getByRole("heading", { name: "Gymkhana Database" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ativar tema escuro" })).toBeInTheDocument();
    expect(container.querySelector(".login-mascot-img")).toHaveAttribute("src", "/Gampa.png");
    expect(container.querySelectorAll(".login-orb")).toHaveLength(2);
    expect(container.querySelector(".ant-card")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Entrar com Google/ }));
    expect(onLogin).toHaveBeenCalledTimes(1);
  });

  it("shows the development bypass only in the development build used by tests", () => {
    renderLogin();
    expect(screen.getByRole("button", { name: /Dev Login/ })).toBeInTheDocument();
  });

  it("starts development login on the same origin as the Vite session", () => {
    const assign = vi.fn();
    vi.stubGlobal("location", { assign });
    renderLogin();

    fireEvent.click(screen.getByRole("button", { name: /Dev Login/ }));

    expect(assign).toHaveBeenCalledTimes(1);
    expect(assign).toHaveBeenCalledWith("/api/auth/dev-login");
  });

  it("toggles html.dark from the login theme button", async () => {
    renderLogin();
    expect(document.documentElement).not.toHaveClass("dark");

    fireEvent.click(screen.getByRole("button", { name: "Ativar tema escuro" }));
    await waitFor(() => expect(document.documentElement).toHaveClass("dark"));

    fireEvent.click(screen.getByRole("button", { name: "Ativar tema claro" }));
    await waitFor(() => expect(document.documentElement).not.toHaveClass("dark"));
  });
});

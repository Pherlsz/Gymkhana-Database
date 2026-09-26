import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "./i18n";
import { LoginScreen } from "./LoginScreen";
import { ThemeProvider } from "./theme";

function renderLogin(onLogin = () => undefined) {
  return render(
    <I18nProvider locale="pt-BR">
      <LoginScreen onLogin={onLogin} />
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
    expect(screen.queryByRole("button", { name: /tema/i })).toBeNull();
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
    expect(String(assign.mock.calls[0]?.[0])).toMatch(/\/api\/auth\/dev-login$/);
  });

  it("stays dark even when the stored theme is light", () => {
    localStorage.setItem("gymkhana-theme", "light");
    render(
      <ThemeProvider forceDark>
        <I18nProvider locale="pt-BR">
          <LoginScreen onLogin={() => undefined} />
        </I18nProvider>
      </ThemeProvider>,
    );
    expect(document.documentElement).toHaveClass("dark");
    expect(screen.queryByRole("button", { name: /tema/i })).toBeNull();
  });
});

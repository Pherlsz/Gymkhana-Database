import { ConfigProvider } from "antd";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "./i18n";
import { LoginScreen } from "./LoginScreen";

function renderLogin(onLogin = () => undefined) {
  return render(
    <I18nProvider locale="pt-BR">
      <ConfigProvider>
        <LoginScreen onLogin={onLogin} />
      </ConfigProvider>
    </I18nProvider>,
  );
}

describe("LoginScreen", () => {
  afterEach(() => cleanup());

  it("renders the legacy layout with Ant Design components and starts Google login", () => {
    const onLogin = vi.fn();
    const { container } = renderLogin(onLogin);

    expect(screen.getByRole("heading", { name: "Gymkhana Database" })).toBeInTheDocument();
    expect(screen.getByText("Faça login para continuar")).toBeInTheDocument();
    expect(container.querySelector(".login-card-shell__glow")).toBeInTheDocument();
    expect(container.querySelector(".ant-card")).toBeInTheDocument();
    expect(container.querySelector(".ant-divider")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Entrar com Google/ }));
    expect(onLogin).toHaveBeenCalledTimes(1);
  });

  it("shows the development bypass only in the development build used by tests", () => {
    renderLogin();
    expect(screen.getByRole("button", { name: /Dev Login/ })).toBeInTheDocument();
  });
});

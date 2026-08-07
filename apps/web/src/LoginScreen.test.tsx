import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { LoginScreen } from "./LoginScreen";

describe("LoginScreen", () => {
  afterEach(() => cleanup());

  it("renders the legacy login card and starts Google login", () => {
    const onLogin = vi.fn();
    const { container } = render(<LoginScreen onLogin={onLogin} />);

    expect(screen.getByRole("heading", { name: "Gymkhana Database" })).toBeInTheDocument();
    expect(screen.getByText("Faça login para continuar")).toBeInTheDocument();
    expect(container.querySelector(".login-card-shell__glow")).toBeInTheDocument();
    expect(container.querySelector(".ant-card")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Entrar com Google" }));
    expect(onLogin).toHaveBeenCalledTimes(1);
  });

  it("shows the development bypass only in the development build used by tests", () => {
    render(<LoginScreen onLogin={() => undefined} />);
    expect(screen.getByRole("button", { name: "Dev Login" })).toBeInTheDocument();
  });
});

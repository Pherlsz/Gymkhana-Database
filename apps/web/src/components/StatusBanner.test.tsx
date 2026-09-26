import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { ReactNode } from "react";
import { I18nProvider } from "../i18n";
import { APIRequestError } from "../lib/api/client";
import { StatusBanner } from "./StatusBanner";

function renderBanner(node: ReactNode) {
  return render(<I18nProvider locale="pt-BR">{node}</I18nProvider>);
}

describe("StatusBanner", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders an error banner from errorAlertProps", () => {
    renderBanner(
      <StatusBanner error={new Error("detalhe da API")} title="Não foi possível salvar" />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Não foi possível salvar");
    expect(screen.getByRole("alert")).toHaveTextContent("detalhe da API");
  });

  it("maps HTTP 409 to a warning using conflictTitle", () => {
    const error = new APIRequestError("version conflict", {
      status: 409,
      code: undefined,
      requestId: undefined,
    });
    renderBanner(
      <StatusBanner
        conflictTitle="Outro usuário alterou este acesso."
        error={error}
        title="Alteração não aplicada"
      />,
    );
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Outro usuário alterou este acesso.");
    expect(alert).not.toHaveTextContent("Alteração não aplicada");
    expect(alert.className).toMatch(/warning/);
  });

  it("renders an optional action on the banner", () => {
    renderBanner(
      <StatusBanner
        action={<button type="button">Vincular</button>}
        title="Já existe uma pessoa com este CPF."
        tone="info"
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Já existe uma pessoa com este CPF.");
    expect(screen.getByRole("button", { name: "Vincular" })).toBeInTheDocument();
  });
});

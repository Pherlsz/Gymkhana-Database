import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { I18nProvider } from "../i18n";
import { QueryView } from "./QueryView";

function renderView(node: ReactNode) {
  return render(<I18nProvider locale="pt-BR">{node}</I18nProvider>);
}

describe("QueryView", () => {
  afterEach(() => {
    cleanup();
  });

  it("shows loading, error with retry, empty, then ready children", () => {
    const onRetry = vi.fn();
    const { rerender } = renderView(
      <QueryView isPending>
        <p>pronto</p>
      </QueryView>,
    );
    expect(screen.getByRole("status")).toHaveTextContent("Carregando…");
    expect(screen.queryByText("pronto")).toBeNull();

    rerender(
      <I18nProvider locale="pt-BR">
        <QueryView errorTitle="Falha ao carregar" isError onRetry={onRetry}>
          <p>pronto</p>
        </QueryView>
      </I18nProvider>,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Falha ao carregar");
    fireEvent.click(screen.getByRole("button", { name: "Tentar novamente" }));
    expect(onRetry).toHaveBeenCalledTimes(1);

    rerender(
      <I18nProvider locale="pt-BR">
        <QueryView empty emptyTitle="Nada aqui">
          <p>pronto</p>
        </QueryView>
      </I18nProvider>,
    );
    expect(screen.getByRole("heading", { name: "Nada aqui" })).toBeInTheDocument();
    expect(screen.queryByText("pronto")).toBeNull();

    rerender(
      <I18nProvider locale="pt-BR">
        <QueryView>
          <p>pronto</p>
        </QueryView>
      </I18nProvider>,
    );
    expect(screen.getByText("pronto")).toBeInTheDocument();
  });
});

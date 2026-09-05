import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { StateCard } from "./StateCard";

describe("StateCard", () => {
  it("renders loading state with status role and title", () => {
    render(
      <StateCard
        description="Aguarde o carregamento..."
        kind="loading"
        title="Carregando catálogo"
      />,
    );

    const statusEl = screen.getByRole("status");
    expect(statusEl).toBeInTheDocument();
    expect(statusEl).toHaveAttribute("aria-busy", "true");
    expect(screen.getByText("Carregando catálogo")).toBeInTheDocument();
    expect(screen.getByText("Aguarde o carregamento...")).toBeInTheDocument();
  });

  it("renders empty state with default title and custom icon", () => {
    render(
      <StateCard
        description="Nenhum item foi encontrado."
        icon={<span data-testid="custom-icon">★</span>}
        kind="empty"
        title="Sem dados"
      />,
    );

    expect(screen.getByText("Sem dados")).toBeInTheDocument();
    expect(screen.getByText("Nenhum item foi encontrado.")).toBeInTheDocument();
    expect(screen.getByTestId("custom-icon")).toBeInTheDocument();
  });

  it("renders error state with alert role", () => {
    render(
      <StateCard
        description="Erro ao conectar com a API."
        kind="error"
        title="Falha na requisição"
      />,
    );

    const alertEl = screen.getByRole("alert");
    expect(alertEl).toBeInTheDocument();
    expect(alertEl).toHaveAttribute("aria-live", "assertive");
    expect(screen.getByText("Falha na requisição")).toBeInTheDocument();
  });

  it("renders warning state and triggers action callback", () => {
    const handleAction = vi.fn();

    render(
      <StateCard
        action={<button onClick={handleAction}>Tentar novamente</button>}
        description="Você precisa de permissão de administrador."
        kind="warning"
        title="Acesso negado"
      />,
    );

    expect(screen.getByText("Acesso negado")).toBeInTheDocument();
    const button = screen.getByRole("button", { name: "Tentar novamente" });
    fireEvent.click(button);
    expect(handleAction).toHaveBeenCalledTimes(1);
  });

  it("applies compact modifier when compact prop is true", () => {
    const { container } = render(
      <StateCard compact description="Pequena descrição" title="Compacto" />,
    );

    expect(container.querySelector(".state-card--compact")).toBeInTheDocument();
  });
});

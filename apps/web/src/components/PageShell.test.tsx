import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { ReactNode } from "react";
import { I18nProvider } from "../i18n";
import { PageShell } from "./PageShell";

function renderShell(node: ReactNode) {
  return render(<I18nProvider locale="pt-BR">{node}</I18nProvider>);
}

describe("PageShell", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders the page title and measure class", () => {
    const { container } = renderShell(
      <PageShell className="admin-page" description="Quem entra" measure title="Administração">
        <p>conteúdo</p>
      </PageShell>,
    );

    expect(screen.getByRole("heading", { name: "Administração" })).toBeInTheDocument();
    expect(screen.getByText("Quem entra")).toBeInTheDocument();
    expect(container.querySelector(".admin-page.page-measure")).not.toBeNull();
  });

  it("omits PageHeader and measure when those props are absent", () => {
    const { container } = renderShell(
      <PageShell className="tables-page">
        <p>grade</p>
      </PageShell>,
    );

    expect(screen.queryByRole("heading")).toBeNull();
    expect(container.querySelector(".page-measure")).toBeNull();
    expect(container.querySelector(".tables-page")).not.toBeNull();
  });
});

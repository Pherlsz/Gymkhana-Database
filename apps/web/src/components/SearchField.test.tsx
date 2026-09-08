import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "../i18n";
import { SearchField } from "./SearchField";

function renderTextField(onSubmit = vi.fn()) {
  function Harness() {
    const [value, setValue] = useState("");
    return (
      <SearchField
        label="Buscar"
        mode="text"
        placeholder="Buscar"
        value={value}
        onChange={setValue}
        onSubmit={onSubmit}
      />
    );
  }
  return { ...render(<Harness />), onSubmit };
}

function renderSuggestField(onSubmit = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function Harness() {
    const [value, setValue] = useState("");
    return (
      <QueryClientProvider client={client}>
        <I18nProvider locale="pt-BR">
          <SearchField
            label="Buscar dados autorizados"
            mode="suggest"
            placeholder="Buscar"
            value={value}
            onChange={setValue}
            onSubmit={onSubmit}
          />
        </I18nProvider>
      </QueryClientProvider>
    );
  }
  return { ...render(<Harness />), onSubmit };
}

describe("SearchField", () => {
  afterEach(() => {
    cleanup();
  });

  it("keeps navbar text mode free of query-language suggestions", () => {
    const { onSubmit } = renderTextField();
    const input = screen.getByRole("searchbox", { name: "Buscar" });

    fireEvent.change(input, { target: { value: "tipo:" } });
    fireEvent.change(input, { target: { value: "/cidade" } });

    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();

    fireEvent.keyDown(input, { key: "Enter" });
    expect(onSubmit).toHaveBeenCalledWith("/cidade");
  });

  it("commits the query only on Enter or blur, not while typing", () => {
    const { onSubmit } = renderSuggestField();
    const input = screen.getByRole("searchbox", { name: "Buscar dados autorizados" });

    fireEvent.change(input, { target: { value: "Ana" } });
    expect(onSubmit).not.toHaveBeenCalled();

    fireEvent.keyDown(input, { key: "Enter" });
    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith("Ana");

    fireEvent.change(input, { target: { value: "Ana Silva" } });
    expect(onSubmit).toHaveBeenCalledTimes(1);

    fireEvent.blur(input);
    expect(onSubmit).toHaveBeenCalledTimes(2);
    expect(onSubmit).toHaveBeenLastCalledWith("Ana Silva");
  });
});

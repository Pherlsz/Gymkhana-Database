import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "../../i18n";
import { CadastroManualPanel } from "./CadastroManualPanel";

const ownerId = "019bf789-4400-7f12-9abc-123456789abc";

const ana = {
  id: ownerId,
  full_name: "Ana da Silva",
  social_name: "Ana",
  cpf: "52998224725",
  email: "ana@example.com",
  mobile_phone: "",
  landline_phone: "",
  address: {
    street: "",
    number: "",
    complement: "",
    neighborhood: "",
    city: "",
    state: "",
    postal_code: "",
  },
  notes: "",
  version: 1,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

vi.mock("../api/client", () => ({
  getProfile: vi.fn(),
  listProfilesLookup: vi.fn(),
}));

import { getProfile, listProfilesLookup } from "../api/client";

function renderPanel(onOwner = vi.fn(), onClearOwner = vi.fn(), recordsOwner?: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return {
    onOwner,
    onClearOwner,
    ...render(
      <QueryClientProvider client={client}>
        <I18nProvider locale="pt-BR">
          <CadastroManualPanel
            recordsOwner={recordsOwner}
            table="documents"
            onClearOwner={onClearOwner}
            onOwner={onOwner}
          />
        </I18nProvider>
      </QueryClientProvider>,
    ),
  };
}

describe("CadastroManualPanel", () => {
  afterEach(() => {
    cleanup();
    vi.mocked(listProfilesLookup).mockReset();
    vi.mocked(getProfile).mockReset();
  });

  it("opens the document form as soon as a person is chosen", async () => {
    vi.mocked(listProfilesLookup).mockResolvedValue({
      profiles: [ana],
      page: { total: 1, limit: 20, offset: 0, sort_field: "full_name", sort_order: "asc" },
    });
    const { onOwner } = renderPanel();

    fireEvent.focus(screen.getByRole("searchbox", { name: "Buscar pessoa dona" }));
    await waitFor(() =>
      expect(screen.getByRole("option", { name: "Ana da Silva" })).toBeInTheDocument(),
    );
    fireEvent.mouseDown(screen.getByRole("option", { name: "Ana da Silva" }));
    expect(onOwner).toHaveBeenCalledWith(ownerId);
  });

  it("lets the user change the selected owner without a second open step", async () => {
    vi.mocked(getProfile).mockResolvedValue(ana);
    const { onClearOwner, onOwner } = renderPanel(vi.fn(), vi.fn(), ownerId);
    await waitFor(() => expect(screen.getByText("Ana da Silva")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Trocar" }));
    expect(onClearOwner).toHaveBeenCalledTimes(1);
    expect(onOwner).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("button", { name: "Abrir formulário de documento" }),
    ).not.toBeInTheDocument();
  });
});

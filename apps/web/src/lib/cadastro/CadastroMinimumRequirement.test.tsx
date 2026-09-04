import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "../../i18n";
import {
  CadastroMinimumRequirement,
  OFFICIAL_DOCUMENT_TYPE_KEYS,
} from "./CadastroMinimumRequirement";

const ownerId = "019bf789-4400-7f12-9abc-123456789abc";

vi.mock("../api/client", () => ({
  listDocuments: vi.fn(),
}));

import { listDocuments } from "../api/client";

function documentWith(technicalKey: string, label: string) {
  return {
    id: "019bf789-4400-7f12-9abc-123456789abd",
    owner_profile_id: ownerId,
    owner_full_name: "Ana da Silva",
    document_type_id: "019bf789-4400-7f12-9abc-123456789abe",
    identifier_value: "123",
    document_date: "",
    notes: "",
    medium: "PHYSICAL",
    type: {
      id: "019bf789-4400-7f12-9abc-123456789abe",
      technical_key: technicalKey,
      label,
      active: true,
      uniqueness_policy: "PER_PROFILE",
      validation_regex: null,
      date_required: false,
    },
    current_use: { profile_id: null },
  };
}

function renderIndicator(documents: unknown[]) {
  vi.mocked(listDocuments).mockResolvedValue({
    documents,
    page: { total: documents.length, limit: 50, offset: 0, sort_field: "identifier_value" },
  } as never);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <I18nProvider locale="pt-BR">
        <CadastroMinimumRequirement ownerId={ownerId} />
      </I18nProvider>
    </QueryClientProvider>,
  );
}

describe("CadastroMinimumRequirement", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("shows the official list as a warning when the person has no official document", async () => {
    renderIndicator([documentWith("ctps", "CTPS")]);
    await waitFor(() => {
      expect(screen.getByText(/Requisito mínimo/i)).toBeInTheDocument();
    });
    expect(screen.queryByText(/Documento mínimo ok/i)).toBeNull();
  });

  it("shows the quiet ok line with the type label when an official document exists", async () => {
    renderIndicator([documentWith("rg", "RG"), documentWith("ctps", "CTPS")]);
    await waitFor(() => {
      expect(screen.getByText(/Documento mínimo ok — RG/i)).toBeInTheDocument();
    });
    expect(screen.queryByText(/Requisito mínimo/i)).toBeNull();
  });

  it("renders the warning (not loading-forever) when the person has no documents", async () => {
    renderIndicator([]);
    await waitFor(() => {
      expect(screen.getByText(/Requisito mínimo/i)).toBeInTheDocument();
    });
  });

  it("keeps the official type keys stable (cpf/rg/cnh/birth certificate)", () => {
    expect(OFFICIAL_DOCUMENT_TYPE_KEYS).toEqual(["cpf", "rg", "cnh", "birth_certificate"]);
  });
});

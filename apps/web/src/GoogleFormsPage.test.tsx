import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./App", () => ({
  useApplicationSession: () => ({ user: { role: "ADMIN" } }),
}));

vi.mock("./lib/api/googleForms", () => ({
  beginGoogleFormsOAuth: vi.fn(),
  cancelGoogleFormsSync: vi.fn(),
  createGoogleFormsSource: vi.fn(),
  disconnectGoogleForms: vi.fn(),
  getGoogleFormsStatus: vi.fn(),
  listGoogleFormsSources: vi.fn(),
  listGoogleFormsSyncs: vi.fn(),
  refreshGoogleFormsSource: vi.fn(),
  requestGoogleFormsSync: vi.fn(),
  saveGoogleFormsMapping: vi.fn(),
  updateGoogleFormsSource: vi.fn(),
}));

vi.mock("./lib/api/operations", () => ({ getOperationsCatalog: vi.fn() }));

import { GoogleFormsPage } from "./GoogleFormsPage";
import {
  getGoogleFormsStatus,
  listGoogleFormsSources,
  listGoogleFormsSyncs,
  requestGoogleFormsSync,
} from "./lib/api/googleForms";
import { getOperationsCatalog } from "./lib/api/operations";

const source = {
  id: "10000000-0000-4000-8000-000000000001",
  provider_form_id: "form_identifier_123",
  title: "Inscrições",
  module: "PROFILES" as const,
  state: "ACTIVE" as const,
  schema_revision: "rev-1",
  sync_mode: "MANUAL" as const,
  poll_interval_seconds: 900,
  pagination_pending: false,
  version: 3,
  created_at: "2026-07-17T10:00:00Z",
  updated_at: "2026-07-17T10:00:00Z",
  questions: [
    {
      id: "name",
      position: 0,
      title: "Nome completo",
      answer_kind: "TEXT" as const,
      required: true,
      supported: true,
      target_field: "full_name",
    },
    {
      id: "files",
      position: 1,
      title: "Comprovante",
      answer_kind: "UNSUPPORTED" as const,
      required: false,
      supported: false,
      unsupported_code: "file_upload",
    },
  ],
};

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <GoogleFormsPage />
    </QueryClientProvider>,
  );
}

describe("GoogleFormsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(listGoogleFormsSources).mockResolvedValue({
      sources: [source],
      total: 1,
      limit: 100,
      offset: 0,
    });
    vi.mocked(listGoogleFormsSyncs).mockResolvedValue({
      runs: [],
      total: 0,
      limit: 100,
      offset: 0,
    });
    vi.mocked(getOperationsCatalog).mockResolvedValue({
      modules: [
        {
          id: "PROFILES",
          label: "Pessoas",
          can_import: true,
          can_export: true,
          can_duplicate: true,
          can_delete: true,
          can_bulk_delete: true,
          fields: [
            {
              id: "full_name",
              label: "Nome completo",
              kind: "TEXT",
              required: true,
              importable: true,
              exportable: true,
            },
          ],
        },
      ],
      limits: {
        maximum_file_size: 1,
        maximum_rows: 1,
        maximum_columns: 1,
        maximum_cells: 1,
        maximum_preview_rows: 1,
        maximum_bulk_selection: 1,
      },
    });
  });

  it("explains when the integration is disabled", async () => {
    vi.mocked(getGoogleFormsStatus).mockResolvedValue({ enabled: false, connected: false });
    renderPage();
    expect(await screen.findByText("Integração desativada")).toBeInTheDocument();
    expect(listGoogleFormsSources).not.toHaveBeenCalled();
  });

  it("shows mappings, unsupported questions and queues a manual sync", async () => {
    vi.mocked(getGoogleFormsStatus).mockResolvedValue({
      enabled: true,
      connected: true,
      connection: {
        id: "20000000-0000-4000-8000-000000000001",
        state: "ACTIVE",
        granted_scopes: [],
        version: 1,
        updated_at: "2026-07-17T10:00:00Z",
      },
    });
    vi.mocked(requestGoogleFormsSync).mockResolvedValue({
      id: "30000000-0000-4000-8000-000000000001",
      source_id: source.id,
      trigger_kind: "MANUAL",
      state: "QUEUED",
      received_count: 0,
      staged_count: 0,
      duplicate_count: 0,
      version: 1,
      created_at: "2026-07-17T10:00:00Z",
      updated_at: "2026-07-17T10:00:00Z",
    });
    renderPage();
    expect(await screen.findByText("Inscrições")).toBeInTheDocument();
    expect(screen.getByText("Não suportada: upload de arquivo")).toBeInTheDocument();
    expect(screen.getByLabelText("Destino para Nome completo")).toHaveValue("full_name");
    fireEvent.click(screen.getByRole("button", { name: "Sincronizar agora" }));
    await waitFor(() => expect(requestGoogleFormsSync).toHaveBeenCalledWith(source));
  });
});

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const sessionState = vi.hoisted(() => ({ role: "ADMIN" }));

vi.mock("./App", () => ({
  useApplicationSession: () => ({ user: { role: sessionState.role } }),
}));

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    search,
    to,
  }: {
    children: ReactNode;
    search?: Record<string, string>;
    to: string;
  }) => <a href={`${to}${search?.selected ? `?selected=${search.selected}` : ""}`}>{children}</a>,
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
  beginGoogleFormsOAuth,
  createGoogleFormsSource,
  getGoogleFormsStatus,
  listGoogleFormsSources,
  listGoogleFormsSyncs,
  requestGoogleFormsSync,
  saveGoogleFormsMapping,
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
  afterEach(cleanup);

  beforeEach(() => {
    vi.clearAllMocks();
    sessionState.role = "ADMIN";
    window.history.replaceState({}, "", "/google-forms");
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

  it("denies actionable controls to members", async () => {
    sessionState.role = "EXTERNAL";
    renderPage();
    expect(await screen.findByText("Acesso administrativo necessário")).toBeInTheDocument();
    expect(getGoogleFormsStatus).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: "Conectar com Google" })).not.toBeInTheDocument();
  });

  it("handles the OAuth return and starts a connection with safe URL state", async () => {
    window.history.replaceState({}, "", "/google-forms?tab=history&google_forms=connected");
    vi.mocked(getGoogleFormsStatus).mockResolvedValue({ enabled: true, connected: false });
    vi.mocked(beginGoogleFormsOAuth).mockResolvedValue();
    renderPage();
    expect(await screen.findByText("Google Forms conectado")).toBeInTheDocument();
    fireEvent.click(await screen.findByRole("button", { name: "Conectar com Google" }));
    await waitFor(() =>
      expect(beginGoogleFormsOAuth).toHaveBeenCalledWith("/google-forms?tab=history"),
    );
  });

  it("adds an explicitly supplied form", async () => {
    vi.mocked(getGoogleFormsStatus).mockResolvedValue(connectedStatus());
    vi.mocked(listGoogleFormsSources).mockResolvedValue({
      sources: [],
      total: 0,
      limit: 100,
      offset: 0,
    });
    vi.mocked(createGoogleFormsSource).mockResolvedValue(source);
    renderPage();
    const reference = await screen.findByLabelText("ID ou URL do formulário");
    fireEvent.change(reference, { target: { value: "form_identifier_123" } });
    fireEvent.click(screen.getByRole("button", { name: "Adicionar formulário" }));
    await waitFor(() =>
      expect(createGoogleFormsSource).toHaveBeenCalledWith("form_identifier_123", "PROFILES"),
    );
  });

  it("shows mappings, unsupported questions and queues a manual sync", async () => {
    vi.mocked(getGoogleFormsStatus).mockResolvedValue(connectedStatus());
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

  it("surfaces mapping errors and schema/reauthorization states", async () => {
    vi.mocked(getGoogleFormsStatus).mockResolvedValue(connectedStatus());
    const drifted = { ...source, state: "SCHEMA_DRIFT" as const };
    const reauth = {
      ...source,
      id: "10000000-0000-4000-8000-000000000002",
      title: "Recadastro",
      state: "NEEDS_REAUTH" as const,
    };
    vi.mocked(listGoogleFormsSources).mockResolvedValue({
      sources: [drifted, reauth],
      total: 2,
      limit: 100,
      offset: 0,
    });
    vi.mocked(saveGoogleFormsMapping).mockRejectedValue(
      new Error("Mapeamento obrigatório ausente"),
    );
    renderPage();
    expect(await screen.findByText("O esquema do formulário mudou")).toBeInTheDocument();
    expect(screen.getByText("A conexão precisa ser refeita")).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "Salvar mapeamento" })[0]!);
    expect(await screen.findByText("Mapeamento obrigatório ausente")).toBeInTheDocument();
    for (const button of screen.getAllByRole("button", { name: "Ativar" })) {
      expect(button).toBeDisabled();
    }
  });

  it("keeps source/history URL state and links staged batches to Operations", async () => {
    window.history.replaceState({}, "", `/google-forms?tab=history&source=${source.id}`);
    vi.mocked(getGoogleFormsStatus).mockResolvedValue(connectedStatus());
    vi.mocked(listGoogleFormsSyncs).mockResolvedValue({
      runs: [
        {
          id: "30000000-0000-4000-8000-000000000001",
          source_id: source.id,
          trigger_kind: "MANUAL",
          state: "RUNNING",
          operation_import_id: "40000000-0000-4000-8000-000000000001",
          received_count: 2,
          staged_count: 1,
          duplicate_count: 1,
          version: 2,
          created_at: "2026-07-17T10:00:00Z",
          updated_at: "2026-07-17T10:01:00Z",
        },
      ],
      total: 1,
      limit: 100,
      offset: 0,
    });
    renderPage();
    expect(await screen.findByText("Sincronizando")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Abrir importação" })).toHaveAttribute(
      "href",
      "/operations?selected=40000000-0000-4000-8000-000000000001",
    );
    expect(window.location.search).toContain("tab=history");
    fireEvent.click(screen.getByRole("tab", { name: "Fontes" }));
    expect(window.location.search).toContain("tab=sources");
    expect(window.location.search).toContain(`source=${source.id}`);
  });
});

function connectedStatus() {
  return {
    enabled: true as const,
    connected: true as const,
    connection: {
      id: "20000000-0000-4000-8000-000000000001",
      state: "ACTIVE" as const,
      granted_scopes: [],
      version: 1,
      updated_at: "2026-07-17T10:00:00Z",
    },
  };
}

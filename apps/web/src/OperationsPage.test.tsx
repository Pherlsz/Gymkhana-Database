import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { normalizeOperationsSearch, parseBulkSelection } from "./OperationsPage";

const importID = "019bf789-4400-7f12-9abc-123456789abc";
const reportImportID = "019bf789-4400-7f12-9abc-123456789abd";

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "content-type": "application/json" },
  });
}

function operationCatalog() {
  return {
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
      maximum_file_size: 26_214_400,
      maximum_rows: 10_000,
      maximum_columns: 256,
      maximum_cells: 1_000_000,
      maximum_preview_rows: 200,
      maximum_bulk_selection: 500,
    },
  };
}

function operationImport() {
  return {
    id: importID,
    module: "PROFILES",
    source_kind: "XLSX",
    original_filename: "people.xlsx",
    declared_size: 120,
    actual_size: 120,
    state: "MAPPING",
    stage: "MAP",
    selected_sheet_index: 0,
    mapping_version: 1,
    unresolved_count: 0,
    validation_error_count: 0,
    inserted_count: 0,
    updated_count: 0,
    linked_count: 0,
    skipped_count: 0,
    errored_count: 0,
    conflicted_count: 0,
    expires_at: "2026-07-18T12:00:00Z",
    version: 3,
    created_at: "2026-07-17T12:00:00Z",
    updated_at: "2026-07-17T12:00:00Z",
    sheets: [{ index: 0, name: "Pessoas", row_count: 1, column_count: 1 }],
    columns: [{ sheet_index: 0, source_column: 0, source_header: "full_name", target_field: "" }],
    preview: [],
  };
}

describe("OperationsPage", () => {
  beforeEach(() => window.history.replaceState(null, "", "/operations"));
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("keeps the selected import in safe URL state and maps only logical catalog fields", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(
          jsonResponse({
            authenticated: true,
            user: { login: "admin", display_name: "Admin", role: "ADMIN" },
          }),
        );
      if (url.endsWith("/api/v1/operations/catalog"))
        return Promise.resolve(jsonResponse(operationCatalog()));
      if (url.includes("/api/v1/operations/imports?"))
        return Promise.resolve(
          jsonResponse({ imports: [operationImport()], total: 1, limit: 100, offset: 0 }),
        );
      if (url.endsWith(`/api/v1/operations/imports/${importID}`))
        return Promise.resolve(jsonResponse(operationImport()));
      if (url.includes("/api/v1/operations/exports?"))
        return Promise.resolve(jsonResponse({ exports: [], total: 0, limit: 100, offset: 0 }));
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "Importações e exportações XLSX" }),
    ).toBeInTheDocument();
    fireEvent.click(await screen.findByRole("button", { name: "Abrir" }));
    await waitFor(() =>
      expect(new URLSearchParams(window.location.search).get("selected")).toBe(importID),
    );
    expect(await screen.findByRole("heading", { name: "people.xlsx" })).toBeInTheDocument();
    const mapping = await screen.findByRole("combobox", { name: "Mapear full_name" });
    expect(mapping).toHaveTextContent("Nome completo *");
    expect(mapping).not.toHaveTextContent(/physical|table|column/i);
    expect(document.body).not.toHaveTextContent(/object_key|river_job|signed/i);
    // antd Table renders substantially more DOM in jsdom than the old plain
    // table, so this full-App flow needs a longer budget than the default.
  }, 20000);

  it("normalizes URL state and validates reinforced bulk selection locally", () => {
    expect(normalizeOperationsSearch({ selected: importID })).toEqual({ selected: importID });
    expect(
      normalizeOperationsSearch({ selected: "https://storage.invalid/private?signed=secret" }),
    ).toEqual({});
    expect(parseBulkSelection(`${importID},7`)).toEqual([{ id: importID, version: 7 }]);
    expect(() => parseBulkSelection(`${importID},0`)).toThrow(/UUID,versão/);
    expect(() => parseBulkSelection("not-an-id,1")).toThrow(/UUID,versão/);
  });

  it("loads a durable final report with safe row references", async () => {
    const completed = {
      ...operationImport(),
      id: reportImportID,
      state: "COMPLETED",
      stage: "REPORT",
      inserted_count: 1,
      linked_count: 3,
    };
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(
          jsonResponse({
            authenticated: true,
            user: { login: "admin", display_name: "Admin", role: "ADMIN" },
          }),
        );
      if (url.endsWith("/api/v1/operations/catalog"))
        return Promise.resolve(jsonResponse(operationCatalog()));
      if (url.endsWith(`/api/v1/operations/imports/${reportImportID}/report`))
        return Promise.resolve(
          jsonResponse({
            import_id: reportImportID,
            state: "COMPLETED",
            inserted: 1,
            updated: 0,
            linked: 3,
            skipped: 0,
            errored: 0,
            conflicted: 0,
            decisions: 3,
            unresolved: 0,
            validation_errors: 0,
            rows: [{ sheet_index: 0, row_number: 2, outcome: "LINKED" }],
          }),
        );
      if (url.endsWith(`/api/v1/operations/imports/${reportImportID}`))
        return Promise.resolve(jsonResponse(completed));
      if (url.includes("/api/v1/operations/imports?"))
        return Promise.resolve(
          jsonResponse({ imports: [completed], total: 1, limit: 100, offset: 0 }),
        );
      if (url.includes("/api/v1/operations/exports?"))
        return Promise.resolve(jsonResponse({ exports: [], total: 0, limit: 100, offset: 0 }));
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState(null, "", `/operations?selected=${reportImportID}`);
    render(<App />);

    expect(await screen.findByRole("heading", { name: "Relatório final" })).toBeInTheDocument();
    const table = await screen.findByRole("table", {
      name: "Resultados por linha da importação",
    });
    expect(within(table).getByText("Vinculada")).toBeInTheDocument();
    expect(within(table).getByText("2")).toBeInTheDocument();
    expect(document.body).not.toHaveTextContent(/object_key|raw_value|signed_url/i);
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining(`/api/v1/operations/imports/${reportImportID}/report`),
      expect.anything(),
    );
  });
});

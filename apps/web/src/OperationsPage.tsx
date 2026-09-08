import { Alert, Button, Flex, Layout, Typography } from "antd";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createColumnHelper } from "@tanstack/react-table";
import { useMemo } from "react";
import { getRouteApi } from "@tanstack/react-router";
import { DataGrid } from "./DataGrid";
import { useI18n } from "./i18n";
import { formatDateTime as formatDate, errorMessage as operationError } from "./lib/formatters";
import {
  importResultLabel,
  operationActive,
  operationTerminal,
  OperationStatus,
} from "./lib/operations/operationLabels";
import { ImportCreator, ImportWorkspace } from "./lib/operations/ImportWorkspace";
import { ExportWorkspace } from "./lib/operations/ExportWorkspace";
import {
  BulkDeleteWorkspace,
  operationIdentifierPattern,
  parseBulkSelection,
} from "./lib/operations/BulkDeleteWorkspace";
import {
  getOperationImport,
  getOperationImportReport,
  getOperationsCatalog,
  listOperationExports,
  listOperationImports,
  type OperationImport,
} from "./lib/api/operations";

export { ImportCreator, ImportWorkspace };
export { BulkDeleteWorkspace, parseBulkSelection };
export { ExportWorkspace };

const adminRoute = getRouteApi("/admin");
const importColumn = createColumnHelper<OperationImport>();

export type OperationsSearchState = {
  selected?: string;
};

export function normalizeOperationsSearch(search: Record<string, unknown>): OperationsSearchState {
  const selected = typeof search.selected === "string" ? search.selected.trim() : "";
  return operationIdentifierPattern.test(selected) ? { selected } : {};
}

export function OperationsPage({ includeImports = true }: { includeImports?: boolean }) {
  const { messages } = useI18n();
  const copy = messages.operations;
  const queryClient = useQueryClient();
  const search = adminRoute.useSearch();
  const navigate = adminRoute.useNavigate();
  const selectedImportID = search.selected;
  const setSelectedImportID = (selected?: string) => {
    void navigate({ search: () => normalizeOperationsSearch({ selected }), to: "/admin" });
  };

  const catalog = useQuery({
    queryKey: ["operations-catalog"],
    queryFn: ({ signal }) => getOperationsCatalog(signal),
    staleTime: 60_000,
  });

  const imports = useQuery({
    queryKey: ["operation-imports"],
    queryFn: ({ signal }) => listOperationImports(signal),
    enabled: includeImports,
    refetchInterval: (query) =>
      query.state.data?.imports.some((value) => operationActive(value.state)) ? 2_000 : false,
  });

  const exports = useQuery({
    queryKey: ["operation-exports"],
    queryFn: ({ signal }) => listOperationExports(signal),
    refetchInterval: (query) =>
      query.state.data?.exports.some(
        (value) => value.state === "QUEUED" || value.state === "RUNNING",
      )
        ? 2_000
        : false,
  });

  const selectedImport = useQuery({
    queryKey: ["operation-import", selectedImportID],
    queryFn: ({ signal }) => getOperationImport(selectedImportID!, signal),
    enabled: Boolean(selectedImportID),
    refetchInterval: (query) =>
      query.state.data && operationActive(query.state.data.state) ? 1_500 : false,
  });

  const selectedReport = useQuery({
    queryKey: ["operation-import-report", selectedImportID],
    queryFn: ({ signal }) => getOperationImportReport(selectedImportID!, signal),
    enabled: Boolean(
      selectedImportID && selectedImport.data && operationTerminal(selectedImport.data.state),
    ),
  });

  const refreshOperations = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["operation-imports"] }),
      queryClient.invalidateQueries({ queryKey: ["operation-exports"] }),
      queryClient.invalidateQueries({ queryKey: ["operation-import", selectedImportID] }),
      queryClient.invalidateQueries({ queryKey: ["operation-import-report", selectedImportID] }),
    ]);
  };

  const moduleLabels = useMemo(
    () => new Map(catalog.data?.modules.map((module) => [module.id, module.label]) ?? []),
    [catalog.data?.modules],
  );

  const importColumns = useMemo(
    () => [
      importColumn.accessor((value) => value.original_filename ?? copy.sourceFallback, {
        id: "source",
        header: messages.common.labels.file,
      }),
      importColumn.accessor("module", {
        header: messages.common.labels.module,
        cell: ({ getValue }) => moduleLabels.get(getValue()) ?? getValue(),
      }),
      importColumn.accessor("state", {
        header: messages.common.labels.status,
        cell: ({ getValue }) => <OperationStatus value={getValue()} />,
      }),
      importColumn.display({
        id: "result",
        header: copy.workspace.outcome,
        cell: ({ row }) => importResultLabel(row.original, messages),
      }),
      importColumn.accessor("created_at", {
        header: messages.common.labels.date,
        cell: ({ getValue }) => formatDate(getValue()),
      }),
      importColumn.display({
        id: "actions",
        header: messages.common.entities.admin,
        cell: ({ row }) => (
          <Button onClick={() => setSelectedImportID(row.original.id)}>
            {messages.common.actions.open}
          </Button>
        ),
      }),
    ],
    [copy.sourceFallback, copy.workspace.outcome, messages, moduleLabels],
  );

  return (
    <Layout className="page-measure">
      <header className="page-header">
        <div className="page-eyebrow">{copy.eyebrow}</div>
        <Typography.Title className="page-title" level={1}>
          {includeImports ? copy.titleWithImports : copy.titleWithoutImports}
        </Typography.Title>
        <Typography.Paragraph className="page-description">
          {includeImports ? copy.descWithImports : copy.descWithoutImports}
        </Typography.Paragraph>
      </header>
      <div className="page-content">
        <Flex gap="1.5rem" vertical>
          {catalog.isError ? (
            <Alert
              description={<>{operationError(catalog.error)}</>}
              message={copy.unavailable}
              type="error"
            />
          ) : null}
          {includeImports && catalog.data ? (
            <ImportCreator
              catalog={catalog.data}
              onCreated={(value) => {
                setSelectedImportID(value.id);
                void refreshOperations();
              }}
            />
          ) : null}
          {includeImports ? (
            <section className="page-section">
              <Typography.Title level={2}>{copy.sectionImports}</Typography.Title>
              <Typography.Paragraph>{copy.sectionImportsDesc}</Typography.Paragraph>
              {imports.isError ? (
                <Alert
                  description={<>{operationError(imports.error)}</>}
                  message={copy.loadError}
                  type="error"
                />
              ) : null}
              <DataGrid
                caption={copy.sectionImports}
                columns={importColumns}
                data={imports.data?.imports ?? []}
                emptyLabel={copy.emptyImports}
                getRowId={(value) => value.id}
                loading={imports.isLoading}
                loadingLabel={copy.loadingImports}
                renderCard={(value) => (
                  <article className="operations-card" key={value.id}>
                    <strong>{value.original_filename ?? copy.sourceFallback}</strong>
                    <span>{moduleLabels.get(value.module) ?? value.module}</span>
                    <OperationStatus value={value.state} />
                    <span>{importResultLabel(value, messages)}</span>
                    <Button onClick={() => setSelectedImportID(value.id)}>{copy.openImport}</Button>
                  </article>
                )}
                selectedRowId={selectedImportID}
              />
            </section>
          ) : null}
          {includeImports && selectedImportID ? (
            <ImportWorkspace
              catalog={catalog.data?.modules ?? []}
              error={selectedImport.error}
              loading={selectedImport.isLoading}
              onClose={() => setSelectedImportID(undefined)}
              onUpdated={refreshOperations}
              report={selectedReport.data}
              reportError={selectedReport.error}
              reportLoading={selectedReport.isLoading}
              value={selectedImport.data}
            />
          ) : null}
          {catalog.data ? (
            <ExportWorkspace
              error={exports.error}
              exports={exports.data?.exports ?? []}
              loading={exports.isLoading}
              modules={catalog.data.modules}
              onUpdated={refreshOperations}
            />
          ) : null}
          {catalog.data?.modules.some((module) => module.can_bulk_delete) ? (
            <BulkDeleteWorkspace
              modules={catalog.data.modules.filter((module) => module.can_bulk_delete)}
              onUpdated={refreshOperations}
            />
          ) : null}
        </Flex>
      </div>
    </Layout>
  );
}

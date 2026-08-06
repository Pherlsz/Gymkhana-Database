import { Alert, Button, Card, Flex, Layout, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { operationsRoute } from "./App";
import { DataGrid } from "./DataGrid";
import { APIRequestError } from "./lib/api/client";
import {
  bulkDeleteOperationRecords,
  cancelOperationImport,
  createOperationExport,
  downloadOperationExport,
  executeOperationImport,
  getOperationImport,
  getOperationImportReport,
  getOperationsCatalog,
  listOperationExports,
  listOperationImports,
  previewOperationImport,
  saveOperationDecisions,
  saveOperationMapping,
  selectOperationSheet,
  uploadOperationImport,
  type OperationDecision,
  type OperationExport,
  type OperationImport,
  type OperationMapping,
  type OperationModule,
  type OperationReport,
} from "./lib/api/operations";

type ImportRow = OperationImport["preview"][number];
type ImportReportRow = OperationReport["rows"][number];
const importColumn = createColumnHelper<OperationImport>();
const exportColumn = createColumnHelper<OperationExport>();
const rowColumn = createColumnHelper<ImportRow>();
const reportRowColumn = createColumnHelper<ImportReportRow>();
const operationIdentifierPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export type OperationsSearchState = { selected?: string };

export function normalizeOperationsSearch(search: Record<string, unknown>): OperationsSearchState {
  const selected = typeof search.selected === "string" ? search.selected.trim() : "";
  return operationIdentifierPattern.test(selected) ? { selected } : {};
}

export function OperationsPage() {
  const queryClient = useQueryClient();
  const search = operationsRoute.useSearch();
  const navigate = operationsRoute.useNavigate();
  const selectedImportID = search.selected;
  const setSelectedImportID = (selected?: string) => {
    void navigate({ search: () => normalizeOperationsSearch({ selected }) });
  };
  const catalog = useQuery({
    queryKey: ["operations-catalog"],
    queryFn: ({ signal }) => getOperationsCatalog(signal),
    staleTime: 60_000,
  });
  const imports = useQuery({
    queryKey: ["operation-imports"],
    queryFn: ({ signal }) => listOperationImports(signal),
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
      importColumn.accessor((value) => value.original_filename ?? "Google Forms", {
        id: "source",
        header: "Origem",
      }),
      importColumn.accessor("module", {
        header: "Módulo",
        cell: ({ getValue }) => moduleLabels.get(getValue()) ?? getValue(),
      }),
      importColumn.accessor("state", {
        header: "Estado",
        cell: ({ getValue }) => <OperationStatus value={getValue()} />,
      }),
      importColumn.display({
        id: "result",
        header: "Resultado",
        cell: ({ row }) => importResultLabel(row.original),
      }),
      importColumn.accessor("created_at", {
        header: "Criada em",
        cell: ({ getValue }) => formatDate(getValue()),
      }),
      importColumn.display({
        id: "actions",
        header: "Ações",
        cell: ({ row }) => (
          <Button onClick={() => setSelectedImportID(row.original.id)}>Abrir</Button>
        ),
      }),
    ],
    [moduleLabels],
  );
  return (
    <Layout style={{ maxWidth: "lg", margin: "0 auto" }}>
      <header className="page-header">
        <div className="page-eyebrow">M8 · Operações</div>
        <Typography.Title level={1} className="page-title">Importações e exportações XLSX</Typography.Title>
        <Typography.Paragraph className="page-description">
          Envie planilhas privadas, revise o mapeamento e as alterações antes da execução e gere
          exportações completas conforme suas permissões.
        </Typography.Paragraph>
      </header>
      <div className="page-content">
        <Flex vertical gap="1.5rem">
          {catalog.isError ? (
            <Alert message="Operações indisponíveis" type="error" description={<>{operationError(catalog.error)}
            </>} />
          ) : null}
          {catalog.data ? (
            <ImportCreator
              catalog={catalog.data}
              onCreated={(value) => {
                setSelectedImportID(value.id);
                void refreshOperations();
              }}
            />
          ) : null}
          <section className="page-section"
          >
      <Typography.Title level={2}>Importações</Typography.Title>
      <Typography.Paragraph>Cada upload é validado e processado em segundo plano. Somente suas operações aparecem aqui.</Typography.Paragraph>
            {imports.isError ? (
              <Alert message="Não foi possível carregar importações" type="error" description={<>{operationError(imports.error)}
              </>} />
            ) : null}
            <DataGrid
              caption="Importações XLSX"
              columns={importColumns}
              data={imports.data?.imports ?? []}
              emptyLabel="Nenhuma importação criada."
              getRowId={(value) => value.id}
              loading={imports.isLoading}
              loadingLabel="Carregando importações…"
              renderCard={(value) => (
                <article className="operations-card" key={value.id}>
                  <strong>{value.original_filename ?? "Google Forms"}</strong>
                  <span>{moduleLabels.get(value.module) ?? value.module}</span>
                  <OperationStatus value={value.state} />
                  <span>{importResultLabel(value)}</span>
                  <Button onClick={() => setSelectedImportID(value.id)}>Abrir importação</Button>
                </article>
              )}
              selectedRowId={selectedImportID}
            />
          </section>
          {selectedImportID ? (
            <ImportWorkspace
              catalog={catalog.data?.modules ?? []}
              loading={selectedImport.isLoading}
              onClose={() => setSelectedImportID(undefined)}
              onUpdated={refreshOperations}
              report={selectedReport.data}
              reportError={selectedReport.error}
              reportLoading={selectedReport.isLoading}
              value={selectedImport.data}
              error={selectedImport.error}
            />
          ) : null}
          {catalog.data ? (
            <ExportWorkspace
              exports={exports.data?.exports ?? []}
              loading={exports.isLoading}
              modules={catalog.data.modules}
              onUpdated={refreshOperations}
              error={exports.error}
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

function ImportCreator({
  catalog,
  onCreated,
}: {
  catalog: Awaited<ReturnType<typeof getOperationsCatalog>>;
  onCreated: (value: OperationImport) => void;
}) {
  const modules = catalog.modules.filter((module) => module.can_import);
  const [module, setModule] = useState<OperationModule>(modules[0]?.id ?? "PROFILES");
  const [file, setFile] = useState<File>();
  const [progress, setProgress] = useState(0);
  const mutation = useMutation({
    mutationFn: async () => {
      if (!file) throw new Error("Selecione uma planilha XLSX.");
      return uploadOperationImport(file, module, setProgress);
    },
    onSuccess: (value) => {
      setFile(undefined);
      onCreated(value);
    },
  });
  if (modules.length === 0) return null;
  return (
    <Card className="operations-creator" style={{ padding: "1rem" }}>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          setProgress(0);
          mutation.mutate();
        }}
      >
        <Flex vertical gap="1rem">
          <div>
            <strong>Nova importação</strong>
            <p className="operations-muted">
              XLSX de até {formatBytes(catalog.limits.maximum_file_size)}, com cabeçalhos na
              primeira linha e sem macros ou vínculos externos.
            </p>
          </div>
          <div className="operations-form-grid">
            <label>
              Módulo
              <select
                value={module}
                onChange={(event) => setModule(event.target.value as OperationModule)}
              >
                {modules.map((value) => (
                  <option key={value.id} value={value.id}>
                    {value.label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Planilha
              <input
                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                type="file"
                onChange={(event) => setFile(event.target.files?.[0])}
              />
            </label>
          </div>
          {mutation.isError ? (
            <Alert message="Não foi possível enviar a planilha" type="error" description={<>{operationError(mutation.error)}
            </>} />
          ) : null}
          {mutation.isPending ? (
            <div aria-live="polite" className="operations-progress">
              <span>Enviando e confirmando… {progress}%</span>
              <progress max={100} value={progress} />
            </div>
          ) : null}
          <Flex>
            <Button disabled={!file || mutation.isPending} htmlType="submit">
              {mutation.isPending ? "Enviando" : "Enviar planilha"}
            </Button>
          </Flex>
        </Flex>
      </form>
    </Card>
  );
}

function ImportWorkspace({
  value,
  catalog,
  loading,
  error,
  onClose,
  onUpdated,
  report,
  reportError,
  reportLoading,
}: {
  value: OperationImport | undefined;
  catalog: Awaited<ReturnType<typeof getOperationsCatalog>>["modules"];
  loading: boolean;
  error: Error | null;
  onClose: () => void;
  onUpdated: () => Promise<void>;
  report: OperationReport | undefined;
  reportError: Error | null;
  reportLoading: boolean;
}) {
  if (loading) return <Alert message="Carregando importação" type="info" description="Aguarde…" />;
  if (error || !value)
    return (
      <Alert message="Não foi possível abrir a importação" type="error" description={<>{operationError(error)}
      </>} />
    );
  const module = catalog.find((candidate) => candidate.id === value.module);
  return (
    <Card className="operations-workspace" style={{ padding: "1rem" }}>
      <Flex vertical gap="1.25rem">
        <Flex align="center" className="operations-heading">
          <div>
            <span className="operations-muted">Importação selecionada</span>
            <h2>{value.original_filename ?? "Importação do Google Forms"}</h2>
          </div>
          <Flex>
            <OperationStatus value={value.state} />
            <Button onClick={onClose}>Fechar</Button>
          </Flex>
        </Flex>
        <ImportSummary value={value} />
        {value.state === "PARSING" || value.state === "QUEUED" || value.state === "RUNNING" ? (
          <Alert message="Processamento em segundo plano" type="info" description="Esta tela é atualizada automaticamente. Você pode sair e retornar depois." />
        ) : null}
        {value.state === "FAILED" ? (
          <Alert message="A importação falhou" type="error" description={<>Código seguro: {value.error_code || "operation_failed"}. Corrija a planilha e crie uma
            nova importação.
          </>} />
        ) : null}
        {value.state === "PREVIEW_READY" && value.conflicted_count > 0 ? (
          <Alert message="Um registro mudou após o preview" type="warning" description="Revalide a planilha e confirme novamente as decisões. As linhas já concluídas não serão
            repetidas." />
        ) : null}
        {value.state === "CANCELLED" ? (
          <Alert message="A importação foi cancelada" type="warning" description="Nenhuma nova linha será processada. Os resultados já confirmados permanecem no relatório
            auditável." />
        ) : null}
        {value.state === "EXPIRED" ? (
          <Alert message="A importação expirou" type="warning" description="O arquivo privado atingiu o prazo de retenção e foi removido. Crie uma nova importação
            para tentar novamente." />
        ) : null}
        {["MAPPING", "PREVIEW_READY", "DECISIONS_REQUIRED", "READY"].includes(value.state) ? (
          <MappingWorkspace module={module} onUpdated={onUpdated} value={value} />
        ) : null}
        {["PREVIEW_READY", "DECISIONS_REQUIRED", "READY", "COMPLETED"].includes(value.state) ? (
          <PreviewWorkspace module={module} onUpdated={onUpdated} value={value} />
        ) : null}
        {operationTerminal(value.state) ? (
          <ImportReportPanel error={reportError} loading={reportLoading} value={report} />
        ) : null}
        {!operationTerminal(value.state) ? (
          <CancelImportButton onUpdated={onUpdated} value={value} />
        ) : null}
      </Flex>
    </Card>
  );
}

function ImportSummary({ value }: { value: OperationImport }) {
  const items = [
    ["Etapa", stageLabel(value.stage)],
    ["Versão", String(value.version)],
    ["Linhas inseridas", String(value.inserted_count)],
    ["Linhas atualizadas", String(value.updated_count)],
    ["Linhas vinculadas", String(value.linked_count)],
    ["Ignoradas", String(value.skipped_count)],
    ["Conflitos", String(value.conflicted_count)],
    ["Erros", String(value.errored_count + value.validation_error_count)],
  ];
  return (
    <dl className="operations-summary">
      {items.map(([label, content]) => (
        <div key={label}>
          <dt>{label}</dt>
          <dd>{content}</dd>
        </div>
      ))}
    </dl>
  );
}

const reportColumns: ColumnDef<ImportReportRow, any>[] = [
  reportRowColumn.accessor("sheet_index", {
    header: "Aba",
    cell: ({ getValue }) => getValue() + 1,
  }),
  reportRowColumn.accessor("row_number", { header: "Linha" }),
  reportRowColumn.accessor("outcome", {
    header: "Resultado",
    cell: ({ getValue }) => reportOutcomeLabel(getValue()),
  }),
  reportRowColumn.accessor("error_code", {
    header: "Código seguro",
    cell: ({ getValue }) => getValue() || "—",
  }),
];

function ImportReportPanel({
  value,
  loading,
  error,
}: {
  value: OperationReport | undefined;
  loading: boolean;
  error: Error | null;
}) {
  if (loading) return <Alert message="Carregando relatório final" type="info" description="Aguarde…" />;
  if (error)
    return (
      <Alert message="Não foi possível carregar o relatório final" type="error" description={<>{operationError(error)}
      </>} />
    );
  if (!value) return null;
  const items = [
    ["Inseridas", value.inserted],
    ["Atualizadas", value.updated],
    ["Vinculadas", value.linked],
    ["Ignoradas", value.skipped],
    ["Com erro", value.errored],
    ["Conflitos", value.conflicted],
    ["Decisões salvas", value.decisions],
    ["Pendentes", value.unresolved],
    ["Erros de validação", value.validation_errors],
  ];
  return (
    <Flex vertical gap="1rem">
      <div>
        <h3>Relatório final</h3>
        <p className="operations-muted">
          Totais duráveis e referências seguras das linhas processadas. Valores da planilha e chaves
          privadas não são incluídos.
        </p>
      </div>
      <dl className="operations-summary">
        {items.map(([label, content]) => (
          <div key={label}>
            <dt>{label}</dt>
            <dd>{content}</dd>
          </div>
        ))}
      </dl>
      <DataGrid
        caption="Resultados por linha da importação"
        columns={reportColumns}
        data={value.rows}
        emptyLabel="Nenhum resultado por linha para exibir."
        getRowId={(row) => `${row.sheet_index}:${row.row_number}`}
        loading={false}
        loadingLabel="Carregando relatório…"
        renderCard={(row) => (
          <article className="operations-card" key={`${row.sheet_index}:${row.row_number}`}>
            <strong>Linha {row.row_number}</strong>
            <span>Aba {row.sheet_index + 1}</span>
            <span>{reportOutcomeLabel(row.outcome)}</span>
            {row.error_code ? <span>Código seguro: {row.error_code}</span> : null}
          </article>
        )}
      />
    </Flex>
  );
}

function MappingWorkspace({
  value,
  module,
  onUpdated,
}: {
  value: OperationImport;
  module: Awaited<ReturnType<typeof getOperationsCatalog>>["modules"][number] | undefined;
  onUpdated: () => Promise<void>;
}) {
  const [mapping, setMapping] = useState<Record<number, string>>({});
  useEffect(() => {
    const fieldIDs = new Set(
      module?.fields.filter((field) => field.importable).map((field) => field.id),
    );
    setMapping(
      Object.fromEntries(
        value.columns.map((column) => [
          column.source_column,
          column.target_field ||
            (fieldIDs.has(column.source_header.trim().toLowerCase())
              ? column.source_header.trim().toLowerCase()
              : ""),
        ]),
      ),
    );
  }, [module?.fields, value.columns, value.mapping_version]);
  const selectSheet = useMutation({
    mutationFn: (sheetIndex: number) => selectOperationSheet(value, sheetIndex),
    onSuccess: onUpdated,
  });
  const save = useMutation({
    mutationFn: () => {
      const selected: OperationMapping = Object.entries(mapping)
        .filter(([, target]) => target)
        .map(([source, target]) => ({ source_column: Number(source), target_field: target }));
      return saveOperationMapping(value, selected);
    },
    onSuccess: onUpdated,
  });
  const preview = useMutation({
    mutationFn: () => previewOperationImport(value),
    onSuccess: onUpdated,
  });
  const mutationError = selectSheet.error ?? save.error ?? preview.error;
  const required = module?.fields.filter((field) => field.required).map((field) => field.id) ?? [];
  const mappedTargets = Object.values(mapping).filter(Boolean);
  const selectedTargets = new Set(mappedTargets);
  const mappingLocked =
    value.inserted_count +
      value.updated_count +
      value.linked_count +
      value.skipped_count +
      value.errored_count >
    0;
  const mappingReady =
    required.every((field) => selectedTargets.has(field)) &&
    mappedTargets.length === selectedTargets.size;
  const mappingDirty = value.columns.some(
    (column) => (column.target_field ?? "") !== (mapping[column.source_column] ?? ""),
  );
  return (
    <Flex vertical gap="1rem">
      <div>
        <h3>1. Selecione a aba</h3>
        <Flex>
          {value.sheets.map((sheet) => (
            <Button
              aria-pressed={value.selected_sheet_index === sheet.index}
              disabled={mappingLocked || selectSheet.isPending}
              key={sheet.index}
              onClick={() => selectSheet.mutate(sheet.index)}
            >
              {sheet.name} · {sheet.row_count} linhas
              {value.selected_sheet_index === sheet.index ? " · selecionada" : ""}
            </Button>
          ))}
        </Flex>
      </div>
      {mappingLocked ? (
        <Alert message="Mapeamento preservado" type="info" description="Há linhas já confirmadas. A aba e o mapeamento ficam bloqueados para que esses resultados
          nunca sejam apagados ou repetidos; revalide o preview para continuar." />
      ) : null}
      {value.selected_sheet_index !== undefined ? (
        <div>
          <h3>2. Mapeie as colunas</h3>
          <p className="operations-muted">
            Apenas identificadores lógicos autorizados são aceitos. Campos obrigatórios estão
            marcados com *.
          </p>
          <div className="operations-mapping">
            {value.columns.map((column) => (
              <label key={column.source_column}>
                <span>{column.source_header}</span>
                <select
                  aria-label={`Mapear ${column.source_header}`}
                  value={mapping[column.source_column] ?? ""}
                  onChange={(event) =>
                    setMapping((current) => ({
                      ...current,
                      [column.source_column]: event.target.value,
                    }))
                  }
                >
                  <option value="">Ignorar coluna</option>
                  {module?.fields
                    .filter((field) => field.importable)
                    .map((field) => (
                      <option key={field.id} value={field.id}>
                        {field.label}
                        {field.required ? " *" : ""}
                      </option>
                    ))}
                </select>
              </label>
            ))}
          </div>
          <Flex>
            <Button
              disabled={mappingLocked || !mappingReady || save.isPending}
              onClick={() => save.mutate()}
            >
              Salvar mapeamento
            </Button>
            <Button
              disabled={!mappingReady || mappingDirty || save.isPending || preview.isPending}
              onClick={() => preview.mutate()}
            >
              Validar e visualizar
            </Button>
          </Flex>
        </div>
      ) : null}
      {mutationError ? (
        <Alert message="Não foi possível preparar a importação" type="error" description={<>{operationError(mutationError)}
        </>} />
      ) : null}
    </Flex>
  );
}

function PreviewWorkspace({
  value,
  module,
  onUpdated,
}: {
  value: OperationImport;
  module: Awaited<ReturnType<typeof getOperationsCatalog>>["modules"][number] | undefined;
  onUpdated: () => Promise<void>;
}) {
  const [actions, setActions] = useState<Record<number, "UPDATE" | "LINK" | "SKIP">>({});
  useEffect(() => {
    setActions(
      Object.fromEntries(
        value.preview
          .filter((row) => row.decision_required)
          .map((row) => [
            row.row_number,
            row.decision === "SKIP" ? "SKIP" : row.decision === "LINK" ? "LINK" : "UPDATE",
          ]),
      ),
    );
  }, [value.mapping_version, value.preview]);
  const decisions = useMutation({
    mutationFn: () => {
      const payload: OperationDecision[] = value.preview
        .filter((row) => row.decision_required)
        .map((row) => {
          const action = actions[row.row_number] ?? "UPDATE";
          return action === "SKIP"
            ? { row_number: row.row_number, action }
            : {
                row_number: row.row_number,
                action,
                target_id: row.target_id!,
                target_version: row.target_version!,
              };
        });
      return saveOperationDecisions(value, payload);
    },
    onSuccess: onUpdated,
  });
  const execute = useMutation({
    mutationFn: () => executeOperationImport(value),
    onSuccess: onUpdated,
  });
  const columns = useMemo(
    () => [
      rowColumn.accessor("row_number", { header: "Linha" }),
      rowColumn.accessor("proposed_action", {
        header: "Ação proposta",
        cell: ({ getValue }) => actionLabel(getValue()),
      }),
      rowColumn.display({
        id: "values",
        header: "Valores",
        cell: ({ row }) =>
          row.original.cells
            .slice(0, 4)
            .map((cell) => cell.raw_value)
            .filter(Boolean)
            .join(" · ") || "—",
      }),
      rowColumn.accessor("validation_error_count", { header: "Erros" }),
      rowColumn.display({
        id: "decision",
        header: "Decisão",
        cell: ({ row }) =>
          row.original.decision_required ? (
            <select
              aria-label={`Decisão da linha ${row.original.row_number}`}
              value={actions[row.original.row_number] ?? "UPDATE"}
              onChange={(event) =>
                setActions((current) => ({
                  ...current,
                  [row.original.row_number]: event.target.value as "UPDATE" | "LINK" | "SKIP",
                }))
              }
            >
              <option value="UPDATE">Atualizar registro</option>
              <option value="LINK">Vincular sem alterar</option>
              <option value="SKIP">Ignorar linha</option>
            </select>
          ) : (
            actionLabel(row.original.decision || row.original.proposed_action)
          ),
      }),
    ],
    [actions],
  );
  return (
    <Flex vertical gap="1rem">
      <div>
        <h3>Preview da planilha</h3>
        <p className="operations-muted">
          Até 200 linhas são exibidas. Valores e zeros à esquerda permanecem exatamente como no
          arquivo; fórmulas são recusadas.
        </p>
      </div>
      {value.validation_error_count > 0 ? (
        <Alert message="Corrija a planilha ou o mapeamento" type="warning" description={<>Há {value.validation_error_count} erro(s) de validação. Nenhuma alteração foi aplicada.
        </>} />
      ) : null}
      <DataGrid
        caption={`Preview de ${module?.label ?? value.module}`}
        columns={columns}
        data={value.preview}
        emptyLabel="Nenhuma linha com dados."
        getRowId={(row) => String(row.row_number)}
        loading={false}
        loadingLabel="Carregando preview…"
        renderCard={(row) => (
          <article className="operations-card" key={row.row_number}>
            <strong>Linha {row.row_number}</strong>
            <span>Ação proposta: {actionLabel(row.proposed_action)}</span>
            <span>
              {row.cells
                .slice(0, 4)
                .map((cell) => cell.raw_value)
                .filter(Boolean)
                .join(" · ") || "Sem valores visíveis"}
            </span>
            {row.decision_required ? (
              <label>
                Decisão da linha {row.row_number}
                <select
                  value={actions[row.row_number] ?? "UPDATE"}
                  onChange={(event) =>
                    setActions((current) => ({
                      ...current,
                      [row.row_number]: event.target.value as "UPDATE" | "LINK" | "SKIP",
                    }))
                  }
                >
                  <option value="UPDATE">Atualizar registro</option>
                  <option value="LINK">Vincular sem alterar</option>
                  <option value="SKIP">Ignorar linha</option>
                </select>
              </label>
            ) : (
              <span>Resultado: {actionLabel(row.decision || row.proposed_action)}</span>
            )}
          </article>
        )}
      />
      {value.state === "DECISIONS_REQUIRED" ? (
        <Button disabled={decisions.isPending} onClick={() => decisions.mutate()}>
          Salvar {value.unresolved_count} decisão(ões)
        </Button>
      ) : null}
      {value.state === "READY" ? (
        <Button disabled={execute.isPending} onClick={() => execute.mutate()}>
          Executar importação
        </Button>
      ) : null}
      {value.state === "COMPLETED" ? (
        <Alert message="Importação concluída" type="info" description={<>{importResultLabel(value)}. Cada linha possui um resultado idempotente e auditável.
        </>} />
      ) : null}
      {decisions.error || execute.error ? (
        <Alert message="Não foi possível avançar" type="error" description={<>{operationError(decisions.error ?? execute.error)}
        </>} />
      ) : null}
    </Flex>
  );
}

function CancelImportButton({
  value,
  onUpdated,
}: {
  value: OperationImport;
  onUpdated: () => Promise<void>;
}) {
  const mutation = useMutation({
    mutationFn: () => cancelOperationImport(value),
    onSuccess: onUpdated,
  });
  return (
    <Flex vertical gap="0.5rem">
      <Flex>
        <Button disabled={mutation.isPending} onClick={() => mutation.mutate()}>
          Cancelar operação
        </Button>
      </Flex>
      {mutation.isError ? (
        <Alert message="Não foi possível cancelar" type="error" description={<>{operationError(mutation.error)}
        </>} />
      ) : null}
    </Flex>
  );
}

function ExportWorkspace({
  modules,
  exports,
  loading,
  error,
  onUpdated,
}: {
  modules: Awaited<ReturnType<typeof getOperationsCatalog>>["modules"];
  exports: OperationExport[];
  loading: boolean;
  error: Error | null;
  onUpdated: () => Promise<void>;
}) {
  const mutation = useMutation({ mutationFn: createOperationExport, onSuccess: onUpdated });
  const download = useMutation({ mutationFn: downloadOperationExport });
  const available = modules.filter((module) => module.can_export);
  const moduleLabels = useMemo(
    () => new Map(modules.map((module) => [module.id, module.label])),
    [modules],
  );
  const columns: ColumnDef<OperationExport, any>[] = useMemo(
    () => [
      exportColumn.accessor("filename", { header: "Arquivo" }),
      exportColumn.accessor("module", {
        header: "Módulo",
        cell: ({ getValue }) => moduleLabels.get(getValue()) ?? getValue(),
      }),
      exportColumn.accessor("state", {
        header: "Estado",
        cell: ({ getValue }) => <OperationStatus value={getValue()} />,
      }),
      exportColumn.accessor("row_count", { header: "Linhas" }),
      exportColumn.display({
        id: "detail",
        header: "Detalhe",
        cell: ({ row }) =>
          row.original.error_code || `Expira ${formatDate(row.original.expires_at)}`,
      }),
      exportColumn.display({
        id: "actions",
        header: "Ações",
        cell: ({ row }) =>
          row.original.state === "COMPLETED" ? (
            <Button disabled={download.isPending} onClick={() => download.mutate(row.original)}>
              Baixar
            </Button>
          ) : null,
      }),
    ],
    [download.isPending, download.mutate, moduleLabels],
  );
  return (
    <section className="page-section"
    >
      <Typography.Title level={2}>Exportações</Typography.Title>
      <Typography.Paragraph>A exportação consulta a tabela completa no servidor e gera um XLSX privado com validade curta.</Typography.Paragraph>
      <Flex vertical gap="1rem">
        <Flex>
          {available.map((module) => (
            <Button
              disabled={mutation.isPending}
              key={module.id}
              onClick={() => mutation.mutate(module.id)}
            >
              Exportar {module.label}
            </Button>
          ))}
        </Flex>
        {mutation.isError || download.isError || error ? (
          <Alert message="Não foi possível concluir a exportação" type="error" description={<>{operationError(mutation.error ?? download.error ?? error)}
          </>} />
        ) : null}
        <DataGrid
          caption="Exportações XLSX"
          columns={columns}
          data={exports}
          emptyLabel="Nenhuma exportação criada."
          getRowId={(value) => value.id}
          loading={loading}
          loadingLabel="Carregando exportações…"
          renderCard={(value) => (
            <article className="operations-card" key={value.id}>
              <strong>{value.filename}</strong>
              <span>{moduleLabels.get(value.module) ?? value.module}</span>
              <OperationStatus value={value.state} />
              <span>{value.row_count} linha(s)</span>
              {value.error_code ? <span>Código seguro: {value.error_code}</span> : null}
              {value.state === "COMPLETED" ? (
                <Button disabled={download.isPending} onClick={() => download.mutate(value)}>
                  Baixar exportação
                </Button>
              ) : null}
            </article>
          )}
        />
      </Flex>
    </section>
  );
}

function BulkDeleteWorkspace({
  modules,
  onUpdated,
}: {
  modules: Awaited<ReturnType<typeof getOperationsCatalog>>["modules"];
  onUpdated: () => Promise<void>;
}) {
  const [module, setModule] = useState<OperationModule>(modules[0]!.id);
  const [selection, setSelection] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [parseError, setParseError] = useState<string>();
  const mutation = useMutation({
    mutationFn: () =>
      bulkDeleteOperationRecords(module, parseBulkSelection(selection), confirmation),
    onSuccess: async () => {
      setSelection("");
      setConfirmation("");
      setParseError(undefined);
      await onUpdated();
    },
  });
  const submit = (event: FormEvent) => {
    event.preventDefault();
    try {
      parseBulkSelection(selection);
      setParseError(undefined);
      mutation.mutate();
    } catch (error) {
      mutation.reset();
      setParseError(operationError(error));
    }
  };
  return (
    <section className="page-section"
    >
      <Typography.Title level={2}>Exclusão em lote</Typography.Title>
      <Typography.Paragraph>A exclusão exige IDs e versões atuais, confirmação literal e permissão administrativa. Toda a seleção é transacional.</Typography.Paragraph>
      <Card className="operations-danger-zone" style={{ padding: "1rem" }}>
        <form onSubmit={submit}>
          <Flex vertical gap="1rem">
            <div className="operations-form-grid">
              <label>
                Módulo
                <select
                  value={module}
                  onChange={(event) => setModule(event.target.value as OperationModule)}
                >
                  {modules.map((value) => (
                    <option key={value.id} value={value.id}>
                      {value.label}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Confirmação
                <input
                  autoComplete="off"
                  placeholder="Confirmar"
                  value={confirmation}
                  onChange={(event) => setConfirmation(event.target.value)}
                />
              </label>
            </div>
            <label>
              Registros — um por linha no formato UUID,versão
              <textarea
                aria-describedby="bulk-delete-help"
                rows={5}
                value={selection}
                onChange={(event) => {
                  setSelection(event.target.value);
                  setParseError(undefined);
                }}
              />
            </label>
            <span className="operations-muted" id="bulk-delete-help">
              Selecionar não altera dados. A exclusão ocorre somente após enviar este formulário com
              a confirmação exata.
            </span>
            {parseError ? (
              <Alert message="Revise a seleção" type="warning" description={<>{parseError}
              </>} />
            ) : null}
            {mutation.isError ? (
              <Alert message="Nenhum registro foi excluído" type="error" description={<>{operationError(mutation.error)}
              </>} />
            ) : null}
            {mutation.isSuccess ? (
              <Alert message="Exclusão concluída" type="info" description={<>{mutation.data.deleted} registro(s) excluído(s).
              </>} />
            ) : null}
            <Flex>
              <Button
                disabled={confirmation !== "Confirmar" || !selection.trim() || mutation.isPending}
                htmlType="submit"
              >
                Excluir seleção
              </Button>
            </Flex>
          </Flex>
        </form>
      </Card>
    </section>
  );
}

export function parseBulkSelection(value: string): Array<{ id: string; version: number }> {
  const items = value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [id, rawVersion, extra] = line.split(",").map((part) => part.trim());
      const version = Number(rawVersion);
      if (
        !id ||
        !operationIdentifierPattern.test(id) ||
        extra ||
        !Number.isSafeInteger(version) ||
        version < 1
      )
        throw new Error("Use uma linha UUID,versão para cada registro.");
      return { id, version };
    });
  if (items.length < 1 || items.length > 500) throw new Error("Selecione entre 1 e 500 registros.");
  return items;
}

function OperationStatus({
  value,
}: {
  value: OperationImport["state"] | OperationExport["state"];
}) {
  return <Tag color={statusTone(value)}>{stateLabel(value)}</Tag>;
}

function operationActive(state: OperationImport["state"]): boolean {
  return ["UPLOADING", "PARSING", "QUEUED", "RUNNING"].includes(state);
}

function operationTerminal(state: OperationImport["state"]): boolean {
  return ["COMPLETED", "FAILED", "CANCELLED", "EXPIRED"].includes(state);
}

function statusTone(
  state: OperationImport["state"] | OperationExport["state"],
): "neutral" | "success" | "warning" | "info" {
  if (state === "COMPLETED") return "success";
  if (state === "FAILED" || state === "CANCELLED" || state === "EXPIRED") return "warning";
  if (state === "RUNNING" || state === "PARSING" || state === "QUEUED") return "info";
  return "neutral";
}

function stateLabel(state: OperationImport["state"] | OperationExport["state"]): string {
  return (
    (
      {
        UPLOADING: "Aguardando envio",
        UPLOADED: "Enviado",
        PARSING: "Lendo planilha",
        MAPPING: "Mapeamento",
        PREVIEW_READY: "Correções necessárias",
        DECISIONS_REQUIRED: "Decisões necessárias",
        READY: "Pronta",
        QUEUED: "Na fila",
        RUNNING: "Executando",
        COMPLETED: "Concluída",
        FAILED: "Falhou",
        CANCELLED: "Cancelada",
        EXPIRED: "Expirada",
      } as Record<string, string>
    )[state] ?? state
  );
}

function stageLabel(stage: OperationImport["stage"]): string {
  return (
    (
      {
        UPLOAD: "Upload",
        PARSE: "Leitura segura",
        MAP: "Mapeamento",
        PREVIEW: "Preview e decisões",
        EXECUTE: "Execução",
        REPORT: "Relatório",
        CLEANUP: "Limpeza",
      } as Record<string, string>
    )[stage] ?? stage
  );
}

function actionLabel(action: ImportRow["proposed_action"] | undefined): string {
  return action === "CREATE"
    ? "Criar"
    : action === "UPDATE"
      ? "Atualizar"
      : action === "LINK"
        ? "Vincular"
        : action === "SKIP"
          ? "Ignorar"
          : action === "ERROR"
            ? "Erro"
            : "—";
}

function reportOutcomeLabel(outcome: ImportReportRow["outcome"]): string {
  return outcome === "INSERTED"
    ? "Inserida"
    : outcome === "UPDATED"
      ? "Atualizada"
      : outcome === "LINKED"
        ? "Vinculada"
        : outcome === "SKIPPED"
          ? "Ignorada"
          : outcome === "ERRORED"
            ? "Erro"
            : outcome === "CONFLICTED"
              ? "Conflito"
              : "—";
}

function importResultLabel(value: OperationImport): string {
  const total =
    value.inserted_count +
    value.updated_count +
    value.linked_count +
    value.skipped_count +
    value.errored_count +
    value.conflicted_count;
  return total === 0
    ? "Sem resultados"
    : `${value.inserted_count} inseridas · ${value.updated_count} atualizadas · ${value.linked_count} vinculadas · ${value.skipped_count} ignoradas · ${value.conflicted_count + value.errored_count} com problema`;
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "short" }).format(
    new Date(value),
  );
}

function formatBytes(value: number): string {
  return `${Math.round(value / (1024 * 1024))} MB`;
}

function operationError(error: unknown): string {
  if (error instanceof APIRequestError)
    return error.requestId ? `${error.message} (requisição ${error.requestId})` : error.message;
  if (error instanceof Error) return error.message;
  return "Ocorreu um erro inesperado.";
}

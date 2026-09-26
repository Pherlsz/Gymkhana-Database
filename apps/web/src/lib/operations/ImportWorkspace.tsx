import { Button, Card, Flex, Spin } from "antd";
import { useMutation } from "@tanstack/react-query";
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table";
import { useEffect, useMemo, useState, type ChangeEvent } from "react";
import { StatusBanner } from "../../components/StatusBanner";
import { DataGrid } from "../../DataGrid";
import { useI18n } from "../../i18n";
import "../../operations.css";
import { formatBytes } from "../formatters";
import { CadastroWorkQuery } from "../cadastro/CadastroWork";
import { OcrDropzoneInline } from "../cadastro/components/OcrDropzoneInline";
import {
  actionLabel,
  importResultLabel,
  operationActive,
  operationTerminal,
  OperationStatus,
  reportOutcomeLabel,
  stageLabel,
} from "./operationLabels";
import {
  cancelOperationImport,
  executeOperationImport,
  previewOperationImport,
  saveOperationDecisions,
  saveOperationMapping,
  selectOperationSheet,
  uploadOperationImport,
  type getOperationsCatalog,
  type ImportReportRow,
  type ImportRow,
  type OperationDecision,
  type OperationImport,
  type OperationMapping,
  type OperationReport,
} from "../api/operations";

const rowColumn = createColumnHelper<ImportRow>();
const reportRowColumn = createColumnHelper<ImportReportRow>();

export function ImportCreator({
  catalog,
  onCreated,
}: {
  catalog: Awaited<ReturnType<typeof getOperationsCatalog>>;
  onCreated: (value: OperationImport) => void;
}) {
  const { messages } = useI18n();
  const copy = messages.operations.creator;
  const modules = catalog.modules.filter((module) => module.can_import);
  const module = modules[0]?.id ?? "PROFILES";
  const [file, setFile] = useState<File>();
  const [fileError, setFileError] = useState<string>();
  const [progress, setProgress] = useState(0);

  const pickFile = (next?: File) => {
    if (!next) return;
    if (isCsvFile(next)) {
      setFile(undefined);
      setFileError(copy.csvRejected);
      return;
    }
    setFileError(undefined);
    setFile(next);
  };

  const mutation = useMutation({
    mutationFn: async () => {
      if (!file) throw new Error(copy.selectFilePrompt);
      return uploadOperationImport(file, module, setProgress);
    },
    onSuccess: (value) => {
      setFile(undefined);
      setFileError(undefined);
      onCreated(value);
    },
  });

  if (modules.length === 0) return null;

  return (
    <Card className="operations-creator">
      <form
        onSubmit={(event) => {
          event.preventDefault();
          setProgress(0);
          mutation.mutate();
        }}
      >
        <Flex gap="1rem" vertical>
          <OcrDropzoneInline
            accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
            actionText={copy.dropAction}
            badgeText="XLSX"
            description={`XLSX de até ${formatBytes(catalog.limits.maximum_file_size)}, ${copy.description}`}
            disabled={mutation.isPending}
            inputId="cadastro-import-xlsx"
            label={copy.sheet}
            queuedName={file?.name}
            title={copy.dropTitle}
            variant="banner"
            onFile={(event: ChangeEvent<HTMLInputElement>) => pickFile(event.target.files?.[0])}
          />
          {fileError ? <StatusBanner title={fileError} tone="warning" /> : null}
          {mutation.isError ? (
            <StatusBanner error={mutation.error} title={copy.uploadError} />
          ) : null}
          {mutation.isPending ? (
            <div aria-live="polite" className="operations-progress">
              <span>
                {copy.uploading} {progress}%
              </span>
              <progress max={100} value={progress} />
            </div>
          ) : null}
          <Flex>
            <Button disabled={!file || mutation.isPending} htmlType="submit">
              {mutation.isPending ? copy.btnSending : copy.btnSend}
            </Button>
          </Flex>
        </Flex>
      </form>
    </Card>
  );
}

function isCsvFile(file: File) {
  const name = file.name.toLowerCase();
  return (
    name.endsWith(".csv") || file.type === "text/csv" || file.type === "text/comma-separated-values"
  );
}

function previewCellSummary(
  row: ImportRow,
  module: Awaited<ReturnType<typeof getOperationsCatalog>>["modules"][number] | undefined,
  columns: OperationImport["columns"],
  copy: { cellErrors: Record<string, string> },
) {
  const fieldByColumn = new Map(
    columns.map((column) => [column.source_column, column.target_field]),
  );
  const labelByField = new Map((module?.fields ?? []).map((field) => [field.id, field.label]));
  return row.cells
    .map((cell) => {
      const field = fieldByColumn.get(cell.source_column);
      if (!field) return "";
      const label = labelByField.get(field) ?? field;
      const error = cell.validation_code
        ? ` (${copy.cellErrors[cell.validation_code] ?? cell.validation_code})`
        : "";
      return `${label}: ${cell.raw_value || "—"}${error}`;
    })
    .filter(Boolean)
    .join(" · ");
}

export function ImportWorkspace({
  value,
  catalog,
  loading,
  error,
  onUpdated,
  report,
  reportError,
  reportLoading,
}: {
  value: OperationImport | undefined;
  catalog: Awaited<ReturnType<typeof getOperationsCatalog>>["modules"];
  loading: boolean;
  error: Error | null;
  onUpdated: () => Promise<void>;
  report: OperationReport | undefined;
  reportError: Error | null;
  reportLoading: boolean;
}) {
  const { messages } = useI18n();
  const copy = messages.operations.workspace;
  const [adjustMapping, setAdjustMapping] = useState(false);
  useEffect(() => {
    setAdjustMapping(false);
  }, [value?.id, value?.state]);

  if (loading || error) {
    return (
      <CadastroWorkQuery
        errorTitle={copy.loadError}
        isError={Boolean(error)}
        isPending={loading}
        onRetry={error ? () => void onUpdated() : undefined}
      />
    );
  }
  if (!value) return null;

  const module = catalog.find((item) => item.id === value.module);
  const resultCount =
    value.inserted_count +
    value.updated_count +
    value.linked_count +
    value.skipped_count +
    value.conflicted_count +
    value.errored_count +
    value.validation_error_count;
  const showCounts =
    value.state !== "MAPPING" &&
    value.stage !== "MAP" &&
    (value.state === "COMPLETED" || resultCount > 0);

  return (
    <div className="operations-workspace">
      <header className="operations-workspace__header">
        <h2 className="operations-workspace__title">{value.original_filename}</h2>
        <p className="operations-workspace__meta">
          <span>{module?.label ?? value.module}</span>
          <span aria-hidden="true">·</span>
          <span>{stageLabel(value.stage, messages)}</span>
          <OperationStatus value={value.state} />
          {operationActive(value.state) ? <Spin size="small" /> : null}
        </p>
      </header>
      {showCounts ? <ImportSummary value={value} /> : null}
      {operationActive(value.state) ? (
        <p className="operations-workspace__note">{copy.activeAlert}</p>
      ) : null}
      {value.state === "FAILED" && value.error_code ? (
        <StatusBanner
          description={
            <>
              {copy.failedSecureCode} {value.error_code}.
            </>
          }
          title={copy.failedTitle}
          tone="error"
        />
      ) : null}
      {value.state === "CANCELLED" ? (
        <p className="operations-workspace__note">{copy.cancelledAlert}</p>
      ) : null}
      {value.state === "MAPPING" || adjustMapping ? (
        <MappingWorkspace module={module} onUpdated={onUpdated} value={value} />
      ) : null}
      {!adjustMapping &&
      (value.stage === "PREVIEW" ||
        value.state === "PREVIEW_READY" ||
        value.state === "DECISIONS_REQUIRED" ||
        value.state === "READY") ? (
        <PreviewWorkspace
          module={module}
          onAdjustMapping={() => setAdjustMapping(true)}
          onUpdated={onUpdated}
          value={value}
        />
      ) : null}
      {value.state === "COMPLETED" ? (
        <ImportReportPanel error={reportError} loading={reportLoading} value={report} />
      ) : null}
      {!operationTerminal(value.state) ? (
        <CancelImportButton onUpdated={onUpdated} value={value} />
      ) : null}
    </div>
  );
}

function ImportSummary({ value }: { value: OperationImport }) {
  const { messages } = useI18n();
  const summary = messages.operations.summary;

  const items = [
    [summary.inserted, value.inserted_count],
    [summary.updated, value.updated_count],
    [summary.linked, value.linked_count],
    [summary.skipped, value.skipped_count],
    [summary.conflicts, value.conflicted_count],
    [summary.errors, value.errored_count + value.validation_error_count],
  ].filter(([, count]) => value.state === "COMPLETED" || Number(count) > 0);
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

function ImportReportPanel({
  value,
  loading,
  error,
}: {
  value: OperationReport | undefined;
  loading: boolean;
  error: Error | null;
}) {
  const { messages } = useI18n();
  const copy = messages.operations.workspace;
  const summary = messages.operations.summary;

  const reportColumns: ColumnDef<ImportReportRow, any>[] = useMemo(
    () => [
      reportRowColumn.accessor("sheet_index", {
        header: copy.reportSheet,
        cell: ({ getValue }) => getValue() + 1,
      }),
      reportRowColumn.accessor("row_number", { header: copy.reportLine }),
      reportRowColumn.accessor("outcome", {
        header: copy.outcome,
        cell: ({ getValue }) => reportOutcomeLabel(getValue(), messages),
      }),
      reportRowColumn.accessor("error_code", {
        header: copy.reportSecureCode,
        cell: ({ getValue }) => getValue() || "—",
      }),
    ],
    [copy, messages],
  );

  if (loading) {
    return <StatusBanner description={copy.reportWait} title={copy.reportLoading} tone="info" />;
  }
  if (error) {
    return <StatusBanner error={error} title={copy.reportLoadError} />;
  }
  if (!value) return null;

  const items = [
    [summary.inserted, value.inserted],
    [summary.updated, value.updated],
    [summary.linked, value.linked],
    [summary.skipped, value.skipped],
    [summary.errors, value.errored],
    [summary.conflicts, value.conflicted],
    [summary.decisionsSaved, value.decisions],
    [summary.pending, value.unresolved],
    [summary.validationErrors, value.validation_errors],
  ];

  return (
    <Flex gap="1rem" vertical>
      <div>
        <h3>{copy.reportTitle}</h3>
        <p className="operations-muted">{copy.reportDesc}</p>
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
        caption={copy.reportTitle}
        columns={reportColumns}
        data={value.rows}
        emptyLabel={copy.reportEmpty}
        getRowId={(row) => `${row.sheet_index}:${row.row_number}`}
        loading={false}
        loadingLabel={copy.reportLoading}
        renderCard={(row) => (
          <article className="operations-card" key={`${row.sheet_index}:${row.row_number}`}>
            <strong>
              {copy.reportLine} {row.row_number}
            </strong>
            <span>
              {copy.reportSheet} {row.sheet_index + 1}
            </span>
            <span>{reportOutcomeLabel(row.outcome, messages)}</span>
            {row.error_code ? (
              <span>
                {copy.reportSecureCode}: {row.error_code}
              </span>
            ) : null}
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
  const { messages } = useI18n();
  const copy = messages.operations.workspace;
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

  const proceed = useMutation({
    mutationFn: async () => {
      const selected: OperationMapping = Object.entries(mapping)
        .filter(([, target]) => target)
        .map(([source, target]) => ({ source_column: Number(source), target_field: target }));
      const current =
        mappingDirty && !mappingLocked ? await saveOperationMapping(value, selected) : value;
      return previewOperationImport(current);
    },
    onSuccess: onUpdated,
  });

  const mutationError = selectSheet.error ?? proceed.error;
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
    <Flex gap="1rem" vertical>
      {value.sheets.length > 1 ? (
        <div>
          <h3>{copy.stepSheet}</h3>
          <Flex>
            {value.sheets.map((sheet) => (
              <Button
                aria-pressed={value.selected_sheet_index === sheet.index}
                disabled={mappingLocked || selectSheet.isPending}
                key={sheet.index}
                onClick={() => selectSheet.mutate(sheet.index)}
              >
                {sheet.name} · {sheet.row_count} {messages.operations.export.linesCount}
                {value.selected_sheet_index === sheet.index ? ` · ${copy.sheetSelected}` : ""}
              </Button>
            ))}
          </Flex>
        </div>
      ) : null}
      {mappingLocked ? (
        <StatusBanner
          description={copy.mappingPreservedDesc}
          title={copy.mappingPreservedTitle}
          tone="info"
        />
      ) : null}
      {value.selected_sheet_index != null ? (
        <div>
          <h3>{copy.stepMap}</h3>
          <p className="operations-muted">{copy.step2Desc}</p>
          {!mappingReady ? (
            <p className="operations-muted">
              {copy.mappingRequired}:{" "}
              {required
                .filter((field) => !selectedTargets.has(field))
                .map((field) => module?.fields.find((item) => item.id === field)?.label ?? field)
                .join(", ")}
            </p>
          ) : null}
          <div className="operations-mapping">
            {value.columns.map((column) => (
              <label key={column.source_column}>
                <span>{column.source_header}</span>
                <select
                  aria-label={`${copy.mapAria} ${column.source_header}`}
                  onChange={(event) =>
                    setMapping((current) => ({
                      ...current,
                      [column.source_column]: event.target.value,
                    }))
                  }
                  value={mapping[column.source_column] ?? ""}
                >
                  <option value="">{copy.ignoreColumn}</option>
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
              disabled={!mappingReady || proceed.isPending || (mappingLocked && mappingDirty)}
              onClick={() => proceed.mutate()}
            >
              {copy.btnContinue}
            </Button>
          </Flex>
        </div>
      ) : null}
      {mutationError ? <StatusBanner error={mutationError} title={copy.prepError} /> : null}
    </Flex>
  );
}

function isPreviewException(row: ImportRow) {
  return (
    row.decision_required ||
    row.validation_error_count > 0 ||
    row.proposed_action === "ERROR" ||
    row.proposed_action === "UPDATE" ||
    row.proposed_action === "LINK"
  );
}

function PreviewWorkspace({
  value,
  module,
  onAdjustMapping,
  onUpdated,
}: {
  value: OperationImport;
  module: Awaited<ReturnType<typeof getOperationsCatalog>>["modules"][number] | undefined;
  onAdjustMapping: () => void;
  onUpdated: () => Promise<void>;
}) {
  const { messages } = useI18n();
  const copy = messages.operations.workspace;
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

  const exceptionRows = useMemo(() => value.preview.filter(isPreviewException), [value.preview]);
  const sheetRows =
    value.sheets.find((sheet) => sheet.index === value.selected_sheet_index)?.row_count ??
    value.preview.length;

  const columns = useMemo(
    () => [
      rowColumn.accessor("row_number", { header: copy.reportLine }),
      rowColumn.accessor("proposed_action", {
        header: copy.proposedAction,
        cell: ({ getValue }) => actionLabel(getValue(), messages),
      }),
      rowColumn.display({
        id: "values",
        header: copy.values,
        cell: ({ row }) => previewCellSummary(row.original, module, value.columns, copy) || "—",
      }),
      rowColumn.accessor("validation_error_count", {
        header: messages.operations.summary.errors,
      }),
      rowColumn.display({
        id: "decision",
        header: copy.decision,
        cell: ({ row }) =>
          row.original.decision_required ? (
            <select
              aria-label={`${copy.decisionAria} ${row.original.row_number}`}
              onChange={(event) =>
                setActions((current) => ({
                  ...current,
                  [row.original.row_number]: event.target.value as "UPDATE" | "LINK" | "SKIP",
                }))
              }
              value={actions[row.original.row_number] ?? "UPDATE"}
            >
              <option value="UPDATE">{copy.actionUpdate}</option>
              <option value="LINK">{copy.actionLink}</option>
              <option value="SKIP">{copy.actionSkip}</option>
            </select>
          ) : (
            actionLabel(row.original.decision || row.original.proposed_action, messages)
          ),
      }),
    ],
    [actions, copy, messages, module, value.columns],
  );

  return (
    <Flex gap="1rem" vertical>
      <div>
        <h3>{copy.previewTitle}</h3>
        <p className="operations-muted">
          {sheetRows} {copy.previewReadyLines}
          {value.unresolved_count ? ` · ${value.unresolved_count} ${copy.previewNeedDecision}` : ""}
          {value.validation_error_count
            ? ` · ${value.validation_error_count} ${copy.previewWithError}`
            : ""}
          {exceptionRows.length === 0 ? ` · ${copy.previewAllClear}` : ""}
        </p>
      </div>
      <Flex>
        <Button onClick={onAdjustMapping}>{copy.btnAdjustColumns}</Button>
      </Flex>
      {value.validation_error_count > 0 ? (
        <StatusBanner
          description={
            <>
              {value.validation_error_count} {copy.validationWarningDesc}
            </>
          }
          title={copy.validationWarning}
          tone="warning"
        />
      ) : null}
      {exceptionRows.length > 0 ? (
        <DataGrid
          caption={`${copy.previewTitle} - ${module?.label ?? value.module}`}
          columns={columns}
          data={exceptionRows}
          emptyLabel={copy.emptyPreview}
          getRowId={(row) => String(row.row_number)}
          loading={false}
          loadingLabel={copy.loadingPreview}
          renderCard={(row) => (
            <article className="operations-card" key={row.row_number}>
              <strong>
                {copy.reportLine} {row.row_number}
              </strong>
              <span>
                {copy.proposedAction}: {actionLabel(row.proposed_action, messages)}
              </span>
              <span>{previewCellSummary(row, module, value.columns, copy) || copy.noValues}</span>
              {row.decision_required ? (
                <label>
                  {copy.decisionAria} {row.row_number}
                  <select
                    onChange={(event) =>
                      setActions((current) => ({
                        ...current,
                        [row.row_number]: event.target.value as "UPDATE" | "LINK" | "SKIP",
                      }))
                    }
                    value={actions[row.row_number] ?? "UPDATE"}
                  >
                    <option value="UPDATE">{copy.actionUpdate}</option>
                    <option value="LINK">{copy.actionLink}</option>
                    <option value="SKIP">{copy.actionSkip}</option>
                  </select>
                </label>
              ) : (
                <span>
                  {copy.decision}: {actionLabel(row.decision || row.proposed_action, messages)}
                </span>
              )}
            </article>
          )}
        />
      ) : null}
      {value.state === "DECISIONS_REQUIRED" ? (
        <Button disabled={decisions.isPending} onClick={() => decisions.mutate()}>
          {copy.btnSaveDecisions} ({value.unresolved_count})
        </Button>
      ) : null}
      {value.state === "READY" ? (
        <Button disabled={execute.isPending} onClick={() => execute.mutate()}>
          {copy.btnExecute}
        </Button>
      ) : null}
      {value.state === "COMPLETED" ? (
        <StatusBanner
          description={
            <>
              {importResultLabel(value, messages)}. {copy.completedDesc}
            </>
          }
          title={copy.completedTitle}
          tone="info"
        />
      ) : null}
      {decisions.error || execute.error ? (
        <StatusBanner error={decisions.error ?? execute.error} title={copy.advanceError} />
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
  const { messages } = useI18n();
  const copy = messages.operations.workspace;
  const mutation = useMutation({
    mutationFn: () => cancelOperationImport(value),
    onSuccess: onUpdated,
  });

  return (
    <Flex gap="0.5rem" vertical>
      <Flex>
        <Button disabled={mutation.isPending} onClick={() => mutation.mutate()}>
          {copy.btnCancelOp}
        </Button>
      </Flex>
      {mutation.isError ? <StatusBanner error={mutation.error} title={copy.cancelError} /> : null}
    </Flex>
  );
}

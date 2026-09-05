import { Alert, Button, Card, Flex, Skeleton, Typography } from "antd";
import { useMutation } from "@tanstack/react-query";
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table";
import { useEffect, useMemo, useState } from "react";
import { DataGrid } from "../../DataGrid";
import { useI18n } from "../../i18n";
import { formatBytes, errorMessage as operationError } from "../formatters";
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
  type OperationModule,
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
  const [module, setModule] = useState<OperationModule>(modules[0]?.id ?? "PROFILES");
  const [file, setFile] = useState<File>();
  const [progress, setProgress] = useState(0);

  const mutation = useMutation({
    mutationFn: async () => {
      if (!file) throw new Error(copy.selectFilePrompt);
      return uploadOperationImport(file, module, setProgress);
    },
    onSuccess: (value) => {
      setFile(undefined);
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
          <div>
            <strong>{copy.title}</strong>
            <p className="operations-muted">
              XLSX de até {formatBytes(catalog.limits.maximum_file_size)}, {copy.description}
            </p>
          </div>
          <div className="operations-form-grid">
            <label>
              {copy.module}
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
              {copy.sheet}
              <input
                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                type="file"
                onChange={(event) => setFile(event.target.files?.[0])}
              />
            </label>
          </div>
          {mutation.isError ? (
            <Alert
              description={<>{operationError(mutation.error)}</>}
              message={copy.uploadError}
              type="error"
            />
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

export function ImportWorkspace({
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
  const { messages } = useI18n();
  const copy = messages.operations.workspace;

  if (loading) {
    return (
      <Card className="operations-workspace">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }
  if (error) {
    return (
      <Card className="operations-workspace">
        <Alert description={<>{operationError(error)}</>} message={copy.loadError} type="error" />
      </Card>
    );
  }
  if (!value) return null;

  const module = catalog.find((item) => item.id === value.module);

  return (
    <Card className="operations-workspace">
      <Flex gap="1.5rem" vertical>
        <div className="operations-workspace__header">
          <div>
            <Typography.Title level={2}>{value.original_filename}</Typography.Title>
            <p className="operations-muted">
              {module?.label ?? value.module} · {copy.headerId} {value.id}
            </p>
          </div>
          <div className="operations-workspace__actions">
            <OperationStatus value={value.state} />
            <Button onClick={onClose}>{messages.common.actions.close}</Button>
          </div>
        </div>
        <ImportSummary value={value} />
        {operationActive(value.state) ? (
          <Alert description={copy.activeAlert} message={copy.activeTitle} type="info" />
        ) : null}
        {value.state === "FAILED" && value.error_code ? (
          <Alert
            description={
              <>
                {copy.failedSecureCode} {value.error_code}.
              </>
            }
            message={copy.failedTitle}
            type="error"
          />
        ) : null}
        {value.state === "CANCELLED" ? (
          <Alert description={copy.cancelledAlert} message={copy.cancelledTitle} type="warning" />
        ) : null}
        {value.stage === "MAP" || value.state === "MAPPING" ? (
          <MappingWorkspace module={module} onUpdated={onUpdated} value={value} />
        ) : null}
        {value.stage === "PREVIEW" ||
        value.state === "PREVIEW_READY" ||
        value.state === "DECISIONS_REQUIRED" ||
        value.state === "READY" ? (
          <PreviewWorkspace module={module} onUpdated={onUpdated} value={value} />
        ) : null}
        {value.state === "COMPLETED" ? (
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
  const { messages } = useI18n();
  const summary = messages.operations.summary;

  const items = [
    [messages.common.labels.stage, stageLabel(value.stage, messages)],
    [messages.common.labels.version, String(value.version)],
    [summary.inserted, String(value.inserted_count)],
    [summary.updated, String(value.updated_count)],
    [summary.linked, String(value.linked_count)],
    [summary.skipped, String(value.skipped_count)],
    [summary.conflicts, String(value.conflicted_count)],
    [summary.errors, String(value.errored_count + value.validation_error_count)],
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
    return <Alert description={copy.reportWait} message={copy.reportLoading} type="info" />;
  }
  if (error) {
    return (
      <Alert
        description={<>{operationError(error)}</>}
        message={copy.reportLoadError}
        type="error"
      />
    );
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
    <Flex gap="1rem" vertical>
      <div>
        <h3>{copy.step1Sheet}</h3>
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
      {mappingLocked ? (
        <Alert
          description={copy.mappingPreservedDesc}
          message={copy.mappingPreservedTitle}
          type="info"
        />
      ) : null}
      {value.selected_sheet_index !== undefined ? (
        <div>
          <h3>{copy.step2Map}</h3>
          <p className="operations-muted">{copy.step2Desc}</p>
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
              disabled={mappingLocked || !mappingReady || save.isPending}
              onClick={() => save.mutate()}
            >
              {copy.btnSaveMapping}
            </Button>
            <Button
              disabled={!mappingReady || mappingDirty || save.isPending || preview.isPending}
              onClick={() => preview.mutate()}
            >
              {copy.btnValidatePreview}
            </Button>
          </Flex>
        </div>
      ) : null}
      {mutationError ? (
        <Alert
          description={<>{operationError(mutationError)}</>}
          message={copy.prepError}
          type="error"
        />
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
        cell: ({ row }) =>
          row.original.cells
            .slice(0, 4)
            .map((cell: { raw_value?: string }) => cell.raw_value)
            .filter(Boolean)
            .join(" · ") || "—",
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
    [actions, copy, messages],
  );

  return (
    <Flex gap="1rem" vertical>
      <div>
        <h3>{copy.previewTitle}</h3>
        <p className="operations-muted">{copy.previewDesc}</p>
      </div>
      {value.validation_error_count > 0 ? (
        <Alert
          description={
            <>
              {value.validation_error_count} {copy.validationWarningDesc}
            </>
          }
          message={copy.validationWarning}
          type="warning"
        />
      ) : null}
      <DataGrid
        caption={`${copy.previewTitle} - ${module?.label ?? value.module}`}
        columns={columns}
        data={value.preview}
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
            <span>
              {row.cells
                .slice(0, 4)
                .map((cell: { raw_value?: string }) => cell.raw_value)
                .filter(Boolean)
                .join(" · ") || copy.noValues}
            </span>
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
        <Alert
          description={
            <>
              {importResultLabel(value, messages)}. {copy.completedDesc}
            </>
          }
          message={copy.completedTitle}
          type="info"
        />
      ) : null}
      {decisions.error || execute.error ? (
        <Alert
          description={<>{operationError(decisions.error ?? execute.error)}</>}
          message={copy.advanceError}
          type="error"
        />
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
      {mutation.isError ? (
        <Alert
          description={<>{operationError(mutation.error)}</>}
          message={copy.cancelError}
          type="error"
        />
      ) : null}
    </Flex>
  );
}

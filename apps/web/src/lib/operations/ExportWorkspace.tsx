import { Alert, Button, Flex, Typography } from "antd";
import { useMutation } from "@tanstack/react-query";
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table";
import { useMemo } from "react";
import { DataGrid } from "../../DataGrid";
import { useI18n } from "../../i18n";
import { formatDateTime as formatDate, errorMessage as operationError } from "../formatters";
import { OperationStatus } from "./operationLabels";
import {
  createOperationExport,
  downloadOperationExport,
  type getOperationsCatalog,
  type OperationExport,
} from "../api/operations";

const exportColumn = createColumnHelper<OperationExport>();

export function ExportWorkspace({
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
  const { messages } = useI18n();
  const copy = messages.operations.export;
  const mutation = useMutation({ mutationFn: createOperationExport, onSuccess: onUpdated });
  const download = useMutation({ mutationFn: downloadOperationExport });
  const available = modules.filter((module) => module.can_export);
  const moduleLabels = useMemo(
    () => new Map(modules.map((module) => [module.id, module.label])),
    [modules],
  );

  const columns: ColumnDef<OperationExport, any>[] = useMemo(
    () => [
      exportColumn.accessor("filename", { header: messages.common.labels.file }),
      exportColumn.accessor("module", {
        header: messages.common.labels.module,
        cell: ({ getValue }) => moduleLabels.get(getValue()) ?? getValue(),
      }),
      exportColumn.accessor("state", {
        header: messages.common.labels.status,
        cell: ({ getValue }) => <OperationStatus value={getValue()} />,
      }),
      exportColumn.accessor("row_count", { header: messages.common.labels.lines }),
      exportColumn.display({
        id: "detail",
        header: messages.common.labels.detail,
        cell: ({ row }) =>
          row.original.error_code
            ? `${copy.secureCode} ${row.original.error_code}`
            : `${copy.expires} ${formatDate(row.original.expires_at)}`,
      }),
      exportColumn.display({
        id: "actions",
        header: messages.common.entities.admin,
        cell: ({ row }) =>
          row.original.state === "COMPLETED" ? (
            <Button disabled={download.isPending} onClick={() => download.mutate(row.original)}>
              {copy.btnDownload}
            </Button>
          ) : null,
      }),
    ],
    [copy, download.isPending, download.mutate, messages.common, moduleLabels],
  );

  return (
    <section className="page-section">
      <Typography.Title level={2}>{copy.title}</Typography.Title>
      <Typography.Paragraph>{copy.description}</Typography.Paragraph>
      <Flex gap="1rem" vertical>
        <Flex>
          {available.map((module) => (
            <Button
              disabled={mutation.isPending}
              key={module.id}
              onClick={() => mutation.mutate(module.id)}
            >
              {copy.btnExport} {module.label}
            </Button>
          ))}
        </Flex>
        {mutation.isError || download.isError || error ? (
          <Alert
            description={<>{operationError(mutation.error ?? download.error ?? error)}</>}
            message={copy.error}
            type="error"
          />
        ) : null}
        <DataGrid
          caption={copy.caption}
          columns={columns}
          data={exports}
          emptyLabel={copy.empty}
          getRowId={(value) => value.id}
          loading={loading}
          loadingLabel={copy.loading}
          renderCard={(value) => (
            <article className="operations-card" key={value.id}>
              <strong>{value.filename}</strong>
              <span>{moduleLabels.get(value.module) ?? value.module}</span>
              <OperationStatus value={value.state} />
              <span>
                {value.row_count} {copy.linesCount}
              </span>
              {value.error_code ? (
                <span>
                  {copy.secureCode} {value.error_code}
                </span>
              ) : null}
              {value.state === "COMPLETED" ? (
                <Button disabled={download.isPending} onClick={() => download.mutate(value)}>
                  {copy.btnDownloadExport}
                </Button>
              ) : null}
            </article>
          )}
        />
      </Flex>
    </section>
  );
}

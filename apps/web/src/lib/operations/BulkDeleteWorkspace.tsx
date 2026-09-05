import { Alert, Button, Card, Flex, Typography } from "antd";
import { useMutation } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { useI18n } from "../../i18n";
import { errorMessage as operationError } from "../formatters";
import {
  bulkDeleteOperationRecords,
  type getOperationsCatalog,
  type OperationModule,
} from "../api/operations";

export const operationIdentifierPattern = /^[0-9a-fA-F-]{36}$/;

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
      ) {
        throw new Error("Use uma linha UUID,versão para cada registro.");
      }
      return { id, version };
    });
  if (items.length < 1 || items.length > 500) {
    throw new Error("Selecione entre 1 e 500 registros.");
  }
  return items;
}

export function BulkDeleteWorkspace({
  modules,
  onUpdated,
}: {
  modules: Awaited<ReturnType<typeof getOperationsCatalog>>["modules"];
  onUpdated: () => Promise<void>;
}) {
  const { messages } = useI18n();
  const copy = messages.operations.bulkDelete;
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
    <section className="page-section">
      <Typography.Title level={2}>{copy.title}</Typography.Title>
      <Typography.Paragraph>{copy.description}</Typography.Paragraph>
      <Card className="operations-danger-zone">
        <form onSubmit={submit}>
          <Flex gap="1rem" vertical>
            <div className="operations-form-grid">
              <label>
                {messages.common.labels.module}
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
                {messages.common.actions.confirm}
                <input
                  autoComplete="off"
                  placeholder={messages.common.placeholders.confirm}
                  value={confirmation}
                  onChange={(event) => setConfirmation(event.target.value)}
                />
              </label>
            </div>
            <label>
              {copy.recordsLabel}
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
              {copy.helpText}
            </span>
            {parseError ? (
              <Alert message={copy.reviewWarning} type="warning" description={<>{parseError}</>} />
            ) : null}
            {mutation.isError ? (
              <Alert
                message={copy.noneDeleted}
                type="error"
                description={<>{operationError(mutation.error)}</>}
              />
            ) : null}
            {mutation.isSuccess ? (
              <Alert
                message={copy.completed}
                type="info"
                description={
                  <>
                    {mutation.data.deleted} {copy.completedCount}
                  </>
                }
              />
            ) : null}
            <Flex>
              <Button
                disabled={
                  confirmation !== messages.common.actions.confirm ||
                  !selection.trim() ||
                  mutation.isPending
                }
                htmlType="submit"
              >
                {copy.btnSubmit}
              </Button>
            </Flex>
          </Flex>
        </form>
      </Card>
    </section>
  );
}

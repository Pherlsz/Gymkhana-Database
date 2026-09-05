import { Tag } from "antd";
import { useI18n } from "../../i18n";
import type { TranslationKeys } from "../../i18n/v1/pt-BR";
import type {
  ImportReportRow,
  ImportRow,
  OperationExport,
  OperationImport,
} from "../api/operations";

export function OperationStatus({
  value,
}: {
  value: OperationImport["state"] | OperationExport["state"];
}) {
  const { messages } = useI18n();
  return <Tag color={statusTone(value)}>{stateLabel(value, messages)}</Tag>;
}

export function operationActive(state: OperationImport["state"]): boolean {
  return ["UPLOADING", "PARSING", "QUEUED", "RUNNING"].includes(state);
}

export function operationTerminal(state: OperationImport["state"]): boolean {
  return ["COMPLETED", "FAILED", "CANCELLED", "EXPIRED"].includes(state);
}

export function statusTone(
  state: OperationImport["state"] | OperationExport["state"],
): "neutral" | "success" | "warning" | "info" {
  if (state === "COMPLETED") return "success";
  if (state === "FAILED" || state === "CANCELLED" || state === "EXPIRED") return "warning";
  if (state === "RUNNING" || state === "PARSING" || state === "QUEUED") return "info";
  return "neutral";
}

function dictLabel(
  dict: string,
  key: string | undefined,
  messages: TranslationKeys,
  fallback = "—",
): string {
  if (!key) return fallback;
  return (
    (messages.operations as unknown as Record<string, Record<string, string>>)[dict]?.[key] ??
    fallback
  );
}

export function stateLabel(
  state: OperationImport["state"] | OperationExport["state"],
  messages: TranslationKeys,
): string {
  return dictLabel("states", state, messages, state);
}

export function stageLabel(stage: OperationImport["stage"], messages: TranslationKeys): string {
  return dictLabel("stages", stage, messages, stage);
}

export function actionLabel(
  action: ImportRow["proposed_action"] | undefined,
  messages: TranslationKeys,
): string {
  return dictLabel("actions", action, messages);
}

export function reportOutcomeLabel(
  outcome: ImportReportRow["outcome"] | undefined,
  messages: TranslationKeys,
): string {
  return dictLabel("outcomes", outcome, messages);
}

export function importResultLabel(value: OperationImport, messages: TranslationKeys): string {
  const total =
    value.inserted_count +
    value.updated_count +
    value.linked_count +
    value.skipped_count +
    value.errored_count +
    value.conflicted_count;
  const s = messages.operations.summary;
  if (total === 0) return s.noResults;
  return `${value.inserted_count} ${s.insertedShort} · ${value.updated_count} ${s.updatedShort} · ${value.linked_count} ${s.linkedShort} · ${value.skipped_count} ${s.skippedShort} · ${value.conflicted_count + value.errored_count} ${s.problemsShort}`;
}

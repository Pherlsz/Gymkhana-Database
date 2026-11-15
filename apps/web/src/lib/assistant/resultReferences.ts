import { formatMessage, plural } from "../../i18n";
import type { ChatMessage } from "./assistantApi";

export type ChatMessageReference = ChatMessage["result_references"][number];

/**
 * The server lists only references worth opening on the grid: they have rows
 * and a criterion. A reply that searched and sampled along the way ends with
 * the result it answered with, so the last one is the default.
 */
export function gridReferences(message: Pick<ChatMessage, "result_references">) {
  return message.result_references ?? [];
}

export function preferredGridReference(message: Pick<ChatMessage, "result_references">): string {
  const references = gridReferences(message);
  return references[references.length - 1]?.id ?? "";
}

const QUERY_PREFIX = "Consulta: ";

export function referenceOptionLabel(
  reference: ChatMessageReference,
  copy: { entities: Record<string, string>; rowsOne: string; rowsOther: string },
): string {
  let label = reference.label;
  if (label.startsWith(QUERY_PREFIX)) {
    const entity = label.slice(QUERY_PREFIX.length);
    label = QUERY_PREFIX + (copy.entities[entity] ?? entity);
  }
  const rows = plural(reference.row_count, copy.rowsOne, copy.rowsOther);
  return `${label} · ${formatMessage(rows, { count: reference.row_count })}`;
}

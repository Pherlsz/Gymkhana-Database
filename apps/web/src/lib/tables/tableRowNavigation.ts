import type { ProfileListSearch } from "../api/client";
import { EMPTY_CELL, cellText, type TableRow } from "./tableRows";

export type TableRowEntityKind = "person" | "document" | "bill";

export function tableRowEntityKind(
  row: TableRow,
  args: { section: "profile" | "documents" | "bills"; isResult: boolean },
): TableRowEntityKind {
  if (args.isResult) {
    const kind = String(row.cells.entityKind ?? "");
    if (kind === "document") return "document";
    if (kind === "bill") return "bill";
    return "person";
  }
  if (args.section === "documents") return "document";
  if (args.section === "bills") return "bill";
  return "person";
}

export function tableRowEntityId(row: TableRow, isResult: boolean): string {
  if (isResult) return String(row.cells.entityId ?? row.id);
  return row.id;
}

export function tableRowAllowsCurrentUse(row: TableRow, kind: TableRowEntityKind): boolean {
  if (kind !== "document" && kind !== "bill") return false;
  return (
    row.cells.medium_key === "PHYSICAL" &&
    (row.cells.status_key === "AVAILABLE" || row.cells.status_key === "IN_USE")
  );
}

export function tableRowInUse(row: TableRow): boolean {
  return row.cells.status_key === "IN_USE";
}

export function tableRowActionSubject(row: TableRow, kind: TableRowEntityKind): string {
  const labeled = cellText(row.cells.entityLabel).trim();
  const raw =
    kind === "document"
      ? cellText(row.cells.identifier)
      : kind === "bill"
        ? cellText(row.cells.reference)
        : cellText(row.cells.full_name);
  const text = (raw.trim() && raw !== EMPTY_CELL ? raw : labeled).trim();
  return text && text !== EMPTY_CELL ? text : "";
}

export function tableRowSearchPatch(
  row: TableRow,
  args: {
    section: "profile" | "documents" | "bills";
    isResult: boolean;
    intent: "view" | "edit";
  },
): Partial<ProfileListSearch> {
  const id = tableRowEntityId(row, args.isResult);
  const kind = tableRowEntityKind(row, args);
  if (kind === "document") {
    return {
      selected: undefined,
      mode: undefined,
      document_selected: id,
      document_mode: args.intent,
      bill_selected: undefined,
      bill_mode: undefined,
    };
  }
  if (kind === "bill") {
    return {
      selected: undefined,
      mode: undefined,
      bill_selected: id,
      bill_mode: args.intent,
      document_selected: undefined,
      document_mode: undefined,
    };
  }
  if (args.isResult) {
    return {
      selected: id,
      mode: args.intent,
      document_selected: undefined,
      document_mode: undefined,
      bill_selected: undefined,
      bill_mode: undefined,
    };
  }
  return { selected: id, mode: args.intent };
}

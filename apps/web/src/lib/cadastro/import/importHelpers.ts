import type { OperationImport } from "../../api/operations";

export function operationActive(state: OperationImport["state"]): boolean {
  return ["UPLOADING", "PARSING", "QUEUED", "RUNNING"].includes(state);
}

export function operationTerminal(state: OperationImport["state"]): boolean {
  return ["COMPLETED", "FAILED", "CANCELLED", "EXPIRED"].includes(state);
}

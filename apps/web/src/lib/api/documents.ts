import { requestJSON, type DocumentRecord } from "./client";

export function getDocument(id: string, signal?: AbortSignal): Promise<DocumentRecord> {
  return requestJSON<DocumentRecord>(
    `/api/v1/documents/${encodeURIComponent(id)}`,
    signal ? { signal } : {},
  );
}

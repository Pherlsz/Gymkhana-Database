import type { paths } from "../../generated/operations-api";
import { APIRequestError, apiURL, requestNoContent } from "./client";

export type OperationsCatalog =
  paths["/api/v1/operations/catalog"]["get"]["responses"][200]["content"]["application/json"];
export type OperationModule = OperationsCatalog["modules"][number]["id"];
export type OperationImport =
  paths["/api/v1/operations/imports/{import_id}"]["get"]["responses"][200]["content"]["application/json"];
export type OperationImportPage =
  paths["/api/v1/operations/imports"]["get"]["responses"][200]["content"]["application/json"];
export type OperationReport =
  paths["/api/v1/operations/imports/{import_id}/report"]["get"]["responses"][200]["content"]["application/json"];
export type OperationExport =
  paths["/api/v1/operations/exports/{export_id}"]["get"]["responses"][200]["content"]["application/json"];
export type OperationExportPage =
  paths["/api/v1/operations/exports"]["get"]["responses"][200]["content"]["application/json"];
export type OperationMapping =
  paths["/api/v1/operations/imports/{import_id}/mapping"]["put"]["requestBody"]["content"]["application/json"]["mapping"];
export type OperationDecision =
  paths["/api/v1/operations/imports/{import_id}/decisions"]["put"]["requestBody"]["content"]["application/json"]["decisions"][number];
export type ImportRow = OperationImport["preview"][number];
export type ImportReportRow = OperationReport["rows"][number];

type ErrorPayload = {
  error?: { code?: string; message?: string };
  request_id?: string;
  field_errors?: Array<{ field: string; code: string; message: string }>;
};

async function readJSON<T>(response: Response): Promise<T> {
  const contentType = response.headers.get("content-type") ?? "";
  if (!contentType.toLowerCase().includes("application/json")) {
    throw new APIRequestError("API response is not JSON", {
      status: response.status,
      code: undefined,
      requestId: response.headers.get("x-request-id") ?? undefined,
    });
  }
  return (await response.json()) as T;
}

async function requestJSON<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(apiURL(path), {
    credentials: "include",
    ...init,
    headers: { Accept: "application/json", ...init.headers },
  });
  if (!response.ok) {
    let payload: ErrorPayload = {};
    try {
      payload = await readJSON<ErrorPayload>(response);
    } catch (error: unknown) {
      if (error instanceof APIRequestError) throw error;
    }
    throw new APIRequestError(payload.error?.message ?? "API request failed", {
      status: response.status,
      code: payload.error?.code,
      requestId: payload.request_id ?? response.headers.get("x-request-id") ?? undefined,
      fieldErrors: payload.field_errors,
    });
  }
  return readJSON<T>(response);
}

function jsonRequest(method: string, body: unknown): RequestInit {
  return {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  };
}

export function getOperationsCatalog(signal?: AbortSignal): Promise<OperationsCatalog> {
  return requestJSON("/api/v1/operations/catalog", signal ? { signal } : {});
}

export function listOperationImports(signal?: AbortSignal): Promise<OperationImportPage> {
  return requestJSON("/api/v1/operations/imports?limit=100&offset=0", signal ? { signal } : {});
}

export function getOperationImport(id: string, signal?: AbortSignal): Promise<OperationImport> {
  return requestJSON(
    `/api/v1/operations/imports/${encodeURIComponent(id)}`,
    signal ? { signal } : {},
  );
}

export function getOperationImportReport(
  id: string,
  signal?: AbortSignal,
): Promise<OperationReport> {
  return requestJSON(
    `/api/v1/operations/imports/${encodeURIComponent(id)}/report`,
    signal ? { signal } : {},
  );
}

export async function uploadOperationImport(
  file: File,
  module: OperationModule,
  onProgress: (percent: number) => void,
): Promise<OperationImport> {
  const grant = await requestJSON<{
    import: OperationImport;
    upload_url: string;
    method: "PUT";
    headers: Record<string, string>;
    expires_at: string;
  }>(
    "/api/v1/operations/imports",
    jsonRequest("POST", {
      module,
      original_filename: file.name,
      declared_size: file.size,
      idempotency_key: crypto.randomUUID(),
    }),
  );
  await uploadSignedWorkbook(grant, file, onProgress);
  return requestJSON<OperationImport>(
    `/api/v1/operations/imports/${encodeURIComponent(grant.import.id)}/confirm`,
    { method: "POST" },
  );
}

function uploadSignedWorkbook(
  grant: { upload_url: string; method: "PUT"; headers: Record<string, string> },
  file: File,
  onProgress: (percent: number) => void,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open(grant.method, grant.upload_url, true);
    for (const [name, value] of Object.entries(grant.headers)) {
      // Browsers set Content-Length from the File body. It remains part of the
      // R2 signature, but JavaScript is intentionally forbidden from setting it.
      if (name.toLowerCase() !== "content-length") request.setRequestHeader(name, value);
    }
    request.upload.addEventListener("progress", (event) => {
      if (event.lengthComputable && event.total > 0)
        onProgress(Math.min(100, Math.round((event.loaded / event.total) * 100)));
    });
    request.addEventListener("error", () =>
      reject(new Error("Não foi possível enviar a planilha.")),
    );
    request.addEventListener("abort", () => reject(new Error("Envio cancelado.")));
    request.addEventListener("load", () => {
      if (request.status >= 200 && request.status < 300) {
        onProgress(100);
        resolve();
        return;
      }
      reject(new Error("O armazenamento privado recusou o envio da planilha."));
    });
    request.send(file);
  });
}

export function selectOperationSheet(
  value: OperationImport,
  sheetIndex: number,
): Promise<OperationImport> {
  return requestJSON(
    `/api/v1/operations/imports/${encodeURIComponent(value.id)}/sheet`,
    jsonRequest("PUT", { version: value.version, sheet_index: sheetIndex }),
  );
}

export function saveOperationMapping(
  value: OperationImport,
  mapping: OperationMapping,
): Promise<OperationImport> {
  return requestJSON(
    `/api/v1/operations/imports/${encodeURIComponent(value.id)}/mapping`,
    jsonRequest("PUT", { version: value.version, mapping }),
  );
}

export function previewOperationImport(value: OperationImport): Promise<OperationImport> {
  return requestJSON(
    `/api/v1/operations/imports/${encodeURIComponent(value.id)}/preview`,
    jsonRequest("POST", { version: value.version }),
  );
}

export function saveOperationDecisions(
  value: OperationImport,
  decisions: OperationDecision[],
): Promise<OperationImport> {
  return requestJSON(
    `/api/v1/operations/imports/${encodeURIComponent(value.id)}/decisions`,
    jsonRequest("PUT", { version: value.version, decisions }),
  );
}

export function executeOperationImport(value: OperationImport): Promise<OperationImport> {
  return requestJSON(
    `/api/v1/operations/imports/${encodeURIComponent(value.id)}/execute`,
    jsonRequest("POST", { version: value.version }),
  );
}

export function cancelOperationImport(value: OperationImport): Promise<OperationImport> {
  return requestJSON(
    `/api/v1/operations/imports/${encodeURIComponent(value.id)}/cancel`,
    jsonRequest("POST", { version: value.version }),
  );
}

export function deleteOperationImport(id: string): Promise<void> {
  return requestNoContent(`/api/v1/operations/imports/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export function listOperationExports(signal?: AbortSignal): Promise<OperationExportPage> {
  return requestJSON("/api/v1/operations/exports?limit=100&offset=0", signal ? { signal } : {});
}

export function createOperationExport(module: OperationModule): Promise<OperationExport> {
  return requestJSON(
    "/api/v1/operations/exports",
    jsonRequest("POST", { module, idempotency_key: crypto.randomUUID() }),
  );
}

export async function downloadOperationExport(value: OperationExport): Promise<void> {
  const grant = await requestJSON<{ url: string; method: "GET"; expires_at: string }>(
    `/api/v1/operations/exports/${encodeURIComponent(value.id)}/download`,
    { method: "POST" },
  );
  const anchor = document.createElement("a");
  anchor.href = grant.url;
  anchor.rel = "noreferrer";
  anchor.download = value.filename;
  anchor.click();
  anchor.remove();
}

export function bulkDeleteOperationRecords(
  module: OperationModule,
  items: Array<{ id: string; version: number }>,
  confirmation: string,
): Promise<{ module: OperationModule; deleted: number }> {
  return requestJSON(
    "/api/v1/operations/bulk-delete",
    jsonRequest("POST", { module, items, confirmation }),
  );
}

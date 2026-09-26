import type { paths } from "../../generated/google-forms-api";
import { APIRequestError, apiURL } from "./client";

export type GoogleFormsStatus =
  paths["/api/v1/google-forms/status"]["get"]["responses"][200]["content"]["application/json"];
export type GoogleFormsSource =
  paths["/api/v1/google-forms/sources/{source_id}"]["get"]["responses"][200]["content"]["application/json"];
export type GoogleFormsSourcePage =
  paths["/api/v1/google-forms/sources"]["get"]["responses"][200]["content"]["application/json"];
export type GoogleFormsSync =
  paths["/api/v1/google-forms/sources/{source_id}/syncs"]["post"]["responses"][202]["content"]["application/json"];
export type GoogleFormsSyncPage =
  paths["/api/v1/google-forms/syncs"]["get"]["responses"][200]["content"]["application/json"];
export type GoogleFormsModule = GoogleFormsSource["module"];
export type GoogleFormsMapping =
  paths["/api/v1/google-forms/sources/{source_id}/mapping"]["put"]["requestBody"]["content"]["application/json"]["mapping"];

type ErrorPayload = {
  error?: { code?: string; message?: string };
  request_id?: string;
};

async function requestJSON<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(apiURL(path), {
    credentials: "include",
    ...init,
    headers: { Accept: "application/json", ...init.headers },
  });
  const contentType = response.headers.get("content-type") ?? "";
  let payload: T | ErrorPayload | undefined;
  if (contentType.toLowerCase().includes("application/json")) {
    payload = (await response.json()) as T | ErrorPayload;
  }
  if (!response.ok) {
    const errorPayload = (payload ?? {}) as ErrorPayload;
    throw new APIRequestError(errorPayload.error?.message ?? "API request failed", {
      status: response.status,
      code: errorPayload.error?.code,
      requestId: errorPayload.request_id ?? response.headers.get("x-request-id") ?? undefined,
    });
  }
  if (payload === undefined) {
    throw new APIRequestError("API response is not JSON", {
      status: response.status,
      code: undefined,
      requestId: response.headers.get("x-request-id") ?? undefined,
    });
  }
  return payload as T;
}

function jsonRequest(method: string, body: unknown): RequestInit {
  return {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  };
}

export function getGoogleFormsStatus(signal?: AbortSignal): Promise<GoogleFormsStatus> {
  return requestJSON("/api/v1/google-forms/status", signal ? { signal } : {});
}

export async function beginGoogleFormsOAuth(returnPath = "/cadastro?mode=forms"): Promise<void> {
  const value = await requestJSON<{ authorization_url: string }>(
    "/api/v1/google-forms/oauth/start",
    jsonRequest("POST", { return_path: returnPath }),
  );
  window.location.assign(value.authorization_url);
}

export function disconnectGoogleForms(version: number): Promise<void> {
  return requestJSON("/api/v1/google-forms/connection", jsonRequest("DELETE", { version }));
}

export function listGoogleFormsSources(signal?: AbortSignal): Promise<GoogleFormsSourcePage> {
  return requestJSON("/api/v1/google-forms/sources?limit=100&offset=0", signal ? { signal } : {});
}

export function createGoogleFormsSource(
  formReference: string,
  module: GoogleFormsModule,
): Promise<GoogleFormsSource> {
  return requestJSON(
    "/api/v1/google-forms/sources",
    jsonRequest("POST", { form_reference: formReference, module }),
  );
}

export function saveGoogleFormsMapping(
  source: GoogleFormsSource,
  mapping: GoogleFormsMapping,
): Promise<GoogleFormsSource> {
  return requestJSON(
    `/api/v1/google-forms/sources/${encodeURIComponent(source.id)}/mapping`,
    jsonRequest("PUT", { version: source.version, mapping }),
  );
}

export function updateGoogleFormsSource(
  source: GoogleFormsSource,
  input: { sync_mode: "MANUAL" | "POLL"; poll_interval_seconds: number; enabled: boolean },
): Promise<GoogleFormsSource> {
  return requestJSON(
    `/api/v1/google-forms/sources/${encodeURIComponent(source.id)}`,
    jsonRequest("PATCH", { version: source.version, ...input }),
  );
}

export function refreshGoogleFormsSource(
  source: GoogleFormsSource,
): Promise<{ source: GoogleFormsSource; drifted: boolean }> {
  return requestJSON(`/api/v1/google-forms/sources/${encodeURIComponent(source.id)}/refresh`, {
    method: "POST",
  });
}

export function requestGoogleFormsSync(source: GoogleFormsSource): Promise<GoogleFormsSync> {
  return requestJSON(
    `/api/v1/google-forms/sources/${encodeURIComponent(source.id)}/syncs`,
    jsonRequest("POST", { idempotency_key: crypto.randomUUID() }),
  );
}

export function listGoogleFormsSyncs(signal?: AbortSignal): Promise<GoogleFormsSyncPage> {
  return requestJSON("/api/v1/google-forms/syncs?limit=100&offset=0", signal ? { signal } : {});
}

export function cancelGoogleFormsSync(value: GoogleFormsSync): Promise<GoogleFormsSync> {
  return requestJSON(
    `/api/v1/google-forms/syncs/${encodeURIComponent(value.id)}/cancel`,
    jsonRequest("POST", { version: value.version }),
  );
}

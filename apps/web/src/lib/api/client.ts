import type { paths } from "../../generated/api";

type LiveHealthResponse =
  paths["/health/live"]["get"]["responses"][200]["content"]["application/json"];
export type AuthSessionResponse =
  paths["/api/auth/session"]["get"]["responses"][200]["content"]["application/json"];

type ErrorPayload = {
  error?: { code?: string; message?: string };
  request_id?: string;
};

type APIRequestErrorOptions = {
  status: number;
  code: string | undefined;
  requestId: string | undefined;
};

export class APIRequestError extends Error {
  readonly status: number;
  readonly code: string | undefined;
  readonly requestId: string | undefined;

  constructor(message: string, options: APIRequestErrorOptions) {
    super(message);
    this.name = "APIRequestError";
    this.status = options.status;
    this.code = options.code;
    this.requestId = options.requestId;
  }
}

export function apiURL(path: keyof paths): string {
  const baseURL = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/$/, "");
  return `${baseURL}${path}`;
}

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

async function throwAPIError(response: Response): Promise<never> {
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
  });
}

export async function getLiveHealth(signal?: AbortSignal): Promise<LiveHealthResponse> {
  const request: RequestInit = { credentials: "include", headers: { Accept: "application/json" } };
  if (signal) request.signal = signal;
  const response = await fetch(apiURL("/health/live"), request);
  if (!response.ok) return throwAPIError(response);
  return readJSON<LiveHealthResponse>(response);
}

export async function getAuthSession(signal?: AbortSignal): Promise<AuthSessionResponse> {
  const request: RequestInit = { credentials: "include", headers: { Accept: "application/json" } };
  if (signal) request.signal = signal;
  const response = await fetch(apiURL("/api/auth/session"), request);
  if (!response.ok) return throwAPIError(response);
  return readJSON<AuthSessionResponse>(response);
}

export async function logout(): Promise<void> {
  const response = await fetch(apiURL("/api/auth/logout"), {
    method: "POST",
    credentials: "include",
    headers: { Accept: "application/json" },
  });
  if (!response.ok) await throwAPIError(response);
}

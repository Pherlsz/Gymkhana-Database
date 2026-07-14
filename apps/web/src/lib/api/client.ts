import type { paths } from "../../generated/api";

type LiveHealthResponse =
  paths["/health/live"]["get"]["responses"][200]["content"]["application/json"];

type ErrorPayload = {
  error?: {
    code?: string;
    message?: string;
  };
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

function apiURL(path: keyof paths): string {
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

export async function getLiveHealth(signal?: AbortSignal): Promise<LiveHealthResponse> {
  const request: RequestInit = { headers: { Accept: "application/json" } };
  if (signal) {
    request.signal = signal;
  }

  const response = await fetch(apiURL("/health/live"), request);

  if (!response.ok) {
    let payload: ErrorPayload = {};
    try {
      payload = await readJSON<ErrorPayload>(response);
    } catch (error: unknown) {
      if (error instanceof APIRequestError) {
        throw error;
      }
    }
    throw new APIRequestError(payload.error?.message ?? "API request failed", {
      status: response.status,
      code: payload.error?.code,
      requestId: payload.request_id ?? response.headers.get("x-request-id") ?? undefined,
    });
  }

  return readJSON<LiveHealthResponse>(response);
}

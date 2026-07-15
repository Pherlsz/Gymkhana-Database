import type { paths } from "../../generated/api";

type LiveHealthResponse =
  paths["/health/live"]["get"]["responses"][200]["content"]["application/json"];
export type AuthSessionResponse =
  paths["/api/auth/session"]["get"]["responses"][200]["content"]["application/json"];
export type AdminUsersResponse =
  paths["/api/admin/users"]["get"]["responses"][200]["content"]["application/json"];
export type AdminUser = AdminUsersResponse["users"][number];
export type UserRole = AdminUser["role"];
export type ProfilePageResponse =
  paths["/api/v1/profiles"]["get"]["responses"][200]["content"]["application/json"];
export type Profile = ProfilePageResponse["profiles"][number];
export type ProfileValuesRequest =
  paths["/api/v1/profiles"]["post"]["requestBody"]["content"]["application/json"];
export type UpdateProfileRequest =
  paths["/api/v1/profiles/{profile_id}"]["put"]["requestBody"]["content"]["application/json"];

type UpdateUserAccessRequest =
  paths["/api/admin/users/{user_id}/access"]["patch"]["requestBody"]["content"]["application/json"];

type FieldError = { field: string; code: string; message: string };
type ErrorPayload = {
  error?: { code?: string; message?: string };
  request_id?: string;
  field_errors?: FieldError[];
};

type APIRequestErrorOptions = {
  status: number;
  code: string | undefined;
  requestId: string | undefined;
  fieldErrors?: FieldError[] | undefined;
};

export class APIRequestError extends Error {
  readonly status: number;
  readonly code: string | undefined;
  readonly requestId: string | undefined;
  readonly fieldErrors: FieldError[];

  constructor(message: string, options: APIRequestErrorOptions) {
    super(message);
    this.name = "APIRequestError";
    this.status = options.status;
    this.code = options.code;
    this.requestId = options.requestId;
    this.fieldErrors = options.fieldErrors ?? [];
  }
}

export function apiURL(path: string): string {
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
    fieldErrors: payload.field_errors,
  });
}

async function requestJSON<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(apiURL(path), {
    credentials: "include",
    ...init,
    headers: { Accept: "application/json", ...init.headers },
  });
  if (!response.ok) return throwAPIError(response);
  return readJSON<T>(response);
}

export async function getLiveHealth(signal?: AbortSignal): Promise<LiveHealthResponse> {
  return requestJSON<LiveHealthResponse>("/health/live", signal ? { signal } : {});
}

export async function getAuthSession(signal?: AbortSignal): Promise<AuthSessionResponse> {
  return requestJSON<AuthSessionResponse>("/api/auth/session", signal ? { signal } : {});
}

export async function logout(): Promise<void> {
  const response = await fetch(apiURL("/api/auth/logout"), {
    method: "POST",
    credentials: "include",
    headers: { Accept: "application/json" },
  });
  if (!response.ok) await throwAPIError(response);
}

export async function listApplicationUsers(signal?: AbortSignal): Promise<AdminUsersResponse> {
  return requestJSON<AdminUsersResponse>(
    "/api/admin/users?limit=100&offset=0",
    signal ? { signal } : {},
  );
}

export async function updateApplicationUserAccess(
  userId: string,
  request: UpdateUserAccessRequest,
): Promise<AdminUser> {
  return requestJSON<AdminUser>(`/api/admin/users/${encodeURIComponent(userId)}/access`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });
}

export type ProfileListSearch = {
  page: number;
  limit: number;
  sort: "full_name" | "cpf" | "email" | "address_city" | "created_at" | "updated_at";
  order: "asc" | "desc";
  full_name: string;
  cpf: string;
  email: string;
  city: string;
  state: string;
  selected: string | undefined;
  mode: "create" | "view" | "edit" | undefined;
};

export async function listProfiles(
  search: ProfileListSearch,
  signal?: AbortSignal,
): Promise<ProfilePageResponse> {
  const query = new URLSearchParams({
    limit: String(search.limit),
    offset: String((search.page - 1) * search.limit),
    sort: search.sort,
    order: search.order,
  });
  for (const key of ["full_name", "cpf", "email", "city", "state"] as const) {
    if (search[key]) query.set(key, search[key]);
  }
  return requestJSON<ProfilePageResponse>(`/api/v1/profiles?${query}`, signal ? { signal } : {});
}

export async function createProfile(request: ProfileValuesRequest): Promise<Profile> {
  return requestJSON<Profile>("/api/v1/profiles", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });
}

export async function updateProfile(id: string, request: UpdateProfileRequest): Promise<Profile> {
  return requestJSON<Profile>(`/api/v1/profiles/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });
}

export async function duplicateProfile(id: string): Promise<Profile> {
  return requestJSON<Profile>(`/api/v1/profiles/${encodeURIComponent(id)}/duplicate`, {
    method: "POST",
  });
}

export async function deleteProfile(
  id: string,
  version: number,
  confirmation: string,
): Promise<void> {
  const response = await fetch(apiURL(`/api/v1/profiles/${encodeURIComponent(id)}`), {
    method: "DELETE",
    credentials: "include",
    headers: { Accept: "application/json", "Content-Type": "application/json" },
    body: JSON.stringify({ version, confirmation }),
  });
  if (!response.ok) await throwAPIError(response);
}

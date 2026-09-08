import type { components, paths } from "../../generated/api";
import type { paths as searchPaths } from "../../generated/search-api";

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
export type Profile = ProfilePageResponse["profiles"][number] & {
  custom_values?: Record<string, string>;
  document_identifiers?: Record<string, string>;
  document_badges?: ProfilePageResponse["profiles"][number]["document_badges"];
  document_presences?: ProfilePageResponse["profiles"][number]["document_presences"];
  cpf_digit_sum?: number | null;
};
export type ProfileDocumentBadge = NonNullable<Profile["document_badges"]>[number];
export type ProfileDocumentPresence = NonNullable<Profile["document_presences"]>[number];
export type ProfileValuesRequest =
  paths["/api/v1/profiles"]["post"]["requestBody"]["content"]["application/json"];
export type UpdateProfileRequest =
  paths["/api/v1/profiles/{profile_id}"]["put"]["requestBody"]["content"]["application/json"];
export type UpsertDocumentPresenceRequest =
  paths["/api/v1/document-presences"]["put"]["requestBody"]["content"]["application/json"];
export type DocumentPresence =
  paths["/api/v1/document-presences"]["put"]["responses"][200]["content"]["application/json"];

export type DocumentTypePageResponse =
  paths["/api/v1/document-types"]["get"]["responses"][200]["content"]["application/json"];
export type DocumentType = DocumentTypePageResponse["types"][number];
export type DocumentPageResponse =
  paths["/api/v1/documents"]["get"]["responses"][200]["content"]["application/json"];
export type DocumentRecord = DocumentPageResponse["documents"][number] & {
  custom_values?: Record<string, string>;
};
export type DocumentValuesRequest =
  paths["/api/v1/documents"]["post"]["requestBody"]["content"]["application/json"];
export type UpdateDocumentRequest =
  paths["/api/v1/documents/{document_id}"]["put"]["requestBody"]["content"]["application/json"];
export type DocumentTypeValuesRequest =
  paths["/api/v1/document-types"]["post"]["requestBody"]["content"]["application/json"];
export type UpdateDocumentTypeRequest =
  paths["/api/v1/document-types/{document_type_id}"]["put"]["requestBody"]["content"]["application/json"];

export type BillTypePageResponse =
  paths["/api/v1/bill-types"]["get"]["responses"][200]["content"]["application/json"];
export type BillType = BillTypePageResponse["types"][number];
export type BillPageResponse =
  paths["/api/v1/bills"]["get"]["responses"][200]["content"]["application/json"];
export type BillRecord = BillPageResponse["bills"][number] & {
  custom_values?: Record<string, string>;
};
export type BillValuesRequest =
  paths["/api/v1/bills"]["post"]["requestBody"]["content"]["application/json"];
export type UpdateBillRequest =
  paths["/api/v1/bills/{bill_id}"]["put"]["requestBody"]["content"]["application/json"];
export type BillTypeValuesRequest =
  paths["/api/v1/bill-types"]["post"]["requestBody"]["content"]["application/json"];
export type UpdateBillTypeRequest =
  paths["/api/v1/bill-types/{bill_type_id}"]["put"]["requestBody"]["content"]["application/json"];

export type CustomField = components["schemas"]["CustomField"];
export type CustomOption = components["schemas"]["CustomOption"];
export type CustomValueInput = components["schemas"]["CustomValueInput"];
export type CustomValueSet = components["schemas"]["CustomValueSet"];
export type CustomTargetKind = components["schemas"]["CustomTargetKind"];
export type CustomValueTargetKind = "profile" | "document" | "bill";
export type SearchCatalogResponse =
  searchPaths["/api/v1/search/catalog"]["get"]["responses"][200]["content"]["application/json"];
export type SearchRequest =
  searchPaths["/api/v1/search"]["post"]["requestBody"]["content"]["application/json"];
export type SearchPageResponse =
  searchPaths["/api/v1/search"]["post"]["responses"][200]["content"]["application/json"];
export type SearchResult = SearchPageResponse["results"][number];

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
  if (import.meta.env.DEV && (path.startsWith("/api/") || path.startsWith("/health/"))) {
    return path;
  }
  const baseURL = (
    import.meta.env.VITE_API_BASE_URL || (import.meta.env.DEV ? "http://localhost:8080" : "")
  ).replace(/\/$/, "");
  return `${baseURL}${path}`;
}

export async function readJSON<T>(response: Response): Promise<T> {
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

export async function throwAPIError(response: Response): Promise<never> {
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

export async function requestJSON<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(apiURL(path), {
    credentials: "include",
    ...init,
    headers: { Accept: "application/json", ...init.headers },
  });
  if (!response.ok) return throwAPIError(response);
  return readJSON<T>(response);
}

export async function requestNoContent(path: string, init: RequestInit): Promise<void> {
  const response = await fetch(apiURL(path), {
    credentials: "include",
    ...init,
    headers: { Accept: "application/json", ...init.headers },
  });
  if (!response.ok) await throwAPIError(response);
}

export function jsonRequest(method: string, body: unknown): RequestInit {
  return {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  };
}

export async function getLiveHealth(signal?: AbortSignal): Promise<LiveHealthResponse> {
  return requestJSON<LiveHealthResponse>("/health/live", signal ? { signal } : {});
}

export async function getAuthSession(signal?: AbortSignal): Promise<AuthSessionResponse> {
  return requestJSON<AuthSessionResponse>("/api/auth/session", signal ? { signal } : {});
}

export async function logout(): Promise<void> {
  await requestNoContent("/api/auth/logout", { method: "POST" });
}

export type ProfileListSearch = {
  page: number;
  limit: number;
  sort:
    | "full_name"
    | "cpf"
    | "email"
    | "address_city"
    | "address_street"
    | "address_neighborhood"
    | "mobile_phone"
    | "birth_date"
    | "created_at"
    | "updated_at";
  order: "asc" | "desc";
  q: string;
  full_name: string;
  cpf: string;
  email: string;
  city: string;
  state: string;
  selected: string | undefined;
  mode: "create" | "view" | "edit" | undefined;
  section: "profile" | "documents" | "bills";
  document_page: number;
  document_limit: number;
  document_sort: "identifier_value" | "type_label" | "document_date" | "created_at" | "updated_at";
  document_order: "asc" | "desc";
  document_identifier: string;
  document_status: "" | "AVAILABLE" | "IN_USE";
  document_medium: "" | "PHYSICAL" | "DIGITAL";
  document_type: string;
  document_selected: string | undefined;
  document_mode: "create" | "view" | "edit" | "types" | undefined;
  bill_page: number;
  bill_limit: number;
  bill_sort:
    | "reference_value"
    | "type_label"
    | "competence"
    | "amount"
    | "created_at"
    | "updated_at";
  bill_order: "asc" | "desc";
  bill_reference: string;
  bill_competence: string;
  bill_status: "" | "AVAILABLE" | "IN_USE";
  bill_medium: "" | "PHYSICAL" | "DIGITAL";
  bill_type: string;
  bill_selected: string | undefined;
  bill_mode: "create" | "view" | "edit" | "types" | undefined;
  records_owner: string | undefined;
  cols: string;
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
  if (search.q) query.set("q", search.q);
  for (const key of ["full_name", "cpf", "email", "city", "state"] as const) {
    if (search[key]) query.set(key, search[key]);
  }
  return requestJSON<ProfilePageResponse>(`/api/v1/profiles?${query}`, signal ? { signal } : {});
}

export async function listDistinctCities(
  filters: Pick<ProfileListSearch, "full_name" | "cpf" | "email" | "state">,
  signal?: AbortSignal,
): Promise<{ values: string[] }> {
  const query = new URLSearchParams({ limit: "500" });
  for (const key of ["full_name", "cpf", "email", "state"] as const) {
    if (filters[key]) query.set(key, filters[key]);
  }
  return requestJSON<{ values: string[] }>(
    `/api/v1/profiles/cities?${query}`,
    signal ? { signal } : {},
  );
}

export async function getProfileListTotals(signal?: AbortSignal): Promise<ProfilePageResponse> {
  return requestJSON<ProfilePageResponse>(
    "/api/v1/profiles?limit=1&offset=0&sort=updated_at&order=desc",
    signal ? { signal } : {},
  );
}

export async function getDocumentListTotals(
  status?: components["schemas"]["DocumentStatus"],
  signal?: AbortSignal,
  typeId?: string,
): Promise<DocumentPageResponse> {
  const query = new URLSearchParams({
    limit: "1",
    offset: "0",
    sort: "updated_at",
    order: "desc",
  });
  if (status) query.set("status", status);
  if (typeId) query.set("document_type_id", typeId);
  return requestJSON<DocumentPageResponse>(`/api/v1/documents?${query}`, signal ? { signal } : {});
}

export async function listDocumentsInUse(signal?: AbortSignal): Promise<DocumentPageResponse> {
  return requestJSON<DocumentPageResponse>(
    "/api/v1/documents?limit=50&offset=0&sort=updated_at&order=desc&status=IN_USE",
    signal ? { signal } : {},
  );
}

export async function getBillListTotals(
  status?: components["schemas"]["BillStatus"],
  signal?: AbortSignal,
  typeId?: string,
): Promise<BillPageResponse> {
  const query = new URLSearchParams({
    limit: "1",
    offset: "0",
    sort: "updated_at",
    order: "desc",
  });
  if (status) query.set("status", status);
  if (typeId) query.set("bill_type_id", typeId);
  return requestJSON<BillPageResponse>(`/api/v1/bills?${query}`, signal ? { signal } : {});
}

export async function listBillsInUse(signal?: AbortSignal): Promise<BillPageResponse> {
  return requestJSON<BillPageResponse>(
    "/api/v1/bills?limit=50&offset=0&sort=updated_at&order=desc&status=IN_USE",
    signal ? { signal } : {},
  );
}

export async function listProfilesLookup(
  q: string,
  signal?: AbortSignal,
): Promise<ProfilePageResponse> {
  const trimmed = q.trim();
  const query = new URLSearchParams({
    limit: "50",
    offset: "0",
    sort: "full_name",
    order: "asc",
  });
  if (trimmed) {
    const cleanDigits = trimmed.replace(/\D/g, "");
    if (cleanDigits.length >= 3 && /^[0-9.\-\s/]+$/.test(trimmed)) {
      query.set("cpf", cleanDigits);
    } else {
      query.set("full_name", trimmed);
    }
  }
  return requestJSON<ProfilePageResponse>(`/api/v1/profiles?${query}`, signal ? { signal } : {});
}

export async function createProfile(request: ProfileValuesRequest): Promise<Profile> {
  return requestJSON<Profile>("/api/v1/profiles", jsonRequest("POST", request));
}

export async function updateProfile(id: string, request: UpdateProfileRequest): Promise<Profile> {
  return requestJSON<Profile>(
    `/api/v1/profiles/${encodeURIComponent(id)}`,
    jsonRequest("PUT", request),
  );
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
  await requestNoContent(
    `/api/v1/profiles/${encodeURIComponent(id)}`,
    jsonRequest("DELETE", { version, confirmation }),
  );
}

export type DocumentListSearch = Pick<
  ProfileListSearch,
  | "q"
  | "document_page"
  | "document_limit"
  | "document_sort"
  | "document_order"
  | "document_identifier"
  | "document_status"
  | "document_medium"
  | "document_type"
>;

export async function listDocumentTypes(signal?: AbortSignal): Promise<DocumentTypePageResponse> {
  return requestJSON<DocumentTypePageResponse>(
    "/api/v1/document-types?limit=1000&offset=0&sort=label&order=asc",
    signal ? { signal } : {},
  );
}

export async function createDocumentType(
  request: DocumentTypeValuesRequest,
): Promise<DocumentType> {
  return requestJSON<DocumentType>("/api/v1/document-types", jsonRequest("POST", request));
}

export async function updateDocumentType(
  id: string,
  request: UpdateDocumentTypeRequest,
): Promise<DocumentType> {
  return requestJSON<DocumentType>(
    `/api/v1/document-types/${encodeURIComponent(id)}`,
    jsonRequest("PUT", request),
  );
}

export async function deleteDocumentType(
  id: string,
  version: number,
  confirmation: string,
): Promise<void> {
  await requestNoContent(
    `/api/v1/document-types/${encodeURIComponent(id)}`,
    jsonRequest("DELETE", { version, confirmation }),
  );
}

export async function getProfile(id: string, signal?: AbortSignal): Promise<Profile> {
  return requestJSON<Profile>(
    `/api/v1/profiles/${encodeURIComponent(id)}`,
    signal ? { signal } : {},
  );
}

export async function listDocuments(
  profileId: string | undefined,
  search: DocumentListSearch,
  signal?: AbortSignal,
): Promise<DocumentPageResponse> {
  const query = new URLSearchParams({
    limit: String(search.document_limit),
    offset: String((search.document_page - 1) * search.document_limit),
    sort: search.document_sort,
    order: search.document_order,
  });
  if (profileId) query.set("owner_profile_id", profileId);
  if (search.q) query.set("q", search.q);
  if (search.document_identifier) query.set("identifier", search.document_identifier);
  if (search.document_status) query.set("status", search.document_status);
  if (search.document_medium) query.set("medium", search.document_medium);
  if (search.document_type) query.set("document_type_id", search.document_type);
  return requestJSON<DocumentPageResponse>(`/api/v1/documents?${query}`, signal ? { signal } : {});
}

export async function upsertDocumentPresence(
  request: UpsertDocumentPresenceRequest,
): Promise<DocumentPresence> {
  return requestJSON<DocumentPresence>("/api/v1/document-presences", jsonRequest("PUT", request));
}

export async function createDocument(request: DocumentValuesRequest): Promise<DocumentRecord> {
  return requestJSON<DocumentRecord>("/api/v1/documents", jsonRequest("POST", request));
}

export async function updateDocument(
  id: string,
  request: UpdateDocumentRequest,
): Promise<DocumentRecord> {
  return requestJSON<DocumentRecord>(
    `/api/v1/documents/${encodeURIComponent(id)}`,
    jsonRequest("PUT", request),
  );
}

export async function duplicateDocument(id: string): Promise<DocumentRecord> {
  return requestJSON<DocumentRecord>(`/api/v1/documents/${encodeURIComponent(id)}/duplicate`, {
    method: "POST",
  });
}

export async function deleteDocument(
  id: string,
  version: number,
  confirmation: string,
): Promise<void> {
  await requestNoContent(
    `/api/v1/documents/${encodeURIComponent(id)}`,
    jsonRequest("DELETE", { version, confirmation }),
  );
}

export async function assignDocumentCurrentUse(
  id: string,
  holderProfileId: string,
): Promise<DocumentRecord["current_use"]> {
  return requestJSON<DocumentRecord["current_use"]>(
    `/api/v1/documents/${encodeURIComponent(id)}/current-use`,
    jsonRequest("PUT", { holder_profile_id: holderProfileId }),
  );
}

export async function returnDocumentCurrentUse(id: string): Promise<void> {
  await requestNoContent(`/api/v1/documents/${encodeURIComponent(id)}/current-use`, {
    method: "DELETE",
  });
}

export type BillListSearch = Pick<
  ProfileListSearch,
  | "q"
  | "bill_page"
  | "bill_limit"
  | "bill_sort"
  | "bill_order"
  | "bill_reference"
  | "bill_competence"
  | "bill_status"
  | "bill_medium"
  | "bill_type"
>;

export async function listBillTypes(signal?: AbortSignal): Promise<BillTypePageResponse> {
  return requestJSON<BillTypePageResponse>(
    "/api/v1/bill-types?limit=1000&offset=0&sort=label&order=asc",
    signal ? { signal } : {},
  );
}

export async function createBillType(request: BillTypeValuesRequest): Promise<BillType> {
  return requestJSON<BillType>("/api/v1/bill-types", jsonRequest("POST", request));
}

export async function updateBillType(
  id: string,
  request: UpdateBillTypeRequest,
): Promise<BillType> {
  return requestJSON<BillType>(
    `/api/v1/bill-types/${encodeURIComponent(id)}`,
    jsonRequest("PUT", request),
  );
}

export async function deleteBillType(
  id: string,
  version: number,
  confirmation: string,
): Promise<void> {
  await requestNoContent(
    `/api/v1/bill-types/${encodeURIComponent(id)}`,
    jsonRequest("DELETE", { version, confirmation }),
  );
}

export async function listBills(
  profileId: string | undefined,
  search: BillListSearch,
  signal?: AbortSignal,
): Promise<BillPageResponse> {
  const query = new URLSearchParams({
    limit: String(search.bill_limit),
    offset: String((search.bill_page - 1) * search.bill_limit),
    sort: search.bill_sort,
    order: search.bill_order,
  });
  if (profileId) query.set("owner_profile_id", profileId);
  if (search.q) query.set("q", search.q);
  if (search.bill_reference) query.set("reference", search.bill_reference);
  if (search.bill_competence) query.set("competence", search.bill_competence);
  if (search.bill_status) query.set("status", search.bill_status);
  if (search.bill_medium) query.set("medium", search.bill_medium);
  if (search.bill_type) query.set("bill_type_id", search.bill_type);
  return requestJSON<BillPageResponse>(`/api/v1/bills?${query}`, signal ? { signal } : {});
}

export async function createBill(request: BillValuesRequest): Promise<BillRecord> {
  return requestJSON<BillRecord>("/api/v1/bills", jsonRequest("POST", request));
}

export async function updateBill(id: string, request: UpdateBillRequest): Promise<BillRecord> {
  return requestJSON<BillRecord>(
    `/api/v1/bills/${encodeURIComponent(id)}`,
    jsonRequest("PUT", request),
  );
}

export async function duplicateBill(id: string): Promise<BillRecord> {
  return requestJSON<BillRecord>(`/api/v1/bills/${encodeURIComponent(id)}/duplicate`, {
    method: "POST",
  });
}

export async function deleteBill(id: string, version: number, confirmation: string): Promise<void> {
  await requestNoContent(
    `/api/v1/bills/${encodeURIComponent(id)}`,
    jsonRequest("DELETE", { version, confirmation }),
  );
}

export async function assignBillCurrentUse(
  id: string,
  holderProfileId: string,
): Promise<BillRecord["current_use"]> {
  return requestJSON<BillRecord["current_use"]>(
    `/api/v1/bills/${encodeURIComponent(id)}/current-use`,
    jsonRequest("PUT", { holder_profile_id: holderProfileId }),
  );
}

export async function returnBillCurrentUse(id: string): Promise<void> {
  await requestNoContent(`/api/v1/bills/${encodeURIComponent(id)}/current-use`, {
    method: "DELETE",
  });
}

export async function getSearchCatalog(signal?: AbortSignal): Promise<SearchCatalogResponse> {
  return requestJSON<SearchCatalogResponse>("/api/v1/search/catalog", signal ? { signal } : {});
}

export async function executeSearch(
  request: SearchRequest,
  signal?: AbortSignal,
): Promise<SearchPageResponse> {
  const body: SearchRequest = {
    q: request.q ?? "",
    limit: Math.trunc(Number(request.limit)) || 50,
    offset: Math.max(0, Math.trunc(Number(request.offset)) || 0),
    sort: request.sort === "updated_at" ? "updated_at" : "relevance",
    order: request.order === "asc" ? "asc" : "desc",
  };
  if (request.terms?.length) body.terms = request.terms;
  if (request.modules?.length) body.modules = request.modules;
  if (request.fields?.length) body.fields = request.fields;
  return requestJSON<SearchPageResponse>("/api/v1/search", {
    ...jsonRequest("POST", body),
    ...(signal ? { signal } : {}),
  });
}

export async function suggestSearchValues(
  input: { field: string; q: string; grain?: "profiles" | "documents" | "bills"; limit?: number },
  signal?: AbortSignal,
): Promise<{ suggestions: Array<{ value: string; label: string }> }> {
  const query = new URLSearchParams({
    field: input.field,
    q: input.q,
    limit: String(input.limit ?? 50),
  });
  if (input.grain) query.set("grain", input.grain);
  return requestJSON(`/api/v1/search/suggest?${query}`, signal ? { signal } : {});
}

export async function listCustomEntityTypes(
  signal?: AbortSignal,
): Promise<components["schemas"]["CustomEntityTypePageResponse"]> {
  return requestJSON(
    "/api/v1/custom-entity-types?limit=1000&offset=0&sort=label&order=asc",
    signal ? { signal } : {},
  );
}

export async function listCustomFields(
  targetKind: CustomTargetKind,
  targetId?: string,
  signal?: AbortSignal,
): Promise<components["schemas"]["CustomFieldPageResponse"]> {
  const query = new URLSearchParams({
    target_kind: targetKind,
    limit: "1000",
    offset: "0",
    sort: "label",
    order: "asc",
  });
  if (targetId) query.set("target_id", targetId);
  return requestJSON(`/api/v1/custom-fields?${query}`, signal ? { signal } : {});
}

export async function listCustomOptions(
  fieldId: string,
  signal?: AbortSignal,
): Promise<components["schemas"]["CustomOptionListResponse"]> {
  return requestJSON(
    `/api/v1/custom-fields/${encodeURIComponent(fieldId)}/options`,
    signal ? { signal } : {},
  );
}

export async function getCustomValues(
  targetKind: CustomValueTargetKind,
  targetId: string,
  signal?: AbortSignal,
): Promise<CustomValueSet> {
  return requestJSON(
    `/api/v1/custom-values/${targetKind}/${encodeURIComponent(targetId)}`,
    signal ? { signal } : {},
  );
}

export async function replaceCustomValues(
  targetKind: CustomValueTargetKind,
  targetId: string,
  version: number,
  values: CustomValueInput[],
): Promise<CustomValueSet> {
  return requestJSON(
    `/api/v1/custom-values/${targetKind}/${encodeURIComponent(targetId)}`,
    jsonRequest("PUT", { version, values }),
  );
}

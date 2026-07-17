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
export type Profile = ProfilePageResponse["profiles"][number];
export type ProfileValuesRequest =
  paths["/api/v1/profiles"]["post"]["requestBody"]["content"]["application/json"];
export type UpdateProfileRequest =
  paths["/api/v1/profiles/{profile_id}"]["put"]["requestBody"]["content"]["application/json"];

export type DocumentTypePageResponse =
  paths["/api/v1/document-types"]["get"]["responses"][200]["content"]["application/json"];
export type DocumentType = DocumentTypePageResponse["types"][number];
export type DocumentPageResponse =
  paths["/api/v1/documents"]["get"]["responses"][200]["content"]["application/json"];
export type DocumentRecord = DocumentPageResponse["documents"][number];
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
export type BillRecord = BillPageResponse["bills"][number];
export type BillValuesRequest =
  paths["/api/v1/bills"]["post"]["requestBody"]["content"]["application/json"];
export type UpdateBillRequest =
  paths["/api/v1/bills/{bill_id}"]["put"]["requestBody"]["content"]["application/json"];
export type BillTypeValuesRequest =
  paths["/api/v1/bill-types"]["post"]["requestBody"]["content"]["application/json"];
export type UpdateBillTypeRequest =
  paths["/api/v1/bill-types/{bill_type_id}"]["put"]["requestBody"]["content"]["application/json"];

export type CustomEntityType = components["schemas"]["CustomEntityType"];
export type CustomEntityTypeValuesRequest = components["schemas"]["CustomEntityTypeValuesRequest"];
export type UpdateCustomEntityTypeRequest = components["schemas"]["UpdateCustomEntityTypeRequest"];
export type CustomField = components["schemas"]["CustomField"];
export type CustomFieldValuesRequest = components["schemas"]["CustomFieldValuesRequest"];
export type UpdateCustomFieldRequest = components["schemas"]["UpdateCustomFieldRequest"];
export type CustomOption = components["schemas"]["CustomOption"];
export type CustomOptionValuesRequest = components["schemas"]["CustomOptionValuesRequest"];
export type UpdateCustomOptionRequest = components["schemas"]["UpdateCustomOptionRequest"];
export type CustomValueInput = components["schemas"]["CustomValueInput"];
export type CustomValueSet = components["schemas"]["CustomValueSet"];
export type CustomEntity = components["schemas"]["CustomEntity"];
export type CreateCustomEntityRequest = components["schemas"]["CreateCustomEntityRequest"];
export type UpdateCustomEntityRequest = components["schemas"]["UpdateCustomEntityRequest"];
export type CustomTargetKind = components["schemas"]["CustomTargetKind"];
export type CustomValueTargetKind = "profile" | "document" | "bill" | "custom_entity";
export type SearchCatalogResponse =
  searchPaths["/api/v1/search/catalog"]["get"]["responses"][200]["content"]["application/json"];
export type SearchRequest =
  searchPaths["/api/v1/search"]["post"]["requestBody"]["content"]["application/json"];
export type SearchPageResponse =
  searchPaths["/api/v1/search"]["post"]["responses"][200]["content"]["application/json"];
export type SearchResult = SearchPageResponse["results"][number];

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

async function requestNoContent(path: string, init: RequestInit): Promise<void> {
  const response = await fetch(apiURL(path), {
    credentials: "include",
    ...init,
    headers: { Accept: "application/json", ...init.headers },
  });
  if (!response.ok) await throwAPIError(response);
}

function jsonRequest(method: string, body: unknown): RequestInit {
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
  return requestJSON<AdminUser>(
    `/api/admin/users/${encodeURIComponent(userId)}/access`,
    jsonRequest("PATCH", request),
  );
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
  section: "profile" | "documents" | "bills";
  document_page: number;
  document_limit: number;
  document_sort: "identifier_value" | "type_label" | "document_date" | "created_at" | "updated_at";
  document_order: "asc" | "desc";
  document_identifier: string;
  document_status: "" | "AVAILABLE" | "IN_USE";
  document_state: "" | "CURRENT" | "REPLACED" | "EXPIRED" | "ARCHIVED";
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
  bill_state: "" | "CURRENT" | "REPLACED" | "EXPIRED" | "ARCHIVED";
  bill_type: string;
  bill_selected: string | undefined;
  bill_mode: "create" | "view" | "edit" | "types" | undefined;
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

export async function listProfilesForSelection(signal?: AbortSignal): Promise<ProfilePageResponse> {
  return requestJSON<ProfilePageResponse>(
    "/api/v1/profiles?limit=1000&offset=0&sort=full_name&order=asc",
    signal ? { signal } : {},
  );
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
  | "document_page"
  | "document_limit"
  | "document_sort"
  | "document_order"
  | "document_identifier"
  | "document_status"
  | "document_state"
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

export async function listDocuments(
  profileId: string,
  search: DocumentListSearch,
  signal?: AbortSignal,
): Promise<DocumentPageResponse> {
  const query = new URLSearchParams({
    owner_profile_id: profileId,
    limit: String(search.document_limit),
    offset: String((search.document_page - 1) * search.document_limit),
    sort: search.document_sort,
    order: search.document_order,
  });
  if (search.document_identifier) query.set("identifier", search.document_identifier);
  if (search.document_status) query.set("status", search.document_status);
  if (search.document_state) query.set("record_state", search.document_state);
  if (search.document_type) query.set("document_type_id", search.document_type);
  return requestJSON<DocumentPageResponse>(`/api/v1/documents?${query}`, signal ? { signal } : {});
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
  | "bill_page"
  | "bill_limit"
  | "bill_sort"
  | "bill_order"
  | "bill_reference"
  | "bill_competence"
  | "bill_status"
  | "bill_state"
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
  profileId: string,
  search: BillListSearch,
  signal?: AbortSignal,
): Promise<BillPageResponse> {
  const query = new URLSearchParams({
    owner_profile_id: profileId,
    limit: String(search.bill_limit),
    offset: String((search.bill_page - 1) * search.bill_limit),
    sort: search.bill_sort,
    order: search.bill_order,
  });
  if (search.bill_reference) query.set("reference", search.bill_reference);
  if (search.bill_competence) query.set("competence", search.bill_competence);
  if (search.bill_status) query.set("status", search.bill_status);
  if (search.bill_state) query.set("record_state", search.bill_state);
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
  return requestJSON<SearchPageResponse>("/api/v1/search", {
    ...jsonRequest("POST", request),
    ...(signal ? { signal } : {}),
  });
}

export async function listCustomEntityTypes(
  signal?: AbortSignal,
): Promise<components["schemas"]["CustomEntityTypePageResponse"]> {
  return requestJSON(
    "/api/v1/custom-entity-types?limit=1000&offset=0&sort=label&order=asc",
    signal ? { signal } : {},
  );
}

export async function createCustomEntityType(
  request: CustomEntityTypeValuesRequest,
): Promise<CustomEntityType> {
  return requestJSON("/api/v1/custom-entity-types", jsonRequest("POST", request));
}

export async function updateCustomEntityType(
  id: string,
  request: UpdateCustomEntityTypeRequest,
): Promise<CustomEntityType> {
  return requestJSON(
    `/api/v1/custom-entity-types/${encodeURIComponent(id)}`,
    jsonRequest("PUT", request),
  );
}

export async function deleteCustomEntityType(
  id: string,
  version: number,
  confirmation: string,
): Promise<void> {
  await requestNoContent(
    `/api/v1/custom-entity-types/${encodeURIComponent(id)}`,
    jsonRequest("DELETE", { version, confirmation }),
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

export async function createCustomField(request: CustomFieldValuesRequest): Promise<CustomField> {
  return requestJSON("/api/v1/custom-fields", jsonRequest("POST", request));
}

export async function updateCustomField(
  id: string,
  request: UpdateCustomFieldRequest,
): Promise<CustomField> {
  return requestJSON(
    `/api/v1/custom-fields/${encodeURIComponent(id)}`,
    jsonRequest("PUT", request),
  );
}

export async function deleteCustomField(
  id: string,
  version: number,
  confirmation: string,
): Promise<void> {
  await requestNoContent(
    `/api/v1/custom-fields/${encodeURIComponent(id)}`,
    jsonRequest("DELETE", { version, confirmation }),
  );
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

export async function createCustomOption(
  fieldId: string,
  request: CustomOptionValuesRequest,
): Promise<CustomOption> {
  return requestJSON(
    `/api/v1/custom-fields/${encodeURIComponent(fieldId)}/options`,
    jsonRequest("POST", request),
  );
}

export async function updateCustomOption(
  fieldId: string,
  optionId: string,
  request: UpdateCustomOptionRequest,
): Promise<CustomOption> {
  return requestJSON(
    `/api/v1/custom-fields/${encodeURIComponent(fieldId)}/options/${encodeURIComponent(optionId)}`,
    jsonRequest("PUT", request),
  );
}

export async function deleteCustomOption(
  fieldId: string,
  optionId: string,
  version: number,
  confirmation: string,
): Promise<void> {
  await requestNoContent(
    `/api/v1/custom-fields/${encodeURIComponent(fieldId)}/options/${encodeURIComponent(optionId)}`,
    jsonRequest("DELETE", { version, confirmation }),
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

export async function listCustomEntities(
  entityTypeId: string,
  ownerProfileId?: string,
  signal?: AbortSignal,
): Promise<components["schemas"]["CustomEntityPageResponse"]> {
  const query = new URLSearchParams({
    entity_type_id: entityTypeId,
    limit: "1000",
    offset: "0",
  });
  if (ownerProfileId) query.set("owner_profile_id", ownerProfileId);
  return requestJSON(`/api/v1/custom-entities?${query}`, signal ? { signal } : {});
}

export async function createCustomEntity(
  request: CreateCustomEntityRequest,
): Promise<CustomEntity> {
  return requestJSON("/api/v1/custom-entities", jsonRequest("POST", request));
}

export async function updateCustomEntity(
  id: string,
  request: UpdateCustomEntityRequest,
): Promise<CustomEntity> {
  return requestJSON(
    `/api/v1/custom-entities/${encodeURIComponent(id)}`,
    jsonRequest("PUT", request),
  );
}

export async function deleteCustomEntity(
  id: string,
  version: number,
  confirmation: string,
): Promise<void> {
  await requestNoContent(
    `/api/v1/custom-entities/${encodeURIComponent(id)}`,
    jsonRequest("DELETE", { version, confirmation }),
  );
}

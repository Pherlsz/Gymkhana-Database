import type { components, paths } from "../../generated/matching-api";
import { APIRequestError, apiURL } from "./client";

export type MatchingCatalog = components["schemas"]["MatchingCatalog"];
export type MatchingAnalysis = components["schemas"]["MatchingAnalysis"];
export type MatchingCase = components["schemas"]["MatchingCase"];
export type MatchingCasePage = components["schemas"]["MatchingCasePage"];
export type MatchingProfile = components["schemas"]["MatchingProfile"];
export type MatchingCaseState = components["schemas"]["CaseState"];
export type MatchingScoreBand = components["schemas"]["ScoreBand"];
export type MatchingCaseSort = components["schemas"]["CaseSort"];
export type MatchingSortOrder = components["schemas"]["SortOrder"];
export type MatchingFieldChoice = components["schemas"]["FieldChoice"];
export type MatchingFieldSource = components["schemas"]["FieldSource"];
export type MatchingMergePreview = components["schemas"]["MergePreview"];
export type MatchingMergeResult = components["schemas"]["MergeResult"];
export type MatchingPreviewRequest = components["schemas"]["MergePreviewRequest"];
export type MatchingMergeRequest = components["schemas"]["MergeProfilesRequest"];

type StartAnalysisRequest =
  paths["/api/v1/matching/analyses"]["post"]["requestBody"]["content"]["application/json"];
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

function postJSON(body?: unknown, signal?: AbortSignal): RequestInit {
  return {
    method: "POST",
    ...(body === undefined
      ? {}
      : { headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }),
    ...(signal ? { signal } : {}),
  };
}

export function getMatchingCatalog(signal?: AbortSignal): Promise<MatchingCatalog> {
  return requestJSON("/api/v1/matching/catalog", signal ? { signal } : {});
}

export function startMatchingAnalysis(
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<MatchingAnalysis> {
  const request: StartAnalysisRequest = { idempotency_key: idempotencyKey };
  return requestJSON("/api/v1/matching/analyses", postJSON(request, signal));
}

export function getMatchingAnalysis(
  analysisId: string,
  signal?: AbortSignal,
): Promise<MatchingAnalysis> {
  return requestJSON(
    `/api/v1/matching/analyses/${encodeURIComponent(analysisId)}`,
    signal ? { signal } : {},
  );
}

export function cancelMatchingAnalysis(
  analysisId: string,
  signal?: AbortSignal,
): Promise<MatchingAnalysis> {
  return requestJSON(
    `/api/v1/matching/analyses/${encodeURIComponent(analysisId)}/cancel`,
    postJSON(undefined, signal),
  );
}

export type MatchingCaseFilters = {
  states: MatchingCaseState[];
  bands: MatchingScoreBand[];
  sort: MatchingCaseSort;
  order: MatchingSortOrder;
  limit: number;
  offset: number;
};

export function listMatchingCases(
  filters: MatchingCaseFilters,
  signal?: AbortSignal,
): Promise<MatchingCasePage> {
  const query = new URLSearchParams({
    limit: String(filters.limit),
    offset: String(filters.offset),
    sort: filters.sort,
    order: filters.order,
  });
  if (filters.states.length > 0) query.set("state", filters.states.join(","));
  if (filters.bands.length > 0) query.set("score_band", filters.bands.join(","));
  return requestJSON(`/api/v1/matching/cases?${query}`, signal ? { signal } : {});
}

export function getMatchingCase(caseId: string, signal?: AbortSignal): Promise<MatchingCase> {
  return requestJSON(
    `/api/v1/matching/cases/${encodeURIComponent(caseId)}`,
    signal ? { signal } : {},
  );
}

export function dismissMatchingCase(
  caseId: string,
  version: number,
  signal?: AbortSignal,
): Promise<MatchingCase> {
  return requestJSON(
    `/api/v1/matching/cases/${encodeURIComponent(caseId)}/dismiss`,
    postJSON({ version }, signal),
  );
}

export function previewMatchingMerge(
  caseId: string,
  request: MatchingPreviewRequest,
  signal?: AbortSignal,
): Promise<MatchingMergePreview> {
  return requestJSON(
    `/api/v1/matching/cases/${encodeURIComponent(caseId)}/merge-preview`,
    postJSON(request, signal),
  );
}

export function mergeMatchingProfiles(
  caseId: string,
  request: MatchingMergeRequest,
  signal?: AbortSignal,
): Promise<MatchingMergeResult> {
  return requestJSON(
    `/api/v1/matching/cases/${encodeURIComponent(caseId)}/merge`,
    postJSON(request, signal),
  );
}

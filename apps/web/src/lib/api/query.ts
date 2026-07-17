import type { components, paths } from "../../generated/query-api";
import { APIRequestError, apiURL } from "./client";

export type QueryCatalog =
  paths["/api/v1/query/catalog"]["get"]["responses"][200]["content"]["application/json"];
export type QueryPlan = components["schemas"]["QueryPlan"];
export type QueryFilterNode = components["schemas"]["FilterNode"];
export type QuerySort = components["schemas"]["QuerySort"];
export type QueryField = components["schemas"]["FieldDefinition"];
export type QueryRelation = components["schemas"]["RelationDefinition"];
export type QueryOperator = components["schemas"]["QueryOperator"];
export type PlanEstimate =
  paths["/api/v1/query/validate"]["post"]["responses"][200]["content"]["application/json"];
export type QueryExecution =
  paths["/api/v1/query/executions"]["post"]["responses"][200]["content"]["application/json"];
export type QueryResultPage =
  paths["/api/v1/query/executions/{execution_id}/result"]["get"]["responses"][200]["content"]["application/json"];
export type QueryResultRow = QueryResultPage["rows"][number];
export type QueryResultCell = QueryResultRow["cells"][number];

type ErrorPayload = {
  error?: { code?: string; message?: string };
  request_id?: string;
  field_errors?: Array<{ field: string; code: string; message: string }>;
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
      fieldErrors: errorPayload.field_errors,
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

function jsonRequest(body: unknown, signal?: AbortSignal): RequestInit {
  return {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    ...(signal ? { signal } : {}),
  };
}

export function getQueryCatalog(signal?: AbortSignal): Promise<QueryCatalog> {
  return requestJSON("/api/v1/query/catalog", signal ? { signal } : {});
}

export function validateQueryPlan(plan: QueryPlan, signal?: AbortSignal): Promise<PlanEstimate> {
  return requestJSON("/api/v1/query/validate", jsonRequest(plan, signal));
}

export function executeQueryPlan(
  plan: QueryPlan,
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<QueryExecution> {
  return requestJSON(
    "/api/v1/query/executions",
    jsonRequest({ idempotency_key: idempotencyKey, plan }, signal),
  );
}

export function getQueryResult(
  executionId: string,
  limit = 100,
  offset = 0,
  signal?: AbortSignal,
): Promise<QueryResultPage> {
  const query = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  return requestJSON(
    `/api/v1/query/executions/${encodeURIComponent(executionId)}/result?${query}`,
    signal ? { signal } : {},
  );
}

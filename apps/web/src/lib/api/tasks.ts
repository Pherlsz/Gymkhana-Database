import type { components, paths } from "../../generated/tasks-api";
import { APIRequestError, apiURL } from "./client";

export type AdvancedQueryCatalog =
  paths["/query/v2/catalog"]["get"]["responses"][200]["content"]["application/json"];
export type AdvancedQueryPlan = components["schemas"]["QueryPlan"];
export type AdvancedPlanEstimate =
  paths["/query/v2/validate"]["post"]["responses"][200]["content"]["application/json"];
export type AdvancedQueryExecution =
  paths["/query/v2/executions"]["post"]["responses"][200]["content"]["application/json"];
export type AdvancedQueryResult =
  paths["/query/v2/executions/{execution_id}/result"]["get"]["responses"][200]["content"]["application/json"];
export type TaskCapability =
  paths["/tasks/capability"]["get"]["responses"][200]["content"]["application/json"];
export type TaskSpec = components["schemas"]["TaskSpec"];
export type TaskDraft = components["schemas"]["TaskDraft"];
export type TaskProposal = components["schemas"]["TaskProposal"];
export type TaskJob = components["schemas"]["TaskJob"];
export type TaskJobPage = components["schemas"]["TaskJobPage"];
export type TaskResultPage = components["schemas"]["TaskResultPage"];
export type TaskComposition = components["schemas"]["Composition"];
export type TaskEvidence = components["schemas"]["Evidence"];

export type TaskEvent = {
  sequence: number;
  kind: string;
  progress_current?: number;
  progress_total?: number;
  candidate_count?: number;
  result_count?: number;
  error_code?: string;
  created_at: string;
};

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
  if (contentType.toLowerCase().includes("json")) {
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
      requestId: response.headers.get("x-request-id") ?? undefined,
    });
  }
  return payload as T;
}

function jsonRequest(method: "POST" | "PUT", body: unknown, signal?: AbortSignal): RequestInit {
  return {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    ...(signal ? { signal } : {}),
  };
}

export function getAdvancedQueryCatalog(signal?: AbortSignal): Promise<AdvancedQueryCatalog> {
  return requestJSON("/api/v1/query/v2/catalog", signal ? { signal } : {});
}

export function validateAdvancedQueryPlan(
  plan: AdvancedQueryPlan,
  signal?: AbortSignal,
): Promise<AdvancedPlanEstimate> {
  return requestJSON("/api/v1/query/v2/validate", jsonRequest("POST", plan, signal));
}

export function executeAdvancedQueryPlan(
  plan: AdvancedQueryPlan,
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<AdvancedQueryExecution> {
  return requestJSON(
    "/api/v1/query/v2/executions",
    jsonRequest("POST", { idempotency_key: idempotencyKey, plan }, signal),
  );
}

export function getAdvancedQueryResult(
  executionId: string,
  limit = 100,
  offset = 0,
  signal?: AbortSignal,
): Promise<AdvancedQueryResult> {
  const query = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  return requestJSON(
    `/api/v1/query/v2/executions/${encodeURIComponent(executionId)}/result?${query}`,
    signal ? { signal } : {},
  );
}

export function getTaskCapability(signal?: AbortSignal): Promise<TaskCapability> {
  return requestJSON("/api/v1/tasks/capability", signal ? { signal } : {});
}

export function interpretTask(taskText: string, signal?: AbortSignal): Promise<TaskProposal> {
  return requestJSON(
    "/api/v1/tasks/interpret",
    jsonRequest("POST", { task_text: taskText }, signal),
  );
}

export function createTaskDraft(spec: TaskSpec, signal?: AbortSignal): Promise<TaskDraft> {
  return requestJSON("/api/v1/tasks/drafts", jsonRequest("POST", { spec }, signal));
}

export function getTaskDraft(draftId: string, signal?: AbortSignal): Promise<TaskDraft> {
  return requestJSON(
    `/api/v1/tasks/drafts/${encodeURIComponent(draftId)}`,
    signal ? { signal } : {},
  );
}

export function reviewTaskDraft(
  draftId: string,
  spec: TaskSpec,
  version: number,
  signal?: AbortSignal,
): Promise<TaskDraft> {
  return requestJSON(
    `/api/v1/tasks/drafts/${encodeURIComponent(draftId)}/review`,
    jsonRequest("PUT", { spec, version }, signal),
  );
}

export function listTaskJobs(
  limit = 100,
  offset = 0,
  signal?: AbortSignal,
): Promise<TaskJobPage> {
  const query = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  return requestJSON(`/api/v1/tasks/jobs?${query}`, signal ? { signal } : {});
}

export function getTaskJob(jobId: string, signal?: AbortSignal): Promise<TaskJob> {
  return requestJSON(`/api/v1/tasks/jobs/${encodeURIComponent(jobId)}`, signal ? { signal } : {});
}

export function startTaskJob(
  draftId: string,
  idempotencyKey: string,
  retryOfJobId?: string,
  signal?: AbortSignal,
): Promise<TaskJob> {
  return requestJSON(
    "/api/v1/tasks/jobs",
    jsonRequest(
      "POST",
      {
        draft_id: draftId,
        idempotency_key: idempotencyKey,
        ...(retryOfJobId ? { retry_of_job_id: retryOfJobId } : {}),
      },
      signal,
    ),
  );
}

export function cancelTaskJob(jobId: string, signal?: AbortSignal): Promise<TaskJob> {
  return requestJSON(
    `/api/v1/tasks/jobs/${encodeURIComponent(jobId)}/cancel`,
    jsonRequest("POST", {}, signal),
  );
}

export function getTaskResults(
  jobId: string,
  limit = 100,
  offset = 0,
  signal?: AbortSignal,
): Promise<TaskResultPage> {
  const query = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  return requestJSON(
    `/api/v1/tasks/jobs/${encodeURIComponent(jobId)}/results?${query}`,
    signal ? { signal } : {},
  );
}

export function subscribeTaskEvents(
  jobId: string,
  onEvent: (event: TaskEvent) => void,
  onError: () => void,
): () => void {
  const source = new EventSource(apiURL(`/api/v1/tasks/jobs/${encodeURIComponent(jobId)}/events`), {
    withCredentials: true,
  });
  const handle = (event: MessageEvent<string>) => {
    try {
      onEvent(JSON.parse(event.data) as TaskEvent);
    } catch {
      onError();
    }
  };
  for (const kind of [
    "JOB_ACCEPTED",
    "JOB_STARTED",
    "CATALOG_VALIDATED",
    "CANDIDATES_STARTED",
    "CANDIDATES_READY",
    "SOLVER_STARTED",
    "JOB_COMPLETED",
    "JOB_INCOMPLETE",
    "JOB_FAILED",
    "JOB_CANCELLED",
  ]) {
    source.addEventListener(kind, handle as EventListener);
  }
  source.onerror = onError;
  return () => source.close();
}

import type { components, paths } from "../../generated/ocr-api";
import { APIRequestError, apiURL, jsonRequest, requestJSON, throwAPIError } from "./client";

export type OCRCapability = components["schemas"]["OCRCapability"];
export type OCRJob = components["schemas"]["OCRJob"];
export type OCRJobPage = components["schemas"]["OCRJobPage"];
export type OCRJobState = components["schemas"]["OCRJobState"];
export type OCRSuggestion = components["schemas"]["OCRSuggestion"];
export type OCRSuggestionView = components["schemas"]["OCRSuggestionView"];
export type OCRSuggestionPage = components["schemas"]["OCRSuggestionPage"];
export type OCRJobEvent = components["schemas"]["OCRJobEvent"];
export type OCRJobEventKind = components["schemas"]["OCRJobEventKind"];
export type OCRApplyReceipt = components["schemas"]["OCRApplyReceipt"];

type StartRequest = paths["/api/v1/ocr/jobs"]["post"]["requestBody"]["content"]["application/json"];
type ReviewRequest =
  paths["/api/v1/ocr/suggestions/{suggestion_id}"]["patch"]["requestBody"]["content"]["application/json"];
type ApplyRequest =
  paths["/api/v1/ocr/jobs/{job_id}/apply"]["post"]["requestBody"]["content"]["application/json"];

const terminalEvents = new Set<OCRJobEventKind>(["JOB_COMPLETED", "JOB_FAILED", "JOB_CANCELLED"]);

export function getOCRCapability(signal?: AbortSignal): Promise<OCRCapability> {
  return requestJSON("/api/v1/ocr/capability", signal ? { signal } : {});
}

export function listOCRJobs(signal?: AbortSignal): Promise<OCRJobPage> {
  return requestJSON("/api/v1/ocr/jobs?limit=100&offset=0", signal ? { signal } : {});
}

export function getOCRJob(jobID: string, signal?: AbortSignal): Promise<OCRJob> {
  return requestJSON(`/api/v1/ocr/jobs/${encodeURIComponent(jobID)}`, signal ? { signal } : {});
}

export function startOCRJob(
  attachmentID: string,
  idempotencyKey: string,
  retryOfJobID?: string,
): Promise<OCRJob> {
  const request: StartRequest = {
    attachment_id: attachmentID,
    idempotency_key: idempotencyKey,
    ...(retryOfJobID ? { retry_of_job_id: retryOfJobID } : {}),
  };
  return requestJSON("/api/v1/ocr/jobs", jsonRequest("POST", request));
}

export function cancelOCRJob(jobID: string): Promise<OCRJob> {
  return requestJSON(`/api/v1/ocr/jobs/${encodeURIComponent(jobID)}/cancel`, { method: "POST" });
}

export function listOCRSuggestions(
  jobID: string,
  signal?: AbortSignal,
): Promise<OCRSuggestionPage> {
  return requestJSON(
    `/api/v1/ocr/jobs/${encodeURIComponent(jobID)}/suggestions?limit=100&offset=0`,
    signal ? { signal } : {},
  );
}

export function reviewOCRSuggestion(
  suggestion: OCRSuggestion,
  action: "ACCEPT" | "REJECT",
  value?: string,
): Promise<OCRSuggestion> {
  const request: ReviewRequest = {
    action,
    version: suggestion.version,
    ...(action === "ACCEPT" && value !== undefined ? { value } : {}),
  };
  return requestJSON(
    `/api/v1/ocr/suggestions/${encodeURIComponent(suggestion.id)}`,
    jsonRequest("PATCH", request),
  );
}

export function applyOCRSuggestions(
  jobID: string,
  suggestions: OCRSuggestion[],
  idempotencyKey: string,
): Promise<OCRApplyReceipt> {
  const request: ApplyRequest = {
    idempotency_key: idempotencyKey,
    selections: suggestions.map((suggestion) => ({
      suggestion_id: suggestion.id,
      version: suggestion.version,
    })),
  };
  return requestJSON(
    `/api/v1/ocr/jobs/${encodeURIComponent(jobID)}/apply`,
    jsonRequest("POST", request),
  );
}

export function isActiveOCRJob(job: OCRJob | null | undefined): boolean {
  return job?.state === "QUEUED" || job?.state === "RUNNING";
}

export function newOCRIdempotencyKey(prefix: "job" | "apply" = "job"): string {
  return `ocr-${prefix}-${crypto.randomUUID()}`;
}

export async function consumeOCRJobEvents(
  jobID: string,
  after: number,
  onEvent: (event: OCRJobEvent) => void,
  signal: AbortSignal,
): Promise<number> {
  let cursor = Math.max(0, Math.trunc(after));
  let reconnects = 0;
  while (!signal.aborted) {
    try {
      const result = await readEventStream(jobID, cursor, onEvent, signal);
      cursor = result.cursor;
      if (result.terminal) return cursor;
      reconnects += 1;
      if (reconnects > 3) throw unavailableStreamError();
    } catch (error: unknown) {
      if (signal.aborted) throw signal.reason;
      reconnects += 1;
      if (reconnects > 3 || error instanceof APIRequestError) throw error;
    }
    await abortableDelay(250 * reconnects, signal);
  }
  throw signal.reason;
}

async function readEventStream(
  jobID: string,
  after: number,
  onEvent: (event: OCRJobEvent) => void,
  signal: AbortSignal,
): Promise<{ cursor: number; terminal: boolean }> {
  const response = await fetch(
    apiURL(`/api/v1/ocr/jobs/${encodeURIComponent(jobID)}/events?after=${after}`),
    {
      credentials: "include",
      headers: { Accept: "text/event-stream", "Last-Event-ID": String(after) },
      signal,
    },
  );
  if (!response.ok) return throwAPIError(response);
  if (!(response.headers.get("content-type") ?? "").toLowerCase().includes("text/event-stream")) {
    throw unavailableStreamError();
  }
  if (!response.body) throw unavailableStreamError();

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let cursor = after;
  let terminal = false;
  while (!terminal) {
    const chunk = await reader.read();
    buffer += decoder.decode(chunk.value, { stream: !chunk.done });
    const frames = splitFrames(buffer);
    buffer = frames.rest;
    for (const frame of frames.complete) {
      const event = parseEventFrame(frame);
      if (!event || event.sequence <= cursor) continue;
      cursor = event.sequence;
      onEvent(event);
      terminal = terminalEvents.has(event.kind);
    }
    if (chunk.done) break;
  }
  return { cursor, terminal };
}

function splitFrames(buffer: string): { complete: string[]; rest: string } {
  const pieces = buffer.replaceAll("\r\n", "\n").split("\n\n");
  return { complete: pieces.slice(0, -1), rest: pieces.at(-1) ?? "" };
}

function parseEventFrame(frame: string): OCRJobEvent | null {
  if (!frame || frame.startsWith(":")) return null;
  const data = frame
    .split("\n")
    .filter((line) => line.startsWith("data:"))
    .map((line) => line.slice(5).trimStart())
    .join("\n");
  if (!data) return null;
  let value: unknown;
  try {
    value = JSON.parse(data);
  } catch {
    throw malformedStreamError();
  }
  if (!isOCRJobEvent(value)) throw malformedStreamError();
  return value;
}

function isOCRJobEvent(value: unknown): value is OCRJobEvent {
  if (!value || typeof value !== "object") return false;
  const event = value as Partial<OCRJobEvent>;
  return (
    Number.isSafeInteger(event.sequence) &&
    typeof event.kind === "string" &&
    [
      "JOB_ACCEPTED",
      "JOB_STARTED",
      "SOURCE_VALIDATED",
      "SUGGESTIONS_READY",
      "JOB_COMPLETED",
      "JOB_FAILED",
      "JOB_CANCELLED",
    ].includes(event.kind) &&
    typeof event.created_at === "string"
  );
}

function unavailableStreamError(): APIRequestError {
  return new APIRequestError("A transmissão OCR foi interrompida antes da conclusão.", {
    status: 503,
    code: "ocr_unavailable",
    requestId: undefined,
  });
}

function malformedStreamError(): APIRequestError {
  return new APIRequestError("A transmissão OCR retornou um evento inválido.", {
    status: 502,
    code: "ocr_malformed_provider",
    requestId: undefined,
  });
}

function abortableDelay(milliseconds: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(resolve, milliseconds);
    signal.addEventListener(
      "abort",
      () => {
        clearTimeout(timeout);
        reject(signal.reason);
      },
      { once: true },
    );
  });
}

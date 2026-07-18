import type { components, paths } from "../../generated/chat-api";
import {
  APIRequestError,
  apiURL,
  jsonRequest,
  requestJSON,
  requestNoContent,
  throwAPIError,
} from "./client";

export type ChatCapability = components["schemas"]["ChatCapability"];
export type ChatThread = components["schemas"]["ChatThread"];
export type ChatThreadPage = components["schemas"]["ChatThreadPage"];
export type ChatMessage = components["schemas"]["ChatMessage"];
export type ChatMessagePage = components["schemas"]["ChatMessagePage"];
export type ChatRun = components["schemas"]["ChatRun"];
export type ChatRunCreation = components["schemas"]["ChatRunCreation"];
export type ChatEvent = components["schemas"]["ChatEvent"];
export type ChatEventKind = components["schemas"]["ChatEventKind"];
export type ChatReferenceResult = components["schemas"]["ChatReferenceResult"];
export type ChatResultReference = components["schemas"]["ChatResultReference"];
export type SearchReferenceData = components["schemas"]["SearchReferenceData"];
export type SearchEvidence = components["schemas"]["SearchEvidence"];
export type QueryReferenceData = components["schemas"]["QueryReferenceData"];
export type QueryReferenceRow = components["schemas"]["QueryResultRow"];

type StartChatTurnRequest =
  paths["/api/v1/chat/threads/{thread_id}/turns"]["post"]["requestBody"]["content"]["application/json"];

const terminalEvents = new Set<ChatEventKind>(["RUN_COMPLETED", "RUN_FAILED", "RUN_CANCELLED"]);

export function getChatCapability(signal?: AbortSignal): Promise<ChatCapability> {
  return requestJSON("/api/v1/chat/capability", signal ? { signal } : {});
}

export function listChatThreads(signal?: AbortSignal): Promise<ChatThreadPage> {
  return requestJSON("/api/v1/chat/threads?limit=100&offset=0", signal ? { signal } : {});
}

export function createChatThread(title?: string): Promise<ChatThread> {
  return requestJSON("/api/v1/chat/threads", jsonRequest("POST", title ? { title } : {}));
}

export function renameChatThread(thread: ChatThread, title: string): Promise<ChatThread> {
  return requestJSON(
    `/api/v1/chat/threads/${encodeURIComponent(thread.id)}`,
    jsonRequest("PATCH", { title, version: thread.version }),
  );
}

export function deleteChatThread(threadID: string): Promise<void> {
  return requestNoContent(`/api/v1/chat/threads/${encodeURIComponent(threadID)}`, {
    method: "DELETE",
  });
}

export function listChatMessages(threadID: string, signal?: AbortSignal): Promise<ChatMessagePage> {
  return requestJSON(
    `/api/v1/chat/threads/${encodeURIComponent(threadID)}/messages?limit=100&offset=0`,
    signal ? { signal } : {},
  );
}

export function setChatActiveResult(
  threadID: string,
  referenceID: string | null,
): Promise<ChatThread> {
  return requestJSON(
    `/api/v1/chat/threads/${encodeURIComponent(threadID)}/active-result`,
    jsonRequest("PUT", { reference_id: referenceID }),
  );
}

export function startChatTurn(
  threadID: string,
  content: string,
  idempotencyKey: string,
  retryOfRunID?: string,
): Promise<ChatRunCreation> {
  const request: StartChatTurnRequest = {
    content,
    idempotency_key: idempotencyKey,
    ...(retryOfRunID ? { retry_of_run_id: retryOfRunID } : {}),
  };
  return requestJSON(
    `/api/v1/chat/threads/${encodeURIComponent(threadID)}/turns`,
    jsonRequest("POST", request),
  );
}

export function getChatRun(runID: string, signal?: AbortSignal): Promise<ChatRun> {
  return requestJSON(`/api/v1/chat/runs/${encodeURIComponent(runID)}`, signal ? { signal } : {});
}

export function cancelChatRun(runID: string): Promise<ChatRun> {
  return requestJSON(`/api/v1/chat/runs/${encodeURIComponent(runID)}/cancel`, {
    method: "POST",
  });
}

export function getChatResultReference(
  referenceID: string,
  signal?: AbortSignal,
): Promise<ChatReferenceResult> {
  return requestJSON(
    `/api/v1/chat/result-references/${encodeURIComponent(referenceID)}?limit=100&offset=0`,
    signal ? { signal } : {},
  );
}

export function isTerminalChatEvent(kind: ChatEventKind): boolean {
  return terminalEvents.has(kind);
}

export function isActiveChatRun(run: ChatRun | null | undefined): boolean {
  return run?.state === "QUEUED" || run?.state === "RUNNING" || run?.state === "TOOL_RUNNING";
}

export async function consumeChatRunEvents(
  runID: string,
  after: number,
  onEvent: (event: ChatEvent) => void,
  signal: AbortSignal,
): Promise<number> {
  let cursor = Math.max(0, Math.trunc(after));
  let reconnects = 0;
  while (!signal.aborted) {
    try {
      const result = await readEventStream(runID, cursor, onEvent, signal);
      cursor = result.cursor;
      if (result.terminal) return cursor;
      reconnects += 1;
      if (reconnects > 3) {
        throw new APIRequestError("A transmissão foi interrompida antes da conclusão.", {
          status: 503,
          code: "chat_unavailable",
          requestId: undefined,
        });
      }
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
  runID: string,
  after: number,
  onEvent: (event: ChatEvent) => void,
  signal: AbortSignal,
): Promise<{ cursor: number; terminal: boolean }> {
  const query = new URLSearchParams({ after: String(after) });
  const response = await fetch(
    apiURL(`/api/v1/chat/runs/${encodeURIComponent(runID)}/events?${query}`),
    {
      credentials: "include",
      headers: { Accept: "text/event-stream", "Last-Event-ID": String(after) },
      signal,
    },
  );
  if (!response.ok) return throwAPIError(response);
  if (!(response.headers.get("content-type") ?? "").toLowerCase().includes("text/event-stream")) {
    throw new APIRequestError("A API não retornou uma transmissão de eventos válida.", {
      status: response.status,
      code: "chat_unavailable",
      requestId: response.headers.get("x-request-id") ?? undefined,
    });
  }
  if (!response.body) {
    throw new APIRequestError("A transmissão de eventos não está disponível neste navegador.", {
      status: response.status,
      code: "chat_unavailable",
      requestId: response.headers.get("x-request-id") ?? undefined,
    });
  }

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
      if (isTerminalChatEvent(event.kind)) terminal = true;
    }
    if (chunk.done) break;
  }
  return { cursor, terminal };
}

function splitFrames(buffer: string): { complete: string[]; rest: string } {
  const normalized = buffer.replaceAll("\r\n", "\n");
  const pieces = normalized.split("\n\n");
  return { complete: pieces.slice(0, -1), rest: pieces.at(-1) ?? "" };
}

function parseEventFrame(frame: string): ChatEvent | null {
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
    throw new APIRequestError("A transmissão retornou um evento inválido.", {
      status: 502,
      code: "chat_malformed_provider",
      requestId: undefined,
    });
  }
  if (!isChatEvent(value)) {
    throw new APIRequestError("A transmissão retornou um evento fora do contrato.", {
      status: 502,
      code: "chat_malformed_provider",
      requestId: undefined,
    });
  }
  return value;
}

function isChatEvent(value: unknown): value is ChatEvent {
  if (!value || typeof value !== "object") return false;
  const event = value as Partial<ChatEvent>;
  return (
    Number.isSafeInteger(event.sequence) &&
    typeof event.kind === "string" &&
    [
      "RUN_ACCEPTED",
      "RUN_STARTED",
      "TEXT_DELTA",
      "TOOL_STARTED",
      "TOOL_COMPLETED",
      "RESULT_REFERENCE",
      "RUN_COMPLETED",
      "RUN_FAILED",
      "RUN_CANCELLED",
    ].includes(event.kind) &&
    typeof event.created_at === "string"
  );
}

function abortableDelay(milliseconds: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timeout = window.setTimeout(resolve, milliseconds);
    signal.addEventListener(
      "abort",
      () => {
        window.clearTimeout(timeout);
        reject(signal.reason);
      },
      { once: true },
    );
  });
}

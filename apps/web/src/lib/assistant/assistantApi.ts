import type { components } from "../../generated/chat-api";
import { apiURL, jsonRequest, requestJSON, requestNoContent } from "../api/client";

export type ChatCapability = components["schemas"]["ChatCapability"];
export type ChatThread = components["schemas"]["ChatThread"];
export type ChatThreadPage = components["schemas"]["ChatThreadPage"];
export type ChatMessage = components["schemas"]["ChatMessage"];
export type ChatMessagePage = components["schemas"]["ChatMessagePage"];
export type ChatRun = components["schemas"]["ChatRun"];
export type ChatRunCreation = components["schemas"]["ChatRunCreation"];
export type ChatEvent = components["schemas"]["ChatEvent"];
export type ChatEventKind = components["schemas"]["ChatEventKind"];

const BASE = "/api/v1/chat";

export function getChatCapability(signal?: AbortSignal): Promise<ChatCapability> {
  return requestJSON<ChatCapability>(`${BASE}/capability`, signal ? { signal } : {});
}

export function listChatThreads(signal?: AbortSignal): Promise<ChatThreadPage> {
  return requestJSON<ChatThreadPage>(`${BASE}/threads?limit=100`, signal ? { signal } : {});
}

export function createChatThread(title?: string): Promise<ChatThread> {
  return requestJSON<ChatThread>(`${BASE}/threads`, jsonRequest("POST", title ? { title } : {}));
}

export function renameChatThread(
  id: string,
  request: { title: string; version: number },
): Promise<ChatThread> {
  return requestJSON<ChatThread>(`${BASE}/threads/${id}`, jsonRequest("PATCH", request));
}

export function deleteChatThread(id: string): Promise<void> {
  return requestNoContent(`${BASE}/threads/${id}`, { method: "DELETE" });
}

export function listChatMessages(threadId: string, signal?: AbortSignal): Promise<ChatMessagePage> {
  return requestJSON<ChatMessagePage>(
    `${BASE}/threads/${threadId}/messages?limit=100`,
    signal ? { signal } : {},
  );
}

export function startChatTurn(
  threadId: string,
  request: { content: string; idempotency_key: string; retry_of_run_id?: string },
): Promise<ChatRunCreation> {
  return requestJSON<ChatRunCreation>(
    `${BASE}/threads/${threadId}/turns`,
    jsonRequest("POST", request),
  );
}

export function cancelChatRun(runId: string): Promise<ChatRun> {
  return requestJSON<ChatRun>(`${BASE}/runs/${runId}/cancel`, { method: "POST" });
}

export function getChatRun(runId: string, signal?: AbortSignal): Promise<ChatRun> {
  return requestJSON<ChatRun>(BASE + "/runs/" + runId, signal ? { signal } : {});
}

export type ChatTableRecorte = {
  table: "people" | "documents" | "bills";
  filters: Record<string, string>;
  columns: string[];
  sort?: string;
  order?: "asc" | "desc";
};

export function getChatTableRecorte(referenceId: string): Promise<ChatTableRecorte> {
  return requestJSON<ChatTableRecorte>(`${BASE}/result-references/${referenceId}/recorte`);
}

export type ChatResultGrid = {
  columns: { key: string; label: string }[];
  rows: {
    id: string;
    entity_kind: string;
    entity_id: string;
    entity_label: string;
    cells: Record<string, string>;
  }[];
  total: number;
  limit: number;
  offset: number;
  summary?: string;
  truncated?: boolean;
};

export function getChatResultPage(
  referenceId: string,
  limit: number,
  offset: number,
  signal?: AbortSignal,
): Promise<ChatResultGrid> {
  const query = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  return requestJSON<ChatResultGrid>(
    `${BASE}/result-references/${referenceId}/page?${query}`,
    signal ? { signal } : {},
  );
}

const TERMINAL_EVENTS: ReadonlySet<ChatEventKind> = new Set([
  "RUN_COMPLETED",
  "RUN_FAILED",
  "RUN_CANCELLED",
]);
const EVENT_KINDS: ChatEventKind[] = [
  "RUN_ACCEPTED",
  "RUN_STARTED",
  "TEXT_DELTA",
  "TOOL_STARTED",
  "TOOL_COMPLETED",
  "RESULT_REFERENCE",
  "RUN_COMPLETED",
  "RUN_FAILED",
  "RUN_CANCELLED",
];

/**
 * Streams persisted run events. The server replays from `after` and closes
 * after the terminal event; EventSource carries the cookie and Last-Event-ID.
 * Returns a stop function. `onEnd` fires once, on terminal event or error.
 */
export function streamChatEvents(
  runId: string,
  onEvent: (event: ChatEvent) => void,
  onEnd: (error?: Error) => void,
): () => void {
  const source = new EventSource(apiURL(`${BASE}/runs/${runId}/events`), { withCredentials: true });
  let done = false;
  let seen = 0;
  const finish = (error?: Error) => {
    if (done) return;
    done = true;
    source.close();
    onEnd(error);
  };
  for (const kind of EVENT_KINDS) {
    source.addEventListener(kind, (raw) => {
      const event = JSON.parse((raw as MessageEvent<string>).data) as ChatEvent;
      if (event.sequence <= seen) return;
      seen = event.sequence;
      onEvent(event);
      if (TERMINAL_EVENTS.has(event.kind)) finish();
    });
  }
  // ponytail: EventSource retries by itself with Last-Event-ID; the server
  // rejects nothing on replay, so we only cap total lifetime via the run timeout.
  source.onerror = () => {
    if (source.readyState === EventSource.CLOSED) finish(new Error("stream closed"));
  };
  return () => finish();
}

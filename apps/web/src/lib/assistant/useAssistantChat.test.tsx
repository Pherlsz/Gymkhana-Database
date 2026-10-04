import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { StrictMode, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  createChatThread,
  deleteChatThread,
  getChatCapability,
  getChatRun,
  listChatMessages,
  listChatThreads,
  startChatTurn,
  streamChatEvents,
} from "./assistantApi";
import { useAssistantChat } from "./useAssistantChat";

vi.mock("./assistantApi", async () => {
  const actual = await vi.importActual<typeof import("./assistantApi")>("./assistantApi");
  return {
    ...actual,
    getChatCapability: vi.fn(),
    listChatThreads: vi.fn(),
    listChatMessages: vi.fn(),
    createChatThread: vi.fn(),
    deleteChatThread: vi.fn(),
    startChatTurn: vi.fn(),
    streamChatEvents: vi.fn(),
    getChatRun: vi.fn(),
    renameChatThread: vi.fn(),
    cancelChatRun: vi.fn(),
  };
});

const thread = {
  id: "thread-1",
  title: "Existing session",
  version: 1,
  created_at: "2026-01-01T00:00:00.000Z",
  updated_at: "2026-01-01T00:00:00.000Z",
};

const emptyPage = { messages: [], total: 0, limit: 100, offset: 0 };

type StreamHandler = {
  onEvent: (event: {
    kind: string;
    sequence: number;
    created_at: string;
    text_delta?: string;
    error_code?: string;
    result_reference_id?: string;
  }) => void;
  onEnd: (error?: Error) => void;
};

const streams: StreamHandler[] = [];
let holdMessages = false;

function event(
  kind: string,
  sequence: number,
  extra: { text_delta?: string; error_code?: string } = {},
) {
  return { kind, sequence, created_at: "2026-01-01T00:00:00.000Z", ...extra };
}

function wrapperFor(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <StrictMode>
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      </StrictMode>
    );
  };
}

function assistants(client: QueryClient) {
  const page = client.getQueryData<{ messages: { role: string; content: string }[] }>([
    "assistant",
    "messages",
    "thread-1",
  ]);
  return (page?.messages ?? []).filter((message) => message.role === "ASSISTANT");
}

describe("useAssistantChat stream contract", () => {
  beforeEach(() => {
    streams.length = 0;
    holdMessages = false;
    vi.mocked(getChatCapability).mockResolvedValue({ enabled: true } as never);
    vi.mocked(listChatThreads).mockResolvedValue({
      threads: [thread],
      total: 1,
      limit: 100,
      offset: 0,
    } as never);
    vi.mocked(listChatMessages).mockImplementation((() => {
      if (holdMessages) return new Promise(() => undefined);
      return Promise.resolve(emptyPage);
    }) as never);
    vi.mocked(startChatTurn).mockResolvedValue({
      run: { id: "run-1", thread_id: thread.id, state: "RUNNING" },
      user_message: {
        id: "user-1",
        thread_id: thread.id,
        run_id: "run-1",
        sequence: 1,
        role: "USER",
        content: "hello",
        result_reference_ids: [],
        created_at: "2026-01-01T00:00:00.000Z",
      },
      created: true,
    } as never);
    vi.mocked(streamChatEvents).mockImplementation(((
      _runId: string,
      onEvent: StreamHandler["onEvent"],
      onEnd: StreamHandler["onEnd"],
    ) => {
      streams.push({ onEvent, onEnd });
      return () => undefined;
    }) as never);
    vi.mocked(getChatRun).mockReset();
    vi.mocked(createChatThread).mockReset();
    vi.mocked(deleteChatThread).mockReset();
  });

  async function startTurn() {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const hook = renderHook(() => useAssistantChat(true), { wrapper: wrapperFor(client) });
    await waitFor(() => expect(hook.result.current.thread?.id).toBe(thread.id));
    act(() => {
      hook.result.current.send("hello");
    });
    await waitFor(() => expect(streams.length).toBeGreaterThan(0));
    return { client, hook };
  }

  it("does not duplicate the assistant message when StrictMode replays setPending", async () => {
    const { client } = await startTurn();
    holdMessages = true;
    act(() => {
      const { onEvent } = streams[0]!;
      onEvent(event("TEXT_DELTA", 1, { text_delta: "I will check. " }));
      onEvent(event("TOOL_STARTED", 2));
      onEvent(event("TOOL_COMPLETED", 3));
      onEvent(event("TEXT_DELTA", 4, { text_delta: "Ana." }));
      onEvent(event("RUN_COMPLETED", 5));
    });
    const saved = assistants(client);
    expect(saved).toHaveLength(1);
    expect(saved[0]?.content).toBe("Ana.");
  });

  it("keeps partial text on RUN_CANCELLED and does not store a message", async () => {
    const { client, hook } = await startTurn();
    act(() => {
      const { onEvent } = streams[0]!;
      onEvent(event("TEXT_DELTA", 1, { text_delta: "partial answer" }));
      onEvent(event("RUN_CANCELLED", 2));
    });
    expect(hook.result.current.pending?.text).toBe("partial answer");
    expect(hook.result.current.pending?.cancelled).toBe(true);
    expect(hook.result.current.pending?.errorCode).toBeUndefined();
    expect(hook.result.current.busy).toBe(false);
    expect(assistants(client)).toHaveLength(0);
  });

  it("treats FAILED with code cancelled as a failure, not the cancel flow", async () => {
    const { hook } = await startTurn();
    act(() => {
      const { onEvent } = streams[0]!;
      onEvent(event("TEXT_DELTA", 1, { text_delta: "kept" }));
      onEvent(event("RUN_FAILED", 2, { error_code: "cancelled" }));
    });
    expect(hook.result.current.pending?.text).toBe("kept");
    expect(hook.result.current.pending?.cancelled).toBeUndefined();
    expect(hook.result.current.pending?.errorCode).toBe("cancelled");
  });

  it("does not show stream_closed when the run already completed", async () => {
    const { hook } = await startTurn();
    vi.mocked(getChatRun).mockResolvedValue({
      id: "run-1",
      state: "COMPLETED",
      error_code: undefined,
    } as never);
    vi.mocked(listChatMessages).mockResolvedValue({
      messages: [
        {
          id: "saved-1",
          thread_id: thread.id,
          role: "ASSISTANT",
          content: "saved reply",
          result_reference_ids: [],
          sequence: 2,
          created_at: "2026-01-01T00:00:00.000Z",
        },
      ],
      total: 1,
      limit: 100,
      offset: 0,
    } as never);
    act(() => {
      streams[0]!.onEvent(event("TEXT_DELTA", 1, { text_delta: "saved reply" }));
      streams[0]!.onEnd(new Error("stream closed"));
    });
    await waitFor(() => expect(hook.result.current.pending).toBeNull());
    expect(hook.result.current.pending?.errorCode).toBeUndefined();
  });

  it("deletes a thread created for a turn that fails to start", async () => {
    vi.mocked(listChatThreads).mockResolvedValue({
      threads: [],
      total: 0,
      limit: 100,
      offset: 0,
    } as never);
    vi.mocked(createChatThread).mockResolvedValue({
      id: "thread-new",
      title: "hello",
      version: 1,
      created_at: "2026-01-01T00:00:00.000Z",
      updated_at: "2026-01-01T00:00:00.000Z",
    } as never);
    vi.mocked(startChatTurn).mockRejectedValue(new Error("turn failed"));
    vi.mocked(deleteChatThread).mockResolvedValue(undefined as never);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const hook = renderHook(() => useAssistantChat(true), { wrapper: wrapperFor(client) });
    await waitFor(() => expect(hook.result.current.chatReady).toBe(true));
    await waitFor(() => expect(hook.result.current.threadsLoading).toBe(false));
    act(() => {
      hook.result.current.send("hello");
    });
    await waitFor(() => expect(deleteChatThread).toHaveBeenCalledWith("thread-new"));
    await waitFor(() => expect(hook.result.current.threads).toEqual([]));
    expect(hook.result.current.thread).toBeNull();
  });
});

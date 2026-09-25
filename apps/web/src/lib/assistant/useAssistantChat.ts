import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { APIRequestError } from "../api/client";
import {
  cancelChatRun,
  createChatThread,
  deleteChatThread,
  getChatCapability,
  listChatMessages,
  listChatThreads,
  renameChatThread,
  startChatTurn,
  streamChatEvents,
  type ChatMessage,
  type ChatMessagePage,
  type ChatThread,
  type ChatThreadPage,
} from "./assistantApi";
import { DEFAULT_THREAD_TITLE, threadTitle } from "./assistantTitle";

const THREADS_KEY = ["assistant", "threads"] as const;
const CAPABILITY_KEY = ["assistant", "capability"] as const;
const messagesKey = (threadId: string) => ["assistant", "messages", threadId] as const;

export type PendingRun = {
  runId: string;
  threadId: string;
  content: string;
  text: string;
  toolRunning: boolean;
  /** Stable public error code after a terminal failure; undefined while running. */
  errorCode?: string;
};

// One float, one active run: the engine already refuses a second run per thread.
export function useAssistantChat(enabled: boolean) {
  const queryClient = useQueryClient();
  const [threadId, setThreadId] = useState<string | null>(null);
  const [pending, setPending] = useState<PendingRun | null>(null);
  const stopStream = useRef<(() => void) | null>(null);

  const capability = useQuery({
    queryKey: CAPABILITY_KEY,
    queryFn: ({ signal }) => getChatCapability(signal),
    enabled,
  });
  const threads = useQuery({
    queryKey: THREADS_KEY,
    queryFn: ({ signal }) => listChatThreads(signal),
    enabled,
  });
  const threadList = threads.data?.threads ?? [];
  const thread = threadList.find((item) => item.id === threadId) ?? threadList[0] ?? null;
  const activeThreadId = thread?.id ?? null;

  const messages = useQuery({
    queryKey: messagesKey(activeThreadId ?? ""),
    queryFn: ({ signal }) => listChatMessages(activeThreadId ?? "", signal),
    enabled: enabled && activeThreadId !== null,
  });

  useEffect(() => () => stopStream.current?.(), []);

  const upsertThread = (next: ChatThread) =>
    queryClient.setQueryData<ChatThreadPage>(THREADS_KEY, (page) => {
      const rest = (page?.threads ?? []).filter((item) => item.id !== next.id);
      return {
        threads: [next, ...rest],
        total: rest.length + 1,
        limit: page?.limit ?? 100,
        offset: 0,
      };
    });

  const appendMessage = (id: string, message: ChatMessage) =>
    queryClient.setQueryData<ChatMessagePage>(messagesKey(id), (page) => ({
      messages: [...(page?.messages ?? []), message],
      total: (page?.total ?? 0) + 1,
      limit: page?.limit ?? 100,
      offset: page?.offset ?? 0,
    }));

  const create = useMutation({
    mutationFn: (title?: string) => createChatThread(title),
    onSuccess: (created) => {
      upsertThread(created);
      setThreadId(created.id);
    },
  });

  const rename = useMutation({
    mutationFn: ({ id, title, version }: { id: string; title: string; version: number }) =>
      renameChatThread(id, { title, version }),
    onSuccess: upsertThread,
    onError: () => void queryClient.invalidateQueries({ queryKey: THREADS_KEY }),
  });

  const remove = useMutation({
    mutationFn: (id: string) => deleteChatThread(id),
    onSuccess: (_, id) => {
      queryClient.setQueryData<ChatThreadPage>(THREADS_KEY, (page) => {
        const rest = (page?.threads ?? []).filter((item) => item.id !== id);
        return { threads: rest, total: rest.length, limit: page?.limit ?? 100, offset: 0 };
      });
      queryClient.removeQueries({ queryKey: messagesKey(id) });
      if (threadId === id) setThreadId(null);
      if (pending?.threadId === id) {
        stopStream.current?.();
        setPending(null);
      }
    },
  });

  const follow = useCallback(
    (runId: string, id: string, content: string) => {
      stopStream.current?.();
      setPending({ runId, threadId: id, content, text: "", toolRunning: false });
      stopStream.current = streamChatEvents(
        runId,
        (event) => {
          setPending((current) => {
            if (!current || current.runId !== runId) return current;
            switch (event.kind) {
              case "TEXT_DELTA":
                return { ...current, text: current.text + (event.text_delta ?? "") };
              case "TOOL_STARTED":
                return { ...current, toolRunning: true };
              case "TOOL_COMPLETED":
                return { ...current, toolRunning: false };
              case "RUN_FAILED":
              case "RUN_CANCELLED":
                return {
                  ...current,
                  toolRunning: false,
                  errorCode: event.error_code || event.kind.toLowerCase(),
                };
              default:
                return current;
            }
          });
          if (event.kind === "RUN_COMPLETED") {
            void queryClient.invalidateQueries({ queryKey: messagesKey(id) });
            void queryClient.invalidateQueries({ queryKey: THREADS_KEY });
            setPending(null);
          }
        },
        (error) => {
          if (error) {
            setPending((current) =>
              current && current.runId === runId && !current.errorCode
                ? { ...current, errorCode: "stream_closed" }
                : current,
            );
          }
        },
      );
    },
    [queryClient],
  );

  const send = useMutation({
    mutationFn: async ({ content, retryOfRunId }: { content: string; retryOfRunId?: string }) => {
      let id = activeThreadId;
      const untitled = Boolean(thread && thread.title === DEFAULT_THREAD_TITLE);
      const title = threadTitle(content);
      if (!id) {
        const created = await createChatThread(title);
        upsertThread(created);
        setThreadId(created.id);
        id = created.id;
      }
      const creation = await startChatTurn(id, {
        content,
        idempotency_key: crypto.randomUUID(),
        ...(retryOfRunId ? { retry_of_run_id: retryOfRunId } : {}),
      });
      return { id, creation, content, untitled, title, version: thread?.version };
    },
    onSuccess: ({ id, creation, content, untitled, title, version }) => {
      appendMessage(id, creation.user_message);
      follow(creation.run.id, id, content);
      if (untitled && title && version && title !== DEFAULT_THREAD_TITLE) {
        rename.mutate({ id, title, version });
      }
    },
  });

  const cancel = useMutation({
    mutationFn: (runId: string) => cancelChatRun(runId),
  });

  const retry = useCallback(() => {
    if (!pending?.errorCode) return;
    const failed = pending;
    setPending(null);
    send.mutate({ content: failed.content, retryOfRunId: failed.runId });
  }, [pending, send]);

  const dismissError = useCallback(() => {
    setPending((current) => (current?.errorCode ? null : current));
  }, []);

  return {
    capability: capability.data ?? null,
    capabilityLoading: capability.isLoading,
    threads: threadList,
    threadsLoading: threads.isLoading,
    thread,
    selectThread: (id: string) => {
      setThreadId(id);
    },
    messages: activeThreadId ? (messages.data?.messages ?? []) : [],
    messagesLoading: messages.isLoading,
    pending: pending && pending.threadId === activeThreadId ? pending : null,
    busy: pending !== null && !pending.errorCode,
    createThread: () => create.mutateAsync(undefined),
    renameThread: (id: string, title: string) => {
      const target = threadList.find((item) => item.id === id);
      if (!target || !title.trim() || target.title === title.trim()) return;
      rename.mutate({ id, title: title.trim(), version: target.version });
    },
    deleteThread: (id: string) => remove.mutate(id),
    send: (content: string) => send.mutate({ content }),
    sendError: errorCode(send.error),
    sessionError: errorCode(create.error) ?? errorCode(rename.error) ?? errorCode(remove.error),
    cancel: () => {
      if (pending && !pending.errorCode) cancel.mutate(pending.runId);
    },
    retry,
    dismissError,
  };
}

export function errorCode(error: unknown): string | null {
  if (!error) return null;
  if (error instanceof APIRequestError) return error.code ?? `http_${error.status}`;
  return "unknown";
}

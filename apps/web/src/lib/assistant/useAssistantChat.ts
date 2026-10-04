import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { APIRequestError } from "../api/client";
import {
  cancelChatRun,
  createChatThread,
  deleteChatThread,
  getChatCapability,
  getChatRun,
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

export const ASSISTANT_THREADS_KEY = ["assistant", "threads"] as const;
export const ASSISTANT_CAPABILITY_KEY = ["assistant", "capability"] as const;
/** Capability rarely changes; keep it warm so opening the float is instant. */
export const ASSISTANT_CAPABILITY_STALE_MS = 10 * 60 * 1000;
/** Thread list is patched locally; only refetch when stale or after errors. */
const ASSISTANT_THREADS_STALE_MS = 60 * 1000;

const messagesKey = (threadId: string) => ["assistant", "messages", threadId] as const;

export type PendingRun = {
  runId: string;
  threadId: string;
  content: string;
  text: string;
  /** Text since the last tool boundary. RUN_COMPLETED persists only this round. */
  roundText: string;
  toolRunning: boolean;
  resultReferenceIds: string[];
  /** Stable public error code after a terminal failure; undefined while running. */
  errorCode?: string;
  /** User cancellation. Partial text stays; this is not a failure and has no retry. */
  cancelled?: boolean;
};

// One float, one active run: the engine already refuses a second run per thread.
export function useAssistantChat(open: boolean) {
  const queryClient = useQueryClient();
  const [threadId, setThreadId] = useState<string | null>(null);
  const [pending, setPending] = useState<PendingRun | null>(null);
  const stopStream = useRef<(() => void) | null>(null);
  const pendingRef = useRef<PendingRun | null>(null);
  const commitPending = (next: PendingRun | null) => {
    pendingRef.current = next;
    setPending(next);
  };

  const capability = useQuery({
    queryKey: ASSISTANT_CAPABILITY_KEY,
    queryFn: ({ signal }) => getChatCapability(signal),
    enabled: open,
    staleTime: ASSISTANT_CAPABILITY_STALE_MS,
    retry: 1,
  });
  const chatReady = capability.data?.enabled === true;
  const threads = useQuery({
    queryKey: ASSISTANT_THREADS_KEY,
    queryFn: ({ signal }) => listChatThreads(signal),
    enabled: open && chatReady,
    staleTime: ASSISTANT_THREADS_STALE_MS,
  });
  const threadList = threads.data?.threads ?? [];
  const thread = threadList.find((item) => item.id === threadId) ?? threadList[0] ?? null;
  const activeThreadId = thread?.id ?? null;

  const messages = useQuery({
    queryKey: messagesKey(activeThreadId ?? ""),
    queryFn: ({ signal }) => listChatMessages(activeThreadId ?? "", signal),
    enabled: open && chatReady && activeThreadId !== null,
    staleTime: 15_000,
  });

  useEffect(() => () => stopStream.current?.(), []);

  const upsertThread = (next: ChatThread) =>
    queryClient.setQueryData<ChatThreadPage>(ASSISTANT_THREADS_KEY, (page) => {
      const rest = (page?.threads ?? []).filter((item) => item.id !== next.id);
      return {
        threads: [next, ...rest],
        total: rest.length + 1,
        limit: page?.limit ?? 100,
        offset: 0,
      };
    });

  const touchThread = (id: string, activeResultReferenceId?: string) =>
    queryClient.setQueryData<ChatThreadPage>(ASSISTANT_THREADS_KEY, (page) => {
      if (!page) return page;
      const existing = page.threads.find((item) => item.id === id);
      if (!existing) return page;
      const next: ChatThread = {
        ...existing,
        updated_at: new Date().toISOString(),
        ...(activeResultReferenceId ? { active_result_reference_id: activeResultReferenceId } : {}),
      };
      return {
        ...page,
        threads: [next, ...page.threads.filter((item) => item.id !== id)],
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
    onError: () => void queryClient.invalidateQueries({ queryKey: ASSISTANT_THREADS_KEY }),
  });

  const remove = useMutation({
    mutationFn: (id: string) => deleteChatThread(id),
    onSuccess: (_, id) => {
      queryClient.setQueryData<ChatThreadPage>(ASSISTANT_THREADS_KEY, (page) => {
        const rest = (page?.threads ?? []).filter((item) => item.id !== id);
        return { threads: rest, total: rest.length, limit: page?.limit ?? 100, offset: 0 };
      });
      queryClient.removeQueries({ queryKey: messagesKey(id) });
      if (threadId === id) setThreadId(null);
      if (pendingRef.current?.threadId === id) {
        stopStream.current?.();
        commitPending(null);
      }
    },
  });

  const follow = useCallback(
    (runId: string, id: string, content: string) => {
      stopStream.current?.();
      commitPending({
        runId,
        threadId: id,
        content,
        text: "",
        roundText: "",
        toolRunning: false,
        resultReferenceIds: [],
      });
      const patchPending = (patch: (current: PendingRun) => PendingRun) => {
        const current = pendingRef.current;
        if (!current || current.runId !== runId) return;
        const next = patch(current);
        pendingRef.current = next;
        setPending(next);
      };
      const finishWithSavedReply = async () => {
        try {
          await queryClient.fetchQuery({
            queryKey: messagesKey(id),
            queryFn: ({ signal }) => listChatMessages(id, signal),
          });
        } catch {
          void queryClient.invalidateQueries({ queryKey: messagesKey(id) });
        }
        if (pendingRef.current?.runId === runId) commitPending(null);
      };
      stopStream.current = streamChatEvents(
        runId,
        (event) => {
          if (event.kind === "RESULT_REFERENCE" && event.result_reference_id) {
            const referenceId = event.result_reference_id;
            patchPending((current) =>
              current.resultReferenceIds.includes(referenceId)
                ? current
                : { ...current, resultReferenceIds: [...current.resultReferenceIds, referenceId] },
            );
            return;
          }
          if (event.kind === "RUN_COMPLETED") {
            const current = pendingRef.current;
            if (!current || current.runId !== runId) return;
            const resultIds = current.resultReferenceIds;
            const persisted = current.roundText.trim();
            if (persisted) {
              appendMessage(id, {
                id: "local-" + runId,
                thread_id: id,
                run_id: runId,
                sequence: Date.now(),
                role: "ASSISTANT",
                content: persisted,
                result_reference_ids: resultIds,
                created_at: new Date().toISOString(),
              });
            }
            touchThread(id, resultIds[resultIds.length - 1]);
            commitPending(null);
            void finishWithSavedReply();
            return;
          }
          if (event.kind === "RUN_CANCELLED") {
            patchPending((current) => ({ ...current, toolRunning: false, cancelled: true }));
            return;
          }
          if (event.kind === "RUN_FAILED") {
            patchPending((current) => ({
              ...current,
              toolRunning: false,
              errorCode: event.error_code || "run_failed",
            }));
            return;
          }
          patchPending((current) => {
            switch (event.kind) {
              case "TEXT_DELTA": {
                const delta = event.text_delta ?? "";
                return {
                  ...current,
                  text: current.text + delta,
                  roundText: current.roundText + delta,
                };
              }
              case "TOOL_STARTED":
                return { ...current, toolRunning: true, roundText: "" };
              case "TOOL_COMPLETED":
                return { ...current, toolRunning: false };
              default:
                return current;
            }
          });
        },
        (error) => {
          if (!error) return;
          void (async () => {
            const current = pendingRef.current;
            if (!current || current.runId !== runId || current.errorCode || current.cancelled) return;
            let run: Awaited<ReturnType<typeof getChatRun>> | null = null;
            try {
              run = await getChatRun(runId);
            } catch {
              run = null;
            }
            const latest = pendingRef.current;
            if (!latest || latest.runId !== runId || latest.errorCode || latest.cancelled) return;
            if (run?.state === "COMPLETED") {
              touchThread(id, current.resultReferenceIds[current.resultReferenceIds.length - 1]);
              await finishWithSavedReply();
              return;
            }
            if (run?.state === "CANCELLED") {
              patchPending((value) => ({ ...value, toolRunning: false, cancelled: true }));
              return;
            }
            if (run?.state === "FAILED") {
              const code = run.error_code || "run_failed";
              patchPending((value) => ({ ...value, toolRunning: false, errorCode: code }));
              return;
            }
            patchPending((value) => ({ ...value, errorCode: "stream_closed" }));
          })();
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
      let createdThreadId: string | null = null;
      const dropCreatedThread = (threadToDrop: string) => {
        queryClient.setQueryData<ChatThreadPage>(ASSISTANT_THREADS_KEY, (page) => {
          const rest = (page?.threads ?? []).filter((item) => item.id !== threadToDrop);
          return { threads: rest, total: rest.length, limit: page?.limit ?? 100, offset: 0 };
        });
        queryClient.removeQueries({ queryKey: messagesKey(threadToDrop) });
        setThreadId((current) => (current === threadToDrop ? null : current));
      };
      try {
        if (!id) {
          const created = await createChatThread(title);
          createdThreadId = created.id;
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
      } catch (error) {
        if (createdThreadId) {
          try {
            await deleteChatThread(createdThreadId);
          } catch {
            // Still drop the local session so a failed first turn does not leave an empty thread open.
          }
          dropCreatedThread(createdThreadId);
        }
        throw error;
      }
    },
    onSuccess: ({ id, creation, content, untitled, title, version }) => {
      appendMessage(id, creation.user_message);
      touchThread(id);
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
    commitPending(null);
    send.mutate({ content: failed.content, retryOfRunId: failed.runId });
  }, [pending, send]);

  const dismissError = useCallback(() => {
    setPending((current) => {
      if (!current?.errorCode) return current;
      pendingRef.current = null;
      return null;
    });
  }, []);

  return {
    capability: capability.data ?? null,
    capabilityLoading: capability.isPending && !capability.data,
    capabilityFailed: capability.isError,
    chatReady,
    threads: threadList,
    thread,
    selectThread: (id: string) => {
      setThreadId(id);
    },
    messages: activeThreadId ? (messages.data?.messages ?? []) : [],
    messagesLoading:
      Boolean(activeThreadId) && (messages.isPending || messages.isFetching) && !messages.data,
    messagesRefreshing: Boolean(activeThreadId) && messages.isFetching && Boolean(messages.data),
    threadsLoading: threads.isPending || (threads.isFetching && threadList.length === 0),
    pending: pending && pending.threadId === activeThreadId ? pending : null,
    // One active turn per float: block compose while startTurn is in flight or a run is streaming.
    busy: send.isPending || (pending !== null && !pending.errorCode && !pending.cancelled),
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
      if (pending && !pending.errorCode && !pending.cancelled) cancel.mutate(pending.runId);
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

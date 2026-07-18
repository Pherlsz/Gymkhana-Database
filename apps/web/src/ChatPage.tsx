import { Alert, Button, Inline, Page, Stack, StatusBadge, Surface } from "@pherlsz/gymkhana-ui";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { chatRoute } from "./App";
import { DataGrid } from "./DataGrid";
import {
  cancelChatRun,
  consumeChatRunEvents,
  createChatThread,
  deleteChatThread,
  getChatCapability,
  getChatResultReference,
  getChatRun,
  isActiveChatRun,
  listChatMessages,
  listChatThreads,
  renameChatThread,
  setChatActiveResult,
  startChatTurn,
  type ChatEvent,
  type ChatMessage,
  type ChatReferenceResult,
  type ChatRun,
  type ChatThread,
  type QueryReferenceData,
  type QueryReferenceRow,
  type SearchEvidence,
  type SearchReferenceData,
} from "./lib/api/chat";
import { APIRequestError } from "./lib/api/client";

export type ChatSearch = { thread: string | undefined };

export function normalizeChatSearch(search: Record<string, unknown>): ChatSearch {
  return { thread: typeof search.thread === "string" && search.thread ? search.thread : undefined };
}

export function ChatPage() {
  const search = chatRoute.useSearch();
  const navigate = chatRoute.useNavigate();
  const queryClient = useQueryClient();
  const capability = useQuery({
    queryKey: ["chat", "capability"],
    queryFn: ({ signal }) => getChatCapability(signal),
    staleTime: 60_000,
  });
  const threads = useQuery({
    queryKey: ["chat", "threads"],
    queryFn: ({ signal }) => listChatThreads(signal),
    enabled: capability.data?.enabled === true,
  });
  const selectedThread = threads.data?.threads.find((thread) => thread.id === search.thread);
  const selectThread = useCallback(
    (thread: string | undefined) => {
      void navigate({ replace: true, search: (current) => ({ ...current, thread }) });
    },
    [navigate],
  );

  useEffect(() => {
    if (!threads.data || search.thread || threads.data.threads.length === 0) return;
    selectThread(threads.data.threads[0]?.id);
  }, [search.thread, selectThread, threads.data]);

  const refreshThreads = useCallback(
    async () => queryClient.invalidateQueries({ queryKey: ["chat", "threads"] }),
    [queryClient],
  );
  const createThread = useMutation({
    mutationFn: () => createChatThread(),
    onSuccess: async (thread) => {
      await refreshThreads();
      selectThread(thread.id);
    },
  });

  if (capability.isPending) {
    return (
      <ChatPageState title="Verificando o Chat">Validando a configuração segura.</ChatPageState>
    );
  }
  if (capability.isError) {
    return (
      <ChatPageState title="Chat indisponível" tone="danger">
        {chatErrorMessage(capability.error)}
      </ChatPageState>
    );
  }
  if (!capability.data.enabled) {
    return (
      <ChatPageState title="Chat ainda não ativado" tone="warning">
        O recurso permanece bloqueado até que provedor, modelo e retenção sejam definidos
        explicitamente. Nenhum dado é enviado a um modelo enquanto estiver desativado.
      </ChatPageState>
    );
  }

  return (
    <Page.Root maxWidth="full">
      <Page.Header>
        <Page.Eyebrow>Assistente privado · somente leitura</Page.Eyebrow>
        <Page.Title>Chat com os dados autorizados</Page.Title>
        <Page.Description>
          Consulte Busca e Consultas tipadas sem SQL, mutações ou acesso fora das suas permissões.
        </Page.Description>
        <Page.Actions>
          <Button disabled={createThread.isPending} onClick={() => createThread.mutate()}>
            {createThread.isPending ? "Criando" : "Nova conversa"}
          </Button>
        </Page.Actions>
      </Page.Header>
      <Page.Content>
        {createThread.isError ? (
          <Alert title="Não foi possível criar a conversa" tone="danger">
            {chatErrorMessage(createThread.error)}
          </Alert>
        ) : null}
        {threads.isError ? (
          <Alert title="Não foi possível listar as conversas" tone="danger">
            {chatErrorMessage(threads.error)}
          </Alert>
        ) : null}
        <div className="chat-workspace">
          <ThreadSidebar
            pending={threads.isPending}
            selectedID={selectedThread?.id}
            threads={threads.data?.threads ?? []}
            onSelect={selectThread}
          />
          {selectedThread ? (
            <Conversation
              capability={capability.data}
              key={selectedThread.id}
              thread={selectedThread}
              onDeleted={async () => {
                selectThread(undefined);
                await refreshThreads();
              }}
              onThreadChanged={refreshThreads}
            />
          ) : (
            <Surface className="chat-empty" tone="raised">
              <Stack gap="3">
                <h2>Comece uma conversa</h2>
                <p>
                  Crie uma thread privada. O conteúdo é separado por usuário e removido conforme a
                  retenção configurada.
                </p>
                <Inline>
                  <Button disabled={createThread.isPending} onClick={() => createThread.mutate()}>
                    Nova conversa
                  </Button>
                </Inline>
              </Stack>
            </Surface>
          )}
        </div>
      </Page.Content>
    </Page.Root>
  );
}

function ChatPageState({
  title,
  tone = "info",
  children,
}: {
  title: string;
  tone?: "info" | "warning" | "danger";
  children: ReactNode;
}) {
  return (
    <Page.Root maxWidth="lg">
      <Page.Header>
        <Page.Eyebrow>Assistente privado</Page.Eyebrow>
        <Page.Title>Chat com os dados autorizados</Page.Title>
      </Page.Header>
      <Page.Content>
        <Alert title={title} tone={tone}>
          {children}
        </Alert>
      </Page.Content>
    </Page.Root>
  );
}

function ThreadSidebar({
  pending,
  selectedID,
  threads,
  onSelect,
}: {
  pending: boolean;
  selectedID: string | undefined;
  threads: ChatThread[];
  onSelect: (id: string) => void;
}) {
  return (
    <Surface className="chat-threads" tone="raised">
      <h2>Conversas</h2>
      {pending ? <p role="status">Carregando conversas…</p> : null}
      {!pending && threads.length === 0 ? <p>Nenhuma conversa privada.</p> : null}
      <nav aria-label="Conversas privadas">
        {threads.map((thread) => (
          <button
            aria-current={thread.id === selectedID ? "page" : undefined}
            className="chat-thread-link"
            key={thread.id}
            type="button"
            onClick={() => onSelect(thread.id)}
          >
            <strong>{thread.title}</strong>
            <span>{formatDateTime(thread.updated_at)}</span>
          </button>
        ))}
      </nav>
    </Surface>
  );
}

function Conversation({
  capability,
  thread,
  onDeleted,
  onThreadChanged,
}: {
  capability: { maximum_message_runes: number };
  thread: ChatThread;
  onDeleted: () => Promise<void>;
  onThreadChanged: () => Promise<unknown>;
}) {
  const queryClient = useQueryClient();
  const [title, setTitle] = useState(thread.title);
  const [composer, setComposer] = useState("");
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [currentRun, setCurrentRun] = useState<ChatRun | null>(null);
  const [liveText, setLiveText] = useState("");
  const [streamError, setStreamError] = useState<Error | null>(null);
  const [activity, setActivity] = useState("Pronto para uma pergunta.");
  const eventCursor = useRef(0);
  const [referenceIDs, setReferenceIDs] = useState<string[]>([]);
  const hydratedRun = useRef<string | null>(null);

  const messages = useQuery({
    queryKey: ["chat", "messages", thread.id],
    queryFn: ({ signal }) => listChatMessages(thread.id, signal),
  });
  const activeReference = useQuery({
    queryKey: ["chat", "reference", thread.active_result_reference_id],
    queryFn: ({ signal }) => getChatResultReference(thread.active_result_reference_id!, signal),
    enabled: Boolean(thread.active_result_reference_id),
    retry: false,
  });
  const streamedReferences = useQueries({
    queries: referenceIDs
      .filter((id) => id !== thread.active_result_reference_id)
      .map((id) => ({
        queryKey: ["chat", "reference", id],
        queryFn: ({ signal }: { signal: AbortSignal }) => getChatResultReference(id, signal),
        staleTime: 30_000,
        retry: false,
      })),
  });

  const refreshConversation = useCallback(async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["chat", "messages", thread.id] }),
      queryClient.invalidateQueries({ queryKey: ["chat", "threads"] }),
    ]);
  }, [queryClient, thread.id]);

  const rename = useMutation({
    mutationFn: () => renameChatThread(thread, title.trim()),
    onSuccess: async (updated) => {
      setTitle(updated.title);
      await onThreadChanged();
    },
  });
  const remove = useMutation({
    mutationFn: () => deleteChatThread(thread.id),
    onSuccess: onDeleted,
  });
  const activeResult = useMutation({
    mutationFn: (referenceID: string | null) => setChatActiveResult(thread.id, referenceID),
    onSuccess: async () => onThreadChanged(),
  });
  const turn = useMutation({
    mutationFn: ({ content, retryOf }: { content: string; retryOf?: string }) =>
      startChatTurn(thread.id, content, newIdempotencyKey(), retryOf),
    onSuccess: (creation) => {
      setComposer("");
      setLiveText("");
      setStreamError(null);
      eventCursor.current = 0;
      setReferenceIDs([]);
      setCurrentRun(creation.run);
      setActivity(creation.created ? "Pergunta aceita." : "Retomando a execução existente.");
      queryClient.setQueryData<Awaited<ReturnType<typeof listChatMessages>>>(
        ["chat", "messages", thread.id],
        (page) => {
          if (!page || page.messages.some((message) => message.id === creation.user_message.id))
            return page;
          return {
            ...page,
            messages: [...page.messages, creation.user_message].toSorted(
              (left, right) => left.sequence - right.sequence,
            ),
            total: page.total + 1,
          };
        },
      );
    },
  });
  const cancel = useMutation({
    mutationFn: () => cancelChatRun(currentRun!.id),
    onSuccess: (run) => {
      setCurrentRun(run);
      setActivity("Cancelamento solicitado.");
    },
  });

  useEffect(() => {
    const last = messages.data?.messages.at(-1);
    if (!last || currentRun || hydratedRun.current === last.run_id) return;
    hydratedRun.current = last.run_id;
    const controller = new AbortController();
    void getChatRun(last.run_id, controller.signal)
      .then((run) => {
        setCurrentRun(run);
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, [currentRun, messages.data?.messages]);

  useEffect(() => {
    const persisted =
      messages.data?.messages.flatMap((message) => message.result_reference_ids ?? []).slice(-20) ??
      [];
    if (persisted.length === 0) return;
    setReferenceIDs((current) => [...new Set([...current, ...persisted])].slice(-20));
  }, [messages.data?.messages]);

  const handleEvent = useCallback((event: ChatEvent) => {
    eventCursor.current = event.sequence;
    switch (event.kind) {
      case "TEXT_DELTA":
        setLiveText((current) => `${current}${event.text_delta ?? ""}`);
        break;
      case "TOOL_STARTED":
        setActivity("Consultando dados autorizados…");
        break;
      case "TOOL_COMPLETED":
        setActivity("Consulta tipada concluída.");
        break;
      case "RESULT_REFERENCE":
        if (event.result_reference_id) {
          const referenceID = event.result_reference_id;
          setReferenceIDs((current) =>
            current.includes(referenceID) ? current : [...current, referenceID],
          );
        }
        break;
      case "RUN_COMPLETED":
        setActivity("Resposta concluída.");
        break;
      case "RUN_FAILED":
        setActivity("A resposta falhou com um erro seguro.");
        break;
      case "RUN_CANCELLED":
        setActivity("Resposta cancelada.");
        break;
      default:
        break;
    }
  }, []);

  useEffect(() => {
    if (!currentRun || !isActiveChatRun(currentRun)) return;
    const runID = currentRun.id;
    const controller = new AbortController();
    void consumeChatRunEvents(runID, eventCursor.current, handleEvent, controller.signal)
      .then(async () => {
        const finalRun = await getChatRun(runID, controller.signal);
        setCurrentRun(finalRun);
        await refreshConversation();
        if (finalRun.state === "COMPLETED") setLiveText("");
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        setStreamError(error instanceof Error ? error : new Error("Transmissão interrompida."));
      });
    return () => controller.abort();
  }, [currentRun?.id, handleEvent, refreshConversation]);

  const submit = (retryOf?: string) => {
    const content = composer.trim() || retryContent(messages.data?.messages, retryOf);
    if (!content || turn.isPending || isActiveChatRun(currentRun)) return;
    turn.mutate({ content, ...(retryOf ? { retryOf } : {}) });
  };
  const resultPages = [
    activeReference.data,
    ...streamedReferences.map((query) => query.data),
  ].filter((result): result is ChatReferenceResult => Boolean(result));
  const referenceError =
    activeReference.error ?? streamedReferences.find((query) => query.isError)?.error;
  const shownReferences = new Set<string>();

  return (
    <Surface aria-busy={isActiveChatRun(currentRun)} className="chat-conversation" tone="raised">
      <div className="chat-conversation__header">
        <div>
          <label className="chat-title-control">
            <span className="visually-hidden">Título da conversa</span>
            <input
              maxLength={120}
              value={title}
              onChange={(event) => setTitle(event.target.value)}
            />
          </label>
          <p>Retida até {formatDateTime(thread.retention_expires_at)}.</p>
        </div>
        <Inline align="center">
          <Button
            disabled={!title.trim() || title.trim() === thread.title || rename.isPending}
            onClick={() => rename.mutate()}
          >
            Salvar título
          </Button>
          {!confirmingDelete ? (
            <Button
              disabled={isActiveChatRun(currentRun)}
              onClick={() => setConfirmingDelete(true)}
            >
              Excluir
            </Button>
          ) : (
            <>
              <Button disabled={remove.isPending} onClick={() => remove.mutate()}>
                Confirmar exclusão
              </Button>
              <Button onClick={() => setConfirmingDelete(false)}>Manter</Button>
            </>
          )}
        </Inline>
      </div>

      {rename.isError || remove.isError ? (
        <Alert title="Não foi possível alterar a conversa" tone="danger">
          {chatErrorMessage(rename.error ?? remove.error)}
        </Alert>
      ) : null}
      {thread.active_result_reference_id ? (
        <div className="chat-active-context">
          <div>
            <strong>Contexto ativo</strong>
            <span>
              {activeReference.data?.reference.label ?? "Revalidando o resultado selecionado…"}
            </span>
          </div>
          <Button disabled={activeResult.isPending} onClick={() => activeResult.mutate(null)}>
            Limpar contexto
          </Button>
        </div>
      ) : null}
      {referenceError || activeResult.isError ? (
        <Alert title="Não foi possível revalidar o contexto" tone="danger">
          {chatErrorMessage(referenceError ?? activeResult.error)}
        </Alert>
      ) : null}

      <div className="chat-messages" role="log" aria-label="Mensagens da conversa">
        {messages.isPending ? <p role="status">Carregando mensagens…</p> : null}
        {messages.isError ? (
          <Alert title="Não foi possível carregar as mensagens" tone="danger">
            {chatErrorMessage(messages.error)}
          </Alert>
        ) : null}
        {!messages.isPending && messages.data?.messages.length === 0 ? (
          <div className="chat-welcome">
            <h2>O que você precisa encontrar?</h2>
            <p>
              Peça uma busca ou consulta. Resultados usados em respostas aparecem como evidências
              explícitas e podem ser escolhidos como contexto para a próxima pergunta.
            </p>
          </div>
        ) : null}
        {messages.data?.messages.map((message) => (
          <MessageBubble key={message.id} message={message} />
        ))}
        {liveText ? (
          <article className="chat-message chat-message--assistant">
            <span className="chat-message__author">Assistente</span>
            <p>{liveText}</p>
          </article>
        ) : null}
      </div>

      <div aria-live="polite" className="chat-activity" role="status">
        <StatusBadge tone={runTone(currentRun)}>{runLabel(currentRun)}</StatusBadge>
        <span>{activity}</span>
      </div>
      {streamError || turn.isError || cancel.isError ? (
        <Alert title="A resposta não pôde ser concluída" tone="danger">
          {chatErrorMessage(streamError ?? turn.error ?? cancel.error)}
        </Alert>
      ) : null}
      {currentRun && (currentRun.state === "FAILED" || currentRun.state === "CANCELLED") ? (
        <Inline>
          <Button disabled={turn.isPending} onClick={() => submit(currentRun.id)}>
            Tentar novamente
          </Button>
        </Inline>
      ) : null}

      {resultPages.map((result) => {
        if (shownReferences.has(result.reference.id)) return null;
        shownReferences.add(result.reference.id);
        return (
          <ChatResultCard
            active={thread.active_result_reference_id === result.reference.id}
            busy={activeResult.isPending}
            key={result.reference.id}
            result={result}
            onActivate={() => activeResult.mutate(result.reference.id)}
          />
        );
      })}

      <form
        className="chat-composer"
        onSubmit={(event) => {
          event.preventDefault();
          submit();
        }}
      >
        <label htmlFor="chat-question">Pergunta</label>
        <textarea
          id="chat-question"
          maxLength={capability.maximum_message_runes}
          placeholder="Ex.: encontre as pessoas de Porto Alegre e mostre nome e telefone"
          rows={3}
          value={composer}
          onChange={(event) => setComposer(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !event.shiftKey) {
              event.preventDefault();
              submit();
            }
          }}
        />
        <div className="chat-composer__footer">
          <span>
            {countRunes(composer)} / {capability.maximum_message_runes}
          </span>
          <Inline>
            {isActiveChatRun(currentRun) ? (
              <Button disabled={cancel.isPending} onClick={() => cancel.mutate()}>
                {cancel.isPending ? "Cancelando" : "Cancelar resposta"}
              </Button>
            ) : null}
            <Button
              disabled={!composer.trim() || turn.isPending || isActiveChatRun(currentRun)}
              type="submit"
            >
              {turn.isPending ? "Enviando" : "Enviar"}
            </Button>
          </Inline>
        </div>
      </form>
    </Surface>
  );
}

function MessageBubble({ message }: { message: ChatMessage }) {
  return (
    <article
      className={`chat-message chat-message--${message.role === "USER" ? "user" : "assistant"}`}
    >
      <span className="chat-message__author">
        {message.role === "USER" ? "Você" : "Assistente"}
      </span>
      <p>{message.content}</p>
    </article>
  );
}

function ChatResultCard({
  active,
  busy,
  result,
  onActivate,
}: {
  active: boolean;
  busy: boolean;
  result: ChatReferenceResult;
  onActivate: () => void;
}) {
  return (
    <section className="chat-result" aria-label={`Evidência: ${result.reference.label}`}>
      <div className="chat-result__header">
        <div>
          <span className="chat-result__eyebrow">Evidência reautorizada</span>
          <h3>{result.reference.label}</h3>
          <p>
            {result.row_count} linha(s) · {result.field_count} campo(s) · expira em{" "}
            {formatDateTime(result.reference.expires_at)}
          </p>
        </div>
        <Inline align="center">
          <StatusBadge tone={result.reference.kind === "QUERY" ? "info" : "neutral"}>
            {result.reference.kind === "QUERY" ? "Consulta" : "Busca"}
          </StatusBadge>
          {active ? (
            <StatusBadge tone="success">Contexto ativo</StatusBadge>
          ) : (
            <Button disabled={busy} onClick={onActivate}>
              Usar nas próximas perguntas
            </Button>
          )}
        </Inline>
      </div>
      {result.reference.kind === "SEARCH" ? (
        <SearchEvidenceGrid data={result.data as SearchReferenceData} />
      ) : (
        <QueryEvidenceGrid data={result.data as QueryReferenceData} />
      )}
    </section>
  );
}

function SearchEvidenceGrid({ data }: { data: SearchReferenceData }) {
  const columns = useMemo<ColumnDef<SearchEvidence, unknown>[]>(
    () => [
      { accessorKey: "entity_label", header: "Registro" },
      { accessorKey: "field_label", header: "Campo" },
      { accessorKey: "preview", header: "Valor autorizado" },
    ],
    [],
  );
  return (
    <DataGrid
      caption="Resultados tipados da busca do Chat"
      columns={columns}
      data={data.results}
      emptyLabel="A busca não retornou evidências."
      getRowId={(row) => `${row.target_kind}:${row.target_id}:${row.field_key}:${row.score}`}
      loading={false}
      loadingLabel="Carregando evidências"
      renderCard={(row) => (
        <Surface className="chat-result-card" tone="raised">
          <Stack gap="2">
            <strong>{row.entity_label}</strong>
            <span>{row.field_label}</span>
            <p>{row.preview}</p>
          </Stack>
        </Surface>
      )}
    />
  );
}

function QueryEvidenceGrid({ data }: { data: QueryReferenceData }) {
  const columns = useMemo<ColumnDef<QueryReferenceRow, unknown>[]>(
    () => [
      { accessorKey: "entity_label", header: "Registro" },
      ...data.columns.map(
        (column): ColumnDef<QueryReferenceRow, unknown> => ({
          id: column.field_key,
          header: column.label,
          cell: ({ row }) => formatQueryCell(row.original, column.position),
        }),
      ),
    ],
    [data.columns],
  );
  return (
    <DataGrid
      caption="Resultado tipado da consulta do Chat"
      columns={columns}
      data={data.rows}
      emptyLabel="A consulta não retornou linhas."
      getRowId={(row) => `${row.entity_kind}:${row.entity_id}:${row.position}`}
      loading={false}
      loadingLabel="Carregando resultado"
      renderCard={(row) => (
        <Surface className="chat-result-card" tone="raised">
          <Stack gap="2">
            <strong>{row.entity_label}</strong>
            {data.columns.map((column) => (
              <span key={column.position}>
                <b>{column.label}:</b> {formatQueryCell(row, column.position)}
              </span>
            ))}
          </Stack>
        </Surface>
      )}
    />
  );
}

function formatQueryCell(row: QueryReferenceRow, position: number): string {
  const cell = row.cells.find((candidate) => candidate.column_position === position);
  if (!cell || cell.is_null) return "—";
  if (typeof cell.value === "boolean") return cell.value ? "Sim" : "Não";
  return `${cell.value ?? "—"}${cell.truncated ? "…" : ""}`;
}

function retryContent(messages: ChatMessage[] | undefined, runID: string | undefined): string {
  if (!runID) return "";
  return (
    messages?.toReversed().find((message) => message.run_id === runID && message.role === "USER")
      ?.content ?? ""
  );
}

function newIdempotencyKey(): string {
  return globalThis.crypto?.randomUUID?.() ?? `chat-${Date.now()}-${Math.random()}`;
}

function countRunes(value: string): number {
  return [...value].length;
}

function runTone(run: ChatRun | null): "neutral" | "info" | "success" | "warning" | "danger" {
  if (!run) return "neutral";
  if (run.state === "COMPLETED") return "success";
  if (run.state === "FAILED") return "danger";
  if (run.state === "CANCELLED") return "warning";
  return "info";
}

function runLabel(run: ChatRun | null): string {
  switch (run?.state) {
    case "QUEUED":
      return "Na fila";
    case "RUNNING":
      return "Respondendo";
    case "TOOL_RUNNING":
      return "Consultando";
    case "COMPLETED":
      return "Concluída";
    case "FAILED":
      return "Falhou";
    case "CANCELLED":
      return "Cancelada";
    default:
      return "Pronto";
  }
}

export function chatErrorMessage(error: unknown): string {
  if (error instanceof APIRequestError) {
    switch (error.code) {
      case "rate_limited":
        return "O limite de perguntas foi atingido. Aguarde um minuto e tente novamente.";
      case "chat_quota_exceeded":
        return "O limite seguro desta conversa foi atingido. Refine a pergunta ou abra outra conversa.";
      case "chat_busy":
        return "Esta conversa já possui uma resposta em andamento.";
      case "chat_timeout":
        return "A resposta excedeu o tempo seguro. Tente uma pergunta mais específica.";
      case "chat_stale_context":
        return "O resultado usado como contexto expirou. Limpe o contexto e tente novamente.";
      case "chat_cancelled":
        return "A resposta foi cancelada.";
      case "forbidden":
        return "Sua sessão não possui mais permissão para este recurso.";
      default:
        return error.message;
    }
  }
  return error instanceof Error ? error.message : "Erro inesperado no Chat.";
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "short" }).format(
    new Date(value),
  );
}

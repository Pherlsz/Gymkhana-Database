import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const threadID = "019bf789-4400-7f12-9abc-123456789abc";
const runID = "019bf789-4400-7f12-9abc-123456789abd";
const userMessageID = "019bf789-4400-7f12-9abc-123456789abe";
const assistantMessageID = "019bf789-4400-7f12-9abc-123456789abf";
const referenceID = "019bf789-4400-7f12-9abc-123456789ac0";
const retryRunID = "019bf789-4400-7f12-9abc-123456789ac1";
const retryUserMessageID = "019bf789-4400-7f12-9abc-123456789ac2";
const retryAssistantMessageID = "019bf789-4400-7f12-9abc-123456789ac3";
const timestamp = "2026-07-18T12:00:00Z";
const maliciousEvidence = "</td><script>globalThis.chatInjected=true</script><td>";

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "content-type": "application/json" },
  });
}

function thread(active = false) {
  return {
    id: threadID,
    title: "Pessoas de Porto Alegre",
    ...(active ? { active_result_reference_id: referenceID } : {}),
    retention_expires_at: "2026-08-18T12:00:00Z",
    version: active ? 2 : 1,
    created_at: timestamp,
    updated_at: timestamp,
  };
}

function run(state: string, id = runID) {
  const terminal = state === "COMPLETED" || state === "FAILED" || state === "CANCELLED";
  return {
    id,
    thread_id: threadID,
    state,
    tool_call_count: state === "COMPLETED" ? 1 : 0,
    input_usage: 10,
    output_usage: state === "COMPLETED" ? 20 : 0,
    result_bytes: 100,
    version: state === "COMPLETED" ? 3 : 1,
    created_at: timestamp,
    updated_at: timestamp,
    ...(terminal ? { completed_at: timestamp } : {}),
  };
}

function eventFrame(sequence: number, kind: string, extra: Record<string, unknown> = {}): string {
  return `id: ${sequence}\nevent: ${kind}\ndata: ${JSON.stringify({ sequence, kind, created_at: timestamp, ...extra })}\n\n`;
}

describe("AI Chat page", () => {
  beforeEach(() => {
    window.history.replaceState(null, "", `/chat?thread=${threadID}`);
    vi.stubGlobal("scrollTo", vi.fn());
  });
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("stays explicitly disabled without contacting thread or provider routes", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(
          jsonResponse({
            authenticated: true,
            user: { login: "member", display_name: "Member Name", role: "EXTERNAL" },
          }),
        );
      if (url.endsWith("/health/live")) return Promise.resolve(jsonResponse({ status: "ok" }));
      if (url.endsWith("/api/v1/chat/capability"))
        return Promise.resolve(
          jsonResponse({
            enabled: false,
            maximum_tool_calls: 8,
            maximum_rows: 100,
            maximum_result_bytes: 262144,
            maximum_usage: 200000,
            maximum_duration_seconds: 45,
            maximum_message_runes: 20000,
          }),
        );
      return Promise.resolve(jsonResponse({ code: "chat_unavailable" }, 503));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);

    expect(await screen.findByText("Chat ainda não ativado")).toBeInTheDocument();
    expect(
      screen.getByText(/Nenhum dado é enviado a um modelo enquanto estiver desativado/),
    ).toBeInTheDocument();
    expect(fetchMock.mock.calls.some(([input]) => String(input).includes("/chat/threads"))).toBe(
      false,
    );
  });

  it("accepts a persisted turn, streams the answer, renders typed evidence, and selects context", async () => {
    let accepted = false;
    let active = false;
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(
          jsonResponse({
            authenticated: true,
            user: { login: "member", display_name: "Member Name", role: "EXTERNAL" },
          }),
        );
      if (url.endsWith("/health/live")) return Promise.resolve(jsonResponse({ status: "ok" }));
      if (url.endsWith("/api/v1/chat/capability"))
        return Promise.resolve(
          jsonResponse({
            enabled: true,
            maximum_tool_calls: 8,
            maximum_rows: 100,
            maximum_result_bytes: 262144,
            maximum_usage: 10000,
            maximum_duration_seconds: 60,
            maximum_message_runes: 20000,
          }),
        );
      if (url.includes("/api/v1/chat/threads?"))
        return Promise.resolve(
          jsonResponse({ threads: [thread(active)], total: 1, limit: 100, offset: 0 }),
        );
      if (url.includes(`/api/v1/chat/threads/${threadID}/messages`))
        return Promise.resolve(
          jsonResponse({
            messages: accepted
              ? [
                  {
                    id: userMessageID,
                    thread_id: threadID,
                    run_id: runID,
                    sequence: 1,
                    role: "USER",
                    content: "Liste as pessoas de Porto Alegre",
                    result_reference_ids: [],
                    created_at: timestamp,
                  },
                  {
                    id: assistantMessageID,
                    thread_id: threadID,
                    run_id: runID,
                    sequence: 2,
                    role: "ASSISTANT",
                    content: "Encontrei uma pessoa autorizada.",
                    result_reference_ids: [referenceID],
                    created_at: timestamp,
                  },
                ]
              : [],
            total: accepted ? 2 : 0,
            limit: 100,
            offset: 0,
          }),
        );
      if (url.endsWith(`/api/v1/chat/threads/${threadID}/turns`) && init?.method === "POST") {
        accepted = true;
        return Promise.resolve(
          jsonResponse(
            {
              run: run("RUNNING"),
              user_message: {
                id: userMessageID,
                thread_id: threadID,
                run_id: runID,
                sequence: 1,
                role: "USER",
                content: "Liste as pessoas de Porto Alegre",
                result_reference_ids: [],
                created_at: timestamp,
              },
              created: true,
            },
            201,
          ),
        );
      }
      if (url.includes(`/api/v1/chat/runs/${runID}/events`))
        return Promise.resolve(
          new Response(
            eventFrame(1, "RUN_STARTED") +
              eventFrame(2, "TEXT_DELTA", { text_delta: "Encontrei " }) +
              eventFrame(3, "TOOL_STARTED", { tool_step_id: runID }) +
              eventFrame(4, "TOOL_COMPLETED", { tool_step_id: runID }) +
              eventFrame(5, "RESULT_REFERENCE", { result_reference_id: referenceID }) +
              eventFrame(6, "TEXT_DELTA", { text_delta: "uma pessoa autorizada." }) +
              eventFrame(7, "RUN_COMPLETED"),
            { headers: { "content-type": "text/event-stream" } },
          ),
        );
      if (url.endsWith(`/api/v1/chat/runs/${runID}`))
        return Promise.resolve(jsonResponse(run("COMPLETED")));
      if (url.includes(`/api/v1/chat/result-references/${referenceID}`))
        return Promise.resolve(
          jsonResponse({
            reference: {
              id: referenceID,
              thread_id: threadID,
              run_id: runID,
              kind: "SEARCH",
              label: "Pessoas encontradas",
              row_count: 1,
              column_count: 3,
              expires_at: "2026-07-19T12:00:00Z",
              created_at: timestamp,
            },
            data: {
              reference_id: referenceID,
              results: [
                {
                  module: "profiles",
                  entity_kind: "profile",
                  entity_id: threadID,
                  profile_id: threadID,
                  target_kind: "profile",
                  target_id: threadID,
                  entity_label: "Ana da Silva",
                  field_key: "profile.full_name",
                  field_label: "Nome completo",
                  preview: maliciousEvidence,
                  score: 100,
                },
              ],
              total: 1,
            },
            row_count: 1,
            field_count: 3,
          }),
        );
      if (
        url.endsWith(`/api/v1/chat/threads/${threadID}/active-result`) &&
        init?.method === "PUT"
      ) {
        const request = JSON.parse(String(init.body)) as { reference_id: string | null };
        active = Boolean(request.reference_id);
        return Promise.resolve(jsonResponse(thread(active)));
      }
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);

    const view = render(<App />);

    expect(
      await screen.findByRole("heading", { name: "Chat com os dados autorizados" }),
    ).toBeInTheDocument();
    fireEvent.change(await screen.findByLabelText("Pergunta"), {
      target: { value: "Liste as pessoas de Porto Alegre" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Enviar" }));

    expect(await screen.findByText("Encontrei uma pessoa autorizada.")).toBeInTheDocument();
    expect(await screen.findAllByText("Ana da Silva")).not.toHaveLength(0);
    expect(
      screen.getByRole("table", { name: "Resultados tipados da busca do Chat" }),
    ).toBeInTheDocument();
    expect(await screen.findAllByText(maliciousEvidence)).not.toHaveLength(0);
    expect(document.querySelector("script")).toBeNull();
    expect(document.querySelector("[onerror]")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Usar nas próximas perguntas" }));

    await waitFor(() => expect(active).toBe(true));
    expect(await screen.findAllByText("Contexto ativo")).not.toHaveLength(0);
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining(`/api/v1/chat/threads/${threadID}/turns`),
      expect.objectContaining({ method: "POST" }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Limpar contexto" }));
    await waitFor(() => expect(active).toBe(false));
    const referenceReadsBeforeReload = fetchMock.mock.calls.filter(([input]) =>
      String(input).includes(`/api/v1/chat/result-references/${referenceID}`),
    ).length;
    view.unmount();
    render(<App />);

    expect(await screen.findAllByText(maliciousEvidence)).not.toHaveLength(0);
    await waitFor(() =>
      expect(
        fetchMock.mock.calls.filter(([input]) =>
          String(input).includes(`/api/v1/chat/result-references/${referenceID}`),
        ).length,
      ).toBeGreaterThan(referenceReadsBeforeReload),
    );
  });

  it("cancels an active stream, preserves partial text, and retries the persisted turn", async () => {
    const encoder = new TextEncoder();
    let cancelled = false;
    let retryStarted = false;
    let retryCompleted = false;
    let firstController: ReadableStreamDefaultController<Uint8Array> | undefined;

    const finishCancelledStream = () => {
      if (!firstController) return;
      firstController.enqueue(encoder.encode(eventFrame(3, "RUN_CANCELLED")));
      firstController.close();
      firstController = undefined;
    };

    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(
          jsonResponse({
            authenticated: true,
            user: { login: "member", display_name: "Member Name", role: "EXTERNAL" },
          }),
        );
      if (url.endsWith("/health/live")) return Promise.resolve(jsonResponse({ status: "ok" }));
      if (url.endsWith("/api/v1/chat/capability"))
        return Promise.resolve(
          jsonResponse({
            enabled: true,
            maximum_tool_calls: 8,
            maximum_rows: 100,
            maximum_result_bytes: 262144,
            maximum_usage: 10000,
            maximum_duration_seconds: 60,
            maximum_message_runes: 20000,
          }),
        );
      if (url.includes("/api/v1/chat/threads?"))
        return Promise.resolve(
          jsonResponse({ threads: [thread()], total: 1, limit: 100, offset: 0 }),
        );
      if (url.includes(`/api/v1/chat/threads/${threadID}/messages`)) {
        const messages = cancelled
          ? [
              {
                id: userMessageID,
                thread_id: threadID,
                run_id: runID,
                sequence: 1,
                role: "USER",
                content: "Cancele e repita",
                created_at: timestamp,
              },
              ...(retryStarted
                ? [
                    {
                      id: retryUserMessageID,
                      thread_id: threadID,
                      run_id: retryRunID,
                      sequence: 2,
                      role: "USER",
                      content: "Cancele e repita",
                      created_at: timestamp,
                    },
                  ]
                : []),
              ...(retryCompleted
                ? [
                    {
                      id: retryAssistantMessageID,
                      thread_id: threadID,
                      run_id: retryRunID,
                      sequence: 3,
                      role: "ASSISTANT",
                      content: "Retry concluído.",
                      created_at: timestamp,
                    },
                  ]
                : []),
            ]
          : [];
        return Promise.resolve(
          jsonResponse({ messages, total: messages.length, limit: 100, offset: 0 }),
        );
      }
      if (url.endsWith(`/api/v1/chat/threads/${threadID}/turns`) && init?.method === "POST") {
        const request = JSON.parse(String(init.body)) as {
          content: string;
          retry_of_run_id?: string;
        };
        if (!retryStarted) {
          return Promise.resolve(
            jsonResponse(
              {
                run: run("RUNNING"),
                user_message: {
                  id: userMessageID,
                  thread_id: threadID,
                  run_id: runID,
                  sequence: 1,
                  role: "USER",
                  content: request.content,
                  created_at: timestamp,
                },
                created: true,
              },
              201,
            ),
          );
        }
        expect(request).toMatchObject({ content: "Cancele e repita", retry_of_run_id: runID });
        return Promise.resolve(
          jsonResponse(
            {
              run: run("RUNNING", retryRunID),
              user_message: {
                id: retryUserMessageID,
                thread_id: threadID,
                run_id: retryRunID,
                sequence: 2,
                role: "USER",
                content: request.content,
                created_at: timestamp,
              },
              created: true,
            },
            201,
          ),
        );
      }
      if (url.includes(`/api/v1/chat/runs/${runID}/events`))
        return Promise.resolve(
          new Response(
            new ReadableStream<Uint8Array>({
              start(controller) {
                firstController = controller;
                controller.enqueue(
                  encoder.encode(
                    eventFrame(1, "RUN_STARTED") +
                      eventFrame(2, "TEXT_DELTA", { text_delta: "Parcial legível." }),
                  ),
                );
                if (cancelled) finishCancelledStream();
              },
            }),
            { headers: { "content-type": "text/event-stream" } },
          ),
        );
      if (url.endsWith(`/api/v1/chat/runs/${runID}/cancel`) && init?.method === "POST") {
        cancelled = true;
        finishCancelledStream();
        return Promise.resolve(
          jsonResponse({ ...run("RUNNING"), cancel_requested_at: timestamp, version: 2 }),
        );
      }
      if (url.endsWith(`/api/v1/chat/runs/${runID}`))
        return Promise.resolve(jsonResponse(run(cancelled ? "CANCELLED" : "RUNNING")));
      if (url.includes(`/api/v1/chat/runs/${retryRunID}/events`))
        return Promise.resolve(
          new Response(
            eventFrame(1, "RUN_STARTED") +
              eventFrame(2, "TEXT_DELTA", { text_delta: "Retry concluído." }) +
              eventFrame(3, "RUN_COMPLETED"),
            { headers: { "content-type": "text/event-stream" } },
          ),
        );
      if (url.endsWith(`/api/v1/chat/runs/${retryRunID}`)) {
        retryCompleted = true;
        return Promise.resolve(jsonResponse(run("COMPLETED", retryRunID)));
      }
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);

    const view = render(<App />);

    fireEvent.change(await screen.findByLabelText("Pergunta"), {
      target: { value: "Cancele e repita" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Enviar" }));
    expect(await screen.findByText("Parcial legível.")).toBeInTheDocument();
    fireEvent.click(await screen.findByRole("button", { name: "Cancelar resposta" }));

    await screen.findByRole("button", { name: "Tentar novamente" });
    expect(screen.getByText("Parcial legível.")).toBeInTheDocument();
    view.unmount();
    render(<App />);
    const retryButton = await screen.findByRole("button", { name: "Tentar novamente" });
    retryStarted = true;
    fireEvent.click(retryButton);

    expect(await screen.findByText("Retry concluído.")).toBeInTheDocument();
    await waitFor(() =>
      expect(
        fetchMock.mock.calls.filter(
          ([input, options]) =>
            String(input).endsWith(`/api/v1/chat/threads/${threadID}/turns`) &&
            (options as RequestInit | undefined)?.method === "POST",
        ),
      ).toHaveLength(2),
    );
  });
});

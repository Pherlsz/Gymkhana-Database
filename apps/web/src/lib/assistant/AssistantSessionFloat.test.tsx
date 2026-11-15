import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { I18nProvider } from "../../i18n";
import { AssistantSessionFloat } from "./AssistantSessionFloat";
import { useAssistantChat } from "./useAssistantChat";

vi.mock("./useAssistantChat", () => ({
  useAssistantChat: vi.fn(),
}));

const thread = {
  id: "thread-1",
  title: "pessoa cujo nome seja Shakira",
  version: 1,
  created_at: "2026-01-01T00:00:00.000Z",
  updated_at: "2026-01-01T00:00:00.000Z",
  retention_expires_at: "2026-02-01T00:00:00.000Z",
};

function chatStub(overrides: Partial<ReturnType<typeof useAssistantChat>> = {}) {
  return {
    capability: {
      enabled: true,
      maximum_tool_calls: 8,
      maximum_rows: 100,
      maximum_result_bytes: 256,
      maximum_usage: 1,
      maximum_duration_seconds: 30,
      maximum_message_runes: 20000,
    },
    capabilityLoading: false,
    capabilityFailed: false,
    chatReady: true,
    threads: [thread],
    thread,
    selectThread: vi.fn(),
    messages: [
      {
        id: "msg-1",
        thread_id: thread.id,
        run_id: "run-1",
        sequence: 1,
        role: "USER" as const,
        content: "pessoa cujo nome seja Shakira, aceitando-se as seguintes variações de grafia.",
        result_reference_ids: [] as string[],
        result_references: [] as {
          id: string;
          kind: "SEARCH" | "QUERY";
          label: string;
          row_count: number;
        }[],
        created_at: "2026-01-01T00:00:00.000Z",
      },
    ],
    messagesLoading: false,
    messagesRefreshing: false,
    threadsLoading: false,
    pending: {
      runId: "run-2",
      threadId: thread.id,
      content: "e documentos?",
      text: "",
      roundText: "",
      toolRunning: true,
      resultReferenceIds: [],
    },
    busy: true,
    createThread: vi.fn(),
    renameThread: vi.fn(),
    deleteThread: vi.fn(),
    send: vi.fn(),
    sendError: null,
    sessionError: null,
    cancel: vi.fn(),
    retry: vi.fn(),
    dismissError: vi.fn(),
    ...overrides,
  };
}

function stubMatchMedia(compact: boolean) {
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      matches: compact && (query.includes("max-width: 599px") || query.includes("width < 600px")),
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });
}

function renderFloat(ui: ReactNode) {
  return render(<I18nProvider locale="pt-BR">{ui}</I18nProvider>);
}

describe("AssistantSessionFloat", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    stubMatchMedia(false);
    vi.mocked(useAssistantChat).mockReturnValue(chatStub());
  });

  it("keeps desktop chrome: sessions rail, expand, and the running turn", () => {
    const onSizeChange = vi.fn();
    renderFloat(
      <AssistantSessionFloat size="float" tableLabel="Pessoas" onSizeChange={onSizeChange} />,
    );

    const dialog = screen.getByRole("dialog", { name: "Sessões" });
    expect(dialog.className).toContain("is-float");
    expect(dialog.className).not.toContain("is-compact");
    expect(screen.getByRole("navigation", { name: "Suas sessões" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ampliar" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Minimizar" })).toBeInTheDocument();
    expect(screen.getByText("Consultando a base…")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Pensando…")).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Minimizar" }));
    expect(onSizeChange).toHaveBeenCalledWith("min");
  });

  it("applies the last pending result when the turn fails", () => {
    const onShowResult = vi.fn();
    vi.mocked(useAssistantChat).mockReturnValue(
      chatStub({
        pending: {
          runId: "run-2",
          threadId: thread.id,
          content: "veiculos",
          text: "",
          roundText: "",
          toolRunning: false,
          resultReferenceIds: ["ref-7"],
          errorCode: "malformed_provider",
        },
        busy: false,
      }),
    );
    renderFloat(
      <AssistantSessionFloat
        onShowResult={onShowResult}
        onSizeChange={vi.fn()}
        size="float"
        tableLabel="Pessoas"
      />,
    );
    expect(onShowResult).toHaveBeenCalledWith("ref-7");
  });

  it("uses a compact sheet on narrow viewports: no expand, sessions closed", () => {
    stubMatchMedia(true);
    renderFloat(<AssistantSessionFloat size="float" tableLabel="Pessoas" onSizeChange={vi.fn()} />);

    const dialog = screen.getByRole("dialog", { name: "Sessões" });
    expect(dialog.className).toContain("is-compact");
    expect(dialog.className).toContain("is-sessions-collapsed");
    expect(screen.queryByRole("navigation", { name: "Suas sessões" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Ampliar" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Mostrar sessões" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Minimizar" })).toBeInTheDocument();
    expect(screen.getByText("esta tabela · Pessoas")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Mostrar sessões" }));
    expect(screen.getByRole("navigation", { name: "Suas sessões" })).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Recolher sessões" })).toHaveLength(1);
  });

  function answeredWith(
    references: { id: string; kind: "SEARCH" | "QUERY"; label: string; row_count: number }[],
  ) {
    return chatStub({
      pending: null,
      busy: false,
      messages: [
        ...chatStub().messages,
        {
          id: "msg-2",
          thread_id: thread.id,
          run_id: "run-1",
          sequence: 2,
          role: "ASSISTANT" as const,
          content: "Encontrei 27 registros que citam primo.",
          result_reference_ids: ["ref-gravatai", "ref-73", "ref-primo", "ref-sample"],
          result_references: references,
          created_at: "2026-01-01T00:00:01.000Z",
        },
      ],
    });
  }

  it("opens the last grid-worthy result instead of the last reference of the reply", () => {
    const onShowResult = vi.fn();
    const float = () => (
      <I18nProvider locale="pt-BR">
        <AssistantSessionFloat
          onShowResult={onShowResult}
          onSizeChange={vi.fn()}
          size="float"
          tableLabel="Pessoas"
        />
      </I18nProvider>
    );
    vi.mocked(useAssistantChat).mockReturnValue(chatStub({ pending: null, busy: false }));
    const view = render(float());
    vi.mocked(useAssistantChat).mockReturnValue(
      answeredWith([{ id: "ref-primo", kind: "SEARCH", label: "Busca: primo", row_count: 27 }]),
    );
    view.rerender(float());
    expect(onShowResult).toHaveBeenCalledWith("ref-primo");
    expect(onShowResult).not.toHaveBeenCalledWith("ref-sample");
    expect(screen.getByRole("button", { name: "Aplicar na grade" })).toBeInTheDocument();
  });

  it("offers each grid-worthy result of a reply with its label and row count", () => {
    const onShowResult = vi.fn();
    vi.mocked(useAssistantChat).mockReturnValue(
      answeredWith([
        { id: "ref-gravatai", kind: "SEARCH", label: "Busca: Gravataí", row_count: 34 },
        { id: "ref-primo", kind: "QUERY", label: "Consulta: profiles", row_count: 1 },
      ]),
    );
    renderFloat(
      <AssistantSessionFloat
        activeResultId="ref-primo"
        onShowResult={onShowResult}
        onSizeChange={vi.fn()}
        size="float"
        tableLabel="Pessoas"
      />,
    );
    const options = screen.getByRole("list", { name: "Resultados desta resposta" });
    expect(options).toHaveTextContent("Consulta: Pessoas · 1 linha · Na grade");
    fireEvent.click(screen.getByRole("button", { name: "Busca: Gravataí · 34 linhas" }));
    expect(onShowResult).toHaveBeenLastCalledWith("ref-gravatai");
  });

  it("offers no grid action when the reply found nothing worth opening", () => {
    vi.mocked(useAssistantChat).mockReturnValue(answeredWith([]));
    renderFloat(
      <AssistantSessionFloat
        onShowResult={vi.fn()}
        onSizeChange={vi.fn()}
        size="float"
        tableLabel="Pessoas"
      />,
    );
    expect(screen.queryByRole("button", { name: "Aplicar na grade" })).not.toBeInTheDocument();
    expect(
      screen.queryByRole("list", { name: "Resultados desta resposta" }),
    ).not.toBeInTheDocument();
  });
});

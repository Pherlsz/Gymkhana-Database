import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AttachmentsPanel } from "./AttachmentsPanel";
import { APIRequestError } from "./lib/api/client";
import type { AttachmentRecord } from "./lib/api/attachments";

const attachmentAPI = vi.hoisted(() => ({
  listAttachments: vi.fn(),
  uploadAttachment: vi.fn(),
  downloadAttachment: vi.fn(),
  trashAttachment: vi.fn(),
  restoreAttachment: vi.fn(),
}));

vi.mock("./lib/api/attachments", () => attachmentAPI);

const owner = { owner_kind: "DOCUMENT" as const, owner_id: "document-1" };
const active: AttachmentRecord = {
  ...owner,
  id: "attachment-1",
  original_filename: "proof.pdf",
  declared_mime: "application/pdf",
  detected_mime: "application/pdf",
  byte_size: 128,
  sha256: "a".repeat(64),
  lifecycle_state: "ACTIVE",
  version: 1,
  created_at: "2026-07-17T12:00:00Z",
  updated_at: "2026-07-17T12:00:00Z",
};
const trashed: AttachmentRecord = {
  ...active,
  lifecycle_state: "TRASHED",
  version: 2,
  deleted_at: "2026-07-17T13:00:00Z",
  purge_after: "2026-07-24T13:00:00Z",
};

function renderPanel() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <AttachmentsPanel owner={owner} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  attachmentAPI.listAttachments.mockReset();
  attachmentAPI.uploadAttachment.mockReset();
  attachmentAPI.downloadAttachment.mockReset();
  attachmentAPI.trashAttachment.mockReset();
  attachmentAPI.restoreAttachment.mockReset();
});

afterEach(cleanup);

describe("AttachmentsPanel", () => {
  it("covers loading, empty, progress, and verified upload success", async () => {
    attachmentAPI.listAttachments.mockResolvedValue([]);
    let completeUpload!: (value: AttachmentRecord) => void;
    attachmentAPI.uploadAttachment.mockImplementation(
      (_owner: unknown, _file: unknown, progress: (value: number) => void) => {
        progress(45);
        return new Promise<AttachmentRecord>((resolve) => {
          completeUpload = resolve;
        });
      },
    );
    renderPanel();

    expect(screen.getByText("Carregando anexos...")).toBeInTheDocument();
    expect(await screen.findByText("Nenhum anexo ativo.")).toBeInTheDocument();
    expect(attachmentAPI.listAttachments).toHaveBeenCalledWith(
      owner,
      false,
      expect.any(AbortSignal),
    );

    const file = new File(["%PDF-1.7"], "proof.pdf", { type: "application/pdf" });
    fireEvent.change(screen.getByLabelText("Adicionar arquivo"), { target: { files: [file] } });
    expect(await screen.findByText("Enviando 45%")).toBeInTheDocument();
    completeUpload(active);
    expect(await screen.findByText("Arquivo verificado e anexado.")).toBeInTheDocument();
    expect(attachmentAPI.uploadAttachment).toHaveBeenCalledWith(owner, file, expect.any(Function));
  });

  it("rejects empty files locally and explains expired upload intents", async () => {
    attachmentAPI.listAttachments.mockResolvedValue([]);
    attachmentAPI.uploadAttachment.mockRejectedValue(
      new APIRequestError("expired", {
        status: 410,
        code: "conflict",
        requestId: "request-expired",
      }),
    );
    renderPanel();
    expect(await screen.findByText("Nenhum anexo ativo.")).toBeInTheDocument();

    const expired = new File(["%PDF-1.7"], "expired.pdf", { type: "application/pdf" });
    fireEvent.change(screen.getByLabelText("Adicionar arquivo"), {
      target: { files: [expired] },
    });
    expect(
      await screen.findByText("O envio expirou. Selecione o arquivo novamente."),
    ).toBeInTheDocument();

    const calls = attachmentAPI.uploadAttachment.mock.calls.length;
    const empty = new File([], "empty.pdf", { type: "application/pdf" });
    fireEvent.change(screen.getByLabelText("Adicionar arquivo"), { target: { files: [empty] } });
    expect(await screen.findByText("Selecione um arquivo que não esteja vazio.")).toBeInTheDocument();
    expect(attachmentAPI.uploadAttachment).toHaveBeenCalledTimes(calls);
  });

  it("surfaces an optimistic conflict while trashing", async () => {
    attachmentAPI.listAttachments.mockResolvedValue([active]);
    attachmentAPI.trashAttachment.mockRejectedValue(
      new APIRequestError("conflict", { status: 409, code: "conflict", requestId: undefined }),
    );
    renderPanel();

    fireEvent.click(await screen.findByRole("button", { name: "Mover para lixeira" }));
    expect(
      await screen.findByText("O anexo foi alterado por outra pessoa. Atualize e tente novamente."),
    ).toBeInTheDocument();
  });

  it("trashes and restores through the recoverable seven-day view", async () => {
    attachmentAPI.listAttachments.mockImplementation(
      async (_owner: unknown, includeTrashed: boolean) => (includeTrashed ? [trashed] : [active]),
    );
    attachmentAPI.trashAttachment.mockResolvedValue(trashed);
    attachmentAPI.restoreAttachment.mockResolvedValue(active);
    renderPanel();

    fireEvent.click(await screen.findByRole("button", { name: "Mover para lixeira" }));
    expect(
      await screen.findByText("Anexo movido para a lixeira por sete dias."),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("checkbox", { name: "Mostrar lixeira" }));
    fireEvent.click(await screen.findByRole("button", { name: "Restaurar" }));
    await waitFor(() =>
      expect(attachmentAPI.restoreAttachment.mock.calls[0]?.[0]).toEqual(trashed),
    );
    expect(await screen.findByText("Anexo restaurado.")).toBeInTheDocument();
  });
});

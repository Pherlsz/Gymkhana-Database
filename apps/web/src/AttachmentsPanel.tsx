function toneToType(tone: string): "info" | "success" | "warning" | "error" {
  return tone === "danger" ? "error" : (tone as any);
}

import { Alert, Button, Card, Flex, Tag } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";
import {
  downloadAttachment,
  listAttachments,
  restoreAttachment,
  trashAttachment,
  uploadAttachment,
  type AttachmentOwner,
  type AttachmentRecord,
} from "./lib/api/attachments";
import { APIRequestError } from "./lib/api/client";

const acceptedMIMEs = [
  "application/pdf",
  "image/jpeg",
  "image/png",
  "image/webp",
  "image/gif",
  "text/plain",
  "text/csv",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  "audio/mpeg",
  "audio/wav",
  "video/mp4",
  "video/webm",
].join(",");

const maximumClientBytes = 50 * 1024 * 1024;

export function AttachmentsPanel({
  owner,
  title = "Anexos privados",
  description = "Arquivos são enviados diretamente ao armazenamento privado e verificados antes de serem registrados.",
}: {
  owner: AttachmentOwner;
  title?: string;
  description?: string;
}) {
  const queryClient = useQueryClient();
  const inputID = useId();
  const [showTrash, setShowTrash] = useState(false);
  const [progress, setProgress] = useState<number | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const queryKey = ["attachments", owner, showTrash] as const;
  const query = useQuery({
    queryKey,
    queryFn: ({ signal }) => listAttachments(owner, showTrash, signal),
  });
  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: ["attachments", owner] });
  };
  const upload = useMutation({
    mutationFn: (file: File) => uploadAttachment(owner, file, setProgress),
    onMutate: () => {
      setProgress(0);
      setNotice(null);
    },
    onSuccess: async () => {
      setNotice("Arquivo verificado e anexado.");
      await refresh();
    },
    onSettled: () => setProgress(null),
  });
  const download = useMutation({ mutationFn: downloadAttachment });
  const trash = useMutation({
    mutationFn: trashAttachment,
    onSuccess: async () => {
      setNotice("Anexo movido para a lixeira por sete dias.");
      await refresh();
    },
  });
  const restore = useMutation({
    mutationFn: restoreAttachment,
    onSuccess: async () => {
      setNotice("Anexo restaurado.");
      await refresh();
    },
  });
  const error = query.error ?? upload.error ?? download.error ?? trash.error ?? restore.error;
  const pending = upload.isPending || download.isPending || trash.isPending || restore.isPending;

  const selectFile = (file: File | undefined) => {
    if (!file) return;
    setNotice(null);
    if (file.size <= 0) {
      setNotice("Selecione um arquivo que não esteja vazio.");
      return;
    }
    if (file.size > maximumClientBytes) {
      setNotice("O arquivo ultrapassa o limite de 50 MB.");
      return;
    }
    if (!acceptedMIMEs.split(",").includes(file.type)) {
      setNotice("O formato deste arquivo ainda não é aceito.");
      return;
    }
    upload.mutate(file);
  };

  return (
    <Card className="attachments-panel">
      <Flex vertical gap="1rem">
        <div className="attachments-panel__header">
          <div>
            <h3>{title}</h3>
            <p>{description}</p>
          </div>
          <label className="attachments-panel__trash-toggle">
            <input
              checked={showTrash}
              type="checkbox"
              onChange={(event) => setShowTrash(event.target.checked)}
            />
            Mostrar lixeira
          </label>
        </div>
        {notice ? (
          <Alert
            title="Anexos"
            type={toneToType(
              notice.includes("Arquivo verificado") ||
                notice.includes("restaurado") ||
                notice.includes("movido")
                ? "success"
                : "warning",
            )}
            description={<>{notice}</>}
          />
        ) : null}
        {error ? (
          <Alert
            message="Não foi possível concluir a operação"
            type="error"
            description={<>{attachmentError(error)}</>}
          />
        ) : null}
        <div className="attachments-panel__upload">
          <label htmlFor={inputID}>Adicionar arquivo</label>
          <input
            id={inputID}
            accept={acceptedMIMEs}
            disabled={pending}
            type="file"
            onChange={(event) => {
              selectFile(event.currentTarget.files?.[0]);
              event.currentTarget.value = "";
            }}
          />
          <small>PDF, imagens, texto/CSV, DOCX, XLSX, áudio e vídeo. Limite: 50 MB.</small>
          {progress !== null ? (
            <div aria-live="polite" className="attachments-panel__progress">
              <progress max={100} value={progress} />
              <span>{progress < 100 ? `Enviando ${progress}%` : "Verificando arquivo"}</span>
            </div>
          ) : null}
        </div>
        {query.isLoading ? <p aria-live="polite">Carregando anexos...</p> : null}
        {!query.isLoading && (query.data?.length ?? 0) === 0 ? (
          <p className="attachments-panel__empty">
            {showTrash ? "Nenhum anexo ativo ou recuperável." : "Nenhum anexo ativo."}
          </p>
        ) : null}
        <div className="attachments-panel__list">
          {query.data?.map((value) => (
            <AttachmentCard
              key={value.id}
              pending={pending}
              value={value}
              onDownload={() => download.mutate(value)}
              onRestore={() => restore.mutate(value)}
              onTrash={() => trash.mutate(value)}
            />
          ))}
        </div>
      </Flex>
    </Card>
  );
}

function AttachmentCard({
  value,
  pending,
  onDownload,
  onTrash,
  onRestore,
}: {
  value: AttachmentRecord;
  pending: boolean;
  onDownload: () => void;
  onTrash: () => void;
  onRestore: () => void;
}) {
  return (
    <article className="attachment-card">
      <div className="attachment-card__metadata">
        <strong>{value.original_filename}</strong>
        <span>
          {formatBytes(value.byte_size)} · {value.detected_mime}
        </span>
        <span>Adicionado em {formatDate(value.created_at)}</span>
        {value.purge_after ? (
          <span>Exclusão definitiva em {formatDate(value.purge_after)}</span>
        ) : null}
      </div>
      <Flex align="center" className="attachment-card__actions">
        <Tag color={value.lifecycle_state === "ACTIVE" ? "success" : "warning"}>
          {value.lifecycle_state === "ACTIVE" ? "Ativo" : "Na lixeira"}
        </Tag>
        {value.lifecycle_state === "ACTIVE" ? (
          <>
            <Button disabled={pending} onClick={onDownload}>
              Baixar
            </Button>
            <Button disabled={pending} onClick={onTrash}>
              Mover para lixeira
            </Button>
          </>
        ) : (
          <Button disabled={pending} onClick={onRestore}>
            Restaurar
          </Button>
        )}
      </Flex>
    </article>
  );
}

function attachmentError(error: unknown): string {
  if (error instanceof APIRequestError) {
    if (error.status === 409)
      return "O anexo foi alterado por outra pessoa. Atualize e tente novamente.";
    if (error.status === 410) return "O envio expirou. Selecione o arquivo novamente.";
    if (error.status === 503) return "O armazenamento privado está indisponível no momento.";
    return error.message;
  }
  return error instanceof Error ? error.message : "Erro inesperado ao processar o anexo.";
}

function formatBytes(value: number): string {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "short" }).format(
    new Date(value),
  );
}

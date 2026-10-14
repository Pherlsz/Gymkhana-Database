import { Button, Card, Checkbox, Flex, Tag } from "antd";
import { InlineStatus } from "./components/InlineStatus";
import { StatusBanner } from "./components/StatusBanner";
import "./attachments.css";
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
import { queryKeys } from "./lib/api/queryKeys";
import { formatBytes, formatDateTime as formatDate } from "./lib/formatters";
import { useI18n, type AppMessages } from "./i18n";

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
  title,
  description,
}: {
  owner: AttachmentOwner;
  title?: string;
  description?: string;
}) {
  const { messages, t } = useI18n();
  const copy = messages.attachments;
  const queryClient = useQueryClient();
  const inputID = useId();
  const [showTrash, setShowTrash] = useState(false);
  const [progress, setProgress] = useState<number | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const queryKey = queryKeys.attachments.byOwner(owner, showTrash);
  const query = useQuery({
    queryKey,
    queryFn: ({ signal }) => listAttachments(owner, showTrash, signal),
  });
  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: queryKeys.attachments.byOwner(owner) });
  };
  const upload = useMutation({
    mutationFn: (file: File) => uploadAttachment(owner, file, setProgress),
    onMutate: () => {
      setProgress(0);
      setNotice(null);
    },
    onSuccess: async () => {
      setNotice(copy.uploaded);
      await refresh();
    },
    onSettled: () => setProgress(null),
  });
  const download = useMutation({ mutationFn: downloadAttachment });
  const trash = useMutation({
    mutationFn: trashAttachment,
    onSuccess: async () => {
      setNotice(copy.trashed);
      await refresh();
    },
  });
  const restore = useMutation({
    mutationFn: restoreAttachment,
    onSuccess: async () => {
      setNotice(copy.restored);
      await refresh();
    },
  });
  const error = query.error ?? upload.error ?? download.error ?? trash.error ?? restore.error;
  const pending = upload.isPending || download.isPending || trash.isPending || restore.isPending;

  const selectFile = (file: File | undefined) => {
    if (!file) return;
    setNotice(null);
    if (file.size <= 0) {
      setNotice(copy.emptyFile);
      return;
    }
    if (file.size > maximumClientBytes) {
      setNotice(copy.tooLarge);
      return;
    }
    if (!acceptedMIMEs.split(",").includes(file.type)) {
      setNotice(copy.unsupportedType);
      return;
    }
    upload.mutate(file);
  };

  return (
    <Card className="attachments-panel">
      <Flex vertical gap="1rem">
        <div className="attachments-panel__header">
          <div>
            <h3>{title ?? copy.title}</h3>
            <p>{description ?? copy.description}</p>
          </div>
          <Checkbox
            checked={showTrash}
            className="attachments-panel__trash-toggle"
            onChange={(event) => setShowTrash(event.target.checked)}
          >
            {copy.showTrash}
          </Checkbox>
        </div>
        {notice ? (
          <StatusBanner description={notice} title={copy.noticeTitle} tone="success" />
        ) : null}
        {error ? (
          <StatusBanner
            description={attachmentError(error, copy)}
            title={copy.operationError}
            tone="error"
          />
        ) : null}
        <div className="attachments-panel__upload">
          <label htmlFor={inputID}>{copy.addFile}</label>
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
          <small>{copy.acceptedHint}</small>
          {progress !== null ? (
            <div aria-live="polite" className="attachments-panel__progress">
              <progress max={100} value={progress} />
              <span>{progress < 100 ? t(copy.sending, { progress }) : copy.verifying}</span>
            </div>
          ) : null}
        </div>
        {query.isLoading ? <InlineStatus kind="loading" label={copy.loading} /> : null}
        {!query.isLoading && (query.data?.length ?? 0) === 0 ? (
          <InlineStatus kind="empty" label={showTrash ? copy.emptyWithTrash : copy.empty} />
        ) : null}
        <div className="attachments-panel__list">
          {query.data?.map((value) => (
            <AttachmentCard
              key={value.id}
              copy={copy}
              pending={pending}
              t={t}
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
  copy,
  t,
  onDownload,
  onTrash,
  onRestore,
}: {
  value: AttachmentRecord;
  pending: boolean;
  copy: AppMessages["attachments"];
  t: (template: string, params?: Record<string, string | number>) => string;
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
        <span>{t(copy.addedAt, { date: formatDate(value.created_at) })}</span>
        {value.purge_after ? (
          <span>{t(copy.purgeAt, { date: formatDate(value.purge_after) })}</span>
        ) : null}
      </div>
      <Flex align="center" className="attachment-card__actions">
        <Tag color={value.lifecycle_state === "ACTIVE" ? "success" : "warning"}>
          {value.lifecycle_state === "ACTIVE" ? copy.active : copy.inTrash}
        </Tag>
        {value.lifecycle_state === "ACTIVE" ? (
          <>
            <Button disabled={pending} onClick={onDownload}>
              {copy.download}
            </Button>
            <Button disabled={pending} onClick={onTrash}>
              {copy.moveToTrash}
            </Button>
          </>
        ) : (
          <Button disabled={pending} onClick={onRestore}>
            {copy.restore}
          </Button>
        )}
      </Flex>
    </article>
  );
}

function attachmentError(
  error: unknown,
  copy: { conflict: string; expired: string; storageUnavailable: string; unexpected: string },
): string {
  if (error instanceof APIRequestError) {
    if (error.status === 409) return copy.conflict;
    if (error.status === 410) return copy.expired;
    if (error.status === 503) return copy.storageUnavailable;
    return error.message;
  }
  return error instanceof Error ? error.message : copy.unexpected;
}

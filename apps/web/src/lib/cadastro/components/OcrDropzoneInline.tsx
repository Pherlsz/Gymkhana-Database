import { Check, FileUp, Paperclip, Sparkles } from "lucide-react";
import { type ChangeEvent, type DragEvent, useState } from "react";

export interface OcrDropzoneProps {
  inputId: string;
  label: string;
  onFile: (event: ChangeEvent<HTMLInputElement>) => void;
  variant?: "inline" | "banner";
  title?: string;
  description?: string;
  badgeText?: string;
  actionText?: string;
  disabled?: boolean;
  queuedName?: string | undefined;
  accept?: string;
}

export function OcrDropzoneInline({
  inputId,
  label,
  onFile,
  variant = "inline",
  title,
  description,
  badgeText,
  actionText,
  disabled,
  queuedName,
  accept = "image/*,application/pdf",
}: OcrDropzoneProps) {
  const [isDragging, setIsDragging] = useState(false);
  const blocked = Boolean(disabled);

  const handleDragOver = (e: DragEvent<HTMLLabelElement>) => {
    e.preventDefault();
    e.stopPropagation();
    if (!blocked) setIsDragging(true);
  };

  const handleDragLeave = (e: DragEvent<HTMLLabelElement>) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
  };

  const handleDrop = (e: DragEvent<HTMLLabelElement>) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
    if (blocked) return;

    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      const input = document.getElementById(inputId) as HTMLInputElement | null;
      if (input) {
        input.files = e.dataTransfer.files;
        const event = new Event("change", { bubbles: true });
        input.dispatchEvent(event);
      }
    }
  };

  const queued = queuedName ? `${label} · ${queuedName}` : label;

  if (variant === "banner") {
    return (
      <label
        className={`dropzone-banner ${isDragging ? "is-dragging" : ""} ${blocked ? "dropzone-banner--disabled" : ""} ${queuedName ? "dropzone-banner--queued" : ""}`}
        htmlFor={blocked ? undefined : inputId}
        onDragLeave={handleDragLeave}
        onDragOver={handleDragOver}
        onDrop={handleDrop}
      >
        <div className="dropzone-banner__lead">
          <div className="dropzone-banner__icon">
            {queuedName ? (
              <Check size={20} strokeWidth={2.2} />
            ) : (
              <Sparkles size={20} strokeWidth={1.8} />
            )}
          </div>
          <div className="dropzone-banner__texts">
            <div className="dropzone-banner__title">
              <span>{title ?? queued}</span>
              {badgeText ? <span className="dropzone-banner__chip">{badgeText}</span> : null}
            </div>
            {description ? <p className="dropzone-banner__desc">{description}</p> : null}
            {queuedName ? <p className="dropzone-banner__queued">{queuedName}</p> : null}
          </div>
        </div>

        {actionText ? (
          <div className="dropzone-banner__action">
            <FileUp size={15} strokeWidth={1.75} />
            <span>{actionText}</span>
          </div>
        ) : null}

        {blocked ? null : (
          <input
            accept={accept}
            id={inputId}
            style={{ display: "none" }}
            type="file"
            onChange={onFile}
          />
        )}
      </label>
    );
  }

  return (
    <label
      className={`dropzone-inline ${isDragging ? "is-dragging" : ""} ${blocked ? "dropzone-inline--disabled" : ""}`}
      htmlFor={blocked ? undefined : inputId}
      onDragLeave={handleDragLeave}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
    >
      <Paperclip aria-hidden size={15} />
      <span>{queued}</span>
      {blocked ? null : (
        <input
          accept={accept}
          id={inputId}
          style={{ display: "none" }}
          type="file"
          onChange={onFile}
        />
      )}
    </label>
  );
}

import { FileUp, Paperclip, Sparkles } from "lucide-react";
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
}

/**
 * OCR Dropzone component used across document and bill forms.
 * Supports both an inline link-style trigger and a rich banner-style dropzone card.
 */
export function OcrDropzoneInline({
  inputId,
  label,
  onFile,
  variant = "inline",
  title,
  description,
  badgeText,
  actionText,
}: OcrDropzoneProps) {
  const [isDragging, setIsDragging] = useState(false);

  const handleDragOver = (e: DragEvent<HTMLLabelElement>) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(true);
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

    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      const input = document.getElementById(inputId) as HTMLInputElement | null;
      if (input) {
        input.files = e.dataTransfer.files;
        const event = new Event("change", { bubbles: true });
        input.dispatchEvent(event);
      }
    }
  };

  if (variant === "banner") {
    return (
      <label
        className={`dropzone-banner ${isDragging ? "is-dragging" : ""}`}
        htmlFor={inputId}
        onDragLeave={handleDragLeave}
        onDragOver={handleDragOver}
        onDrop={handleDrop}
      >
        <div className="dropzone-banner__lead">
          <div className="dropzone-banner__icon">
            <Sparkles size={20} strokeWidth={1.8} />
          </div>
          <div className="dropzone-banner__texts">
            <div className="dropzone-banner__title">
              <span>{title ?? label}</span>
              {badgeText ? <span className="dropzone-banner__chip">{badgeText}</span> : null}
            </div>
            {description ? <p className="dropzone-banner__desc">{description}</p> : null}
          </div>
        </div>

        {actionText ? (
          <div className="dropzone-banner__action">
            <FileUp size={15} strokeWidth={1.75} />
            <span>{actionText}</span>
          </div>
        ) : null}

        <input
          accept="image/*,application/pdf"
          id={inputId}
          style={{ display: "none" }}
          type="file"
          onChange={onFile}
        />
      </label>
    );
  }

  return (
    <label
      className={`dropzone-inline ${isDragging ? "is-dragging" : ""}`}
      htmlFor={inputId}
      onDragLeave={handleDragLeave}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
    >
      <Paperclip aria-hidden size={15} />
      <span>{label}</span>
      <input
        accept="image/*,application/pdf"
        id={inputId}
        style={{ display: "none" }}
        type="file"
        onChange={onFile}
      />
    </label>
  );
}

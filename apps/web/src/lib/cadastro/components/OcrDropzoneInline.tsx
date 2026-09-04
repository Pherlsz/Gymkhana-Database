import { Paperclip } from "lucide-react";
import type { ChangeEvent } from "react";

/**
 * Inline OCR dropzone button used below document and bill forms.
 * Renders a styled label+input pair that triggers a file picker for
 * images and PDFs. Three copies of this block were previously duplicated
 * across DocumentFormFields, BillFormFields and CadastroSingleScreen.
 */
export function OcrDropzoneInline({
  inputId,
  label,
  onFile,
}: {
  inputId: string;
  label: string;
  onFile: (event: ChangeEvent<HTMLInputElement>) => void;
}) {
  return (
    <label className="dropzone-inline" htmlFor={inputId}>
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

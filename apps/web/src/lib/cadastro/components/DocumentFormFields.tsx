import { DatePicker, Input, Segmented, Select } from "antd";
import dayjs from "dayjs";
import type { ChangeEvent } from "react";
import type { DocumentType } from "../../api/client";
import { OcrDropzoneInline } from "./OcrDropzoneInline";

export interface DocumentFormFieldsState {
  docTypeId: string;
  docIdentifier: string;
  docDate?: string | undefined;
  docValidUntil?: string | undefined;
  docMedium: "PHYSICAL" | "DIGITAL";
  docCustody: "ORGANIZATION" | "OWNER";
  docNotes: string;
}

export interface DocumentFormFieldsProps {
  state: DocumentFormFieldsState;
  onChange: (patch: Partial<DocumentFormFieldsState>) => void;
  documentTypes: DocumentType[];
  onFileDrop: (event: ChangeEvent<HTMLInputElement>) => void;
  fileInputId: string;
  copy: {
    fieldDocType: string;
    fieldDocNumber: string;
    placeholderDocNumber: string;
    fieldDocDate: string;
    fieldDocValidUntil: string;
    fieldDocMedium: string;
    tagPhysical: string;
    tagDigital: string;
    fieldDocCustody: string;
    fieldDocCustodyOrg: string;
    fieldDocCustodyOwner: string;
    fieldDocNotes: string;
    placeholderDocNotes: string;
    ocrInlineDropzoneDoc: string;
  };
}

export function DocumentFormFields({
  state,
  onChange,
  documentTypes,
  onFileDrop,
  fileInputId,
  copy,
}: DocumentFormFieldsProps) {
  return (
    <div className="cadastro-document-fields">
      <div className="cadastro-grid">
        {/* Tipo de Documento */}
        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-doc-type">
              {copy.fieldDocType} <span className="cadastro-field__required">*</span>
            </label>
            <Select
              id="cad-doc-type"
              options={documentTypes.map((t) => ({ label: t.label, value: t.id }))}
              style={{ width: "100%" }}
              value={state.docTypeId || undefined}
              onChange={(v) => onChange({ docTypeId: v ?? "" })}
            />
          </div>
        </div>

        {/* Número / Identificador */}
        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-doc-identifier">
              {copy.fieldDocNumber} <span className="cadastro-field__required">*</span>
            </label>
            <Input
              id="cad-doc-identifier"
              placeholder={copy.placeholderDocNumber}
              value={state.docIdentifier}
              onChange={(e) => onChange({ docIdentifier: e.target.value })}
            />
          </div>
        </div>

        {/* Data de Emissão */}
        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-doc-date">
              {copy.fieldDocDate}
            </label>
            <DatePicker
              format="DD/MM/YYYY"
              id="cad-doc-date"
              style={{ width: "100%" }}
              value={state.docDate ? dayjs(state.docDate) : null}
              onChange={(d) => onChange({ docDate: d ? d.format("YYYY-MM-DD") : undefined })}
            />
          </div>
        </div>

        {/* Data de Validade */}
        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-doc-valid">
              {copy.fieldDocValidUntil}
            </label>
            <DatePicker
              format="DD/MM/YYYY"
              id="cad-doc-valid"
              style={{ width: "100%" }}
              value={state.docValidUntil ? dayjs(state.docValidUntil) : null}
              onChange={(d) =>
                onChange({ docValidUntil: d ? d.format("YYYY-MM-DD") : undefined })
              }
            />
          </div>
        </div>

        {/* Meio (Físico / Digital) */}
        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label">{copy.fieldDocMedium}</label>
            <Segmented
              block
              options={[
                { label: copy.tagPhysical, value: "PHYSICAL" },
                { label: copy.tagDigital, value: "DIGITAL" },
              ]}
              value={state.docMedium}
              onChange={(v) => onChange({ docMedium: v as "PHYSICAL" | "DIGITAL" })}
            />
          </div>
        </div>

        {/* Custódia (se físico) */}
        {state.docMedium === "PHYSICAL" && (
          <div className="cadastro-col-6">
            <div className="cadastro-field">
              <label className="cadastro-field__label">{copy.fieldDocCustody}</label>
              <Select
                options={[
                  { label: copy.fieldDocCustodyOrg, value: "ORGANIZATION" },
                  { label: copy.fieldDocCustodyOwner, value: "OWNER" },
                ]}
                style={{ width: "100%" }}
                value={state.docCustody}
                onChange={(v) => onChange({ docCustody: v as "ORGANIZATION" | "OWNER" })}
              />
            </div>
          </div>
        )}

        {/* Observações */}
        <div className="cadastro-col-12">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-doc-notes">
              {copy.fieldDocNotes}
            </label>
            <Input.TextArea
              id="cad-doc-notes"
              placeholder={copy.placeholderDocNotes}
              rows={2}
              value={state.docNotes}
              onChange={(e) => onChange({ docNotes: e.target.value })}
            />
          </div>
        </div>
      </div>

      {/* Inline OCR Dropzone */}
      <OcrDropzoneInline
        inputId={fileInputId}
        label={copy.ocrInlineDropzoneDoc}
        onFile={onFileDrop}
      />
    </div>
  );
}

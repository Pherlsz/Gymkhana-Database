import { DatePicker, Input, Segmented, Select } from "antd";
import dayjs from "dayjs";
import { Cloud } from "lucide-react";
import type { ChangeEvent } from "react";
import { useI18n } from "../../../i18n";
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
  copy?: Record<string, string>;
}

export function DocumentFormFields({
  state,
  onChange,
  documentTypes,
  onFileDrop,
  fileInputId,
  copy: customCopy,
}: DocumentFormFieldsProps) {
  const { messages } = useI18n();
  const copy = { ...messages.tables.cadastro, ...customCopy };
  return (
    <div className="cadastro-document-fields">
      {/* Smart OCR Dropzone at the top for rapid document ingestion */}
      <OcrDropzoneInline
        actionText={copy.ocrBannerAction}
        badgeText={copy.ocrBannerBadge}
        description={copy.ocrBannerDesc}
        inputId={fileInputId}
        label={copy.ocrInlineDropzoneDoc}
        title={copy.ocrBannerTitle}
        variant="banner"
        onFile={onFileDrop}
      />

      <div className="cadastro-grid">
        {/* Row 1: Tipo de Documento e Número */}
        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-doc-type">
              {copy.fieldDocType} <span className="cadastro-field__required">*</span>
            </label>
            <Select
              id="cad-doc-type"
              options={documentTypes.map((t) => ({ label: t.label, value: t.id }))}
              placeholder={copy.pickerSelectType}
              style={{ width: "100%" }}
              value={state.docTypeId || undefined}
              onChange={(v) => onChange({ docTypeId: v ?? "" })}
            />
          </div>
        </div>

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

        {/* Row 2: Datas (Emissão e Validade) */}
        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-doc-date">
              {copy.fieldDocDate}
            </label>
            <DatePicker
              format="DD/MM/YYYY"
              id="cad-doc-date"
              placeholder={copy.placeholderDate}
              style={{ width: "100%" }}
              value={state.docDate ? dayjs(state.docDate) : null}
              onChange={(d) => onChange({ docDate: d ? d.format("YYYY-MM-DD") : undefined })}
            />
          </div>
        </div>

        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-doc-valid">
              {copy.fieldDocValidUntil}
            </label>
            <DatePicker
              format="DD/MM/YYYY"
              id="cad-doc-valid"
              placeholder={copy.placeholderDate}
              style={{ width: "100%" }}
              value={state.docValidUntil ? dayjs(state.docValidUntil) : null}
              onChange={(d) => onChange({ docValidUntil: d ? d.format("YYYY-MM-DD") : undefined })}
            />
          </div>
        </div>

        {/* Row 3: Meio e Guarda / Custódia */}
        <div className="cadastro-col-6">
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

        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label">{copy.fieldDocCustody}</label>
            {state.docMedium === "PHYSICAL" ? (
              <Select
                options={[
                  { label: copy.fieldDocCustodyOrg, value: "ORGANIZATION" },
                  { label: copy.fieldDocCustodyOwner, value: "OWNER" },
                ]}
                style={{ width: "100%" }}
                value={state.docCustody}
                onChange={(v) => onChange({ docCustody: v as "ORGANIZATION" | "OWNER" })}
              />
            ) : (
              <div className="cadastro-field__static-info">
                <Cloud size={14} />
                <span>{copy.fieldDocDigitalStorageNotice}</span>
              </div>
            )}
          </div>
        </div>

        {/* Row 4: Observações */}
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
    </div>
  );
}

import { DatePicker, Input, Segmented, Select } from "antd";
import dayjs from "dayjs";
import { Cloud } from "lucide-react";
import type { ChangeEvent } from "react";
import { RecordCustomFieldsSection, type CustomDraftValue } from "../../../RecordCustomFields";
import { useI18n } from "../../../i18n";
import type { DocumentType } from "../../api/client";
import { fileFromInput } from "../cadastroValidate";
import { OcrDropzoneInline } from "./OcrDropzoneInline";

export interface DocumentFormFieldsState {
  docTypeId: string;
  docIdentifier: string;
  docDate?: string | undefined;
  docValidUntil?: string | undefined;
  docMedium: "PHYSICAL" | "DIGITAL";
  docCustody: "ORGANIZATION" | "OWNER";
  docNotes: string;
  customDraft: Record<string, CustomDraftValue>;
  file?: File | undefined;
}

export interface DocumentFormFieldsProps {
  state: DocumentFormFieldsState;
  onChange: (patch: Partial<DocumentFormFieldsState>) => void;
  documentTypes: DocumentType[];
  fileInputId: string;
  lockType?: boolean | undefined;
  attachmentsEnabled?: boolean | undefined;
  disabled?: boolean | undefined;
  showDropzone?: boolean | undefined;
  copy?: Record<string, string>;
}

export function DocumentFormFields({
  state,
  onChange,
  documentTypes,
  fileInputId,
  lockType,
  attachmentsEnabled,
  disabled,
  showDropzone = true,
  copy: customCopy,
}: DocumentFormFieldsProps) {
  const { messages } = useI18n();
  const labels = messages.common.labels;
  const copy = { ...messages.tables.cadastro, ...customCopy };
  const isDisabled = Boolean(disabled);
  const dropEnabled = attachmentsEnabled !== false;

  const handleFile = (event: ChangeEvent<HTMLInputElement>) => {
    const file = fileFromInput(event);
    if (file) onChange({ file, docMedium: "DIGITAL" });
    event.target.value = "";
  };

  return (
    <div className="cadastro-document-fields">
      {showDropzone ? (
        <OcrDropzoneInline
          actionText={copy.ocrBannerAction}
          badgeText={copy.ocrBannerBadge}
          description={copy.ocrBannerDesc}
          disabled={!dropEnabled || isDisabled}
          inputId={fileInputId}
          label={copy.ocrInlineDropzoneDoc}
          queuedName={state.file?.name}
          title={copy.ocrBannerTitle}
          variant="banner"
          onFile={handleFile}
        />
      ) : null}

      <div className="cadastro-grid">
        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-doc-type">
              {copy.fieldDocType} <span className="cadastro-field__required">*</span>
            </label>
            <Select
              disabled={isDisabled || Boolean(lockType)}
              id="cad-doc-type"
              options={documentTypes.map((t) => ({ label: t.label, value: t.id }))}
              placeholder={copy.pickerSelectType}
              style={{ width: "100%" }}
              value={state.docTypeId || undefined}
              onChange={(v) => onChange({ docTypeId: v ?? "", customDraft: {} })}
            />
          </div>
        </div>

        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-doc-identifier">
              {copy.fieldDocNumber} <span className="cadastro-field__required">*</span>
            </label>
            <Input
              disabled={isDisabled}
              id="cad-doc-identifier"
              placeholder={copy.placeholderDocNumber}
              value={state.docIdentifier}
              onChange={(e) => onChange({ docIdentifier: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-doc-date">
              {copy.fieldDocDate}
            </label>
            <DatePicker
              disabled={isDisabled}
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
              {labels.validUntil}
            </label>
            <DatePicker
              disabled={isDisabled}
              format="DD/MM/YYYY"
              id="cad-doc-valid"
              placeholder={copy.placeholderDate}
              style={{ width: "100%" }}
              value={state.docValidUntil ? dayjs(state.docValidUntil) : null}
              onChange={(d) => onChange({ docValidUntil: d ? d.format("YYYY-MM-DD") : undefined })}
            />
          </div>
        </div>

        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label">{labels.medium}</label>
            <Segmented
              block
              disabled={isDisabled}
              options={[
                { label: labels.physical, value: "PHYSICAL" },
                { label: labels.digital, value: "DIGITAL" },
              ]}
              value={state.docMedium}
              onChange={(v) => onChange({ docMedium: v as "PHYSICAL" | "DIGITAL" })}
            />
          </div>
        </div>

        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label">{labels.custody}</label>
            {state.docMedium === "PHYSICAL" ? (
              <Select
                disabled={isDisabled}
                options={[
                  { label: labels.organization, value: "ORGANIZATION" },
                  { label: labels.owner, value: "OWNER" },
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

        <div className="cadastro-col-12">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-doc-notes">
              {copy.fieldDocNotes}
            </label>
            <Input.TextArea
              disabled={isDisabled}
              id="cad-doc-notes"
              placeholder={copy.placeholderDocNotes}
              rows={2}
              value={state.docNotes}
              onChange={(e) => onChange({ docNotes: e.target.value })}
            />
          </div>
        </div>
      </div>

      <RecordCustomFieldsSection
        definitionTargetId={state.docTypeId || undefined}
        definitionTargetKind="DOCUMENT_TYPE"
        disabled={isDisabled}
        draft={state.customDraft}
        onDraftChange={(customDraft) => onChange({ customDraft })}
      />
    </div>
  );
}

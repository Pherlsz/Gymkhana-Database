import { Button, DatePicker, Input, Segmented, Select } from "antd";
import { FileText, Plus, Trash2 } from "lucide-react";
import { type ChangeEvent, useId, useState } from "react";
import dayjs from "dayjs";
import { useI18n } from "../../../i18n";
import type { DocumentType } from "../../api/client";
import { INITIAL_INLINE_DOC, type InlineDocState, type PendingDoc } from "../types";
import { CadastroSection } from "./CadastroSection";
import { OcrDropzoneInline } from "./OcrDropzoneInline";

export interface PendingDocumentsSectionProps {
  documents: PendingDoc[];
  onAddDoc: (doc: PendingDoc) => void;
  onRemoveDoc: (id: string) => void;
  documentTypes: DocumentType[];
  defaultOpen?: boolean;
  open?: boolean;
  onToggleOpen?: () => void;
}

export function PendingDocumentsSection({
  documents,
  onAddDoc,
  onRemoveDoc,
  documentTypes,
  defaultOpen = true,
  open,
  onToggleOpen,
}: PendingDocumentsSectionProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;

  const [addingDoc, setAddingDoc] = useState(false);
  const [inlineDoc, setInlineDoc] = useState<InlineDocState>(INITIAL_INLINE_DOC);
  const inlineDocFileId = useId();

  const handleConfirmAdd = () => {
    const type = documentTypes.find((t) => t.id === inlineDoc.typeId);
    const typeName = type?.label || copy.docFallbackDefault;
    const newDoc: PendingDoc = {
      id: String(Date.now()),
      typeId: inlineDoc.typeId || documentTypes?.[0]?.id || "",
      typeName,
      number: inlineDoc.number.trim(),
      date: inlineDoc.date,
      validUntil: inlineDoc.validUntil,
      medium: inlineDoc.medium,
      custody: inlineDoc.custody,
      notes: inlineDoc.notes.trim(),
      tag: "manual",
    };
    onAddDoc(newDoc);
    setInlineDoc(INITIAL_INLINE_DOC);
    setAddingDoc(false);
  };

  const handleFileDrop = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const fallbackType = documentTypes?.[0];
    const newDoc: PendingDoc = {
      id: String(Date.now()),
      typeId: inlineDoc.typeId || fallbackType?.id || "",
      typeName: fallbackType?.label || copy.docFallbackDefault,
      number: "01234567890",
      notes: copy.ocrExtractedNote.replace("{filename}", file.name),
      medium: "DIGITAL",
      tag: "ocr",
    };
    onAddDoc(newDoc);
    setAddingDoc(false);
    e.target.value = "";
  };

  return (
    <CadastroSection
      badge={<span className="cadastro-group__badge">{documents.length}</span>}
      defaultOpen={defaultOpen}
      hint={copy.sectionDocumentsHint}
      icon={<FileText size={18} strokeWidth={1.75} />}
      onToggleOpen={onToggleOpen}
      open={open}
      title={copy.sectionDocumentsTitle}
    >
      <div className="strip">
        {documents.length === 0 ? (
          <div className="emptyrow">{copy.sectionDocumentsEmpty}</div>
        ) : (
          documents.map((doc) => (
            <div key={doc.id} className="srow">
              <span className="mk">
                <FileText size={16} strokeWidth={1.75} />
              </span>
              <div className="tx">
                <b>{doc.typeName}</b>
                <span>
                  {doc.number ? `nº ${doc.number}` : copy.docNumberEmpty}
                  {doc.notes ? ` · ${doc.notes}` : ""}
                </span>
              </div>
              <span className="grow" />
              <span className={`tag ${doc.tag === "ocr" ? "gold" : ""}`}>
                {doc.tag === "ocr" ? copy.tagOcr : copy.tagPhysical}
              </span>
              <div className="acts">
                <button type="button" onClick={() => onRemoveDoc(doc.id)}>
                  <Trash2 size={13} style={{ marginRight: 4 }} />
                  {copy.actionRemove}
                </button>
              </div>
            </div>
          ))
        )}
      </div>

      {addingDoc ? (
        <div className="inline-addform">
          <div className="cadastro-grid">
            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label">{copy.fieldDocType}</label>
                <Select
                  options={
                    documentTypes.map((t: DocumentType) => ({
                      value: t.id,
                      label: t.label,
                    })) ?? []
                  }
                  value={inlineDoc.typeId || documentTypes?.[0]?.id}
                  onChange={(v) => {
                    if (v) setInlineDoc((prev) => ({ ...prev, typeId: v }));
                  }}
                />
              </div>
            </div>
            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label">{copy.fieldDocNumber}</label>
                <Input
                  placeholder={copy.placeholderDocNumber}
                  value={inlineDoc.number}
                  onChange={(e) => setInlineDoc((prev) => ({ ...prev, number: e.target.value }))}
                />
              </div>
            </div>
            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label">{copy.fieldDocDate}</label>
                <DatePicker
                  format="DD/MM/YYYY"
                  placeholder={copy.placeholderDate}
                  style={{ width: "100%" }}
                  value={inlineDoc.date ? dayjs(inlineDoc.date) : null}
                  onChange={(d) =>
                    setInlineDoc((prev) => ({
                      ...prev,
                      date: d ? d.format("YYYY-MM-DD") : undefined,
                    }))
                  }
                />
              </div>
            </div>
            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label">{copy.fieldDocValidUntil}</label>
                <DatePicker
                  format="DD/MM/YYYY"
                  placeholder={copy.placeholderDate}
                  style={{ width: "100%" }}
                  value={inlineDoc.validUntil ? dayjs(inlineDoc.validUntil) : null}
                  onChange={(d) =>
                    setInlineDoc((prev) => ({
                      ...prev,
                      validUntil: d ? d.format("YYYY-MM-DD") : undefined,
                    }))
                  }
                />
              </div>
            </div>
            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label">{copy.fieldDocMedium}</label>
                <Segmented
                  block
                  options={[
                    { label: copy.tagPhysical, value: "PHYSICAL" },
                    { label: copy.tagDigital, value: "DIGITAL" },
                  ]}
                  value={inlineDoc.medium}
                  onChange={(v) =>
                    setInlineDoc((prev) => ({
                      ...prev,
                      medium: v as "PHYSICAL" | "DIGITAL",
                    }))
                  }
                />
              </div>
            </div>
            {inlineDoc.medium === "PHYSICAL" && (
              <div className="cadastro-col-4">
                <div className="cadastro-field">
                  <label className="cadastro-field__label">{copy.fieldDocCustody}</label>
                  <Select
                    options={[
                      { label: copy.fieldDocCustodyOrg, value: "ORGANIZATION" },
                      { label: copy.fieldDocCustodyOwner, value: "OWNER" },
                    ]}
                    value={inlineDoc.custody}
                    onChange={(v) =>
                      setInlineDoc((prev) => ({
                        ...prev,
                        custody: v as "ORGANIZATION" | "OWNER",
                      }))
                    }
                  />
                </div>
              </div>
            )}
            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label">{copy.fieldDocNotes}</label>
                <Input
                  placeholder={copy.placeholderDocNotes}
                  value={inlineDoc.notes}
                  onChange={(e) => setInlineDoc((prev) => ({ ...prev, notes: e.target.value }))}
                />
              </div>
            </div>
          </div>

          <OcrDropzoneInline
            inputId={inlineDocFileId}
            label={copy.ocrInlineDropzoneDoc}
            onFile={handleFileDrop}
          />

          <div className="inline-actions">
            <Button onClick={() => setAddingDoc(false)}>{copy.btnCancelAdd}</Button>
            <Button type="primary" onClick={handleConfirmAdd}>
              {copy.btnConfirmAdd}
            </Button>
          </div>
        </div>
      ) : (
        <button
          className="addrow"
          type="button"
          onClick={() => {
            setInlineDoc((prev) => ({
              ...prev,
              typeId: documentTypes?.[0]?.id || "",
            }));
            setAddingDoc(true);
          }}
        >
          <Plus size={15} strokeWidth={2} />
          <span>{copy.btnAddDocument}</span>
        </button>
      )}
    </CadastroSection>
  );
}

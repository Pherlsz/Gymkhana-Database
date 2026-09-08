import { FileText } from "lucide-react";
import { type ChangeEvent, useId, useState } from "react";
import { useI18n } from "../../../i18n";
import type { DocumentType } from "../../api/client";
import { INITIAL_DOC_FIELDS, type PendingDoc } from "../types";
import { CadastroStagedSection } from "./CadastroStagedSection";
import { DocumentFormFields, type DocumentFormFieldsState } from "./DocumentFormFields";

export interface PendingDocumentsSectionProps {
  documents: PendingDoc[];
  onAddDoc: (doc: PendingDoc) => void;
  onRemoveDoc: (id: string) => void;
  documentTypes: DocumentType[];
  defaultOpen?: boolean | undefined;
  open?: boolean | undefined;
  onToggleOpen?: (() => void) | undefined;
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
  const { messages, t } = useI18n();
  const copy = messages.tables.cadastro;
  const labels = messages.common.labels;
  const actions = messages.common.actions;
  const entities = messages.common.entities;

  const [addingDoc, setAddingDoc] = useState(false);
  const [docState, setDocState] = useState<DocumentFormFieldsState>(INITIAL_DOC_FIELDS);
  const inlineDocFileId = useId();

  const handleConfirmAdd = () => {
    const type = documentTypes.find((dt) => dt.id === docState.docTypeId);
    const typeName = type?.label || copy.docFallbackDefault;
    const newDoc: PendingDoc = {
      id: String(Date.now()),
      typeId: docState.docTypeId || documentTypes?.[0]?.id || "",
      typeName,
      number: docState.docIdentifier.trim(),
      date: docState.docDate,
      validUntil: docState.docValidUntil,
      medium: docState.docMedium,
      custody: docState.docCustody,
      notes: docState.docNotes.trim(),
      tag: "manual",
    };
    onAddDoc(newDoc);
    setDocState(INITIAL_DOC_FIELDS);
    setAddingDoc(false);
  };

  const handleFileDrop = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const fallbackType = documentTypes?.[0];
    const newDoc: PendingDoc = {
      id: String(Date.now()),
      typeId: docState.docTypeId || fallbackType?.id || "",
      typeName: fallbackType?.label || copy.docFallbackDefault,
      number: "01234567890",
      notes: t(copy.ocrExtractedNote, { filename: file.name }),
      medium: "DIGITAL",
      tag: "ocr",
    };
    onAddDoc(newDoc);
    setAddingDoc(false);
    setDocState(INITIAL_DOC_FIELDS);
    e.target.value = "";
  };

  return (
    <CadastroStagedSection
      addLabel={copy.btnAddDocument}
      adding={addingDoc}
      cancelLabel={actions.cancel}
      confirmLabel={actions.add}
      count={documents.length}
      defaultOpen={defaultOpen}
      emptyText={copy.sectionDocumentsEmpty}
      hint={copy.sectionDocumentsHint}
      icon={<FileText size={18} strokeWidth={1.75} />}
      items={documents.map((doc) => ({
        id: doc.id,
        title: doc.typeName,
        subtitle: doc.number
          ? `nº ${doc.number}${doc.notes ? ` · ${doc.notes}` : ""}`
          : doc.notes || copy.docNumberEmpty,
        tagText: doc.tag === "ocr" ? labels.ocr : labels.physical,
        isOcr: doc.tag === "ocr",
      }))}
      modifier="documents"
      onCancelAdd={() => {
        setAddingDoc(false);
        setDocState(INITIAL_DOC_FIELDS);
      }}
      onConfirmAdd={handleConfirmAdd}
      onRemoveItem={onRemoveDoc}
      onStartAdd={() => {
        setDocState({
          ...INITIAL_DOC_FIELDS,
          docTypeId: documentTypes?.[0]?.id || "",
        });
        setAddingDoc(true);
      }}
      onToggleOpen={onToggleOpen}
      open={open}
      removeLabel={actions.remove}
      title={entities.documents}
    >
      <DocumentFormFields
        documentTypes={documentTypes}
        fileInputId={inlineDocFileId}
        state={docState}
        onChange={(patch) => setDocState((prev) => ({ ...prev, ...patch }))}
        onFileDrop={handleFileDrop}
      />
    </CadastroStagedSection>
  );
}

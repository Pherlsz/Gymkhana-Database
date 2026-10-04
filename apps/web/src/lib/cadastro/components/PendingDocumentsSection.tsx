import { FileText } from "lucide-react";
import { useId, useState } from "react";
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
  attachmentsEnabled?: boolean | undefined;
  defaultOpen?: boolean | undefined;
  open?: boolean | undefined;
  onToggleOpen?: (() => void) | undefined;
}

export function PendingDocumentsSection({
  documents,
  onAddDoc,
  onRemoveDoc,
  documentTypes,
  attachmentsEnabled,
  defaultOpen = true,
  open,
  onToggleOpen,
}: PendingDocumentsSectionProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const labels = messages.common.labels;
  const actions = messages.common.actions;
  const entities = messages.common.entities;

  const [addingDoc, setAddingDoc] = useState(false);
  const [docState, setDocState] = useState<DocumentFormFieldsState>(INITIAL_DOC_FIELDS);
  const [formError, setFormError] = useState<string | null>(null);
  const inlineDocFileId = useId();

  const handleConfirmAdd = () => {
    const type = documentTypes.find((dt) => dt.id === docState.docTypeId);
    if (!type) {
      setFormError(copy.errorDocTypeRequired);
      return;
    }
    if (!docState.docIdentifier.trim()) {
      setFormError(copy.errorDocNumberRequired);
      return;
    }
    const newDoc: PendingDoc = {
      id: String(Date.now()),
      typeId: type.id,
      typeName: type.label,
      typeKey: type.technical_key,
      number: docState.docIdentifier.trim(),
      date: docState.docDate,
      validUntil: docState.docValidUntil,
      medium: docState.docMedium,
      custody: docState.docCustody,
      notes: docState.docNotes.trim(),
      tag: docState.file ? "queued" : "manual",
      customDraft: docState.customDraft,
      file: docState.file,
    };
    onAddDoc(newDoc);
    setDocState(INITIAL_DOC_FIELDS);
    setFormError(null);
    setAddingDoc(false);
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
      error={formError ?? undefined}
      hint={copy.sectionDocumentsHint}
      icon={<FileText size={18} strokeWidth={1.75} />}
      items={documents.map((doc) => ({
        id: doc.id,
        title: doc.typeName,
        subtitle: [
          doc.number ? `nº ${doc.number}` : doc.notes || copy.docNumberEmpty,
          doc.file?.name,
          doc.saveStatus === "saved" ? copy.itemSaved : "",
          doc.saveStatus === "error" ? copy.itemFailed : "",
        ]
          .filter(Boolean)
          .join(" · "),
        tagText: doc.tag === "queued" ? labels.ocr : labels.physical,
        isOcr: doc.tag === "queued",
      }))}
      modifier="documents"
      onCancelAdd={() => {
        setAddingDoc(false);
        setDocState(INITIAL_DOC_FIELDS);
        setFormError(null);
      }}
      onConfirmAdd={handleConfirmAdd}
      onRemoveItem={onRemoveDoc}
      onStartAdd={() => {
        setFormError(null);
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
        attachmentsEnabled={attachmentsEnabled}
        documentTypes={documentTypes}
        fileInputId={inlineDocFileId}
        state={docState}
        onChange={(patch) => setDocState((prev) => ({ ...prev, ...patch }))}
      />
    </CadastroStagedSection>
  );
}

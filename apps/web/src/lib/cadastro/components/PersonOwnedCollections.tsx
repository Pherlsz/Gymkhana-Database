import { Button } from "antd";
import { FileText, Zap } from "lucide-react";
import { useId, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useI18n } from "../../../i18n";
import {
  listBills,
  listDocuments,
  listDocumentTypes,
  type BillListSearch,
  type DocumentListSearch,
  type Profile,
} from "../../api/client";
import type { AttachmentOwner } from "../../api/attachments";
import { getOCRCapability } from "../../api/ocr";
import { queryKeys } from "../../api/queryKeys";
import { errorMessage } from "../../formatters";
import { persistDocumentForm, storedRecordOf } from "../cadastroPersist";
import { OcrReviewPanel } from "../OcrReviewPanel";
import { INITIAL_DOC_FIELDS } from "../types";
import { useAttachmentsEnabled } from "../useAttachmentsEnabled";
import { CadastroStagedSection } from "./CadastroStagedSection";
import { DocumentFormFields, type DocumentFormFieldsState } from "./DocumentFormFields";

export const OWNER_DOC_SEARCH: DocumentListSearch = {
  q: "",
  document_page: 1,
  document_limit: 50,
  document_sort: "identifier_value",
  document_order: "asc",
  document_identifier: "",
  document_status: "",
  document_medium: "",
  document_type: "",
};

export const OWNER_BILL_SEARCH: BillListSearch = {
  q: "",
  bill_page: 1,
  bill_limit: 50,
  bill_sort: "reference_value",
  bill_order: "asc",
  bill_reference: "",
  bill_competence: "",
  bill_status: "",
  bill_medium: "",
  bill_type: "",
};

export function PersonOwnedCollections({
  profile,
  editable,
  onOpenDocument,
  onOpenBill,
}: {
  profile: Profile;
  editable: boolean;
  onOpenDocument?: ((id: string) => void) | undefined;
  onOpenBill?: ((id: string) => void) | undefined;
}) {
  const { messages } = useI18n();
  const cadastro = messages.tables.cadastro;
  const inspector = messages.tables.inspector;
  const labels = messages.common.labels;
  const entities = messages.common.entities;
  const actions = messages.common.actions;
  const queryClient = useQueryClient();
  const fileInputId = useId();
  const attachmentsEnabled = useAttachmentsEnabled();
  const dropEnabled = attachmentsEnabled.data !== false;

  const [addingDoc, setAddingDoc] = useState(false);
  const [addingSaving, setAddingSaving] = useState(false);
  const [docState, setDocState] = useState<DocumentFormFieldsState>(INITIAL_DOC_FIELDS);
  const [formError, setFormError] = useState<string | null>(null);
  const [storedDocument, setStoredDocument] = useState<{ id: string; version: number } | null>(null);
  const [ocrOwner, setOcrOwner] = useState<AttachmentOwner | null>(null);

  const documentTypes = useQuery({
    queryKey: queryKeys.types.documents,
    queryFn: ({ signal }) => listDocumentTypes(signal),
  });
  const ocrCapability = useQuery({
    queryKey: queryKeys.ocr.capability,
    queryFn: ({ signal }) => getOCRCapability(signal),
  });
  const documentsQuery = useQuery({
    queryKey: queryKeys.records.documents(profile.id, OWNER_DOC_SEARCH),
    queryFn: ({ signal }) => listDocuments(profile.id, OWNER_DOC_SEARCH, signal),
  });
  const billsQuery = useQuery({
    queryKey: queryKeys.records.bills(profile.id, OWNER_BILL_SEARCH),
    queryFn: ({ signal }) => listBills(profile.id, OWNER_BILL_SEARCH, signal),
  });

  const documents = documentsQuery.data?.documents ?? [];
  const bills = billsQuery.data?.bills ?? [];
  const activeTypes = (documentTypes.data?.types ?? []).filter((type) => type.active);
  const resetAddForm = () => {
    setDocState(INITIAL_DOC_FIELDS);
    setAddingDoc(false);
    setFormError(null);
    setStoredDocument(null);
  };

  const handleConfirmAdd = async () => {
    if (addingSaving) return;
    const type = activeTypes.find((item) => item.id === docState.docTypeId);
    if (!type) {
      setFormError(cadastro.errorDocTypeRequired);
      return;
    }
    if (!docState.docIdentifier.trim()) {
      setFormError(cadastro.errorDocNumberRequired);
      return;
    }
    setAddingSaving(true);
    setFormError(null);
    try {
      const saved = await persistDocumentForm({
        ownerProfileId: profile.id,
        fields: { ...docState, docTypeId: type.id },
        ...(storedDocument ? { existing: storedDocument } : {}),
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.records.documents() }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tables.documents() }),
        queryClient.invalidateQueries({ queryKey: queryKeys.profiles.detail(profile.id) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tables.profiles() }),
        queryClient.invalidateQueries({
          queryKey: queryKeys.attachments.byOwner({
            owner_kind: "DOCUMENT",
            owner_id: saved.record.id,
          }),
        }),
      ]);
      resetAddForm();
      if (saved.ocrOwner && ocrCapability.data?.enabled) {
        setOcrOwner(saved.ocrOwner);
      }
    } catch (caught) {
      const stored = storedRecordOf(caught);
      if (stored) setStoredDocument(stored);
      setFormError(errorMessage(caught) || messages.common.labels.saveError);
    } finally {
      setAddingSaving(false);
    }
  };

  return (
    <>
      <CadastroStagedSection
        addLabel={cadastro.btnAddDocument}
        adding={addingDoc}
        allowAdd={editable}
        cancelLabel={actions.cancel}
        confirmLabel={actions.add}
        count={documents.length}
        defaultOpen
        emptyText={documentsQuery.isLoading ? inspector.loading : cadastro.sectionDocumentsEmpty}
        error={formError ?? undefined}
        hint={cadastro.sectionDocumentsHint}
        icon={<FileText size={18} strokeWidth={1.75} />}
        items={documents.map((document) => ({
          id: document.id,
          title: document.type.label,
          subtitle: document.identifier_value
            ? `nº ${document.identifier_value}`
            : cadastro.docNumberEmpty,
          tagText: document.medium === "DIGITAL" ? labels.digital : labels.physical,
        }))}
        modifier="documents"
        onCancelAdd={resetAddForm}
        onConfirmAdd={() => void handleConfirmAdd()}
        onOpenItem={onOpenDocument}
        onStartAdd={() => {
          setOcrOwner(null);
          setFormError(null);
          setDocState({
            ...INITIAL_DOC_FIELDS,
            docTypeId: activeTypes[0]?.id || "",
          });
          setAddingDoc(true);
        }}
        title={entities.documents}
      >
        <DocumentFormFields
          attachmentsEnabled={dropEnabled}
          documentTypes={activeTypes}
          fileInputId={fileInputId}
          state={docState}
          onChange={(patch) => setDocState((current) => ({ ...current, ...patch }))}
        />
      </CadastroStagedSection>

      {ocrOwner ? (
        <div className="cadastro-ocr cadastro-ocr__group">
          <p className="cadastro-review__lead">{cadastro.reviewDone}</p>
          <OcrReviewPanel owner={ocrOwner} />
          <Button onClick={() => setOcrOwner(null)}>{actions.close}</Button>
        </div>
      ) : null}

      <CadastroStagedSection
        allowAdd={false}
        count={bills.length}
        defaultOpen
        emptyText={billsQuery.isLoading ? inspector.loading : cadastro.sectionBillsEmpty}
        hint={cadastro.sectionBillsHint}
        icon={<Zap size={18} strokeWidth={1.75} />}
        items={bills.map((bill) => ({
          id: bill.id,
          title: bill.type.label,
          subtitle: [
            bill.reference_value
              ? `${cadastro.fieldBillInstallation} ${bill.reference_value}`
              : cadastro.docNumberEmpty,
            bill.amount ? `R$ ${bill.amount.replace(".", ",")}` : "",
          ]
            .filter(Boolean)
            .join(" · "),
          tagText: cadastro.tagActive,
        }))}
        modifier="bills"
        onOpenItem={onOpenBill}
        title={cadastro.sectionBillsTitle}
      />
    </>
  );
}

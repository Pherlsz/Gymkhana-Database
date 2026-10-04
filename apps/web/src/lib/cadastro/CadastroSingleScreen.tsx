import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useId, useMemo, useState } from "react";
import { StatusBanner } from "../../components/StatusBanner";
import { useI18n } from "../../i18n";
import type { AttachmentOwner } from "../api/attachments";
import { listBillTypes, listDocumentTypes, listProfilesLookup, type Profile } from "../api/client";
import { getOCRCapability } from "../api/ocr";
import { errorMessage } from "../formatters";
import { queryKeys } from "../api/queryKeys";
import { MINIMUM_REQUIREMENT_QUERY } from "./cadastroMinimumRequirement";
import { announceSaved } from "./cadastroFeedback";
import { buildProfilePayload, mapProfileToState } from "./cadastroPayloads";
import {
  persistBillForm,
  persistPendingBill,
  persistPendingDocument,
  persistDocumentForm,
  resolveOrCreateProfile,
} from "./cadastroPersist";
import {
  cpfDigits,
  isCompetenceMonth,
  isCompleteCpf,
  matchesValidationRegex,
} from "./cadastroValidate";
import { CadastroStickyBar } from "./components/CadastroStickyBar";
import { CadastroWorkShell } from "./CadastroWork";
import { BillMode } from "./modes/BillMode";
import { DocumentMode } from "./modes/DocumentMode";
import { PersonMode } from "./modes/PersonMode";
import { OcrReviewPanel } from "./OcrReviewPanel";
import {
  type CadastroSingleScreenProps,
  INITIAL_BILL_FIELDS,
  INITIAL_COMPLEMENTARY,
  INITIAL_DEMOGRAPHICS,
  INITIAL_DOC_FIELDS,
  INITIAL_FAMILY,
  isOfficialDoc,
  isOfficialDocKey,
  type PendingBill,
  type PendingDoc,
} from "./types";
import { useAttachmentsEnabled } from "./useAttachmentsEnabled";

export type { PendingDoc, PendingBill, CadastroSingleScreenProps };

export function CadastroSingleScreen({
  targetTable = "people",
  typeId,
  onCancel,
  onSuccess,
}: CadastroSingleScreenProps) {
  const { messages, t } = useI18n();
  const copy = messages.tables.cadastro;
  const queryClient = useQueryClient();
  const mode = targetTable;
  const attachmentsEnabled = useAttachmentsEnabled();
  const dropEnabled = attachmentsEnabled.data !== false;

  const documentTypes = useQuery({
    queryKey: queryKeys.types.documents,
    queryFn: ({ signal }) => listDocumentTypes(signal),
  });
  const billTypes = useQuery({
    queryKey: queryKeys.types.bills,
    queryFn: ({ signal }) => listBillTypes(signal),
  });
  const ocrCapability = useQuery({
    queryKey: queryKeys.ocr.capability,
    queryFn: ({ signal }) => getOCRCapability(signal),
  });

  const [selectedProfile, setSelectedProfile] = useState<Profile | null>(null);
  const [demographics, setDemographics] = useState(INITIAL_DEMOGRAPHICS);
  const [family, setFamily] = useState(INITIAL_FAMILY);
  const [complementary, setComplementary] = useState(INITIAL_COMPLEMENTARY);
  const [docFields, setDocFields] = useState(INITIAL_DOC_FIELDS);
  const [billFields, setBillFields] = useState(INITIAL_BILL_FIELDS);
  const [documents, setDocuments] = useState<PendingDoc[]>([]);
  const [bills, setBills] = useState<PendingBill[]>([]);
  const [saving, setSaving] = useState(false);
  const [persistError, setPersistError] = useState<string | null>(null);
  const [ocrOwner, setOcrOwner] = useState<AttachmentOwner | null>(null);
  const [savedName, setSavedName] = useState("");

  const standaloneDocFileId = useId();
  const standaloneBillFileId = useId();

  useEffect(() => {
    const seededDoc = typeId || documentTypes.data?.types?.[0]?.id;
    if (mode === "documents" && seededDoc) {
      setDocFields((prev) => (prev.docTypeId ? prev : { ...prev, docTypeId: seededDoc }));
    }
    const seededBill = typeId || billTypes.data?.types?.[0]?.id;
    if (mode === "bills" && seededBill) {
      setBillFields((prev) => (prev.billTypeId ? prev : { ...prev, billTypeId: seededBill }));
    }
  }, [billTypes.data?.types, documentTypes.data?.types, mode, typeId]);

  const applyProfileToState = (p: Profile) => {
    setSelectedProfile(p);
    const mapped = mapProfileToState(p);
    setDemographics(mapped.demographics);
    setFamily(mapped.family);
    setComplementary(mapped.complementary);
  };

  const handleClearProfile = () => {
    setSelectedProfile(null);
    setDemographics(INITIAL_DEMOGRAPHICS);
    setFamily(INITIAL_FAMILY);
    setComplementary(INITIAL_COMPLEMENTARY);
  };

  const handleDraftName = (name: string) => {
    setDemographics((prev) => ({ ...prev, fullName: name }));
  };

  const handleSelectNewName = (name: string) => {
    handleClearProfile();
    setDemographics((prev) => ({ ...prev, fullName: name }));
  };

  const cpfLookup = useQuery({
    queryKey: ["cadastro-cpf-lookup", cpfDigits(demographics.cpf)],
    queryFn: ({ signal }) => listProfilesLookup(cpfDigits(demographics.cpf), signal),
    enabled: isCompleteCpf(demographics.cpf) && !selectedProfile,
  });
  const existingMatch =
    cpfLookup.data?.profiles.find(
      (profile) =>
        cpfDigits(profile.cpf ?? "") === cpfDigits(demographics.cpf) &&
        profile.id !== selectedProfile?.id,
    ) ?? null;

  const matchingOfficialDocs = useMemo(() => {
    const list: string[] = [];
    if (selectedProfile?.document_identifiers) {
      Object.keys(selectedProfile.document_identifiers).forEach((key) => {
        if (isOfficialDocKey(key) && !list.includes(key)) list.push(key);
      });
    }
    documents.forEach((d) => {
      if (
        (isOfficialDocKey(d.typeKey) || isOfficialDoc(d.typeName)) &&
        !list.includes(d.typeName)
      ) {
        list.push(d.typeName);
      }
    });
    if (isCompleteCpf(demographics.cpf) && !list.includes("CPF")) list.push("CPF");
    return list;
  }, [demographics.cpf, documents, selectedProfile]);

  const hasMinimumRequirement = matchingOfficialDocs.length > 0;

  const selectedDocType =
    documentTypes.data?.types?.find((type) => type.id === docFields.docTypeId) ??
    documentTypes.data?.types?.[0];
  const selectedBillType =
    billTypes.data?.types?.find((type) => type.id === billFields.billTypeId) ??
    billTypes.data?.types?.[0];

  const validate = (): string | null => {
    if (mode === "documents") {
      if (!docFields.docTypeId && !selectedDocType) return copy.errorDocTypeRequired;
      if (!docFields.docIdentifier.trim()) return copy.errorDocNumberRequired;
      const regex = selectedDocType?.validation_regex ?? "";
      if (!matchesValidationRegex(docFields.docIdentifier, regex)) return copy.errorIdentifierRegex;
      if (selectedDocType?.date_required && !docFields.docDate) return copy.errorDocDateRequired;
      if (!selectedProfile && !demographics.fullName.trim()) return copy.errorHolderRequired;
      return null;
    }
    if (mode === "bills") {
      if (!billFields.billTypeId && !selectedBillType) return copy.errorBillTypeRequired;
      if (!billFields.billInstallation.trim()) return copy.errorBillInstallationRequired;
      if (!isCompetenceMonth(billFields.billCompetence)) return copy.errorBillCompetenceRequired;
      if (!billFields.billAmount.trim()) return copy.errorBillAmountRequired;
      if (
        !selectedProfile &&
        !demographics.fullName.trim() &&
        !billFields.billPrintedHolder.trim()
      ) {
        return copy.errorHolderRequired;
      }
      return null;
    }
    if (!demographics.fullName.trim() && !selectedProfile) {
      return messages.common.validation.fullNameRequired;
    }
    return null;
  };

  const blockReason = validate();

  const finish = (name: string) => {
    if (onSuccess) onSuccess(name);
    else onCancel();
  };

  const handlePersist = async () => {
    setSaving(true);
    setPersistError(null);
    try {
      const payload = buildProfilePayload(
        demographics,
        family,
        complementary,
        billFields.billPrintedHolder || demographics.fullName || copy.holderFallbackDefault,
        copy.holderFallbackDefault,
        selectedProfile?.notes ?? "",
      );
      const profile = await resolveOrCreateProfile({ selectedProfile, payload });
      setSelectedProfile(profile);
      const ownerId = profile.id;
      let lastOcr: AttachmentOwner | null = null;

      if (mode === "documents") {
        const fields = {
          ...docFields,
          docTypeId: docFields.docTypeId || selectedDocType?.id || "",
        };
        const saved = await persistDocumentForm({ ownerProfileId: ownerId, fields });
        lastOcr = saved.ocrOwner;
      } else if (mode === "bills") {
        const fields = {
          ...billFields,
          billTypeId: billFields.billTypeId || selectedBillType?.id || "",
        };
        const saved = await persistBillForm({
          ownerProfileId: ownerId,
          fields,
          demographics,
          fallbackHolder: copy.holderFallbackDefault,
        });
        lastOcr = saved.ocrOwner;
      } else {
        for (const doc of documents) {
          const saved = await persistPendingDocument({ ownerProfileId: ownerId, doc });
          if (saved.ocrOwner) lastOcr = saved.ocrOwner;
        }
        for (const bill of bills) {
          const saved = await persistPendingBill({
            ownerProfileId: ownerId,
            bill,
            demographics,
            fallbackHolder: copy.holderFallbackDefault,
          });
          if (saved.ocrOwner) lastOcr = saved.ocrOwner;
        }
      }

      void queryClient.invalidateQueries({ queryKey: queryKeys.tables.profiles() });
      void queryClient.invalidateQueries({ queryKey: ["global-search"] });
      void queryClient.invalidateQueries({ queryKey: queryKeys.profiles.all });
      void queryClient.invalidateQueries({ queryKey: queryKeys.records.documents() });
      void queryClient.invalidateQueries({ queryKey: queryKeys.records.bills() });
      void queryClient.invalidateQueries({ queryKey: MINIMUM_REQUIREMENT_QUERY });

      const name =
        demographics.fullName.trim() ||
        billFields.billPrintedHolder.trim() ||
        profile.full_name.trim();
      const successMsg =
        mode === "documents"
          ? t(copy.savedSuccessDoc, {
              type: selectedDocType?.label || copy.docFallbackDefault,
              name,
            })
          : mode === "bills"
            ? copy.savedSuccessBill.replace("{name}", name)
            : t(copy.savedSuccessPerson, { name });
      announceSaved(undefined, successMsg);

      if (lastOcr && ocrCapability.data?.enabled) {
        setSavedName(name);
        setOcrOwner(lastOcr);
        return;
      }
      finish(name);
    } catch (caught) {
      setPersistError(errorMessage(caught) || messages.common.labels.saveError);
    } finally {
      setSaving(false);
    }
  };

  const handlePrimary = () => {
    const error = validate();
    if (error) {
      setPersistError(error);
      return;
    }
    void handlePersist();
  };

  const crumbCurrentTitle =
    mode === "documents"
      ? copy.crumbNewDocument
      : mode === "bills"
        ? copy.crumbNewBill
        : copy.crumbNewPerson;

  const saveLabel =
    mode === "documents"
      ? copy.actionSaveDocument
      : mode === "bills"
        ? copy.actionSaveBill
        : copy.actionSavePerson;

  if (ocrOwner) {
    return (
      <CadastroWorkShell backLabel={copy.crumbHome} current={crumbCurrentTitle} onBack={onCancel}>
        <div className="cadastro-single">
          <p className="cadastro-review__lead">{copy.reviewDone}</p>
          <OcrReviewPanel owner={ocrOwner} />
          <CadastroStickyBar
            cancelButtonText={messages.common.actions.close}
            onCancel={() => finish(savedName)}
            onSave={() => finish(savedName)}
            saveButtonText={messages.common.actions.close}
            saving={false}
          />
        </div>
      </CadastroWorkShell>
    );
  }

  return (
    <CadastroWorkShell backLabel={copy.crumbHome} current={crumbCurrentTitle} onBack={onCancel}>
      <div className="cadastro-single">
        {persistError ? <StatusBanner title={persistError} tone="error" /> : null}
        {mode === "documents" && (
          <DocumentMode
            attachmentsEnabled={dropEnabled}
            cpf={demographics.cpf}
            docFields={docFields}
            documentTypes={documentTypes.data?.types ?? []}
            fileInputId={standaloneDocFileId}
            holderName={demographics.fullName}
            onChangeCpf={(cpf) => setDemographics((prev) => ({ ...prev, cpf }))}
            onChangeDocFields={(patch) => setDocFields((prev) => ({ ...prev, ...patch }))}
            onClearProfile={handleClearProfile}
            onDraftName={handleDraftName}
            onSelectNewName={handleSelectNewName}
            onSelectProfile={applyProfileToState}
            selectedProfile={selectedProfile}
          />
        )}

        {mode === "bills" && (
          <BillMode
            attachmentsEnabled={dropEnabled}
            billFields={billFields}
            billTypes={billTypes.data?.types ?? []}
            cpf={demographics.cpf}
            fileInputId={standaloneBillFileId}
            holderName={demographics.fullName}
            onChangeBillFields={(patch) => {
              setBillFields((prev) => ({ ...prev, ...patch }));
              if (patch.billPrintedHolder !== undefined && !demographics.fullName) {
                setDemographics((prev) => ({
                  ...prev,
                  fullName: patch.billPrintedHolder || "",
                }));
              }
            }}
            onChangeCpf={(cpf) => setDemographics((prev) => ({ ...prev, cpf }))}
            onClearProfile={handleClearProfile}
            onDraftName={handleDraftName}
            onSelectNewName={handleSelectNewName}
            onSelectProfile={applyProfileToState}
            selectedProfile={selectedProfile}
          />
        )}

        {mode === "people" && (
          <PersonMode
            attachmentsEnabled={dropEnabled}
            billTypes={billTypes.data?.types ?? []}
            bills={bills}
            complementary={complementary}
            demographics={demographics}
            documentTypes={documentTypes.data?.types ?? []}
            documents={documents}
            existingMatch={existingMatch}
            family={family}
            hasMinimumRequirement={hasMinimumRequirement}
            matchingOfficialDocs={matchingOfficialDocs}
            onAddBill={(bill) => setBills((prev) => [...prev, bill])}
            onAddDoc={(doc) => setDocuments((prev) => [...prev, doc])}
            onChangeComplementary={(patch) => setComplementary((prev) => ({ ...prev, ...patch }))}
            onChangeDemographics={(patch) => setDemographics((prev) => ({ ...prev, ...patch }))}
            onChangeFamily={(patch) => setFamily((prev) => ({ ...prev, ...patch }))}
            onClearProfile={handleClearProfile}
            onDraftName={handleDraftName}
            onRemoveBill={(id) => setBills((prev) => prev.filter((b) => b.id !== id))}
            onRemoveDoc={(id) => setDocuments((prev) => prev.filter((d) => d.id !== id))}
            onSelectNewName={handleSelectNewName}
            onSelectProfile={applyProfileToState}
            onUseExisting={applyProfileToState}
            selectedProfile={selectedProfile}
          />
        )}

        <CadastroStickyBar
          cancelButtonText={messages.common.actions.cancel}
          onCancel={onCancel}
          onSave={handlePrimary}
          saveButtonText={saving ? copy.savingRecord : saveLabel}
          saveDisabled={Boolean(blockReason)}
          saveHint={blockReason ?? undefined}
          saving={saving}
          summaryBlocked={Boolean(blockReason)}
          summaryText={
            blockReason ??
            (mode === "people"
              ? t(copy.summaryCount, {
                  docs: documents.length,
                  bills: bills.length,
                })
              : undefined)
          }
        />
      </div>
    </CadastroWorkShell>
  );
}

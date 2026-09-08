import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, message } from "antd";
import { ArrowLeft } from "lucide-react";
import { type ChangeEvent, useId, useMemo, useState } from "react";
import { useI18n } from "../../i18n";
import {
  createBill,
  createDocument,
  createProfile,
  listBillTypes,
  listDocumentTypes,
  type Profile,
} from "../api/client";
import { queryKeys } from "../api/queryKeys";
import { MINIMUM_REQUIREMENT_QUERY } from "./CadastroMinimumRequirement";
import { announceSaved } from "./cadastroFeedback";
import { buildProfilePayload, mapProfileToState } from "./cadastroPayloads";
import { CadastroStickyBar } from "./components/CadastroStickyBar";
import { BillMode } from "./modes/BillMode";
import { DocumentMode } from "./modes/DocumentMode";
import { PersonMode } from "./modes/PersonMode";
import {
  type CadastroSingleScreenProps,
  INITIAL_BILL_FIELDS,
  INITIAL_COMPLEMENTARY,
  INITIAL_DEMOGRAPHICS,
  INITIAL_DOC_FIELDS,
  INITIAL_FAMILY,
  isOfficialDoc,
  type PendingBill,
  type PendingDoc,
} from "./types";

export type { PendingDoc, PendingBill, CadastroSingleScreenProps };

export function CadastroSingleScreen({
  targetTable = "people",
  onCancel,
  onSuccess,
}: CadastroSingleScreenProps) {
  const { messages, t } = useI18n();
  const copy = messages.tables.cadastro;
  const queryClient = useQueryClient();

  const mode = targetTable;

  const documentTypes = useQuery({
    queryKey: queryKeys.types.documents,
    queryFn: ({ signal }) => listDocumentTypes(signal),
  });
  const billTypes = useQuery({
    queryKey: queryKeys.types.bills,
    queryFn: ({ signal }) => listBillTypes(signal),
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

  const standaloneDocFileId = useId();
  const standaloneBillFileId = useId();

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

  const handleSelectNewName = (name: string) => {
    handleClearProfile();
    setDemographics((prev) => ({ ...prev, fullName: name }));
  };

  const matchingOfficialDocs = useMemo(() => {
    const list: string[] = [];
    if (selectedProfile?.document_identifiers) {
      Object.keys(selectedProfile.document_identifiers).forEach((key) => {
        if (isOfficialDoc(key)) list.push(key);
      });
    }
    documents.forEach((d) => {
      if (isOfficialDoc(d.typeName) && !list.includes(d.typeName)) {
        list.push(d.typeName);
      }
    });
    if (demographics.cpf.trim().length >= 11 && !list.includes("CPF")) {
      list.push("CPF");
    }
    return list;
  }, [demographics.cpf, documents, selectedProfile]);

  const hasMinimumRequirement = matchingOfficialDocs.length > 0;

  const handleStandaloneDocOcrDrop = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setDocFields((prev) => ({
      ...prev,
      docIdentifier: "12.345.678-9",
      docDate: "2022-05-10",
      docValidUntil: "2032-05-10",
      docNotes: copy.ocrExtractedNote.replace("{filename}", file.name),
      docMedium: "DIGITAL",
    }));
    void message.success(copy.ocrSuccessDoc);
    e.target.value = "";
  };

  const handleStandaloneBillOcrDrop = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setBillFields((prev) => ({
      ...prev,
      billProvider: "Enel SP",
      billInstallation: "400123456",
      billCompetence: "2026-09",
      billAmount: "142,50",
      billPrintedHolder: prev.billPrintedHolder || demographics.fullName || "Mariana Albuquerque",
      billPrintedAddress: "Rua das Acácias, 120 — São Paulo/SP",
      billNotes: copy.ocrExtractedNote.replace("{filename}", file.name),
      billMedium: "DIGITAL",
    }));
    if (!demographics.fullName) {
      setDemographics((prev) => ({ ...prev, fullName: "Mariana Albuquerque" }));
    }
    void message.success(copy.ocrSuccessBill);
    e.target.value = "";
  };

  const resolveOrCreateProfile = async (fallbackName: string): Promise<string> => {
    if (mode !== "people" && selectedProfile) return selectedProfile.id;
    const payload = buildProfilePayload(
      demographics,
      family,
      complementary,
      fallbackName,
      copy.holderFallbackDefault,
    );
    const profile = await createProfile(payload);
    return profile.id;
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      if (mode === "documents") {
        const resolvedProfileId = await resolveOrCreateProfile(
          demographics.fullName || copy.holderFallbackDefault,
        );
        const typeId = docFields.docTypeId || documentTypes.data?.types?.[0]?.id || "";
        await createDocument({
          owner_profile_id: resolvedProfileId,
          document_type_id: typeId,
          identifier_value: docFields.docIdentifier.trim() || "—",
          document_date: docFields.docDate || (new Date().toISOString().split("T")[0] as string),
          ...(docFields.docValidUntil ? { valid_until: docFields.docValidUntil } : {}),
          medium: docFields.docMedium,
          ...(docFields.docMedium === "PHYSICAL" && docFields.docCustody
            ? { idle_custody: docFields.docCustody }
            : {}),
          notes: docFields.docNotes.trim(),
        });

        void queryClient.invalidateQueries({ queryKey: queryKeys.records.documents() });
        void queryClient.invalidateQueries({ queryKey: queryKeys.tables.profiles() });
        void queryClient.invalidateQueries({ queryKey: queryKeys.profiles.all });
        void queryClient.invalidateQueries({ queryKey: MINIMUM_REQUIREMENT_QUERY });

        const typeLabel =
          documentTypes.data?.types?.find((type) => type.id === typeId)?.label || copy.docFallbackDefault;
        const msg = t(copy.savedSuccessDoc, {
          type: typeLabel,
          name: demographics.fullName.trim() || copy.holderFallbackDefault,
        });
        announceSaved(undefined, msg);
        if (onSuccess) onSuccess(demographics.fullName.trim());
        else onCancel();
        return;
      }

      if (mode === "bills") {
        const resolvedProfileId = await resolveOrCreateProfile(
          billFields.billPrintedHolder || demographics.fullName || copy.holderFallbackDefault,
        );
        const typeId = billFields.billTypeId || billTypes.data?.types?.[0]?.id || "";
        await createBill({
          owner_profile_id: resolvedProfileId,
          bill_type_id: typeId,
          printed_holder_name:
            billFields.billPrintedHolder?.trim() ||
            demographics.fullName.trim() ||
            copy.holderFallbackDefault,
          printed_address:
            billFields.billPrintedAddress?.trim() ||
            demographics.address.trim() ||
            copy.addressNotProvided,
          reference_value: billFields.billInstallation.trim() || "—",
          competence: billFields.billCompetence.trim() || "2026-09",
          amount: billFields.billAmount.trim() || "0",
          currency: "BRL",
          notes: billFields.billNotes.trim() || billFields.billProvider.trim(),
          medium: billFields.billMedium,
        });

        void queryClient.invalidateQueries({ queryKey: queryKeys.records.bills() });
        void queryClient.invalidateQueries({ queryKey: queryKeys.tables.profiles() });
        void queryClient.invalidateQueries({ queryKey: queryKeys.profiles.all });

        const msg = copy.savedSuccessBill.replace(
          "{name}",
          demographics.fullName.trim() ||
            billFields.billPrintedHolder?.trim() ||
            copy.holderFallbackDefault,
        );
        announceSaved(undefined, msg);
        if (onSuccess) onSuccess(demographics.fullName.trim());
        else onCancel();
        return;
      }

      if (!demographics.fullName.trim()) {
        void message.error(messages.common.validation.fullNameRequired);
        setSaving(false);
        return;
      }

      const resolvedProfileId = await resolveOrCreateProfile(demographics.fullName.trim());

      for (const doc of documents) {
        if (doc.typeId) {
          await createDocument({
            owner_profile_id: resolvedProfileId,
            document_type_id: doc.typeId,
            identifier_value: doc.number || "—",
            document_date: doc.date || (new Date().toISOString().split("T")[0] as string),
            ...(doc.validUntil ? { valid_until: doc.validUntil } : {}),
            notes: doc.notes,
            medium: doc.medium,
            ...(doc.medium === "PHYSICAL" && doc.custody ? { idle_custody: doc.custody } : {}),
          });
        }
      }

      for (const b of bills) {
        if (b.typeId) {
          await createBill({
            owner_profile_id: resolvedProfileId,
            bill_type_id: b.typeId,
            printed_holder_name: demographics.fullName.trim(),
            printed_address: demographics.address.trim() || copy.addressNotProvided,
            reference_value: b.installation || "—",
            competence: b.competence || "2026-09",
            amount: b.amount || "0",
            currency: "BRL",
            notes: b.provider || "",
            medium: b.medium,
          });
        }
      }

      void queryClient.invalidateQueries({ queryKey: queryKeys.tables.profiles() });
      void queryClient.invalidateQueries({ queryKey: queryKeys.profiles.all });
      void queryClient.invalidateQueries({ queryKey: queryKeys.records.documents() });
      void queryClient.invalidateQueries({ queryKey: queryKeys.records.bills() });
      void queryClient.invalidateQueries({ queryKey: MINIMUM_REQUIREMENT_QUERY });

      const successMsg = t(copy.savedSuccessPerson, {
        name: demographics.fullName.trim(),
      });
      announceSaved(undefined, successMsg);

      if (onSuccess) onSuccess(demographics.fullName.trim());
      else onCancel();
    } catch {
      void message.error(copy.savingRecord);
    } finally {
      setSaving(false);
    }
  };

  const crumbCurrentTitle =
    mode === "documents"
      ? copy.crumbNewDocument
      : mode === "bills"
        ? copy.crumbNewBill
        : copy.crumbNewPerson;

  return (
    <div className="cadastro-single">
      <nav aria-label={copy.crumbHome} className="cadastro-crumb">
        <Button
          className="cadastro-crumb__btn"
          type="text"
          size="small"
          icon={<ArrowLeft size={13} />}
          onClick={onCancel}
        >
          {copy.crumbHome}
        </Button>
        <span className="cadastro-crumb__sep">/</span>
        <span className="cadastro-crumb__current">{crumbCurrentTitle}</span>
      </nav>

      {mode === "documents" && (
        <DocumentMode
          cpf={demographics.cpf}
          docFields={docFields}
          documentTypes={documentTypes.data?.types ?? []}
          fileInputId={standaloneDocFileId}
          holderName={demographics.fullName}
          onChangeCpf={(cpf) => setDemographics((prev) => ({ ...prev, cpf }))}
          onChangeDocFields={(patch) => setDocFields((prev) => ({ ...prev, ...patch }))}
          onChangeSelectedProfile={setSelectedProfile}
          onClearProfile={handleClearProfile}
          onFileDrop={handleStandaloneDocOcrDrop}
          onSelectNewName={handleSelectNewName}
          onSelectProfile={applyProfileToState}
          selectedProfile={selectedProfile}
        />
      )}

      {mode === "bills" && (
        <BillMode
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
          onChangeSelectedProfile={setSelectedProfile}
          onClearProfile={handleClearProfile}
          onFileDrop={handleStandaloneBillOcrDrop}
          onSelectNewName={handleSelectNewName}
          onSelectProfile={applyProfileToState}
          selectedProfile={selectedProfile}
        />
      )}

      {mode === "people" && (
        <PersonMode
          billTypes={billTypes.data?.types ?? []}
          bills={bills}
          complementary={complementary}
          demographics={demographics}
          documentTypes={documentTypes.data?.types ?? []}
          documents={documents}
          family={family}
          hasMinimumRequirement={hasMinimumRequirement}
          matchingOfficialDocs={matchingOfficialDocs}
          onAddBill={(bill) => setBills((prev) => [...prev, bill])}
          onAddDoc={(doc) => setDocuments((prev) => [...prev, doc])}
          onChangeComplementary={(patch) => setComplementary((prev) => ({ ...prev, ...patch }))}
          onChangeDemographics={(patch) => setDemographics((prev) => ({ ...prev, ...patch }))}
          onChangeFamily={(patch) => setFamily((prev) => ({ ...prev, ...patch }))}
          onRemoveBill={(id) => setBills((prev) => prev.filter((b) => b.id !== id))}
          onRemoveDoc={(id) => setDocuments((prev) => prev.filter((d) => d.id !== id))}
        />
      )}

      <CadastroStickyBar
        cancelButtonText={messages.common.actions.cancel}
        onCancel={onCancel}
        onSave={handleSave}
        saveButtonText={
          saving
            ? copy.savingRecord
            : mode === "documents"
              ? copy.actionSaveDocument
              : mode === "bills"
                ? copy.actionSaveBill
                : copy.actionSavePerson
        }
        saving={saving}
        summaryText={
          mode === "people"
            ? t(copy.summaryCount, {
                docs: documents.length,
                bills: bills.length,
              })
            : undefined
        }
      />
    </div>
  );
}

import { Check } from "lucide-react";
import { useI18n } from "../../../i18n";
import { StatusBanner } from "../../../components/StatusBanner";
import type { BillType, DocumentType, Profile } from "../../api/client";
import { HolderSearchSelect } from "../components/HolderSearchSelect";
import { HolderSelectedCard } from "../components/HolderSelectedCard";
import { PendingBillsSection } from "../components/PendingBillsSection";
import { PendingDocumentsSection } from "../components/PendingDocumentsSection";
import { PersonRecordFields } from "../components/PersonRecordFields";
import type { PersonComplementaryState } from "../components/PersonComplementaryGroup";
import type { PersonDemographicsState } from "../components/PersonDemographicsGroup";
import type { PersonFamilyState } from "../components/PersonFamilyGroup";
import type { PendingBill, PendingDoc } from "../types";

export interface PersonModeProps {
  demographics: PersonDemographicsState;
  onChangeDemographics: (patch: Partial<PersonDemographicsState>) => void;
  family: PersonFamilyState;
  onChangeFamily: (patch: Partial<PersonFamilyState>) => void;
  complementary: PersonComplementaryState;
  onChangeComplementary: (patch: Partial<PersonComplementaryState>) => void;
  documents: PendingDoc[];
  onAddDoc: (doc: PendingDoc) => void;
  onRemoveDoc: (id: string) => void;
  documentTypes: DocumentType[];
  bills: PendingBill[];
  onAddBill: (bill: PendingBill) => void;
  onRemoveBill: (id: string) => void;
  billTypes: BillType[];
  hasMinimumRequirement: boolean;
  matchingOfficialDocs: string[];
  selectedProfile: Profile | null;
  onSelectProfile: (p: Profile) => void;
  onSelectNewName: (name: string) => void;
  onDraftName: (name: string) => void;
  onClearProfile: () => void;
  existingMatch: Profile | null;
  onUseExisting: (profile: Profile) => void;
  attachmentsEnabled?: boolean;
}

export function PersonMode({
  demographics,
  onChangeDemographics,
  family,
  onChangeFamily,
  complementary,
  onChangeComplementary,
  documents,
  onAddDoc,
  onRemoveDoc,
  documentTypes,
  bills,
  onAddBill,
  onRemoveBill,
  billTypes,
  hasMinimumRequirement,
  matchingOfficialDocs,
  selectedProfile,
  onSelectProfile,
  onSelectNewName,
  onDraftName,
  onClearProfile,
  existingMatch,
  onUseExisting,
  attachmentsEnabled,
}: PersonModeProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;

  return (
    <>
      <PersonRecordFields
        complementary={complementary}
        demographics={demographics}
        defaultOpenComplementary={false}
        defaultOpenFamily={false}
        extraTitle={
          <span
            className={`minreq-pill ${
              hasMinimumRequirement ? "minreq-pill--satisfied" : "minreq-pill--pending"
            }`}
          >
            {hasMinimumRequirement ? (
              <>
                <Check size={14} strokeWidth={2.5} />
                <span>
                  {copy.minReqSatisfied.replace(
                    "{types}",
                    matchingOfficialDocs.slice(0, 2).join(", "),
                  )}
                </span>
              </>
            ) : (
              <span>{copy.minReqPending}</span>
            )}
          </span>
        }
        family={family}
        hideName={Boolean(selectedProfile)}
        identityLead={
          <>
            <div className="cadastro-field">
              <label className="cadastro-field__label" htmlFor="cad-person-existing">
                {copy.holderLinkExisting}
              </label>
              {selectedProfile ? (
                <HolderSelectedCard
                  autoFilledNotice={copy.holderAutoFilledNotice}
                  onUnlink={onClearProfile}
                  profile={selectedProfile}
                  selectedTitle={copy.holderSelectedTitle}
                  unlinkText={copy.holderUnlinkButton}
                />
              ) : (
                <HolderSearchSelect
                  ariaLabel={copy.holderSelectPlaceholder}
                  createNewOptionText={copy.holderCreateNewOption}
                  id="cad-person-existing"
                  onClear={onClearProfile}
                  onDraftName={onDraftName}
                  onSelectNewName={onSelectNewName}
                  onSelectProfile={onSelectProfile}
                  placeholder={copy.holderSelectPlaceholder}
                  selectedProfile={selectedProfile}
                  value={demographics.fullName}
                />
              )}
            </div>

            {existingMatch && !selectedProfile ? (
              <StatusBanner
                action={
                  <button
                    className="cadastro-linkbtn"
                    type="button"
                    onClick={() => onUseExisting(existingMatch)}
                  >
                    {copy.holderLinkExisting}
                  </button>
                }
                title={copy.holderExistsHint}
                tone="info"
              />
            ) : null}
          </>
        }
        onChangeComplementary={onChangeComplementary}
        onChangeDemographics={onChangeDemographics}
        onChangeFamily={onChangeFamily}
      />

      <PendingDocumentsSection
        attachmentsEnabled={attachmentsEnabled}
        defaultOpen={true}
        documentTypes={documentTypes}
        documents={documents}
        onAddDoc={onAddDoc}
        onRemoveDoc={onRemoveDoc}
      />

      <PendingBillsSection
        attachmentsEnabled={attachmentsEnabled}
        billTypes={billTypes}
        bills={bills}
        defaultOpen={false}
        onAddBill={onAddBill}
        onRemoveBill={onRemoveBill}
      />
    </>
  );
}

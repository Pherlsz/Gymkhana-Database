import { Car, Check, User, Users } from "lucide-react";
import { useI18n } from "../../../i18n";
import type { BillType, DocumentType } from "../../api/client";
import {
  CadastroSection,
  PendingBillsSection,
  PendingDocumentsSection,
  PersonComplementaryGroup,
  type PersonComplementaryState,
  PersonDemographicsGroup,
  type PersonDemographicsState,
  PersonFamilyGroup,
  type PersonFamilyState,
} from "../components";
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
}: PersonModeProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;

  return (
    <>
      <CadastroSection
        defaultOpen={true}
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
        hint={copy.sectionIdentityHint}
        icon={<User size={18} strokeWidth={1.75} />}
        title={copy.sectionIdentityTitle}
      >
        <PersonDemographicsGroup state={demographics} onChange={onChangeDemographics} />
      </CadastroSection>

      <CadastroSection
        defaultOpen={false}
        hint={copy.sectionFamilyHint}
        icon={<Users size={18} strokeWidth={1.75} />}
        title={copy.sectionFamilyTitle}
      >
        <PersonFamilyGroup state={family} onChange={onChangeFamily} />
      </CadastroSection>

      <CadastroSection
        defaultOpen={false}
        hint={copy.sectionComplementaryHint}
        icon={<Car size={18} strokeWidth={1.75} />}
        title={copy.sectionComplementaryTitle}
      >
        <PersonComplementaryGroup state={complementary} onChange={onChangeComplementary} />
      </CadastroSection>

      <PendingDocumentsSection
        defaultOpen={true}
        documentTypes={documentTypes}
        documents={documents}
        onAddDoc={onAddDoc}
        onRemoveDoc={onRemoveDoc}
      />

      <PendingBillsSection
        billTypes={billTypes}
        bills={bills}
        defaultOpen={true}
        onAddBill={onAddBill}
        onRemoveBill={onRemoveBill}
      />
    </>
  );
}

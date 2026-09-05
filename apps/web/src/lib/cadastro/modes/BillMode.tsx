import { Zap } from "lucide-react";
import type { ChangeEvent } from "react";
import { useI18n } from "../../../i18n";
import type { BillType, Profile } from "../../api/client";
import {
  BillFormFields,
  type BillFormFieldsState,
  CadastroHolderSection,
  CadastroSection,
} from "../components";

export interface BillModeProps {
  billFields: BillFormFieldsState;
  onChangeBillFields: (patch: Partial<BillFormFieldsState>) => void;
  billTypes: BillType[];
  fileInputId: string;
  onFileDrop: (e: ChangeEvent<HTMLInputElement>) => void;
  holderName: string;
  selectedProfile: Profile | null;
  onSelectProfile: (p: Profile) => void;
  onSelectNewName: (name: string) => void;
  onClearProfile: () => void;
  onChangeSelectedProfile: (p: Profile | null) => void;
  defaultOpenPrimary?: boolean;
  defaultOpenHolder?: boolean;
}

export function BillMode({
  billFields,
  onChangeBillFields,
  billTypes,
  fileInputId,
  onFileDrop,
  holderName,
  selectedProfile,
  onSelectProfile,
  onSelectNewName,
  onClearProfile,
  onChangeSelectedProfile,
  defaultOpenPrimary = true,
  defaultOpenHolder = true,
}: BillModeProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;

  return (
    <>
      <CadastroSection
        defaultOpen={defaultOpenPrimary}
        hint={copy.sectionBillDataHint}
        icon={<Zap size={18} strokeWidth={1.75} />}
        title={copy.sectionBillData}
      >
        <BillFormFields
          billTypes={billTypes}
          fileInputId={fileInputId}
          onFileDrop={onFileDrop}
          state={billFields}
          onChange={onChangeBillFields}
        />
      </CadastroSection>

      <CadastroHolderSection
        defaultOpen={defaultOpenHolder}
        holderName={holderName}
        inputId="cad-bill-holder"
        onChangeSelectedProfile={onChangeSelectedProfile}
        onClearProfile={onClearProfile}
        onSelectNewName={onSelectNewName}
        onSelectProfile={onSelectProfile}
        selectedProfile={selectedProfile}
      />
    </>
  );
}

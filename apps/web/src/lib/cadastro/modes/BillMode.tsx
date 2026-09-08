import { Zap } from "lucide-react";
import type { ChangeEvent } from "react";
import { useI18n } from "../../../i18n";
import type { BillType, Profile } from "../../api/client";
import { BillFormFields, type BillFormFieldsState } from "../components/BillFormFields";
import { CadastroHolderSection } from "../components/CadastroHolderSection";
import { CadastroSection } from "../components/CadastroSection";

export interface BillModeProps {
  billFields: BillFormFieldsState;
  onChangeBillFields: (patch: Partial<BillFormFieldsState>) => void;
  billTypes: BillType[];
  fileInputId: string;
  onFileDrop: (e: ChangeEvent<HTMLInputElement>) => void;
  holderName: string;
  cpf?: string | undefined;
  onChangeCpf?: ((cpf: string) => void) | undefined;
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
  cpf,
  onChangeCpf,
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
        badge={
          <span className="cadastro-group__badge cadastro-group__badge--bills">
            {copy.badgeConsumption}
          </span>
        }
        defaultOpen={defaultOpenPrimary}
        hint={copy.sectionBillDataHint}
        icon={<Zap size={18} strokeWidth={1.75} />}
        modifier="bills"
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
        cpf={cpf}
        cpfInputId="cad-bill-holder-cpf"
        defaultOpen={defaultOpenHolder}
        holderName={holderName}
        inputId="cad-bill-holder"
        onChangeCpf={onChangeCpf}
        onChangeSelectedProfile={onChangeSelectedProfile}
        onClearProfile={onClearProfile}
        onSelectNewName={onSelectNewName}
        onSelectProfile={onSelectProfile}
        selectedProfile={selectedProfile}
      />
    </>
  );
}

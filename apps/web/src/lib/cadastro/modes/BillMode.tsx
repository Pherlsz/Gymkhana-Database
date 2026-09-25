import { Zap } from "lucide-react";
import { useI18n } from "../../../i18n";
import type { BillType, Profile } from "../../api/client";
import { BillFormFields, type BillFormFieldsState } from "../components/BillFormFields";
import { CadastroHolderSection } from "../components/CadastroHolderSection";
import { CadastroSection, CadastroSectionBadge } from "../components/CadastroSection";

export interface BillModeProps {
  billFields: BillFormFieldsState;
  onChangeBillFields: (patch: Partial<BillFormFieldsState>) => void;
  billTypes: BillType[];
  fileInputId: string;
  holderName: string;
  cpf?: string | undefined;
  onChangeCpf?: ((cpf: string) => void) | undefined;
  selectedProfile: Profile | null;
  onSelectProfile: (p: Profile) => void;
  onSelectNewName: (name: string) => void;
  onDraftName: (name: string) => void;
  onClearProfile: () => void;
  lockType?: boolean;
  attachmentsEnabled?: boolean;
  defaultOpenPrimary?: boolean;
  defaultOpenHolder?: boolean;
}

export function BillMode({
  billFields,
  onChangeBillFields,
  billTypes,
  fileInputId,
  holderName,
  cpf,
  onChangeCpf,
  selectedProfile,
  onSelectProfile,
  onSelectNewName,
  onDraftName,
  onClearProfile,
  lockType,
  attachmentsEnabled,
  defaultOpenPrimary = true,
  defaultOpenHolder = true,
}: BillModeProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;

  return (
    <>
      <CadastroSection
        badge={
          <CadastroSectionBadge modifier="bills">{copy.badgeConsumption}</CadastroSectionBadge>
        }
        defaultOpen={defaultOpenPrimary}
        hint={copy.sectionBillDataHint}
        icon={<Zap size={18} strokeWidth={1.75} />}
        modifier="bills"
        title={copy.sectionBillData}
      >
        <BillFormFields
          attachmentsEnabled={attachmentsEnabled}
          billTypes={billTypes}
          fileInputId={fileInputId}
          lockType={lockType}
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
        onClearProfile={onClearProfile}
        onDraftName={onDraftName}
        onSelectNewName={onSelectNewName}
        onSelectProfile={onSelectProfile}
        selectedProfile={selectedProfile}
      />
    </>
  );
}

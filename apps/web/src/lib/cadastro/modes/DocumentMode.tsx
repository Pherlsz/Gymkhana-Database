import { FileText } from "lucide-react";
import { useI18n } from "../../../i18n";
import type { DocumentType, Profile } from "../../api/client";
import { CadastroHolderSection } from "../components/CadastroHolderSection";
import { CadastroSection, CadastroSectionBadge } from "../components/CadastroSection";
import { DocumentFormFields, type DocumentFormFieldsState } from "../components/DocumentFormFields";

export interface DocumentModeProps {
  docFields: DocumentFormFieldsState;
  onChangeDocFields: (patch: Partial<DocumentFormFieldsState>) => void;
  documentTypes: DocumentType[];
  fileInputId: string;
  holderName: string;
  cpf: string;
  onChangeCpf: (cpf: string) => void;
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

export function DocumentMode({
  docFields,
  onChangeDocFields,
  documentTypes,
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
}: DocumentModeProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;

  return (
    <>
      <CadastroSection
        badge={
          <CadastroSectionBadge modifier="documents">{copy.badgeOfficial}</CadastroSectionBadge>
        }
        defaultOpen={defaultOpenPrimary}
        hint={copy.sectionDocDataHint}
        icon={<FileText size={18} strokeWidth={1.75} />}
        modifier="documents"
        title={copy.sectionDocData}
      >
        <DocumentFormFields
          attachmentsEnabled={attachmentsEnabled}
          documentTypes={documentTypes}
          fileInputId={fileInputId}
          lockType={lockType}
          state={docFields}
          onChange={onChangeDocFields}
        />
      </CadastroSection>

      <CadastroHolderSection
        cpf={cpf}
        cpfInputId="cad-doc-holder-cpf"
        defaultOpen={defaultOpenHolder}
        holderName={holderName}
        inputId="cad-doc-holder"
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

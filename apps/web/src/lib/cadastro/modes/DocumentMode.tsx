import { FileText } from "lucide-react";
import type { ChangeEvent } from "react";
import { useI18n } from "../../../i18n";
import type { DocumentType, Profile } from "../../api/client";
import {
  CadastroHolderSection,
  CadastroSection,
  DocumentFormFields,
  type DocumentFormFieldsState,
} from "../components";

export interface DocumentModeProps {
  docFields: DocumentFormFieldsState;
  onChangeDocFields: (patch: Partial<DocumentFormFieldsState>) => void;
  documentTypes: DocumentType[];
  fileInputId: string;
  onFileDrop: (e: ChangeEvent<HTMLInputElement>) => void;
  holderName: string;
  cpf: string;
  onChangeCpf: (cpf: string) => void;
  selectedProfile: Profile | null;
  onSelectProfile: (p: Profile) => void;
  onSelectNewName: (name: string) => void;
  onClearProfile: () => void;
  onChangeSelectedProfile: (p: Profile | null) => void;
  defaultOpenPrimary?: boolean;
  defaultOpenHolder?: boolean;
}

export function DocumentMode({
  docFields,
  onChangeDocFields,
  documentTypes,
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
}: DocumentModeProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;

  return (
    <>
      <CadastroSection
        badge={
          <span className="cadastro-group__badge cadastro-group__badge--documents">
            {copy.badgeOfficial}
          </span>
        }
        defaultOpen={defaultOpenPrimary}
        hint={copy.sectionDocDataHint}
        icon={<FileText size={18} strokeWidth={1.75} />}
        modifier="documents"
        title={copy.sectionDocData}
      >
        <DocumentFormFields
          documentTypes={documentTypes}
          fileInputId={fileInputId}
          onFileDrop={onFileDrop}
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
        onChangeSelectedProfile={onChangeSelectedProfile}
        onClearProfile={onClearProfile}
        onSelectNewName={onSelectNewName}
        onSelectProfile={onSelectProfile}
        selectedProfile={selectedProfile}
      />
    </>
  );
}

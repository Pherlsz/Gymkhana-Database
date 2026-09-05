import { Input } from "antd";
import { User } from "lucide-react";
import { useI18n } from "../../../i18n";
import type { Profile } from "../../api/client";
import { CadastroSection } from "./CadastroSection";
import { HolderSearchSelect } from "./HolderSearchSelect";
import { HolderSelectedCard } from "./HolderSelectedCard";

export interface CadastroHolderSectionProps {
  defaultOpen?: boolean;
  open?: boolean;
  onToggleOpen?: () => void;
  holderName: string;
  selectedProfile: Profile | null;
  onSelectProfile: (p: Profile) => void;
  onSelectNewName: (name: string) => void;
  onClearProfile: () => void;
  onChangeSelectedProfile: (p: Profile | null) => void;
  inputId: string;
  cpf?: string | undefined;
  onChangeCpf?: ((cpf: string) => void) | undefined;
  cpfInputId?: string | undefined;
}

export function CadastroHolderSection({
  defaultOpen = true,
  open,
  onToggleOpen,
  holderName,
  selectedProfile,
  onSelectProfile,
  onSelectNewName,
  onClearProfile,
  onChangeSelectedProfile,
  inputId,
  cpf,
  onChangeCpf,
  cpfInputId,
}: CadastroHolderSectionProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;

  const showCpf = typeof onChangeCpf === "function";

  return (
    <CadastroSection
      defaultOpen={defaultOpen}
      hint={copy.sectionHolderHint}
      icon={<User size={18} strokeWidth={1.75} />}
      onToggleOpen={onToggleOpen}
      open={open}
      title={copy.sectionHolderTitle}
    >
      <div className="cadastro-grid">
        <div className={showCpf ? "cadastro-col-8" : "cadastro-col-12"}>
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor={inputId}>
              {copy.fieldHolderName}
            </label>
            {selectedProfile ? (
              <HolderSelectedCard
                autoFilledNotice={copy.holderAutoFilledNotice}
                changeText={copy.holderChangeButton}
                onChange={() => onChangeSelectedProfile(null)}
                onUnlink={onClearProfile}
                profile={selectedProfile}
                selectedTitle={copy.holderSelectedTitle}
                unlinkText={copy.holderUnlinkButton}
              />
            ) : (
              <HolderSearchSelect
                ariaLabel={copy.fieldHolderName}
                createNewOptionText={copy.holderCreateNewOption}
                id={inputId}
                onClear={onClearProfile}
                onSelectNewName={onSelectNewName}
                onSelectProfile={onSelectProfile}
                placeholder={copy.holderSelectPlaceholder}
                selectedProfile={selectedProfile}
                value={holderName}
              />
            )}
          </div>
        </div>

        {showCpf && (
          <div className="cadastro-col-4">
            <div className="cadastro-field">
              <label className="cadastro-field__label" htmlFor={cpfInputId}>
                {copy.fieldCpf}
              </label>
              <Input
                disabled={Boolean(selectedProfile)}
                id={cpfInputId}
                placeholder={copy.placeholderCpf}
                value={cpf ?? ""}
                onChange={(e) => onChangeCpf(e.target.value)}
              />
            </div>
          </div>
        )}
      </div>
    </CadastroSection>
  );
}

import { Input } from "antd";
import { User } from "lucide-react";
import { useI18n } from "../../../i18n";
import type { Profile } from "../../api/client";
import { CadastroSection } from "./CadastroSection";
import { HolderSearchSelect } from "./HolderSearchSelect";
import { HolderSelectedCard } from "./HolderSelectedCard";

export interface CadastroHolderSectionProps {
  defaultOpen?: boolean | undefined;
  open?: boolean | undefined;
  onToggleOpen?: (() => void) | undefined;
  holderName: string;
  selectedProfile: Profile | null;
  onSelectProfile?: ((p: Profile) => void) | undefined;
  onSelectNewName?: ((name: string) => void) | undefined;
  onDraftName?: ((name: string) => void) | undefined;
  onClearProfile?: (() => void) | undefined;
  inputId: string;
  cpf?: string | undefined;
  onChangeCpf?: ((cpf: string) => void) | undefined;
  cpfInputId?: string | undefined;
  locked?: boolean | undefined;
}

export function CadastroHolderSection({
  defaultOpen = true,
  open,
  onToggleOpen,
  holderName,
  selectedProfile,
  onSelectProfile,
  onSelectNewName,
  onDraftName,
  onClearProfile,
  inputId,
  cpf,
  onChangeCpf,
  cpfInputId,
  locked = false,
}: CadastroHolderSectionProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;

  const showCpf = typeof onChangeCpf === "function";
  const canSearch = Boolean(onSelectProfile && onSelectNewName && onClearProfile);

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
        <div className={showCpf && !locked ? "cadastro-col-8" : "cadastro-col-12"}>
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor={inputId}>
              {copy.fieldHolderName}
            </label>
            {selectedProfile ? (
              <HolderSelectedCard
                profile={selectedProfile}
                selectedTitle={copy.holderSelectedTitle}
                unlinkText={copy.holderUnlinkButton}
                {...(locked
                  ? {}
                  : {
                      autoFilledNotice: copy.holderAutoFilledNotice,
                      onUnlink: onClearProfile,
                    })}
              />
            ) : canSearch ? (
              <HolderSearchSelect
                ariaLabel={copy.fieldHolderName}
                createNewOptionText={copy.holderCreateNewOption}
                id={inputId}
                onClear={onClearProfile!}
                onDraftName={onDraftName}
                onSelectNewName={onSelectNewName!}
                onSelectProfile={onSelectProfile!}
                placeholder={copy.holderSelectPlaceholder}
                selectedProfile={selectedProfile}
                value={holderName}
              />
            ) : (
              <p className="cadastro-field__static">{holderName}</p>
            )}
          </div>
        </div>

        {showCpf && !locked ? (
          <div className="cadastro-col-4">
            <div className="cadastro-field">
              <label className="cadastro-field__label" htmlFor={cpfInputId}>
                {messages.common.labels.cpf}
              </label>
              <Input
                disabled={Boolean(selectedProfile)}
                id={cpfInputId}
                placeholder={messages.common.placeholders.cpf}
                value={cpf ?? ""}
                onChange={(e) => onChangeCpf(e.target.value)}
              />
            </div>
          </div>
        ) : null}
      </div>
    </CadastroSection>
  );
}

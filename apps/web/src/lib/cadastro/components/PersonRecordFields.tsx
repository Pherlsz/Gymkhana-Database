import { Car, User, Users } from "lucide-react";
import type { ReactNode } from "react";
import { useI18n } from "../../../i18n";
import { complementaryHasValues, familyHasValues } from "../cadastroPayloads";
import { CadastroSection } from "./CadastroSection";
import {
  PersonComplementaryGroup,
  type PersonComplementaryState,
} from "./PersonComplementaryGroup";
import { PersonDemographicsGroup, type PersonDemographicsState } from "./PersonDemographicsGroup";
import { PersonFamilyGroup, type PersonFamilyState } from "./PersonFamilyGroup";

export interface PersonRecordFieldsProps {
  demographics: PersonDemographicsState;
  onChangeDemographics: (patch: Partial<PersonDemographicsState>) => void;
  family: PersonFamilyState;
  onChangeFamily: (patch: Partial<PersonFamilyState>) => void;
  complementary: PersonComplementaryState;
  onChangeComplementary: (patch: Partial<PersonComplementaryState>) => void;
  disabled?: boolean | undefined;
  hideName?: boolean | undefined;
  identityLead?: ReactNode;
  extraTitle?: ReactNode;
  defaultOpenFamily?: boolean | undefined;
  defaultOpenComplementary?: boolean | undefined;
  defaultShowMoreDetails?: boolean | undefined;
}

export function PersonRecordFields({
  demographics,
  onChangeDemographics,
  family,
  onChangeFamily,
  complementary,
  onChangeComplementary,
  disabled,
  hideName,
  identityLead,
  extraTitle,
  defaultOpenFamily,
  defaultOpenComplementary,
  defaultShowMoreDetails,
}: PersonRecordFieldsProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const openFamily = defaultOpenFamily ?? familyHasValues(family);
  const openComplementary = defaultOpenComplementary ?? complementaryHasValues(complementary);

  return (
    <>
      <CadastroSection
        defaultOpen
        extraTitle={extraTitle}
        hint={copy.sectionIdentityHint}
        icon={<User size={18} strokeWidth={1.75} />}
        title={copy.sectionIdentityTitle}
      >
        {identityLead}
        <PersonDemographicsGroup
          defaultShowMore={Boolean(defaultShowMoreDetails)}
          disabled={Boolean(disabled)}
          hideName={Boolean(hideName)}
          state={demographics}
          onChange={onChangeDemographics}
        />
      </CadastroSection>

      <CadastroSection
        defaultOpen={openFamily}
        hint={copy.sectionFamilyHint}
        icon={<Users size={18} strokeWidth={1.75} />}
        title={copy.sectionFamilyTitle}
      >
        <PersonFamilyGroup disabled={Boolean(disabled)} state={family} onChange={onChangeFamily} />
      </CadastroSection>

      <CadastroSection
        defaultOpen={openComplementary}
        hint={copy.sectionComplementaryHint}
        icon={<Car size={18} strokeWidth={1.75} />}
        title={copy.sectionComplementaryTitle}
      >
        <PersonComplementaryGroup
          disabled={Boolean(disabled)}
          state={complementary}
          onChange={onChangeComplementary}
        />
      </CadastroSection>
    </>
  );
}

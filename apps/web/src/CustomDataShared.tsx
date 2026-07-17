import type {
  CustomEntityTypeValues,
  CustomField,
  CustomFieldKind,
  CustomFieldValues,
  CustomOption,
  CustomOptionValues,
} from "./lib/api/customdata";

export const emptyType: CustomEntityTypeValues = {
  technical_key: "",
  label: "",
  active: true,
  profile_cardinality: "MANY_PER_PROFILE",
};
export const emptyField: CustomFieldValues = {
  target_kind: "PROFILE",
  technical_key: "",
  label: "",
  field_kind: "TEXT",
  required: false,
  active: true,
};
export const emptyOption: CustomOptionValues = {
  technical_key: "",
  label: "",
  active: true,
  sort_order: 0,
};
export const fieldKinds: CustomFieldKind[] = [
  "TEXT",
  "LONG_TEXT",
  "INTEGER",
  "DECIMAL",
  "BOOLEAN",
  "CIVIL_DATE",
  "CIVIL_MONTH",
  "EMAIL",
  "PHONE",
  "SINGLE_SELECT",
  "MULTI_SELECT",
  "ATTACHMENT",
];

export function SectionTitle({ title, description }: { title: string; description: string }) {
  return (
    <div className="custom-section-title">
      <h2>{title}</h2>
      <p>{description}</p>
    </div>
  );
}
export function fieldKindLabel(value: CustomFieldKind) {
  return (
    {
      TEXT: "Texto",
      LONG_TEXT: "Texto longo",
      INTEGER: "Inteiro",
      DECIMAL: "Decimal",
      BOOLEAN: "Sim/Não",
      CIVIL_DATE: "Data",
      CIVIL_MONTH: "Mês",
      EMAIL: "E-mail",
      PHONE: "Telefone",
      SINGLE_SELECT: "Seleção única",
      MULTI_SELECT: "Seleção múltipla",
      ATTACHMENT: "Anexo privado",
    } as Record<CustomFieldKind, string>
  )[value];
}
export function numberOrUndefined(value: string) {
  const parsed = Number(value);
  return value === "" || !Number.isFinite(parsed) ? undefined : Math.trunc(parsed);
}
export function fieldValues(value: CustomField): CustomFieldValues {
  return {
    target_kind: value.target_kind,
    ...(value.target_id ? { target_id: value.target_id } : {}),
    technical_key: value.technical_key,
    label: value.label,
    field_kind: value.field_kind,
    required: value.required,
    active: value.active,
    ...(value.minimum_length ? { minimum_length: value.minimum_length } : {}),
    ...(value.maximum_length ? { maximum_length: value.maximum_length } : {}),
    ...(value.validation_regex ? { validation_regex: value.validation_regex } : {}),
    ...(value.minimum_decimal ? { minimum_decimal: value.minimum_decimal } : {}),
    ...(value.maximum_decimal ? { maximum_decimal: value.maximum_decimal } : {}),
  };
}
export function optionValues(value: CustomOption): CustomOptionValues {
  return {
    technical_key: value.technical_key,
    label: value.label,
    active: value.active,
    sort_order: value.sort_order,
  };
}

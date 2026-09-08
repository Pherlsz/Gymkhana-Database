import { Checkbox, Input, Select } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import "./customdata.css";
import {
  getCustomValues,
  listCustomFields,
  listCustomOptions,
  replaceCustomValues,
  type CustomField,
  type CustomTargetKind,
  type CustomValueInput,
  type CustomValueSet,
  type CustomValueTargetKind,
} from "./lib/api/customdata";
import { APIRequestError } from "./lib/api/client";
import { useI18n } from "./i18n";
import { queryKeys } from "./lib/api/queryKeys";

export type CustomDraftValue = string | boolean | string[] | null;

export function scalarFieldsOf(fields: CustomField[]): CustomField[] {
  return fields.filter((field) => field.active && field.field_kind !== "ATTACHMENT");
}

export function useTypeCustomFields(
  definitionTargetKind: CustomTargetKind,
  definitionTargetId: string | undefined,
) {
  const query = useQuery({
    queryKey: queryKeys.customData.fields(definitionTargetKind, definitionTargetId),
    queryFn: ({ signal }) => listCustomFields(definitionTargetKind, definitionTargetId, signal),
  });
  return { scalarFields: scalarFieldsOf(query.data?.fields ?? []), query };
}

export function useSeedRecordDraft(
  valueTargetKind: CustomValueTargetKind,
  recordId: string | undefined,
  setDraft: (value: Record<string, CustomDraftValue>) => void,
) {
  const query = useQuery({
    queryKey: queryKeys.customData.values(valueTargetKind, recordId),
    queryFn: ({ signal }) => getCustomValues(valueTargetKind, recordId!, signal),
    enabled: Boolean(recordId),
  });
  useEffect(() => {
    if (query.data) setDraft(draftFromValueSet(query.data));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data]);
  return query;
}

export function CustomFieldInputGrid(props: {
  fields: CustomField[];
  draft: Record<string, CustomDraftValue>;
  disabled: boolean;
  onChange: (value: Record<string, CustomDraftValue>) => void;
}) {
  return (
    <div className="custom-values__grid">
      {props.fields.map((field) => (
        <CustomFieldControl
          key={field.id}
          disabled={props.disabled}
          field={field}
          value={props.draft[field.id] ?? null}
          onChange={(next) => props.onChange({ ...props.draft, [field.id]: next })}
        />
      ))}
    </div>
  );
}

function CustomFieldControl(props: {
  field: CustomField;
  value: CustomDraftValue;
  disabled: boolean;
  onChange: (value: CustomDraftValue) => void;
}) {
  const { messages } = useI18n();
  const copy = messages.admin.customValues;
  const selectField =
    props.field.field_kind === "SINGLE_SELECT" || props.field.field_kind === "MULTI_SELECT";
  const options = useQuery({
    queryKey: queryKeys.customData.options(props.field.id),
    queryFn: ({ signal }) => listCustomOptions(props.field.id, signal),
    enabled: selectField,
  });
  const label = `${props.field.label}${props.field.required ? " *" : ""}`;

  if (props.field.field_kind === "ATTACHMENT") return null;
  if (props.field.field_kind === "BOOLEAN") {
    return (
      <label>
        {label}
        <Select
          disabled={props.disabled}
          onChange={(value) => props.onChange(value === "" ? null : value === "true")}
          options={[
            { value: "", label: copy.notProvided },
            { value: "true", label: copy.yes },
            { value: "false", label: copy.no },
          ]}
          value={props.value === true ? "true" : props.value === false ? "false" : ""}
        />
      </label>
    );
  }
  if (props.field.field_kind === "SINGLE_SELECT") {
    const selected = Array.isArray(props.value) ? (props.value[0] ?? "") : "";
    return (
      <label>
        {label}
        <Select
          disabled={props.disabled || options.isLoading}
          onChange={(value) => props.onChange(value ? [value] : [])}
          options={[
            { value: "", label: copy.notProvided },
            ...(options.data?.options.map((option) => ({
              value: option.id,
              label: `${option.label}${option.active ? "" : copy.inactiveSuffix}`,
              disabled: !option.active && option.id !== selected,
            })) ?? []),
          ]}
          value={selected}
        />
      </label>
    );
  }
  if (props.field.field_kind === "MULTI_SELECT") {
    const selected = Array.isArray(props.value) ? props.value : [];
    return (
      <fieldset className="custom-values__choices" disabled={props.disabled || options.isLoading}>
        <legend>{label}</legend>
        {options.data?.options.map((option) => (
          <Checkbox
            checked={selected.includes(option.id)}
            disabled={!option.active && !selected.includes(option.id)}
            key={option.id}
            onChange={(event) =>
              props.onChange(
                event.target.checked
                  ? [...selected, option.id]
                  : selected.filter((value) => value !== option.id),
              )
            }
          >
            {option.label}
            {option.active ? "" : copy.inactiveSuffix}
          </Checkbox>
        ))}
      </fieldset>
    );
  }
  if (props.field.field_kind === "LONG_TEXT") {
    return (
      <label className="custom-values__wide">
        {label}
        <Input.TextArea
          disabled={props.disabled}
          maxLength={props.field.maximum_length || undefined}
          rows={4}
          value={typeof props.value === "string" ? props.value : ""}
          onChange={(event) => props.onChange(event.target.value)}
        />
      </label>
    );
  }
  const inputType =
    props.field.field_kind === "EMAIL"
      ? "email"
      : props.field.field_kind === "PHONE"
        ? "tel"
        : props.field.field_kind === "CIVIL_DATE"
          ? "date"
          : props.field.field_kind === "CIVIL_MONTH"
            ? "month"
            : props.field.field_kind === "INTEGER" || props.field.field_kind === "DECIMAL"
              ? "number"
              : "text";
  return (
    <label>
      {label}
      <Input
        disabled={props.disabled}
        maxLength={props.field.maximum_length || undefined}
        min={props.field.minimum_decimal || undefined}
        max={props.field.maximum_decimal || undefined}
        step={props.field.field_kind === "DECIMAL" ? "any" : undefined}
        type={inputType}
        value={typeof props.value === "string" ? props.value : ""}
        onChange={(event) => props.onChange(event.target.value)}
      />
    </label>
  );
}

export function RecordCustomFieldsSection(props: {
  definitionTargetKind: CustomTargetKind;
  definitionTargetId: string | undefined;
  disabled: boolean;
  draft: Record<string, CustomDraftValue>;
  onDraftChange: (value: Record<string, CustomDraftValue>) => void;
}) {
  const { scalarFields, query } = useTypeCustomFields(
    props.definitionTargetKind,
    props.definitionTargetId,
  );
  if (query.isLoading || scalarFields.length === 0) return null;
  return (
    <div className="record-form__wide">
      <CustomFieldInputGrid
        fields={scalarFields}
        draft={props.draft}
        disabled={props.disabled}
        onChange={props.onDraftChange}
      />
    </div>
  );
}

export function draftFromValueSet(valueSet: CustomValueSet): Record<string, CustomDraftValue> {
  return draftFromStoredValues(valueSet.values ?? []);
}

export function draftFromStoredValues(
  values: CustomValueSet["values"] | undefined,
): Record<string, CustomDraftValue> {
  const result: Record<string, CustomDraftValue> = {};
  for (const value of values ?? []) {
    switch (value.field_kind) {
      case "TEXT":
      case "LONG_TEXT":
      case "EMAIL":
      case "PHONE":
        result[value.field_definition_id] = value.text ?? "";
        break;
      case "INTEGER":
        result[value.field_definition_id] =
          value.integer === undefined ? "" : String(value.integer);
        break;
      case "DECIMAL":
        result[value.field_definition_id] = value.decimal ?? "";
        break;
      case "BOOLEAN":
        result[value.field_definition_id] = value.boolean ?? null;
        break;
      case "CIVIL_DATE":
        result[value.field_definition_id] = value.civil_date ?? "";
        break;
      case "CIVIL_MONTH":
        result[value.field_definition_id] = value.civil_month ?? "";
        break;
      case "SINGLE_SELECT":
      case "MULTI_SELECT":
        result[value.field_definition_id] = value.option_ids ?? [];
        break;
    }
  }
  return result;
}

function inputFromDraft(field: CustomField, value: CustomDraftValue): CustomValueInput[] {
  if (field.field_kind === "ATTACHMENT") return [];
  const base: CustomValueInput = { field_definition_id: field.id, field_kind: field.field_kind };
  if (field.field_kind === "BOOLEAN") {
    return typeof value === "boolean" ? [{ ...base, boolean: value }] : [];
  }
  if (field.field_kind === "SINGLE_SELECT" || field.field_kind === "MULTI_SELECT") {
    return Array.isArray(value) && value.length > 0 ? [{ ...base, option_ids: value }] : [];
  }
  const text = typeof value === "string" ? value.trim() : "";
  if (text === "") return [];
  switch (field.field_kind) {
    case "INTEGER": {
      const integer = Number(text);
      return Number.isSafeInteger(integer) ? [{ ...base, integer }] : [{ ...base, text }];
    }
    case "DECIMAL":
      return [{ ...base, decimal: text }];
    case "CIVIL_DATE":
      return [{ ...base, civil_date: text }];
    case "CIVIL_MONTH":
      return [{ ...base, civil_month: text }];
    default:
      return [{ ...base, text }];
  }
}

export function customInputsFromDraft(
  fields: CustomField[],
  draft: Record<string, CustomDraftValue>,
): CustomValueInput[] {
  return fields.flatMap((field) => inputFromDraft(field, draft[field.id] ?? null));
}

export function customDataError(error: unknown): string {
  if (error instanceof APIRequestError && error.status === 409) {
    return "Outro usuário alterou o registro. Recarregue os dados e tente novamente.";
  }
  if (error instanceof APIRequestError && error.fieldErrors.length > 0) {
    return error.fieldErrors.map((field) => `${field.field}: ${field.message}`).join(" · ");
  }
  return error instanceof Error ? error.message : "Erro inesperado.";
}

export async function saveRecordCustomValues(args: {
  valueTargetKind: CustomValueTargetKind;
  definitionTargetKind: CustomTargetKind;
  definitionTargetId: string | undefined;
  recordId: string;
  recordVersion: number;
  draft: Record<string, CustomDraftValue>;
  force: boolean;
}): Promise<import("./lib/api/customdata").CustomValueSet | null> {
  const response = await listCustomFields(args.definitionTargetKind, args.definitionTargetId);
  const inputs = customInputsFromDraft(scalarFieldsOf(response.fields ?? []), args.draft);
  if (!args.force && inputs.length === 0) return null;
  return replaceCustomValues(args.valueTargetKind, args.recordId, args.recordVersion, inputs);
}

export function recordCustomFieldError(error: unknown): string {
  return customDataError(error);
}

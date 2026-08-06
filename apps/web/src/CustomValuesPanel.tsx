import { Alert, Button, Inline, Stack, StatusBadge, Surface } from "./ui";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { AttachmentsPanel } from "./AttachmentsPanel";
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

export type CustomDraftValue = string | boolean | string[] | null;

type Props = {
  definitionTargetKind: CustomTargetKind;
  definitionTargetId?: string;
  valueTargetKind: CustomValueTargetKind;
  valueTargetId: string;
  onSaved?: () => void | Promise<void>;
};

export function CustomValuesPanel(props: Props) {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<Record<string, CustomDraftValue>>({});
  const fields = useQuery({
    queryKey: ["custom-fields", props.definitionTargetKind, props.definitionTargetId ?? "global"],
    queryFn: ({ signal }) =>
      listCustomFields(props.definitionTargetKind, props.definitionTargetId, signal),
  });
  const values = useQuery({
    queryKey: ["custom-values", props.valueTargetKind, props.valueTargetId],
    queryFn: ({ signal }) => getCustomValues(props.valueTargetKind, props.valueTargetId, signal),
  });
  const activeFields = useMemo(
    () => (fields.data?.fields ?? []).filter((field) => field.active),
    [fields.data?.fields],
  );
  const scalarFields = useMemo(
    () => activeFields.filter((field) => field.field_kind !== "ATTACHMENT"),
    [activeFields],
  );
  const attachmentFields = useMemo(
    () => activeFields.filter((field) => field.field_kind === "ATTACHMENT"),
    [activeFields],
  );

  useEffect(() => {
    if (values.data) setDraft(draftFromValueSet(values.data));
  }, [values.data]);

  const save = useMutation({
    mutationFn: async () => {
      if (!values.data) throw new Error("Os dados personalizados ainda não foram carregados.");
      return replaceCustomValues(
        props.valueTargetKind,
        props.valueTargetId,
        values.data.version,
        customInputsFromDraft(scalarFields, draft),
      );
    },
    onSuccess: async (updated) => {
      queryClient.setQueryData(
        ["custom-values", props.valueTargetKind, props.valueTargetId],
        updated,
      );
      setDraft(draftFromValueSet(updated));
      setEditing(false);
      await props.onSaved?.();
    },
  });

  if (fields.isLoading || values.isLoading) {
    return <p className="custom-values__empty">Carregando dados personalizados...</p>;
  }
  if (fields.isError || values.isError) {
    return (
      <Alert title="Não foi possível carregar dados personalizados" tone="danger">
        {customDataError(fields.error ?? values.error)}
      </Alert>
    );
  }
  if (activeFields.length === 0) {
    return (
      <Surface className="custom-values" tone="raised">
        <span className="custom-values__empty">
          Nenhum campo personalizado ativo para este registro.
        </span>
      </Surface>
    );
  }

  return (
    <Stack gap="4">
      {scalarFields.length > 0 ? (
        <Surface className="custom-values" tone="raised">
          <Stack gap="4">
            <Inline align="center" className="custom-values__header">
              <div>
                <strong>Dados personalizados</strong>
                <p>Campos tipados definidos pela administração.</p>
              </div>
              <StatusBadge tone={editing ? "info" : "neutral"}>
                {editing ? "Editando" : "Somente leitura"}
              </StatusBadge>
            </Inline>
            {save.isError ? (
              <Alert title="Não foi possível salvar" tone="danger">
                {customDataError(save.error)}
              </Alert>
            ) : null}
            <CustomFieldInputGrid
              disabled={!editing || save.isPending}
              draft={draft}
              fields={scalarFields}
              onChange={setDraft}
            />
            <Inline>
              {editing ? (
                <>
                  <Button disabled={save.isPending} onClick={() => save.mutate()}>
                    {save.isPending ? "Salvando" : "Salvar dados personalizados"}
                  </Button>
                  <Button
                    disabled={save.isPending}
                    onClick={() => {
                      setDraft(values.data ? draftFromValueSet(values.data) : {});
                      setEditing(false);
                    }}
                  >
                    Cancelar
                  </Button>
                </>
              ) : (
                <Button onClick={() => setEditing(true)}>Editar dados personalizados</Button>
              )}
            </Inline>
          </Stack>
        </Surface>
      ) : null}
      {attachmentFields.map((field) => (
        <AttachmentsPanel
          key={field.id}
          description="Campo de anexo privado. O arquivo é verificado antes de aparecer no registro."
          owner={{
            owner_kind: "CUSTOM_FIELD",
            owner_id: props.valueTargetId,
            custom_target_kind: attachmentTargetKind(props.valueTargetKind),
            field_definition_id: field.id,
          }}
          title={field.label}
        />
      ))}
    </Stack>
  );
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
  const selectField =
    props.field.field_kind === "SINGLE_SELECT" || props.field.field_kind === "MULTI_SELECT";
  const options = useQuery({
    queryKey: ["custom-field-options", props.field.id],
    queryFn: ({ signal }) => listCustomOptions(props.field.id, signal),
    enabled: selectField,
  });
  const label = `${props.field.label}${props.field.required ? " *" : ""}`;

  if (props.field.field_kind === "ATTACHMENT") return null;
  if (props.field.field_kind === "BOOLEAN") {
    return (
      <label>
        {label}
        <select
          disabled={props.disabled}
          value={props.value === true ? "true" : props.value === false ? "false" : ""}
          onChange={(event) =>
            props.onChange(event.target.value === "" ? null : event.target.value === "true")
          }
        >
          <option value="">Não informado</option>
          <option value="true">Sim</option>
          <option value="false">Não</option>
        </select>
      </label>
    );
  }
  if (props.field.field_kind === "SINGLE_SELECT") {
    const selected = Array.isArray(props.value) ? (props.value[0] ?? "") : "";
    return (
      <label>
        {label}
        <select
          disabled={props.disabled || options.isLoading}
          value={selected}
          onChange={(event) => props.onChange(event.target.value ? [event.target.value] : [])}
        >
          <option value="">Não informado</option>
          {options.data?.options.map((option) => (
            <option
              key={option.id}
              disabled={!option.active && option.id !== selected}
              value={option.id}
            >
              {option.label}
              {option.active ? "" : " (inativa)"}
            </option>
          ))}
        </select>
      </label>
    );
  }
  if (props.field.field_kind === "MULTI_SELECT") {
    const selected = Array.isArray(props.value) ? props.value : [];
    return (
      <fieldset className="custom-values__choices" disabled={props.disabled || options.isLoading}>
        <legend>{label}</legend>
        {options.data?.options.map((option) => (
          <label key={option.id}>
            <input
              checked={selected.includes(option.id)}
              disabled={!option.active && !selected.includes(option.id)}
              type="checkbox"
              onChange={(event) =>
                props.onChange(
                  event.target.checked
                    ? [...selected, option.id]
                    : selected.filter((value) => value !== option.id),
                )
              }
            />
            {option.label}
            {option.active ? "" : " (inativa)"}
          </label>
        ))}
      </fieldset>
    );
  }
  if (props.field.field_kind === "LONG_TEXT") {
    return (
      <label className="custom-values__wide">
        {label}
        <textarea
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
      <input
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

function attachmentTargetKind(value: CustomValueTargetKind) {
  switch (value) {
    case "profile":
      return "PROFILE" as const;
    case "document":
      return "DOCUMENT" as const;
    case "bill":
      return "BILL" as const;
    case "custom_entity":
      return "CUSTOM_ENTITY" as const;
  }
}

import { Alert, Button, Card, Checkbox, Flex, Input, Select, Tag } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useI18n } from "./i18n";
import {
  emptyField,
  fieldKinds,
  fieldKindLabel,
  fieldValues,
  numberOrUndefined,
  SectionTitle,
} from "./CustomDataShared";
import { CustomOptionsAdmin } from "./CustomOptionsAdmin";
import { customDataError } from "./CustomValuesPanel";
import { listBillTypes, listDocumentTypes } from "./lib/api/client";
import {
  createCustomField,
  deleteCustomField,
  listCustomEntityTypes,
  listCustomFields,
  updateCustomField,
  type CustomField,
  type CustomFieldKind,
  type CustomFieldValues,
  type CustomTargetKind,
} from "./lib/api/customdata";

function scopedFieldValues(
  values: CustomFieldValues,
  targetKind: CustomTargetKind,
  targetId: string,
): CustomFieldValues {
  const { target_id: _targetId, ...rest } = values;
  return {
    ...rest,
    target_kind: targetKind,
    ...(targetKind !== "PROFILE" && targetId ? { target_id: targetId } : {}),
  };
}

function withOptionalNumber(
  values: CustomFieldValues,
  key: "minimum_length" | "maximum_length",
  rawValue: string,
): CustomFieldValues {
  const next = { ...values };
  delete next[key];
  const parsed = numberOrUndefined(rawValue);
  if (parsed !== undefined) next[key] = parsed;
  return next;
}

function withOptionalString(
  values: CustomFieldValues,
  key: "validation_regex",
  rawValue: string,
): CustomFieldValues {
  const next = { ...values };
  delete next[key];
  if (rawValue !== "") next[key] = rawValue;
  return next;
}

export function CustomFieldsAdmin() {
  const { messages } = useI18n();
  const copy = messages.admin.customFields;
  const queryClient = useQueryClient();
  const types = useQuery({
    queryKey: ["custom-entity-types"],
    queryFn: ({ signal }) => listCustomEntityTypes(signal),
  });
  const documentTypes = useQuery({
    queryKey: ["document-types"],
    queryFn: ({ signal }) => listDocumentTypes(signal),
  });
  const billTypes = useQuery({
    queryKey: ["bill-types"],
    queryFn: ({ signal }) => listBillTypes(signal),
  });
  const [targetKind, setTargetKind] = useState<CustomTargetKind>("PROFILE");
  const [targetId, setTargetId] = useState("");
  const [draft, setDraft] = useState<CustomFieldValues>(emptyField);
  const [editing, setEditing] = useState<CustomField | null>(null);
  const [selected, setSelected] = useState<CustomField | null>(null);
  const fields = useQuery({
    queryKey: ["custom-fields", targetKind, targetId || "global"],
    queryFn: ({ signal }) => listCustomFields(targetKind, targetId || undefined, signal),
    enabled: targetKind === "PROFILE" || Boolean(targetId),
  });

  useEffect(() => {
    setDraft((current) => scopedFieldValues(current, targetKind, targetId));
    setEditing(null);
    setSelected(null);
  }, [targetKind, targetId]);

  const save = useMutation({
    mutationFn: () =>
      editing ? updateCustomField(editing.id, editing.version, draft) : createCustomField(draft),
    onSuccess: async () => {
      setEditing(null);
      setDraft(scopedFieldValues(emptyField, targetKind, targetId));
      await queryClient.invalidateQueries({ queryKey: ["custom-fields", targetKind] });
    },
  });
  const remove = useMutation({
    mutationFn: (value: CustomField) =>
      deleteCustomField(value.id, value.version, messages.common.actions.confirm),
    onSuccess: async () => {
      setSelected(null);
      await queryClient.invalidateQueries({ queryKey: ["custom-fields", targetKind] });
    },
  });
  const targets =
    targetKind === "DOCUMENT_TYPE"
      ? (documentTypes.data?.types.map((value) => ({ id: value.id, label: value.label })) ?? [])
      : targetKind === "BILL_TYPE"
        ? (billTypes.data?.types.map((value) => ({ id: value.id, label: value.label })) ?? [])
        : targetKind === "CUSTOM_ENTITY_TYPE"
          ? (types.data?.types.map((value) => ({ id: value.id, label: value.label })) ?? [])
          : [];

  return (
    <Flex vertical gap="1rem">
      <SectionTitle
        title={copy.title}
        description={copy.description}
      />
      {save.isError || remove.isError || fields.isError ? (
        <Alert
          message={copy.errorTitle}
          type="error"
          description={<>{customDataError(save.error ?? remove.error ?? fields.error)}</>}
        />
      ) : null}
      <Card className="custom-admin-form">
        <div className="custom-admin-grid">
          <label>
            {copy.context}
            <Select
              value={targetKind}
              onChange={(value) => {
                setTargetKind(value as CustomTargetKind);
                setTargetId("");
              }}
              options={[
                { value: "PROFILE", label: copy.targetProfile },
                { value: "DOCUMENT_TYPE", label: copy.targetDocumentType },
                { value: "BILL_TYPE", label: copy.targetBillType },
                { value: "CUSTOM_ENTITY_TYPE", label: copy.targetCustomEntityType },
              ]}
            />
          </label>
          {targetKind !== "PROFILE" ? (
            <label>
              {copy.target}
              <Select
                value={targetId}
                onChange={(value) => setTargetId(value)}
                options={[
                  { value: "", label: copy.targetSelectPlaceholder },
                  ...targets.map((value) => ({ value: value.id, label: value.label })),
                ]}
              />
            </label>
          ) : null}
          <label>
            {copy.technicalKey}
            <Input
              disabled={Boolean(editing)}
              value={draft.technical_key}
              onChange={(event) => setDraft({ ...draft, technical_key: event.target.value })}
            />
          </label>
          <label>
            {copy.name}
            <Input
              value={draft.label}
              onChange={(event) => setDraft({ ...draft, label: event.target.value })}
            />
          </label>
          <label>
            {copy.valueKind}
            <Select
              disabled={Boolean(editing)}
              value={draft.field_kind}
              onChange={(value) =>
                setDraft({ ...draft, field_kind: value as CustomFieldKind })
              }
              options={fieldKinds.map((value) => ({
                value,
                label: fieldKindLabel(value),
              }))}
            />
          </label>
          <label>
            {copy.minLength}
            <Input
              inputMode="numeric"
              value={draft.minimum_length ?? ""}
              onChange={(event) =>
                setDraft((current) =>
                  withOptionalNumber(current, "minimum_length", event.target.value),
                )
              }
            />
          </label>
          <label>
            {copy.maxLength}
            <Input
              inputMode="numeric"
              value={draft.maximum_length ?? ""}
              onChange={(event) =>
                setDraft((current) =>
                  withOptionalNumber(current, "maximum_length", event.target.value),
                )
              }
            />
          </label>
          <label>
            {copy.optionalRegex}
            <Input
              value={draft.validation_regex ?? ""}
              onChange={(event) =>
                setDraft((current) =>
                  withOptionalString(current, "validation_regex", event.target.value),
                )
              }
            />
          </label>
          <Checkbox
            checked={draft.required}
            className="custom-admin-check"
            onChange={(event) => setDraft({ ...draft, required: event.target.checked })}
          >
            {copy.required}
          </Checkbox>
          <Checkbox
            checked={draft.active}
            className="custom-admin-check"
            onChange={(event) => setDraft({ ...draft, active: event.target.checked })}
          >
            {copy.active}
          </Checkbox>
        </div>
        <Flex>
          <Button
            disabled={
              save.isPending ||
              !draft.technical_key ||
              !draft.label ||
              (targetKind !== "PROFILE" && !targetId)
            }
            onClick={() => save.mutate()}
            type="primary"
          >
            {editing ? copy.saveField : copy.createField}
          </Button>
          {editing ? (
            <Button
              onClick={() => {
                setEditing(null);
                setDraft(scopedFieldValues(emptyField, targetKind, targetId));
              }}
            >
              {copy.cancel}
            </Button>
          ) : null}
        </Flex>
      </Card>
      <div className="custom-admin-list">
        {fields.data?.fields?.map((value) => (
          <Card key={value.id} className="custom-admin-card">
            <Flex vertical gap="0.75rem">
              <Flex align="center" className="custom-admin-card__header">
                <div>
                  <strong>{value.label}</strong>
                  <p>
                    {value.technical_key} · {fieldKindLabel(value.field_kind)}
                  </p>
                </div>
                <Tag color={value.active ? "success" : "neutral"}>
                  {value.active ? copy.active : copy.inactive}
                </Tag>
              </Flex>
              <Flex>
                <Button
                  onClick={() => {
                    setEditing(value);
                    setDraft(fieldValues(value));
                  }}
                >
                  {copy.edit}
                </Button>
                {value.field_kind === "SINGLE_SELECT" || value.field_kind === "MULTI_SELECT" ? (
                  <Button onClick={() => setSelected(value)}>{copy.options}</Button>
                ) : null}
                <Button danger disabled={remove.isPending} onClick={() => remove.mutate(value)}>
                  {copy.delete}
                </Button>
              </Flex>
            </Flex>
          </Card>
        ))}
      </div>
      {selected ? <CustomOptionsAdmin field={selected} onClose={() => setSelected(null)} /> : null}
    </Flex>
  );
}

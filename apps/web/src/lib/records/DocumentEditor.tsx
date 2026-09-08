import { Alert, Card, Input, Select } from "antd";
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ConfirmDelete } from "../../components/ConfirmDelete";
import { useI18n } from "../../i18n";
import {
  createDocument,
  updateDocument,
  type DocumentRecord,
  type DocumentType,
  type DocumentValuesRequest,
  type Profile,
} from "../api/client";
import {
  RecordCustomFieldsSection,
  recordCustomFieldError,
  saveRecordCustomValues,
  useSeedRecordDraft,
  type CustomDraftValue,
} from "../../RecordCustomFields";
import {
  CurrentUseControls,
  EditorActions,
  EditorHeader,
  MissingRecord,
  documentValues,
  media,
  mediumLabel,
  withRecordMedium,
} from "./RecordEditorCommon";

export function DocumentEditor(props: {
  mode: "create" | "view" | "edit";
  profile: Profile;
  record: DocumentRecord | undefined;
  types: DocumentType[];
  lockType?: boolean | undefined;
  initialTypeId?: string | undefined;
  canDelete: boolean;
  pending: boolean;
  onClose: () => void;
  onEdit: () => void;
  onSaved: (value: DocumentRecord, message: string) => Promise<void>;
  onDuplicate: (value: DocumentRecord) => void;
  onDelete: (value: DocumentRecord, confirmation: string) => void;
}) {
  const [values, setValues] = useState<DocumentValuesRequest>(() =>
    props.record
      ? documentValues(props.record)
      : {
          owner_profile_id: props.profile.id,
          document_type_id:
            props.initialTypeId && props.types.some((value) => value.id === props.initialTypeId)
              ? props.initialTypeId
              : (props.types.find((value) => value.active)?.id ?? ""),
          identifier_value: "",
          document_date: "",
          notes: "",
          medium: "PHYSICAL",
          idle_custody: "ORGANIZATION",
        },
  );
  const { messages } = useI18n();
  const panel = messages.records.panel;
  const common = messages.common;
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [customDraft, setCustomDraft] = useState<Record<string, CustomDraftValue>>({});
  useSeedRecordDraft("document", props.record?.id, setCustomDraft);
  const queryClient = useQueryClient();
  const editable = props.mode !== "view";
  const submit = async () => {
    setSaving(true);
    setError(null);
    try {
      const saved = props.record
        ? await updateDocument(props.record.id, { ...values, version: props.record.version })
        : await createDocument(values);
      const updatedCustomValues = await saveRecordCustomValues({
        valueTargetKind: "document",
        definitionTargetKind: "DOCUMENT_TYPE",
        definitionTargetId: saved.type.id,
        recordId: saved.id,
        recordVersion: saved.version,
        draft: customDraft,
        force: Boolean(props.record),
      });
      if (updatedCustomValues) {
        queryClient.setQueryData(["custom-values", "document", saved.id], updatedCustomValues);
      }
      await props.onSaved(saved, props.record ? panel.docUpdatedNotice : panel.docCreatedNotice);
    } catch (caught) {
      setError(recordCustomFieldError(caught));
    } finally {
      setSaving(false);
    }
  };
  if (props.mode !== "create" && !props.record) return <MissingRecord onClose={props.onClose} />;
  return (
    <Card className="record-editor">
      <EditorHeader
        closeLabel={panel.closeRecord}
        title={
          props.record
            ? `${props.record.type.label} · ${props.record.identifier_value}`
            : panel.newDoc
        }
        onClose={props.onClose}
      />
      {error ? <Alert message={panel.saveError} type="error" description={<>{error}</>} /> : null}
      <div className="record-form">
        <label>
          {common.labels.type}
          <Select
            disabled={!editable || Boolean(props.lockType)}
            onChange={(value) => setValues({ ...values, document_type_id: value })}
            options={[
              { value: "", label: panel.selectPlaceholder },
              ...props.types
                .filter((value) => value.active || value.id === values.document_type_id)
                .map((value) => ({
                  value: value.id,
                  label: value.label,
                })),
            ]}
            value={values.document_type_id}
          />
        </label>
        <label>
          {common.labels.identifier}
          <Input
            disabled={!editable}
            value={values.identifier_value}
            onChange={(event) => setValues({ ...values, identifier_value: event.target.value })}
          />
        </label>
        <label>
          {common.labels.date}
          <Input
            disabled={!editable}
            type="date"
            value={values.document_date}
            onChange={(event) => setValues({ ...values, document_date: event.target.value })}
          />
        </label>
        <label>
          {common.labels.validUntil}
          <Input
            disabled={!editable}
            type="date"
            value={values.valid_until ?? ""}
            onChange={(event) => setValues({ ...values, valid_until: event.target.value })}
          />
        </label>
        <label>
          {common.labels.medium}
          <Select
            disabled={!editable}
            onChange={(value) =>
              setValues(
                withRecordMedium(values, value as DocumentValuesRequest["medium"]),
              )
            }
            options={media.map((value) => ({
              value,
              label: mediumLabel(value, messages),
            }))}
            value={values.medium}
          />
        </label>
        {values.medium === "PHYSICAL" ? (
          <label>
            {common.labels.custody}
            <Select
              disabled={!editable}
              onChange={(value) =>
                setValues({
                  ...values,
                  idle_custody: value as NonNullable<
                    DocumentValuesRequest["idle_custody"]
                  >,
                })
              }
              options={[
                { value: "ORGANIZATION", label: panel.custodyOrg },
                { value: "OWNER", label: panel.custodyOwner },
              ]}
              value={values.idle_custody ?? "ORGANIZATION"}
            />
          </label>
        ) : null}
        <label className="record-form__wide">
          {common.labels.notes}
          <Input.TextArea
            disabled={!editable}
            rows={4}
            value={values.notes}
            onChange={(event) => setValues({ ...values, notes: event.target.value })}
          />
        </label>
        <RecordCustomFieldsSection
          definitionTargetKind="DOCUMENT_TYPE"
          definitionTargetId={values.document_type_id || undefined}
          disabled={!editable}
          draft={customDraft}
          onDraftChange={setCustomDraft}
        />
      </div>
      <EditorActions
        editable={editable}
        saving={saving}
        record={props.record}
        onSave={submit}
        onEdit={props.onEdit}
        onDuplicate={props.onDuplicate}
      />
      {props.record && props.record.medium === "PHYSICAL" ? (
        <CurrentUseControls
          kind="document"
          record={props.record}
          onChanged={async (message) => {
            await props.onSaved(props.record!, message);
          }}
        />
      ) : null}
      {props.record && props.canDelete ? (
        <ConfirmDelete
          cancelLabel={common.actions.cancel}
          confirmLabel={panel.deleteTitle}
          confirmationLabel={panel.deleteDocConfirmPrompt}
          confirmationWord={common.actions.confirm}
          description={panel.deleteDocDesc}
          pending={props.pending}
          title={panel.deleteTitle}
          onCancel={props.onClose}
          onConfirm={(word) => props.onDelete(props.record!, word)}
        />
      ) : null}
    </Card>
  );
}

import { Alert, Card, Input, Select } from "antd";
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ConfirmDelete } from "../../components/ConfirmDelete";
import { useI18n } from "../../i18n";
import {
  createBill,
  updateBill,
  type BillRecord,
  type BillType,
  type BillValuesRequest,
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
  billValues,
  media,
  mediumLabel,
  profileAddress,
  withRecordMedium,
} from "./RecordEditorCommon";

export function BillEditor(props: {
  mode: "create" | "view" | "edit";
  profile: Profile;
  record: BillRecord | undefined;
  types: BillType[];
  lockType?: boolean | undefined;
  initialTypeId?: string | undefined;
  canDelete: boolean;
  pending: boolean;
  onClose: () => void;
  onEdit: () => void;
  onSaved: (value: BillRecord, message: string) => Promise<void>;
  onDuplicate: (value: BillRecord) => void;
  onDelete: (value: BillRecord, confirmation: string) => void;
}) {
  const [values, setValues] = useState<BillValuesRequest>(() =>
    props.record
      ? billValues(props.record)
      : {
          owner_profile_id: props.profile.id,
          bill_type_id:
            props.initialTypeId && props.types.some((value) => value.id === props.initialTypeId)
              ? props.initialTypeId
              : (props.types.find((value) => value.active)?.id ?? ""),
          printed_holder_name: props.profile.full_name,
          printed_address: profileAddress(props.profile),
          reference_value: "",
          competence: "",
          amount: "",
          currency: "BRL",
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
  useSeedRecordDraft("bill", props.record?.id, setCustomDraft);
  const queryClient = useQueryClient();
  const editable = props.mode !== "view";
  const submit = async () => {
    setSaving(true);
    setError(null);
    try {
      const saved = props.record
        ? await updateBill(props.record.id, {
            ...values,
            owner_profile_id: values.owner_profile_id ?? props.record.owner_profile_id,
            version: props.record.version,
          })
        : await createBill(values);
      const updatedCustomValues = await saveRecordCustomValues({
        valueTargetKind: "bill",
        definitionTargetKind: "BILL_TYPE",
        definitionTargetId: saved.type.id,
        recordId: saved.id,
        recordVersion: saved.version,
        draft: customDraft,
        force: Boolean(props.record),
      });
      if (updatedCustomValues) {
        queryClient.setQueryData(["custom-values", "bill", saved.id], updatedCustomValues);
      }
      await props.onSaved(saved, props.record ? panel.billUpdatedNotice : panel.billCreatedNotice);
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
            ? `${props.record.type.label} · ${props.record.reference_value}`
            : panel.newBill
        }
        onClose={props.onClose}
      />
      {error ? <Alert message={panel.saveError} type="error" description={<>{error}</>} /> : null}
      <div className="record-form">
        <label>
          {common.labels.type}
          <Select
            disabled={!editable || Boolean(props.lockType)}
            onChange={(value) => setValues({ ...values, bill_type_id: value })}
            options={[
              { value: "", label: panel.selectPlaceholder },
              ...props.types
                .filter((value) => value.active || value.id === values.bill_type_id)
                .map((value) => ({
                  value: value.id,
                  label: value.label,
                })),
            ]}
            value={values.bill_type_id}
          />
        </label>
        <label>
          {common.labels.reference}
          <Input
            disabled={!editable}
            value={values.reference_value}
            onChange={(event) => setValues({ ...values, reference_value: event.target.value })}
          />
        </label>
        <label>
          {common.labels.competence}
          <Input
            disabled={!editable}
            placeholder={panel.competencePlaceholder}
            value={values.competence}
            onChange={(event) => setValues({ ...values, competence: event.target.value })}
          />
        </label>
        <label>
          {common.labels.amount}
          <Input
            disabled={!editable}
            inputMode="decimal"
            value={values.amount}
            onChange={(event) => setValues({ ...values, amount: event.target.value })}
          />
        </label>
        <label>
          {common.labels.currency}
          <Input
            disabled={!editable}
            maxLength={3}
            value={values.currency}
            onChange={(event) =>
              setValues({ ...values, currency: event.target.value.toUpperCase() })
            }
          />
        </label>
        <label>
          {common.labels.medium}
          <Select
            disabled={!editable}
            onChange={(value) =>
              setValues(withRecordMedium(values, value as BillValuesRequest["medium"]))
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
                  idle_custody: value as NonNullable<BillValuesRequest["idle_custody"]>,
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
          {common.labels.printedHolder}
          <Input
            disabled={!editable}
            value={values.printed_holder_name}
            onChange={(event) => setValues({ ...values, printed_holder_name: event.target.value })}
          />
        </label>
        <label className="record-form__wide">
          {common.labels.printedAddress}
          <Input
            disabled={!editable}
            value={values.printed_address}
            onChange={(event) => setValues({ ...values, printed_address: event.target.value })}
          />
        </label>
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
          definitionTargetKind="BILL_TYPE"
          definitionTargetId={values.bill_type_id || undefined}
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
          kind="bill"
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
          confirmationLabel={panel.deleteBillConfirmPrompt}
          confirmationWord={common.actions.confirm}
          description={panel.deleteBillDesc}
          pending={props.pending}
          title={panel.deleteTitle}
          onCancel={props.onClose}
          onConfirm={(word) => props.onDelete(props.record!, word)}
        />
      ) : null}
    </Card>
  );
}

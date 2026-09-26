import { Flex, Input } from "antd";
import { UserRound, Zap } from "lucide-react";
import { type ReactNode, useId, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ConfirmDelete } from "../../components/ConfirmDelete";
import { useI18n } from "../../i18n";
import {
  createBill,
  updateBill,
  type BillRecord,
  type BillType,
  type Profile,
} from "../api/client";
import {
  recordCustomFieldError,
  saveRecordCustomValues,
  useSeedRecordDraft,
  type CustomDraftValue,
} from "../../RecordCustomFields";
import { CadastroHolderSection } from "../cadastro/components/CadastroHolderSection";
import { CadastroSection, CadastroSectionBadge } from "../cadastro/components/CadastroSection";
import { StatusBanner } from "../../components/StatusBanner";
import { BillFormFields } from "../cadastro/components/BillFormFields";
import { RecordScreen } from "../cadastro/components/RecordScreen";
import { billFormFromValues, billValuesFromForm } from "../cadastro/recordFieldMaps";
import {
  CurrentUseControls,
  EditorActions,
  MissingRecord,
  billValues,
  profileAddress,
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
  onOpenOwner?: (() => void) | undefined;
  children?: ReactNode;
}) {
  const fileInputId = useId();
  const initialValues = props.record
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
        medium: "PHYSICAL" as const,
        idle_custody: "ORGANIZATION" as const,
      };
  const [fields, setFields] = useState(() => billFormFromValues(initialValues));
  const [currency, setCurrency] = useState(initialValues.currency);
  const [idleCustody, setIdleCustody] = useState<"ORGANIZATION" | "OWNER">(
    initialValues.idle_custody ?? "ORGANIZATION",
  );
  const { messages } = useI18n();
  const panel = messages.records.panel;
  const common = messages.common;
  const recordCopy = messages.tables.record;
  const cadastro = messages.tables.cadastro;
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  useSeedRecordDraft("bill", props.record?.id, (draft: Record<string, CustomDraftValue>) =>
    setFields((current) => ({ ...current, customDraft: draft })),
  );
  const queryClient = useQueryClient();
  const editable = props.mode !== "view";
  const submit = async () => {
    setSaving(true);
    setError(null);
    try {
      const values = billValuesFromForm(props.profile.id, fields, {
        currency,
        ...(fields.billMedium === "PHYSICAL" ? { idleCustody } : {}),
      });
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
        draft: fields.customDraft,
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
    <RecordScreen
      ariaLabel={recordCopy.billAria}
      closeLabel={messages.tables.inspector.close}
      eyebrow={recordCopy.billEyebrow}
      footer={
        <Flex className="profile-panel__actions" vertical gap="0.75rem">
          <EditorActions
            editable={editable}
            saving={saving}
            record={props.record}
            onSave={() => void submit()}
            onEdit={props.onEdit}
            onDuplicate={props.onDuplicate}
          />
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
        </Flex>
      }
      onClose={props.onClose}
      onOpenOwner={props.onOpenOwner}
      openOwnerLabel={recordCopy.openOwner}
      ownerLabel={recordCopy.owner}
      ownerName={props.profile.full_name}
      title={props.record ? props.record.reference_value : panel.newBill}
    >
      {error ? <StatusBanner description={error} title={panel.saveError} tone="error" /> : null}
      <CadastroSection
        badge={
          <CadastroSectionBadge modifier="bills">{cadastro.badgeConsumption}</CadastroSectionBadge>
        }
        defaultOpen
        hint={cadastro.sectionBillDataHint}
        icon={<Zap size={18} strokeWidth={1.75} />}
        modifier="bills"
        title={cadastro.sectionBillData}
      >
        <BillFormFields
          billTypes={props.types.filter((value) => value.active || value.id === fields.billTypeId)}
          disabled={!editable}
          fileInputId={fileInputId}
          lockType={props.lockType}
          showDropzone={props.mode === "create"}
          state={fields}
          onChange={(patch) => {
            setFields((current) => ({ ...current, ...patch }));
            if (patch.billMedium === "DIGITAL") setIdleCustody("ORGANIZATION");
          }}
        />
        <div className="cadastro-grid" style={{ marginTop: "var(--gym-space-4)" }}>
          <div className="cadastro-col-6">
            <div className="cadastro-field">
              <label className="cadastro-field__label" htmlFor="cad-bill-currency">
                {common.labels.currency}
              </label>
              <Input
                disabled={!editable}
                id="cad-bill-currency"
                maxLength={3}
                value={currency}
                onChange={(event) => setCurrency(event.target.value.toUpperCase())}
              />
            </div>
          </div>
        </div>
      </CadastroSection>
      <CadastroHolderSection
        defaultOpen
        holderName={props.profile.full_name}
        inputId="record-bill-holder"
        locked
        selectedProfile={props.profile}
      />
      {props.record && props.record.medium === "PHYSICAL" ? (
        <CadastroSection
          defaultOpen
          icon={<UserRound size={18} strokeWidth={1.75} />}
          title={panel.currentUseTitle}
        >
          <CurrentUseControls
            framed={false}
            kind="bill"
            record={props.record}
            onChanged={async (message) => {
              await props.onSaved(props.record!, message);
            }}
          />
        </CadastroSection>
      ) : null}
      {props.children}
    </RecordScreen>
  );
}

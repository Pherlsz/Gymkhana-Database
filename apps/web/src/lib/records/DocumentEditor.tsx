import { Flex } from "antd";
import { FileText, UserRound } from "lucide-react";
import { type ReactNode, useId, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ConfirmDelete } from "../../components/ConfirmDelete";
import { useI18n } from "../../i18n";
import { uploadAttachment } from "../api/attachments";
import {
  createDocument,
  updateDocument,
  type DocumentRecord,
  type DocumentType,
  type Profile,
} from "../api/client";
import { useAttachmentsEnabled } from "../cadastro/useAttachmentsEnabled";
import {
  recordCustomFieldError,
  saveRecordCustomValues,
  useSeedRecordDraft,
  type CustomDraftValue,
} from "../../RecordCustomFields";
import { CadastroHolderSection } from "../cadastro/components/CadastroHolderSection";
import { CadastroSection, CadastroSectionBadge } from "../cadastro/components/CadastroSection";
import { StatusBanner } from "../../components/StatusBanner";
import { DocumentFormFields } from "../cadastro/components/DocumentFormFields";
import { RecordScreen } from "../cadastro/components/RecordScreen";
import { documentFormFromValues, documentValuesFromForm } from "../cadastro/recordFieldMaps";
import {
  CurrentUseControls,
  EditorActions,
  MissingRecord,
  documentValues,
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
  onOpenOwner?: (() => void) | undefined;
  children?: ReactNode;
}) {
  const fileInputId = useId();
  const [fields, setFields] = useState(() => {
    if (props.record) return documentFormFromValues(documentValues(props.record));
    const typeId =
      props.initialTypeId && props.types.some((value) => value.id === props.initialTypeId)
        ? props.initialTypeId
        : (props.types.find((value) => value.active)?.id ?? "");
    return documentFormFromValues({
      owner_profile_id: props.profile.id,
      document_type_id: typeId,
      identifier_value: "",
      document_date: "",
      notes: "",
      medium: "PHYSICAL",
      idle_custody: "ORGANIZATION",
    });
  });
  const { messages } = useI18n();
  const panel = messages.records.panel;
  const common = messages.common;
  const recordCopy = messages.tables.record;
  const cadastro = messages.tables.cadastro;
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  useSeedRecordDraft("document", props.record?.id, (draft: Record<string, CustomDraftValue>) =>
    setFields((current) => ({ ...current, customDraft: draft })),
  );
  const queryClient = useQueryClient();
  const attachmentsEnabled = useAttachmentsEnabled();
  const editable = props.mode !== "view";
  const submit = async () => {
    setSaving(true);
    setError(null);
    try {
      const values = documentValuesFromForm(props.profile.id, fields);
      const saved = props.record
        ? await updateDocument(props.record.id, { ...values, version: props.record.version })
        : await createDocument(values);
      const updatedCustomValues = await saveRecordCustomValues({
        valueTargetKind: "document",
        definitionTargetKind: "DOCUMENT_TYPE",
        definitionTargetId: saved.type.id,
        recordId: saved.id,
        recordVersion: saved.version,
        draft: fields.customDraft,
        force: Boolean(props.record),
      });
      if (updatedCustomValues) {
        queryClient.setQueryData(["custom-values", "document", saved.id], updatedCustomValues);
      }
      if (fields.file) {
        await uploadAttachment(
          { owner_kind: "DOCUMENT", owner_id: saved.id },
          fields.file,
          () => undefined,
        );
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
    <RecordScreen
      ariaLabel={recordCopy.documentAria}
      closeLabel={messages.tables.inspector.close}
      eyebrow={recordCopy.documentEyebrow}
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
              confirmationLabel={panel.deleteDocConfirmPrompt}
              confirmationWord={common.actions.confirm}
              description={panel.deleteDocDesc}
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
      title={props.record ? props.record.identifier_value : panel.newDoc}
    >
      {error ? <StatusBanner description={error} title={panel.saveError} tone="error" /> : null}
      <CadastroSection
        badge={
          <CadastroSectionBadge modifier="documents">{cadastro.badgeOfficial}</CadastroSectionBadge>
        }
        defaultOpen
        hint={cadastro.sectionDocDataHint}
        icon={<FileText size={18} strokeWidth={1.75} />}
        modifier="documents"
        title={cadastro.sectionDocData}
      >
        <DocumentFormFields
          attachmentsEnabled={attachmentsEnabled.data !== false}
          disabled={!editable}
          documentTypes={props.types.filter(
            (value) => value.active || value.id === fields.docTypeId,
          )}
          fileInputId={fileInputId}
          lockType={props.lockType}
          showDropzone={editable}
          state={fields}
          onChange={(patch) => setFields((current) => ({ ...current, ...patch }))}
        />
      </CadastroSection>
      <CadastroHolderSection
        defaultOpen
        holderName={props.profile.full_name}
        inputId="record-doc-holder"
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
            kind="document"
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

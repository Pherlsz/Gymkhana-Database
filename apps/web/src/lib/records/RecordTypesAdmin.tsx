import { Button, Card, Checkbox, Flex, Input, Select } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";
import { ConfirmDelete } from "../../components/ConfirmDelete";
import { StateCard } from "../../components/StateCard";
import { StatusBanner } from "../../components/StatusBanner";
import { useI18n } from "../../i18n";
import {
  createBillType,
  createDocumentType,
  deleteBillType,
  deleteDocumentType,
  listBillTypes,
  listDocumentTypes,
  updateBillType,
  updateDocumentType,
  type BillType,
  type BillTypeValuesRequest,
  type DocumentType,
  type DocumentTypeValuesRequest,
} from "../api/client";

export function TypesAdminShell(props: {
  title: string;
  canAdminister: boolean;
  onBack: () => void;
  error: unknown;
  list: ReactNode;
  form: ReactNode;
}) {
  const { messages } = useI18n();

  if (!props.canAdminister) {
    return (
      <StateCard
        compact
        description={messages.common.status.restrictedDescription}
        kind="warning"
        title={messages.common.status.restricted}
        onBack={props.onBack}
      />
    );
  }

  return (
    <Flex gap="1rem" vertical>
      <div className="records-section-header">
        <h3>{props.title}</h3>
        <Button onClick={props.onBack}>{messages.common.actions.back}</Button>
      </div>
      {props.error ? (
        <StatusBanner error={props.error} title={messages.records.types.errorTitle} />
      ) : null}
      <div className="types-admin">
        <Card className="type-list">{props.list}</Card>
        <Card className="type-form">{props.form}</Card>
      </div>
    </Flex>
  );
}

type BaseType = {
  id: string;
  version: number;
  technical_key: string;
  label: string;
  active: boolean;
};

type GenericAdminProps<
  T extends BaseType,
  V extends { technical_key: string; label: string; active: boolean },
> = {
  title: string;
  entityName: string;
  deleteTitle: string;
  deleteDescription: string;
  canAdminister: boolean;
  onBack: () => void;
  onNotice: (message: string) => void;
  queryKey: string[];
  listFn: (signal?: AbortSignal) => Promise<{ types: T[] }>;
  createFn: (values: V) => Promise<unknown>;
  updateFn: (id: string, values: V & { version: number }) => Promise<unknown>;
  deleteFn: (id: string, version: number, confirmation: string) => Promise<unknown>;
  defaultValues: V;
  extractValues: (selected: T) => V;
  renderExtraFields?: (values: V, onChange: (patch: Partial<V>) => void) => ReactNode;
};

function RecordTypesAdmin<
  T extends BaseType,
  V extends { technical_key: string; label: string; active: boolean },
>(props: GenericAdminProps<T, V>) {
  const { messages } = useI18n();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: props.queryKey,
    queryFn: ({ signal }) => props.listFn(signal),
  });
  const [selected, setSelected] = useState<T>();
  const [values, setValues] = useState<V>(props.defaultValues);
  const [prevSelected, setPrevSelected] = useState<T | undefined>(undefined);
  const [confirmation, setConfirmation] = useState("");

  if (selected !== prevSelected) {
    setPrevSelected(selected);
    setValues(selected ? props.extractValues(selected) : props.defaultValues);
  }

  const save = useMutation({
    mutationFn: () =>
      selected
        ? props.updateFn(selected.id, { ...values, version: selected.version })
        : props.createFn(values),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: props.queryKey });
      setSelected(undefined);
      setValues(props.defaultValues);
      props.onNotice(`${props.entityName} ${messages.records.types.savedNotice}`);
    },
  });

  const remove = useMutation({
    mutationFn: () => props.deleteFn(selected!.id, selected!.version, confirmation),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: props.queryKey });
      setSelected(undefined);
      setConfirmation("");
      props.onNotice(`${props.entityName} ${messages.records.types.deletedNotice}`);
    },
  });

  const updatePatch = (patch: Partial<V>) => setValues((prev) => ({ ...prev, ...patch }));

  return (
    <TypesAdminShell
      canAdminister={props.canAdminister}
      error={query.error ?? save.error ?? remove.error}
      onBack={props.onBack}
      title={props.title}
      list={query.data?.types.map((type) => (
        <Button
          className="type-list-item"
          key={type.id}
          onClick={() => setSelected(type)}
          type="text"
        >
          <strong>{type.label}</strong>
          <span>
            {`${type.technical_key} · ${type.active ? messages.common.status.active : messages.common.status.inactive}`}
          </span>
        </Button>
      ))}
      form={
        <>
          <label>
            {messages.common.labels.technicalKey}
            <Input
              disabled={Boolean(selected)}
              onChange={(e) => updatePatch({ technical_key: e.target.value } as Partial<V>)}
              value={values.technical_key}
            />
          </label>
          <label>
            {messages.common.labels.name}
            <Input
              onChange={(e) => updatePatch({ label: e.target.value } as Partial<V>)}
              value={values.label}
            />
          </label>
          {props.renderExtraFields ? props.renderExtraFields(values, updatePatch) : null}
          <Checkbox
            checked={values.active}
            className="record-checkbox"
            onChange={(e) => updatePatch({ active: e.target.checked } as Partial<V>)}
          >
            {messages.common.status.active}
          </Checkbox>
          <Flex>
            <Button disabled={save.isPending} onClick={() => save.mutate()}>
              {messages.records.types.saveType}
            </Button>
            <Button
              onClick={() => {
                setSelected(undefined);
                setValues(props.defaultValues);
              }}
            >
              {messages.common.actions.new}
            </Button>
          </Flex>
          {selected ? (
            <ConfirmDelete
              cancelLabel={messages.common.actions.cancel}
              confirmLabel={messages.records.types.deleteType}
              confirmationLabel={messages.records.types.confirmPrompt}
              confirmationWord={messages.common.actions.confirm}
              description={props.deleteDescription}
              onCancel={() => setSelected(undefined)}
              onConfirm={() => remove.mutate()}
              pending={remove.isPending}
              title={props.deleteTitle}
            />
          ) : null}
        </>
      }
    />
  );
}

const DEFAULT_DOC_VALUES: DocumentTypeValuesRequest = {
  technical_key: "",
  label: "",
  active: true,
  uniqueness_policy: "NONE",
  validation_regex: "",
  date_required: false,
};

export function DocumentTypesAdmin(props: {
  canAdminister: boolean;
  onBack: () => void;
  onNotice: (message: string) => void;
}) {
  const { messages } = useI18n();

  return (
    <RecordTypesAdmin<DocumentType, DocumentTypeValuesRequest>
      canAdminister={props.canAdminister}
      createFn={createDocumentType}
      defaultValues={DEFAULT_DOC_VALUES}
      deleteDescription={messages.records.types.deleteDocDesc}
      deleteFn={deleteDocumentType}
      deleteTitle={messages.records.types.deleteDocTitle}
      entityName={messages.records.types.entityDoc}
      extractValues={(sel) => ({
        technical_key: sel.technical_key,
        label: sel.label,
        active: sel.active,
        uniqueness_policy: sel.uniqueness_policy,
        validation_regex: sel.validation_regex,
        date_required: sel.date_required,
      })}
      listFn={listDocumentTypes}
      onBack={props.onBack}
      onNotice={props.onNotice}
      queryKey={["document-types"]}
      title={messages.records.types.documentsTitle}
      updateFn={updateDocumentType}
      renderExtraFields={(values, updatePatch) => (
        <>
          <label>
            {messages.common.labels.uniqueness}
            <Select
              onChange={(value) =>
                updatePatch({
                  uniqueness_policy: value as DocumentTypeValuesRequest["uniqueness_policy"],
                })
              }
              options={[
                { value: "NONE", label: messages.records.types.uniqueness.none },
                { value: "PER_PROFILE", label: messages.records.types.uniqueness.perProfile },
                { value: "GLOBAL_BY_TYPE", label: messages.records.types.uniqueness.global },
              ]}
              value={values.uniqueness_policy}
            />
          </label>
          <label>
            {messages.common.labels.validationRegex}
            <Input
              onChange={(e) => updatePatch({ validation_regex: e.target.value })}
              value={values.validation_regex}
            />
          </label>
          <Checkbox
            checked={values.date_required}
            className="record-checkbox"
            onChange={(e) => updatePatch({ date_required: e.target.checked })}
          >
            {messages.common.labels.dateRequired}
          </Checkbox>
        </>
      )}
    />
  );
}

const DEFAULT_BILL_VALUES: BillTypeValuesRequest = {
  technical_key: "",
  label: "",
  active: true,
};

export function BillTypesAdmin(props: {
  canAdminister: boolean;
  onBack: () => void;
  onNotice: (message: string) => void;
}) {
  const { messages } = useI18n();

  return (
    <RecordTypesAdmin<BillType, BillTypeValuesRequest>
      canAdminister={props.canAdminister}
      createFn={createBillType}
      defaultValues={DEFAULT_BILL_VALUES}
      deleteDescription={messages.records.types.deleteBillDesc}
      deleteFn={deleteBillType}
      deleteTitle={messages.records.types.deleteBillTitle}
      entityName={messages.records.types.entityBill}
      extractValues={(sel) => ({
        technical_key: sel.technical_key,
        label: sel.label,
        active: sel.active,
      })}
      listFn={listBillTypes}
      onBack={props.onBack}
      onNotice={props.onNotice}
      queryKey={["bill-types"]}
      title={messages.records.types.billsTitle}
      updateFn={updateBillType}
    />
  );
}

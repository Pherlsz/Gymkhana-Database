import { Button, Card, Flex, Input, Tag } from "antd";
import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { createColumnHelper } from "@tanstack/react-table";
import { SearchField } from "../../components/SearchField";
import { StateCard } from "../../components/StateCard";
import { StatusBanner } from "../../components/StatusBanner";
import { useI18n } from "../../i18n";
import type { CatalogV1 } from "../../i18n/v1/pt-BR";
import {
  APIRequestError,
  assignBillCurrentUse,
  assignDocumentCurrentUse,
  returnBillCurrentUse,
  returnDocumentCurrentUse,
  type BillRecord,
  type BillValuesRequest,
  type DocumentRecord,
  type DocumentValuesRequest,
  type Profile,
  type ProfileListSearch,
} from "../api/client";
import { errorMessage } from "../formatters";

export const media = ["PHYSICAL", "DIGITAL"] as const;

export type CurrentUseRecord = {
  id: string;
  status?: "AVAILABLE" | "IN_USE" | undefined;
  current_use?: {
    holder_profile_id?: string;
    holder_full_name?: string;
  } | null;
};

export function CurrentUseControls(props: {
  kind: "document" | "bill";
  record: CurrentUseRecord;
  onChanged: (message: string) => Promise<void>;
  framed?: boolean | undefined;
}) {
  const { messages } = useI18n();
  const panel = messages.records.panel;
  const framed = props.framed !== false;
  const [query, setQuery] = useState(props.record.current_use?.holder_full_name ?? "");
  const [holder, setHolder] = useState(props.record.current_use?.holder_profile_id ?? "");
  const [error, setError] = useState<string | null>(null);
  const assign = useMutation({
    mutationFn: () =>
      props.kind === "document"
        ? assignDocumentCurrentUse(props.record.id, holder)
        : assignBillCurrentUse(props.record.id, holder),
    onSuccess: async () =>
      props.onChanged(
        props.record.status === "IN_USE"
          ? panel.currentUseReplacedNotice
          : panel.currentUseAssignedNotice,
      ),
    onError: (caught) => setError(errorMessage(caught)),
  });
  const giveBack = useMutation({
    mutationFn: () =>
      props.kind === "document"
        ? returnDocumentCurrentUse(props.record.id)
        : returnBillCurrentUse(props.record.id),
    onSuccess: async () => props.onChanged(panel.currentUseReturnedNotice),
    onError: (caught) => setError(errorMessage(caught)),
  });
  const body = (
    <Flex vertical gap="0.75rem">
      {framed ? <strong>{panel.currentUseTitle}</strong> : null}
      {error ? (
        <StatusBanner description={error} title={panel.currentUseError} tone="error" />
      ) : null}
      <label>
        {panel.holderLabel}
        <SearchField
          label={panel.holderLabel}
          lookup
          mode="suggest"
          placeholder={panel.searchHolderPlaceholder}
          value={query}
          onChange={setQuery}
          onPick={(id, name) => {
            setHolder(id);
            setQuery(name);
          }}
        />
      </label>
      <Flex>
        <Button disabled={!holder || assign.isPending} onClick={() => assign.mutate()}>
          {props.record.status === "IN_USE" ? panel.substituteHolder : panel.assignUse}
        </Button>
        {props.record.status === "IN_USE" ? (
          <Button disabled={giveBack.isPending} onClick={() => giveBack.mutate()}>
            {panel.returnUse}
          </Button>
        ) : null}
      </Flex>
    </Flex>
  );
  if (!framed) return body;
  return <Card className="current-use">{body}</Card>;
}

export function EditorHeader({
  title,
  onClose,
  closeLabel,
}: {
  title: string;
  onClose: () => void;
  closeLabel: string;
}) {
  return (
    <div className="record-editor__header">
      <h3>{title}</h3>
      <Button onClick={onClose}>{closeLabel}</Button>
    </div>
  );
}

export function EditorActions<T extends DocumentRecord | BillRecord>(props: {
  editable: boolean;
  saving: boolean;
  record: T | undefined;
  onSave: () => void;
  onEdit: () => void;
  onDuplicate: (value: T) => void;
}) {
  const { messages } = useI18n();
  const panel = messages.records.panel;
  return (
    <Flex className="record-editor__actions">
      {props.editable ? (
        <Button disabled={props.saving} onClick={props.onSave}>
          {props.saving ? panel.saving : panel.save}
        </Button>
      ) : (
        <Button onClick={props.onEdit}>{panel.edit}</Button>
      )}
      {props.record ? (
        <Button onClick={() => props.onDuplicate(props.record!)}>{panel.duplicate}</Button>
      ) : null}
    </Flex>
  );
}

export function MissingRecord({ onClose }: { onClose: () => void }) {
  const { messages } = useI18n();
  const panel = messages.records.panel;
  return (
    <StateCard
      compact
      description={panel.notFoundHint}
      kind="error"
      title={panel.notFound}
      backLabel={messages.common.actions.close}
      onBack={onClose}
    />
  );
}

export function RecordStatus({ value }: { value?: "AVAILABLE" | "IN_USE" | "" }) {
  const { messages } = useI18n();
  const panel = messages.records.panel;
  if (value !== "AVAILABLE" && value !== "IN_USE") return null;
  return (
    <Tag color={value === "IN_USE" ? "info" : "success"}>
      {value === "IN_USE" ? panel.statusInUse : panel.statusAvailable}
    </Tag>
  );
}

export function RecordCard(props: {
  title: string;
  lines: string[];
  status?: "AVAILABLE" | "IN_USE" | "";
  onOpen: () => void;
}) {
  const { messages } = useI18n();
  return (
    <Card className="record-card">
      <Flex vertical gap="0.5rem">
        <strong>{props.title}</strong>
        {props.lines.map((line) => (
          <span key={line}>{line}</span>
        ))}
        <RecordStatus value={props.status ?? ""} />
        <Button onClick={props.onOpen}>{messages.records.panel.open}</Button>
      </Flex>
    </Card>
  );
}

export function RecordInlineInput(props: {
  value: string;
  ariaLabel: string;
  type?: string;
  inputMode?: "decimal";
  onSave: (value: string) => Promise<void>;
}) {
  const [value, setValue] = useState(props.value);
  const [prevValue, setPrevValue] = useState(props.value);
  if (props.value !== prevValue) {
    setPrevValue(props.value);
    setValue(props.value);
  }
  return (
    <Input
      aria-label={props.ariaLabel}
      className="inline-editor"
      {...(props.inputMode ? { inputMode: props.inputMode } : {})}
      {...(props.type ? { type: props.type as any } : {})}
      value={value}
      onChange={(event) => setValue(event.target.value)}
      onBlur={() => {
        if (value !== props.value) void props.onSave(value).catch(() => setValue(props.value));
      }}
    />
  );
}

export function documentValues(value: DocumentRecord): DocumentValuesRequest {
  return {
    owner_profile_id: value.owner_profile_id,
    document_type_id: value.document_type_id,
    identifier_value: value.identifier_value,
    document_date: value.document_date,
    notes: value.notes,
    medium: value.medium,
    ...physicalCustody(value.medium, value.idle_custody),
    ...(value.valid_until ? { valid_until: value.valid_until } : {}),
  };
}

export function billValues(value: BillRecord): BillValuesRequest {
  return {
    owner_profile_id: value.owner_profile_id,
    bill_type_id: value.bill_type_id,
    printed_holder_name: value.printed_holder_name,
    printed_address: value.printed_address,
    reference_value: value.reference_value,
    competence: value.competence,
    amount: value.amount,
    currency: value.currency,
    notes: value.notes,
    medium: value.medium,
    ...physicalCustody(value.medium, value.idle_custody),
  };
}

export function recordSearch<T>(search: ProfileListSearch, prefix: string): T {
  const result: Record<string, unknown> = { q: search.q };
  for (const [key, value] of Object.entries(search)) {
    if (key.startsWith(`${prefix}_`)) result[key] = value;
  }
  return result as T;
}

export function profileAddress(value: Profile) {
  return [
    value.address.street,
    value.address.number,
    value.address.complement,
    value.address.neighborhood,
    value.address.city,
    value.address.state,
    value.address.postal_code,
  ]
    .filter(Boolean)
    .join(", ");
}

export function mediumLabel(value: string, messages: CatalogV1) {
  return value === "DIGITAL"
    ? messages.records.panel.mediaDigital
    : messages.records.panel.mediaPhysical;
}

export function custodyLabel(value: string, messages: CatalogV1) {
  if (value === "OWNER") return messages.records.panel.custodyOwner;
  if (value === "ORGANIZATION") return messages.records.panel.custodyOrg;
  return "";
}

export function physicalCustody(
  medium: "PHYSICAL" | "DIGITAL",
  custody?: "ORGANIZATION" | "OWNER",
): { idle_custody?: "ORGANIZATION" | "OWNER" } {
  if (medium !== "PHYSICAL") return {};
  return { idle_custody: custody === "OWNER" ? "OWNER" : "ORGANIZATION" };
}

export function withRecordMedium<T extends { medium: string; idle_custody?: string }>(
  values: T,
  medium: T["medium"],
): T {
  if (medium === "DIGITAL") {
    const next = { ...values, medium };
    delete next.idle_custody;
    return next;
  }
  return { ...values, medium, idle_custody: values.idle_custody ?? "ORGANIZATION" };
}

export function conflictMessage(error: unknown, messages: CatalogV1) {
  return error instanceof APIRequestError && error.status === 409
    ? messages.records.panel.conflictNotice
    : errorMessage(error);
}

const documentColumn = createColumnHelper<DocumentRecord>();
const billColumn = createColumnHelper<BillRecord>();

export function createDocumentColumns(
  onSave: (value: DocumentRecord, patch: Partial<DocumentValuesRequest>) => Promise<void>,
  onOpen: (value: DocumentRecord) => void,
  messages: CatalogV1,
) {
  const common = messages.common;
  const panel = messages.records.panel;
  return [
    documentColumn.accessor((value) => value.type.label, {
      id: "type",
      header: common.labels.type,
    }),
    documentColumn.accessor("identifier_value", {
      header: common.labels.identifier,
      cell: ({ row }) => (
        <RecordInlineInput
          ariaLabel={`${common.labels.identifier} de ${row.original.type.label}`}
          value={row.original.identifier_value}
          onSave={(next) => onSave(row.original, { identifier_value: next })}
        />
      ),
    }),
    documentColumn.accessor("document_date", {
      header: common.labels.date,
      cell: ({ row }) => (
        <RecordInlineInput
          ariaLabel={`${common.labels.date} de ${row.original.type.label}`}
          type="date"
          value={row.original.document_date}
          onSave={(next) => onSave(row.original, { document_date: next })}
        />
      ),
    }),
    documentColumn.accessor("medium", {
      header: common.labels.medium,
      cell: ({ getValue }) => mediumLabel(getValue(), messages),
    }),
    documentColumn.accessor("idle_custody", {
      header: common.labels.custody,
      cell: ({ getValue }) => custodyLabel(getValue() ?? "", messages),
    }),
    documentColumn.accessor("valid_until", {
      header: common.labels.validUntil,
      cell: ({ row }) => (
        <RecordInlineInput
          ariaLabel={`${common.labels.validUntil} de ${row.original.type.label}`}
          type="date"
          value={row.original.valid_until ?? ""}
          onSave={(next) => onSave(row.original, { valid_until: next })}
        />
      ),
    }),
    documentColumn.accessor("status", {
      header: common.labels.status,
      cell: ({ getValue }) => <RecordStatus value={getValue() ?? ""} />,
    }),
    documentColumn.display({
      id: "actions",
      header: "",
      cell: ({ row }) => <Button onClick={() => onOpen(row.original)}>{panel.open}</Button>,
    }),
  ];
}

export function createBillColumns(
  onSave: (value: BillRecord, patch: Partial<BillValuesRequest>) => Promise<void>,
  onOpen: (value: BillRecord) => void,
  messages: CatalogV1,
) {
  const common = messages.common;
  const panel = messages.records.panel;
  return [
    billColumn.accessor((value) => value.type.label, { id: "type", header: common.labels.type }),
    billColumn.accessor("reference_value", {
      header: common.labels.reference,
      cell: ({ row }) => (
        <RecordInlineInput
          ariaLabel={`${common.labels.reference} de ${row.original.type.label}`}
          value={row.original.reference_value}
          onSave={(next) => onSave(row.original, { reference_value: next })}
        />
      ),
    }),
    billColumn.accessor("competence", {
      header: common.labels.competence,
      cell: ({ row }) => (
        <RecordInlineInput
          ariaLabel={`${common.labels.competence} de ${row.original.type.label}`}
          value={row.original.competence}
          onSave={(next) => onSave(row.original, { competence: next })}
        />
      ),
    }),
    billColumn.accessor("amount", {
      header: common.labels.amount,
      cell: ({ row }) => (
        <RecordInlineInput
          ariaLabel={`${common.labels.amount} de ${row.original.type.label}`}
          inputMode="decimal"
          value={row.original.amount}
          onSave={(next) => onSave(row.original, { amount: next })}
        />
      ),
    }),
    billColumn.accessor("medium", {
      header: common.labels.medium,
      cell: ({ getValue }) => mediumLabel(getValue(), messages),
    }),
    billColumn.accessor("idle_custody", {
      header: common.labels.custody,
      cell: ({ getValue }) => custodyLabel(getValue() ?? "", messages),
    }),
    billColumn.accessor("status", {
      header: common.labels.status,
      cell: ({ getValue }) => <RecordStatus value={getValue() ?? ""} />,
    }),
    billColumn.display({
      id: "actions",
      header: "",
      cell: ({ row }) => <Button onClick={() => onOpen(row.original)}>{panel.open}</Button>,
    }),
  ];
}

import { Alert, Button, Card, Checkbox, Flex, Tag } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createColumnHelper } from "@tanstack/react-table";
import { useEffect, useState } from "react";
import { AttachmentsPanel } from "./AttachmentsPanel";
import { ConfirmDelete } from "./components/ConfirmDelete";
import {
  RecordCustomFieldsSection,
  recordCustomFieldError,
  saveRecordCustomValues,
  useSeedRecordDraft,
  type CustomDraftValue,
} from "./RecordCustomFields";
import { CadastroOcrSection } from "./lib/cadastro/CadastroPanel";
import { SearchField } from "./components/SearchField";
import { DataGrid, DataGridPagination } from "./DataGrid";
import { BillFilters, DocumentFilters } from "./lib/records/RecordFilters";
import {
  APIRequestError,
  assignBillCurrentUse,
  assignDocumentCurrentUse,
  createBill,
  createBillType,
  createDocument,
  createDocumentType,
  deleteBill,
  deleteBillType,
  deleteDocument,
  deleteDocumentType,
  duplicateBill,
  duplicateDocument,
  listBillTypes,
  listBills,
  listDocumentTypes,
  listDocuments,
  returnBillCurrentUse,
  returnDocumentCurrentUse,
  updateBill,
  updateBillType,
  updateDocument,
  updateDocumentType,
  type BillRecord,
  type BillType,
  type BillTypeValuesRequest,
  type BillValuesRequest,
  type DocumentRecord,
  type DocumentType,
  type DocumentTypeValuesRequest,
  type DocumentValuesRequest,
  type Profile,
  type ProfileListSearch,
  type UserRole,
} from "./lib/api/client";

type Props = {
  profile: Profile;
  role: UserRole;
  section: "documents" | "bills";
  search: ProfileListSearch;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
  onNotice: (message: string) => void;
};

const media = ["PHYSICAL", "DIGITAL"] as const;
const documentColumn = createColumnHelper<DocumentRecord>();
const billColumn = createColumnHelper<BillRecord>();

export function ProfileRecordsPanel(props: Props) {
  return props.section === "documents" ? (
    <DocumentsSection {...props} />
  ) : (
    <BillsSection {...props} />
  );
}

function DocumentsSection({ profile, role, search, onSearch, onNotice }: Props) {
  const queryClient = useQueryClient();
  const types = useQuery({
    queryKey: ["document-types"],
    queryFn: ({ signal }) => listDocumentTypes(signal),
  });
  const records = useQuery({
    queryKey: ["documents", profile.id, documentSearch(search)],
    queryFn: ({ signal }) => listDocuments(profile.id, documentSearch(search), signal),
  });
  const selected = records.data?.documents.find((value) => value.id === search.document_selected);
  const canAdministerTypes = role === "ADMIN" || role === "SUPERADMIN";
  const canDelete = canAdministerTypes;
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["documents", profile.id] }),
      queryClient.invalidateQueries({ queryKey: ["document-types"] }),
    ]);
  };
  const duplicateMutation = useMutation({
    mutationFn: duplicateDocument,
    onSuccess: async (value) => {
      await refresh();
      onNotice("Documento duplicado. Revise a cópia antes de continuar.");
      onSearch({ document_selected: value.id, document_mode: "edit" });
    },
  });
  const deleteMutation = useMutation({
    mutationFn: ({ value, confirmation }: { value: DocumentRecord; confirmation: string }) =>
      deleteDocument(value.id, value.version, confirmation),
    onSuccess: async () => {
      await refresh();
      onNotice("Documento excluído permanentemente.");
      onSearch({ document_selected: undefined, document_mode: undefined });
    },
  });
  const inlineUpdate = async (value: DocumentRecord, patch: Partial<DocumentValuesRequest>) => {
    try {
      await updateDocument(value.id, {
        owner_profile_id: value.owner_profile_id,
        document_type_id: value.document_type_id,
        identifier_value: value.identifier_value,
        document_date: value.document_date,
        notes: value.notes,
        medium: value.medium,
        ...physicalCustody(value.medium, value.idle_custody),
        ...(value.valid_until ? { valid_until: value.valid_until } : {}),
        version: value.version,
        ...patch,
      });
      onNotice("Documento atualizado.");
      await refresh();
    } catch (error) {
      await refresh();
      onNotice(conflictMessage(error));
      throw error;
    }
  };
  const totalPages = Math.max(
    1,
    Math.ceil((records.data?.page.total ?? 0) / search.document_limit),
  );

  if (search.document_mode === "types") {
    return (
      <DocumentTypesAdmin
        canAdminister={canAdministerTypes}
        onBack={() => onSearch({ document_mode: undefined })}
        onNotice={onNotice}
      />
    );
  }

  return (
    <Flex vertical gap="1rem">
      <SectionHeader
        title="Documentos"
        description="Identificadores preservam zeros à esquerda e caracteres alfanuméricos."
        onCreate={() => onSearch({ document_selected: undefined, document_mode: "create" })}
        {...(canAdministerTypes ? { onTypes: () => onSearch({ document_mode: "types" }) } : {})}
      />
      {records.isError ? (
        <RecordsError title="Não foi possível carregar documentos" error={records.error} />
      ) : null}
      <DocumentFilters search={search} types={types.data?.types ?? []} onSearch={onSearch} />
      <DataGrid
        caption={`Documentos de ${profile.full_name}`}
        cardsClassName="records-cards"
        className="records-list"
        columns={createDocumentColumns(inlineUpdate, (value) =>
          onSearch({ document_selected: value.id, document_mode: "view" }),
        )}
        data={records.data?.documents ?? []}
        emptyLabel="Nenhum documento cadastrado para esta pessoa."
        getRowId={(value) => value.id}
        loading={records.isLoading}
        loadingLabel="Carregando documentos..."
        renderCard={(value) => (
          <RecordCard
            key={value.id}
            title={`${value.type.label} · ${value.identifier_value}`}
            lines={[value.document_date || "Data não informada", mediumLabel(value.medium)]}
            status={value.status ?? ""}
            onOpen={() => onSearch({ document_selected: value.id, document_mode: "view" })}
          />
        )}
        selectedRowId={search.document_selected}
        tableClassName="records-table"
        tableWrapClassName="records-table-wrap"
      />
      <DataGridPagination
        page={search.document_page}
        totalPages={totalPages}
        total={records.data?.page.total ?? 0}
        label="documentos"
        onPage={(page) => onSearch({ document_page: page })}
      />
      {search.document_mode === "create" ||
      search.document_mode === "view" ||
      search.document_mode === "edit" ? (
        <DocumentEditor
          key={`${search.document_mode}:${selected?.id ?? "new"}:${selected?.version ?? 0}`}
          mode={search.document_mode}
          profile={profile}
          record={selected}
          types={types.data?.types ?? []}
          canDelete={canDelete}
          pending={duplicateMutation.isPending || deleteMutation.isPending}
          onClose={() => onSearch({ document_selected: undefined, document_mode: undefined })}
          onEdit={() => onSearch({ document_mode: "edit" })}
          onSaved={async (value, message) => {
            await refresh();
            onNotice(message);
            onSearch({ document_selected: value.id, document_mode: "view" });
          }}
          onDuplicate={(value) => duplicateMutation.mutate(value.id)}
          onDelete={(value, confirmation) => deleteMutation.mutate({ value, confirmation })}
        />
      ) : null}
      {selected && (search.document_mode === "view" || search.document_mode === "edit") ? (
        <>
          <AttachmentsPanel
            key={`document-attachments:${selected.id}`}
            owner={{ owner_kind: "DOCUMENT", owner_id: selected.id }}
            title="Anexos do documento"
            description="Arquivos privados vinculados exclusivamente a este documento."
          />
          <CadastroOcrSection owner={{ owner_kind: "DOCUMENT", owner_id: selected.id }} />
        </>
      ) : null}
    </Flex>
  );
}

function BillsSection({ profile, role, search, onSearch, onNotice }: Props) {
  const queryClient = useQueryClient();
  const types = useQuery({
    queryKey: ["bill-types"],
    queryFn: ({ signal }) => listBillTypes(signal),
  });
  const records = useQuery({
    queryKey: ["bills", profile.id, billSearch(search)],
    queryFn: ({ signal }) => listBills(profile.id, billSearch(search), signal),
  });
  const selected = records.data?.bills.find((value) => value.id === search.bill_selected);
  const canAdministerTypes = role === "ADMIN" || role === "SUPERADMIN";
  const canDelete = canAdministerTypes;
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["bills", profile.id] }),
      queryClient.invalidateQueries({ queryKey: ["bill-types"] }),
    ]);
  };
  const duplicateMutation = useMutation({
    mutationFn: duplicateBill,
    onSuccess: async (value) => {
      await refresh();
      onNotice("Conta/comprovante duplicado. Revise a cópia antes de continuar.");
      onSearch({ bill_selected: value.id, bill_mode: "edit" });
    },
  });
  const deleteMutation = useMutation({
    mutationFn: ({ value, confirmation }: { value: BillRecord; confirmation: string }) =>
      deleteBill(value.id, value.version, confirmation),
    onSuccess: async () => {
      await refresh();
      onNotice("Conta/comprovante excluído permanentemente.");
      onSearch({ bill_selected: undefined, bill_mode: undefined });
    },
  });
  const inlineUpdate = async (value: BillRecord, patch: Partial<BillValuesRequest>) => {
    try {
      await updateBill(value.id, {
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
        version: value.version,
        ...patch,
      });
      onNotice("Conta/comprovante atualizado.");
      await refresh();
    } catch (error) {
      await refresh();
      onNotice(conflictMessage(error));
      throw error;
    }
  };
  const totalPages = Math.max(1, Math.ceil((records.data?.page.total ?? 0) / search.bill_limit));

  if (search.bill_mode === "types") {
    return (
      <BillTypesAdmin
        canAdminister={canAdministerTypes}
        onBack={() => onSearch({ bill_mode: undefined })}
        onNotice={onNotice}
      />
    );
  }

  return (
    <Flex vertical gap="1rem">
      <SectionHeader
        title="Contas e comprovantes"
        description="Os dados impressos permanecem independentes do cadastro atual da pessoa."
        onCreate={() => onSearch({ bill_selected: undefined, bill_mode: "create" })}
        {...(canAdministerTypes ? { onTypes: () => onSearch({ bill_mode: "types" }) } : {})}
      />
      {records.isError ? (
        <RecordsError title="Não foi possível carregar contas" error={records.error} />
      ) : null}
      <BillFilters search={search} types={types.data?.types ?? []} onSearch={onSearch} />
      <DataGrid
        caption={`Contas e comprovantes de ${profile.full_name}`}
        cardsClassName="records-cards"
        className="records-list"
        columns={createBillColumns(inlineUpdate, (value) =>
          onSearch({ bill_selected: value.id, bill_mode: "view" }),
        )}
        data={records.data?.bills ?? []}
        emptyLabel="Nenhuma conta ou comprovante cadastrado para esta pessoa."
        getRowId={(value) => value.id}
        loading={records.isLoading}
        loadingLabel="Carregando contas..."
        renderCard={(value) => (
          <RecordCard
            key={value.id}
            title={`${value.type.label} · ${value.reference_value}`}
            lines={[
              mediumLabel(value.medium),
              value.competence,
              `${value.currency} ${value.amount}`,
            ]}
            status={value.status ?? ""}
            onOpen={() => onSearch({ bill_selected: value.id, bill_mode: "view" })}
          />
        )}
        selectedRowId={search.bill_selected}
        tableClassName="records-table"
        tableWrapClassName="records-table-wrap"
      />
      <DataGridPagination
        page={search.bill_page}
        totalPages={totalPages}
        total={records.data?.page.total ?? 0}
        label="contas/comprovantes"
        onPage={(page) => onSearch({ bill_page: page })}
      />
      {search.bill_mode === "create" ||
      search.bill_mode === "view" ||
      search.bill_mode === "edit" ? (
        <BillEditor
          key={`${search.bill_mode}:${selected?.id ?? "new"}:${selected?.version ?? 0}`}
          mode={search.bill_mode}
          profile={profile}
          record={selected}
          types={types.data?.types ?? []}
          canDelete={canDelete}
          pending={duplicateMutation.isPending || deleteMutation.isPending}
          onClose={() => onSearch({ bill_selected: undefined, bill_mode: undefined })}
          onEdit={() => onSearch({ bill_mode: "edit" })}
          onSaved={async (value, message) => {
            await refresh();
            onNotice(message);
            onSearch({ bill_selected: value.id, bill_mode: "view" });
          }}
          onDuplicate={(value) => duplicateMutation.mutate(value.id)}
          onDelete={(value, confirmation) => deleteMutation.mutate({ value, confirmation })}
        />
      ) : null}
      {selected && (search.bill_mode === "view" || search.bill_mode === "edit") ? (
        <>
          <AttachmentsPanel
            key={`bill-attachments:${selected.id}`}
            owner={{ owner_kind: "BILL", owner_id: selected.id }}
            title="Anexos da conta ou comprovante"
            description="Arquivos privados vinculados exclusivamente a este registro."
          />
          <CadastroOcrSection owner={{ owner_kind: "BILL", owner_id: selected.id }} />
        </>
      ) : null}
    </Flex>
  );
}

function SectionHeader(props: {
  title: string;
  description: string;
  onCreate: () => void;
  onTypes?: () => void;
}) {
  return (
    <div className="records-section-header">
      <div>
        <h3>{props.title}</h3>
        <p>{props.description}</p>
      </div>
      <Flex>
        {props.onTypes ? <Button onClick={props.onTypes}>Administrar tipos</Button> : null}
        <Button onClick={props.onCreate}>Novo registro</Button>
      </Flex>
    </div>
  );
}

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
      await props.onSaved(saved, props.record ? "Documento atualizado." : "Documento criado.");
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
        title={
          props.record
            ? `${props.record.type.label} · ${props.record.identifier_value}`
            : "Novo documento"
        }
        onClose={props.onClose}
      />
      {error ? (
        <Alert message="Não foi possível salvar" type="error" description={<>{error}</>} />
      ) : null}
      <div className="record-form">
        <label>
          Tipo
          <select
            disabled={!editable || Boolean(props.lockType)}
            value={values.document_type_id}
            onChange={(event) => setValues({ ...values, document_type_id: event.target.value })}
          >
            <option value="">Selecione</option>
            {props.types
              .filter((value) => value.active || value.id === values.document_type_id)
              .map((value) => (
                <option key={value.id} value={value.id}>
                  {value.label}
                </option>
              ))}
          </select>
        </label>
        <label>
          Identificador
          <input
            disabled={!editable}
            value={values.identifier_value}
            onChange={(event) => setValues({ ...values, identifier_value: event.target.value })}
          />
        </label>
        <label>
          Data
          <input
            disabled={!editable}
            type="date"
            value={values.document_date}
            onChange={(event) => setValues({ ...values, document_date: event.target.value })}
          />
        </label>
        <label>
          Validade
          <input
            disabled={!editable}
            type="date"
            value={values.valid_until ?? ""}
            onChange={(event) => setValues({ ...values, valid_until: event.target.value })}
          />
        </label>
        <label>
          Meio
          <select
            disabled={!editable}
            value={values.medium}
            onChange={(event) =>
              setValues(withMedium(values, event.target.value as DocumentValuesRequest["medium"]))
            }
          >
            {media.map((value) => (
              <option key={value} value={value}>
                {mediumLabel(value)}
              </option>
            ))}
          </select>
        </label>
        {values.medium === "PHYSICAL" ? (
          <label>
            Guarda
            <select
              disabled={!editable}
              value={values.idle_custody ?? "ORGANIZATION"}
              onChange={(event) =>
                setValues({
                  ...values,
                  idle_custody: event.target.value as NonNullable<
                    DocumentValuesRequest["idle_custody"]
                  >,
                })
              }
            >
              <option value="ORGANIZATION">Organização</option>
              <option value="OWNER">Com o dono</option>
            </select>
          </label>
        ) : null}
        <label className="record-form__wide">
          Observações
          <textarea
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
          cancelLabel="Cancelar"
          confirmLabel="Excluir permanentemente"
          confirmationLabel="Digite Confirmar para excluir este documento."
          confirmationWord="Confirmar"
          description="A exclusão removerá permanentemente este documento e não poderá ser desfeita."
          pending={props.pending}
          title="Exclusão permanente"
          onCancel={props.onClose}
          onConfirm={(word) => props.onDelete(props.record!, word)}
        />
      ) : null}
    </Card>
  );
}

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
      await props.onSaved(
        saved,
        props.record ? "Conta/comprovante atualizado." : "Conta/comprovante criado.",
      );
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
        title={
          props.record
            ? `${props.record.type.label} · ${props.record.reference_value}`
            : "Nova conta ou comprovante"
        }
        onClose={props.onClose}
      />
      {error ? (
        <Alert message="Não foi possível salvar" type="error" description={<>{error}</>} />
      ) : null}
      <div className="record-form">
        <label>
          Tipo
          <select
            disabled={!editable || Boolean(props.lockType)}
            value={values.bill_type_id}
            onChange={(event) => setValues({ ...values, bill_type_id: event.target.value })}
          >
            <option value="">Selecione</option>
            {props.types
              .filter((value) => value.active || value.id === values.bill_type_id)
              .map((value) => (
                <option key={value.id} value={value.id}>
                  {value.label}
                </option>
              ))}
          </select>
        </label>
        <label>
          Referência
          <input
            disabled={!editable}
            value={values.reference_value}
            onChange={(event) => setValues({ ...values, reference_value: event.target.value })}
          />
        </label>
        <label>
          Competência
          <input
            disabled={!editable}
            placeholder="AAAA-MM"
            value={values.competence}
            onChange={(event) => setValues({ ...values, competence: event.target.value })}
          />
        </label>
        <label>
          Valor
          <input
            disabled={!editable}
            inputMode="decimal"
            value={values.amount}
            onChange={(event) => setValues({ ...values, amount: event.target.value })}
          />
        </label>
        <label>
          Moeda
          <input
            disabled={!editable}
            maxLength={3}
            value={values.currency}
            onChange={(event) =>
              setValues({ ...values, currency: event.target.value.toUpperCase() })
            }
          />
        </label>
        <label>
          Meio
          <select
            disabled={!editable}
            value={values.medium}
            onChange={(event) =>
              setValues(withBillMedium(values, event.target.value as BillValuesRequest["medium"]))
            }
          >
            {media.map((value) => (
              <option key={value} value={value}>
                {mediumLabel(value)}
              </option>
            ))}
          </select>
        </label>
        {values.medium === "PHYSICAL" ? (
          <label>
            Guarda
            <select
              disabled={!editable}
              value={values.idle_custody ?? "ORGANIZATION"}
              onChange={(event) =>
                setValues({
                  ...values,
                  idle_custody: event.target.value as NonNullable<
                    BillValuesRequest["idle_custody"]
                  >,
                })
              }
            >
              <option value="ORGANIZATION">Organização</option>
              <option value="OWNER">Com o dono</option>
            </select>
          </label>
        ) : null}
        <label className="record-form__wide">
          Titular impresso
          <input
            disabled={!editable}
            value={values.printed_holder_name}
            onChange={(event) => setValues({ ...values, printed_holder_name: event.target.value })}
          />
        </label>
        <label className="record-form__wide">
          Endereço impresso
          <input
            disabled={!editable}
            value={values.printed_address}
            onChange={(event) => setValues({ ...values, printed_address: event.target.value })}
          />
        </label>
        <label className="record-form__wide">
          Observações
          <textarea
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
          cancelLabel="Cancelar"
          confirmLabel="Excluir permanentemente"
          confirmationLabel="Digite Confirmar para excluir este registro."
          confirmationWord="Confirmar"
          description="A exclusão removerá permanentemente este registro de conta/comprovante e não poderá ser desfeita."
          pending={props.pending}
          title="Exclusão permanente"
          onCancel={props.onClose}
          onConfirm={(word) => props.onDelete(props.record!, word)}
        />
      ) : null}
    </Card>
  );
}

function CurrentUseControls(
  props:
    | { kind: "document"; record: DocumentRecord; onChanged: (message: string) => Promise<void> }
    | { kind: "bill"; record: BillRecord; onChanged: (message: string) => Promise<void> },
) {
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
        props.record.status === "IN_USE" ? "Pessoa em uso substituída." : "Uso atual atribuído.",
      ),
    onError: (caught) => setError(errorMessage(caught)),
  });
  const giveBack = useMutation({
    mutationFn: () =>
      props.kind === "document"
        ? returnDocumentCurrentUse(props.record.id)
        : returnBillCurrentUse(props.record.id),
    onSuccess: async () => props.onChanged("Registro devolvido e disponibilizado."),
    onError: (caught) => setError(errorMessage(caught)),
  });
  return (
    <Card className="current-use">
      <Flex vertical gap="0.75rem">
        <strong>Uso atual</strong>
        {error ? (
          <Alert message="Não foi possível alterar o uso" type="error" description={<>{error}</>} />
        ) : null}
        <label>
          Pessoa em uso
          <SearchField
            label="Pessoa em uso"
            lookup
            mode="suggest"
            placeholder="Buscar pessoa…"
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
            {props.record.status === "IN_USE" ? "Substituir pessoa" : "Atribuir uso"}
          </Button>
          {props.record.status === "IN_USE" ? (
            <Button disabled={giveBack.isPending} onClick={() => giveBack.mutate()}>
              Devolver
            </Button>
          ) : null}
        </Flex>
      </Flex>
    </Card>
  );
}

function DocumentTypesAdmin({
  canAdminister,
  onBack,
  onNotice,
}: {
  canAdminister: boolean;
  onBack: () => void;
  onNotice: (message: string) => void;
}) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ["document-types"],
    queryFn: ({ signal }) => listDocumentTypes(signal),
  });
  const [selected, setSelected] = useState<DocumentType>();
  const [values, setValues] = useState<DocumentTypeValuesRequest>({
    technical_key: "",
    label: "",
    active: true,
    uniqueness_policy: "NONE",
    validation_regex: "",
    date_required: false,
  });
  const [confirmation, setConfirmation] = useState("");
  useEffect(() => {
    if (selected)
      setValues({
        technical_key: selected.technical_key,
        label: selected.label,
        active: selected.active,
        uniqueness_policy: selected.uniqueness_policy,
        validation_regex: selected.validation_regex,
        date_required: selected.date_required,
      });
  }, [selected]);
  const save = useMutation({
    mutationFn: () =>
      selected
        ? updateDocumentType(selected.id, { ...values, version: selected.version })
        : createDocumentType(values),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["document-types"] });
      setSelected(undefined);
      setValues({
        technical_key: "",
        label: "",
        active: true,
        uniqueness_policy: "NONE",
        validation_regex: "",
        date_required: false,
      });
      onNotice("Tipo de documento salvo.");
    },
  });
  const remove = useMutation({
    mutationFn: () => deleteDocumentType(selected!.id, selected!.version, confirmation),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["document-types"] });
      setSelected(undefined);
      setConfirmation("");
      onNotice("Tipo de documento excluído.");
    },
  });
  return (
    <TypesAdminShell
      title="Tipos de documento"
      canAdminister={canAdminister}
      onBack={onBack}
      error={query.error ?? save.error ?? remove.error}
      list={query.data?.types.map((value) => (
        <button className="type-list-item" key={value.id} onClick={() => setSelected(value)}>
          <strong>{value.label}</strong>
          <span>
            {value.technical_key} · {value.active ? "Ativo" : "Inativo"}
          </span>
        </button>
      ))}
      form={
        <>
          <label>
            Chave técnica
            <input
              disabled={Boolean(selected)}
              value={values.technical_key}
              onChange={(event) => setValues({ ...values, technical_key: event.target.value })}
            />
          </label>
          <label>
            Nome
            <input
              value={values.label}
              onChange={(event) => setValues({ ...values, label: event.target.value })}
            />
          </label>
          <label>
            Unicidade
            <select
              value={values.uniqueness_policy}
              onChange={(event) =>
                setValues({
                  ...values,
                  uniqueness_policy: event.target
                    .value as DocumentTypeValuesRequest["uniqueness_policy"],
                })
              }
            >
              <option value="NONE">Sem restrição</option>
              <option value="PER_PROFILE">Por pessoa</option>
              <option value="GLOBAL_BY_TYPE">Global por tipo</option>
            </select>
          </label>
          <label>
            Expressão de validação
            <input
              value={values.validation_regex}
              onChange={(event) => setValues({ ...values, validation_regex: event.target.value })}
            />
          </label>
          <Checkbox
            checked={values.date_required}
            className="record-checkbox"
            onChange={(event) => setValues({ ...values, date_required: event.target.checked })}
          >
            Data obrigatória
          </Checkbox>
          <Checkbox
            checked={values.active}
            className="record-checkbox"
            onChange={(event) => setValues({ ...values, active: event.target.checked })}
          >
            Ativo
          </Checkbox>
          <Flex>
            <Button disabled={save.isPending} onClick={() => save.mutate()}>
              Salvar tipo
            </Button>
            <Button
              onClick={() => {
                setSelected(undefined);
                setValues({
                  technical_key: "",
                  label: "",
                  active: true,
                  uniqueness_policy: "NONE",
                  validation_regex: "",
                  date_required: false,
                });
              }}
            >
              Novo
            </Button>
          </Flex>
          {selected ? (
            <ConfirmDelete
              cancelLabel="Cancelar"
              confirmLabel="Excluir tipo"
              confirmationLabel="Digite Confirmar para excluir o tipo não utilizado."
              confirmationWord="Confirmar"
              description="Esta ação exclui a definição do tipo de documento do sistema."
              pending={remove.isPending}
              title="Excluir tipo de documento"
              onCancel={() => setSelected(undefined)}
              onConfirm={() => remove.mutate()}
            />
          ) : null}
        </>
      }
    />
  );
}

function BillTypesAdmin({
  canAdminister,
  onBack,
  onNotice,
}: {
  canAdminister: boolean;
  onBack: () => void;
  onNotice: (message: string) => void;
}) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ["bill-types"],
    queryFn: ({ signal }) => listBillTypes(signal),
  });
  const [selected, setSelected] = useState<BillType>();
  const [values, setValues] = useState<BillTypeValuesRequest>({
    technical_key: "",
    label: "",
    active: true,
  });
  const [confirmation, setConfirmation] = useState("");
  useEffect(() => {
    if (selected)
      setValues({
        technical_key: selected.technical_key,
        label: selected.label,
        active: selected.active,
      });
  }, [selected]);
  const save = useMutation({
    mutationFn: () =>
      selected
        ? updateBillType(selected.id, { ...values, version: selected.version })
        : createBillType(values),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["bill-types"] });
      setSelected(undefined);
      setValues({ technical_key: "", label: "", active: true });
      onNotice("Tipo de conta/comprovante salvo.");
    },
  });
  const remove = useMutation({
    mutationFn: () => deleteBillType(selected!.id, selected!.version, confirmation),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["bill-types"] });
      setSelected(undefined);
      setConfirmation("");
      onNotice("Tipo de conta/comprovante excluído.");
    },
  });
  return (
    <TypesAdminShell
      title="Tipos de conta e comprovante"
      canAdminister={canAdminister}
      onBack={onBack}
      error={query.error ?? save.error ?? remove.error}
      list={query.data?.types.map((value) => (
        <button className="type-list-item" key={value.id} onClick={() => setSelected(value)}>
          <strong>{value.label}</strong>
          <span>{value.technical_key}</span>
        </button>
      ))}
      form={
        <>
          <label>
            Chave técnica
            <input
              disabled={Boolean(selected)}
              value={values.technical_key}
              onChange={(event) => setValues({ ...values, technical_key: event.target.value })}
            />
          </label>
          <label>
            Nome
            <input
              value={values.label}
              onChange={(event) => setValues({ ...values, label: event.target.value })}
            />
          </label>
          <Checkbox
            checked={values.active}
            className="record-checkbox"
            onChange={(event) => setValues({ ...values, active: event.target.checked })}
          >
            Ativo
          </Checkbox>
          <Flex>
            <Button disabled={save.isPending} onClick={() => save.mutate()}>
              Salvar tipo
            </Button>
            <Button
              onClick={() => {
                setSelected(undefined);
                setValues({
                  technical_key: "",
                  label: "",
                  active: true,
                });
              }}
            >
              Novo
            </Button>
          </Flex>
          {selected ? (
            <ConfirmDelete
              cancelLabel="Cancelar"
              confirmLabel="Excluir tipo"
              confirmationLabel="Digite Confirmar para excluir o tipo não utilizado."
              confirmationWord="Confirmar"
              description="Esta ação exclui a definição do tipo de conta/comprovante do sistema."
              pending={remove.isPending}
              title="Excluir tipo de conta"
              onCancel={() => setSelected(undefined)}
              onConfirm={() => remove.mutate()}
            />
          ) : null}
        </>
      }
    />
  );
}

function TypesAdminShell(props: {
  title: string;
  canAdminister: boolean;
  onBack: () => void;
  error: unknown;
  list: React.ReactNode;
  form: React.ReactNode;
}) {
  if (!props.canAdminister)
    return (
      <Alert
        message="Acesso restrito"
        type="error"
        description="Somente administradores podem alterar tipos."
      />
    );
  return (
    <Flex vertical gap="1rem">
      <div className="records-section-header">
        <h3>{props.title}</h3>
        <Button onClick={props.onBack}>Voltar</Button>
      </div>
      {props.error ? (
        <RecordsError title="Não foi possível alterar tipos" error={props.error} />
      ) : null}
      <div className="types-admin">
        <Card className="type-list">{props.list}</Card>
        <Card className="type-form">{props.form}</Card>
      </div>
    </Flex>
  );
}

function EditorHeader({ title, onClose }: { title: string; onClose: () => void }) {
  return (
    <div className="record-editor__header">
      <h3>{title}</h3>
      <Button onClick={onClose}>Fechar registro</Button>
    </div>
  );
}
function EditorActions<T extends DocumentRecord | BillRecord>(props: {
  editable: boolean;
  saving: boolean;
  record: T | undefined;
  onSave: () => void;
  onEdit: () => void;
  onDuplicate: (value: T) => void;
}) {
  return (
    <Flex className="record-editor__actions">
      {props.editable ? (
        <Button disabled={props.saving} onClick={props.onSave}>
          {props.saving ? "Salvando" : "Salvar"}
        </Button>
      ) : (
        <Button onClick={props.onEdit}>Editar</Button>
      )}
      {props.record ? (
        <Button onClick={() => props.onDuplicate(props.record!)}>Duplicar</Button>
      ) : null}
    </Flex>
  );
}
function MissingRecord({ onClose }: { onClose: () => void }) {
  return (
    <Alert
      message="Registro não encontrado"
      type="error"
      description={
        <>
          <Flex vertical gap="0.75rem">
            <span>Atualize a lista e tente novamente.</span>
            <Button onClick={onClose}>Fechar</Button>
          </Flex>
        </>
      }
    />
  );
}
function RecordsError({ title, error }: { title: string; error: unknown }) {
  return <Alert title={title} type="error" description={<>{errorMessage(error)}</>} />;
}
function RecordStatus({ value }: { value?: "AVAILABLE" | "IN_USE" | "" }) {
  if (value !== "AVAILABLE" && value !== "IN_USE") return null;
  return (
    <Tag color={value === "IN_USE" ? "info" : "success"}>
      {value === "IN_USE" ? "Em uso" : "Disponível"}
    </Tag>
  );
}

function createDocumentColumns(
  onSave: (value: DocumentRecord, patch: Partial<DocumentValuesRequest>) => Promise<void>,
  onOpen: (value: DocumentRecord) => void,
) {
  return [
    documentColumn.accessor((value) => value.type.label, {
      id: "type",
      header: "Tipo",
    }),
    documentColumn.accessor("identifier_value", {
      header: "Identificador",
      cell: ({ row }) => (
        <RecordInlineInput
          ariaLabel={`Identificador de ${row.original.type.label}`}
          value={row.original.identifier_value}
          onSave={(next) => onSave(row.original, { identifier_value: next })}
        />
      ),
    }),
    documentColumn.accessor("document_date", {
      header: "Data",
      cell: ({ row }) => (
        <RecordInlineInput
          ariaLabel={`Data de ${row.original.type.label}`}
          type="date"
          value={row.original.document_date}
          onSave={(next) => onSave(row.original, { document_date: next })}
        />
      ),
    }),
    documentColumn.accessor("medium", {
      header: "Meio",
      cell: ({ getValue }) => mediumLabel(getValue()),
    }),
    documentColumn.accessor("idle_custody", {
      header: "Guarda",
      cell: ({ getValue }) => custodyLabel(getValue() ?? ""),
    }),
    documentColumn.accessor("valid_until", {
      header: "Validade",
      cell: ({ row }) => (
        <RecordInlineInput
          ariaLabel={`Validade de ${row.original.type.label}`}
          type="date"
          value={row.original.valid_until ?? ""}
          onSave={(next) => onSave(row.original, { valid_until: next })}
        />
      ),
    }),
    documentColumn.accessor("status", {
      header: "Status",
      cell: ({ getValue }) => <RecordStatus value={getValue() ?? ""} />,
    }),
    documentColumn.display({
      id: "actions",
      header: "",
      cell: ({ row }) => <Button onClick={() => onOpen(row.original)}>Abrir</Button>,
    }),
  ];
}

function createBillColumns(
  onSave: (value: BillRecord, patch: Partial<BillValuesRequest>) => Promise<void>,
  onOpen: (value: BillRecord) => void,
) {
  return [
    billColumn.accessor((value) => value.type.label, { id: "type", header: "Tipo" }),
    billColumn.accessor("reference_value", {
      header: "Referência",
      cell: ({ row }) => (
        <RecordInlineInput
          ariaLabel={`Referência de ${row.original.type.label}`}
          value={row.original.reference_value}
          onSave={(next) => onSave(row.original, { reference_value: next })}
        />
      ),
    }),
    billColumn.accessor("competence", {
      header: "Competência",
      cell: ({ row }) => (
        <RecordInlineInput
          ariaLabel={`Competência de ${row.original.type.label}`}
          value={row.original.competence}
          onSave={(next) => onSave(row.original, { competence: next })}
        />
      ),
    }),
    billColumn.accessor("amount", {
      header: "Valor",
      cell: ({ row }) => (
        <RecordInlineInput
          ariaLabel={`Valor de ${row.original.type.label}`}
          inputMode="decimal"
          value={row.original.amount}
          onSave={(next) => onSave(row.original, { amount: next })}
        />
      ),
    }),
    billColumn.accessor("medium", {
      header: "Meio",
      cell: ({ getValue }) => mediumLabel(getValue()),
    }),
    billColumn.accessor("idle_custody", {
      header: "Guarda",
      cell: ({ getValue }) => custodyLabel(getValue() ?? ""),
    }),
    billColumn.accessor("status", {
      header: "Status",
      cell: ({ getValue }) => <RecordStatus value={getValue() ?? ""} />,
    }),
    billColumn.display({
      id: "actions",
      header: "",
      cell: ({ row }) => <Button onClick={() => onOpen(row.original)}>Abrir</Button>,
    }),
  ];
}

function RecordCard(props: {
  title: string;
  lines: string[];
  status?: "AVAILABLE" | "IN_USE" | "";
  onOpen: () => void;
}) {
  return (
    <Card className="record-card">
      <Flex vertical gap="0.5rem">
        <strong>{props.title}</strong>
        {props.lines.map((line) => (
          <span key={line}>{line}</span>
        ))}
        <RecordStatus value={props.status ?? ""} />
        <Button onClick={props.onOpen}>Abrir</Button>
      </Flex>
    </Card>
  );
}

function RecordInlineInput(props: {
  value: string;
  ariaLabel: string;
  type?: string;
  inputMode?: "decimal";
  onSave: (value: string) => Promise<void>;
}) {
  const [value, setValue] = useState(props.value);
  useEffect(() => setValue(props.value), [props.value]);
  return (
    <input
      aria-label={props.ariaLabel}
      className="inline-editor"
      inputMode={props.inputMode}
      type={props.type}
      value={value}
      onChange={(event) => setValue(event.target.value)}
      onBlur={() => {
        if (value !== props.value) void props.onSave(value).catch(() => setValue(props.value));
      }}
    />
  );
}

function documentValues(value: DocumentRecord): DocumentValuesRequest {
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
function billValues(value: BillRecord): BillValuesRequest {
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
function documentSearch(search: ProfileListSearch) {
  return {
    q: search.q,
    document_page: search.document_page,
    document_limit: search.document_limit,
    document_sort: search.document_sort,
    document_order: search.document_order,
    document_identifier: search.document_identifier,
    document_status: search.document_status,
    document_medium: search.document_medium,
    document_type: search.document_type,
  };
}
function billSearch(search: ProfileListSearch) {
  return {
    q: search.q,
    bill_page: search.bill_page,
    bill_limit: search.bill_limit,
    bill_sort: search.bill_sort,
    bill_order: search.bill_order,
    bill_reference: search.bill_reference,
    bill_competence: search.bill_competence,
    bill_status: search.bill_status,
    bill_medium: search.bill_medium,
    bill_type: search.bill_type,
  };
}
function profileAddress(value: Profile) {
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
function mediumLabel(value: string) {
  return value === "DIGITAL" ? "Digital" : "Físico";
}
function custodyLabel(value: string) {
  if (value === "OWNER") return "Com o dono";
  if (value === "ORGANIZATION") return "Organização";
  return "";
}
function physicalCustody(
  medium: "PHYSICAL" | "DIGITAL",
  custody?: "ORGANIZATION" | "OWNER",
): { idle_custody?: "ORGANIZATION" | "OWNER" } {
  if (medium !== "PHYSICAL") return {};
  return { idle_custody: custody === "OWNER" ? "OWNER" : "ORGANIZATION" };
}
function withMedium(
  values: DocumentValuesRequest,
  medium: DocumentValuesRequest["medium"],
): DocumentValuesRequest {
  if (medium === "DIGITAL") {
    const next = { ...values, medium };
    delete next.idle_custody;
    return next;
  }
  return { ...values, medium, idle_custody: values.idle_custody ?? "ORGANIZATION" };
}
function withBillMedium(
  values: BillValuesRequest,
  medium: BillValuesRequest["medium"],
): BillValuesRequest {
  if (medium === "DIGITAL") {
    const next = { ...values, medium };
    delete next.idle_custody;
    return next;
  }
  return { ...values, medium, idle_custody: values.idle_custody ?? "ORGANIZATION" };
}
function conflictMessage(error: unknown) {
  return error instanceof APIRequestError && error.status === 409
    ? "Outro usuário alterou o registro. Os dados foram recarregados."
    : errorMessage(error);
}
function errorMessage(error: unknown) {
  if (error instanceof APIRequestError && error.fieldErrors.length > 0)
    return error.fieldErrors.map((field) => `${field.field}: ${field.message}`).join(" · ");
  return error instanceof Error ? error.message : "Erro inesperado.";
}

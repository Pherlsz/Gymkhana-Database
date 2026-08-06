import { Alert, Button, Card, Flex, Tag } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createColumnHelper } from "@tanstack/react-table";
import { useEffect, useState } from "react";
import { AttachmentsPanel } from "./AttachmentsPanel";
import { DataGrid, DataGridPagination } from "./DataGrid";
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
  listProfilesForSelection,
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

const recordStates = ["CURRENT", "REPLACED", "EXPIRED", "ARCHIVED"] as const;
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
        record_state: value.record_state,
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
            lines={[
              value.document_date || "Data não informada",
              recordStateLabel(value.record_state),
            ]}
            status={value.status}
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
        <AttachmentsPanel
          key={`document-attachments:${selected.id}`}
          owner={{ owner_kind: "DOCUMENT", owner_id: selected.id }}
          title="Anexos do documento"
          description="Arquivos privados vinculados exclusivamente a este documento."
        />
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
        record_state: value.record_state,
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
            lines={[value.competence, `${value.currency} ${value.amount}`]}
            status={value.status}
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
        <AttachmentsPanel
          key={`bill-attachments:${selected.id}`}
          owner={{ owner_kind: "BILL", owner_id: selected.id }}
          title="Anexos da conta ou comprovante"
          description="Arquivos privados vinculados exclusivamente a este registro."
        />
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

function DocumentFilters({
  search,
  types,
  onSearch,
}: {
  search: ProfileListSearch;
  types: DocumentType[];
  onSearch: (patch: Partial<ProfileListSearch>) => void;
}) {
  return (
    <Card className="record-filters" style={{ padding: "1rem" }}>
      <label>
        Identificador
        <input
          value={search.document_identifier}
          onChange={(event) =>
            onSearch({ document_identifier: event.target.value, document_page: 1 })
          }
        />
      </label>
      <label>
        Tipo
        <select
          value={search.document_type}
          onChange={(event) => onSearch({ document_type: event.target.value, document_page: 1 })}
        >
          <option value="">Todos</option>
          {types.map((value) => (
            <option key={value.id} value={value.id}>
              {value.label}
            </option>
          ))}
        </select>
      </label>
      <label>
        Status
        <select
          value={search.document_status}
          onChange={(event) =>
            onSearch({
              document_status: event.target.value as ProfileListSearch["document_status"],
              document_page: 1,
            })
          }
        >
          <option value="">Todos</option>
          <option value="AVAILABLE">Disponível</option>
          <option value="IN_USE">Em uso</option>
        </select>
      </label>
      <label>
        Estado
        <select
          value={search.document_state}
          onChange={(event) =>
            onSearch({
              document_state: event.target.value as ProfileListSearch["document_state"],
              document_page: 1,
            })
          }
        >
          <option value="">Todos</option>
          {recordStates.map((value) => (
            <option key={value} value={value}>
              {recordStateLabel(value)}
            </option>
          ))}
        </select>
      </label>
      <label>
        Ordenar
        <select
          value={`${search.document_sort}:${search.document_order}`}
          onChange={(event) => {
            const [sort, order] = event.target.value.split(":") as [
              ProfileListSearch["document_sort"],
              ProfileListSearch["document_order"],
            ];
            onSearch({ document_sort: sort, document_order: order });
          }}
        >
          <option value="identifier_value:asc">Identificador A–Z</option>
          <option value="updated_at:desc">Atualizados recentemente</option>
          <option value="document_date:desc">Data mais recente</option>
          <option value="type_label:asc">Tipo A–Z</option>
        </select>
      </label>
    </Card>
  );
}

function BillFilters({
  search,
  types,
  onSearch,
}: {
  search: ProfileListSearch;
  types: BillType[];
  onSearch: (patch: Partial<ProfileListSearch>) => void;
}) {
  return (
    <Card className="record-filters" style={{ padding: "1rem" }}>
      <label>
        Referência
        <input
          value={search.bill_reference}
          onChange={(event) => onSearch({ bill_reference: event.target.value, bill_page: 1 })}
        />
      </label>
      <label>
        Competência
        <input
          placeholder="AAAA-MM"
          value={search.bill_competence}
          onChange={(event) => onSearch({ bill_competence: event.target.value, bill_page: 1 })}
        />
      </label>
      <label>
        Tipo
        <select
          value={search.bill_type}
          onChange={(event) => onSearch({ bill_type: event.target.value, bill_page: 1 })}
        >
          <option value="">Todos</option>
          {types.map((value) => (
            <option key={value.id} value={value.id}>
              {value.label}
            </option>
          ))}
        </select>
      </label>
      <label>
        Status
        <select
          value={search.bill_status}
          onChange={(event) =>
            onSearch({
              bill_status: event.target.value as ProfileListSearch["bill_status"],
              bill_page: 1,
            })
          }
        >
          <option value="">Todos</option>
          <option value="AVAILABLE">Disponível</option>
          <option value="IN_USE">Em uso</option>
        </select>
      </label>
      <label>
        Estado
        <select
          value={search.bill_state}
          onChange={(event) =>
            onSearch({
              bill_state: event.target.value as ProfileListSearch["bill_state"],
              bill_page: 1,
            })
          }
        >
          <option value="">Todos</option>
          {recordStates.map((value) => (
            <option key={value} value={value}>
              {recordStateLabel(value)}
            </option>
          ))}
        </select>
      </label>
      <label>
        Ordenar
        <select
          value={`${search.bill_sort}:${search.bill_order}`}
          onChange={(event) => {
            const [sort, order] = event.target.value.split(":") as [
              ProfileListSearch["bill_sort"],
              ProfileListSearch["bill_order"],
            ];
            onSearch({ bill_sort: sort, bill_order: order });
          }}
        >
          <option value="reference_value:asc">Referência A–Z</option>
          <option value="updated_at:desc">Atualizados recentemente</option>
          <option value="competence:desc">Competência recente</option>
          <option value="amount:desc">Maior valor</option>
          <option value="type_label:asc">Tipo A–Z</option>
        </select>
      </label>
    </Card>
  );
}

function DocumentEditor(props: {
  mode: "create" | "view" | "edit";
  profile: Profile;
  record: DocumentRecord | undefined;
  types: DocumentType[];
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
          document_type_id: props.types.find((value) => value.active)?.id ?? "",
          identifier_value: "",
          document_date: "",
          notes: "",
          record_state: "CURRENT",
        },
  );
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const editable = props.mode !== "view";
  const submit = async () => {
    setSaving(true);
    setError(null);
    try {
      const saved = props.record
        ? await updateDocument(props.record.id, { ...values, version: props.record.version })
        : await createDocument(values);
      await props.onSaved(saved, props.record ? "Documento atualizado." : "Documento criado.");
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setSaving(false);
    }
  };
  if (props.mode !== "create" && !props.record) return <MissingRecord onClose={props.onClose} />;
  return (
    <Card className="record-editor" style={{ padding: "1rem" }}>
      <EditorHeader
        title={
          props.record
            ? `${props.record.type.label} · ${props.record.identifier_value}`
            : "Novo documento"
        }
        onClose={props.onClose}
      />
      {error ? (
        <Alert message="Não foi possível salvar" type="error" description={<>{error}
        </>} />
      ) : null}
      <div className="record-form">
        <label>
          Tipo
          <select
            disabled={!editable}
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
          Estado
          <select
            disabled={!editable}
            value={values.record_state}
            onChange={(event) =>
              setValues({
                ...values,
                record_state: event.target.value as DocumentValuesRequest["record_state"],
              })
            }
          >
            {recordStates.map((value) => (
              <option key={value} value={value}>
                {recordStateLabel(value)}
              </option>
            ))}
          </select>
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
      </div>
      <EditorActions
        editable={editable}
        saving={saving}
        record={props.record}
        onSave={submit}
        onEdit={props.onEdit}
        onDuplicate={props.onDuplicate}
      />
      {props.record ? (
        <CurrentUseControls
          kind="document"
          record={props.record}
          onChanged={async (message) => {
            await props.onSaved(props.record!, message);
          }}
        />
      ) : null}
      {props.record && props.canDelete ? (
        <DeleteBox
          confirmation={confirmation}
          pending={props.pending}
          label="documento"
          onConfirmation={setConfirmation}
          onDelete={() => props.onDelete(props.record!, confirmation)}
        />
      ) : null}
    </Card>
  );
}

function BillEditor(props: {
  mode: "create" | "view" | "edit";
  profile: Profile;
  record: BillRecord | undefined;
  types: BillType[];
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
          bill_type_id: props.types.find((value) => value.active)?.id ?? "",
          printed_holder_name: props.profile.full_name,
          printed_address: profileAddress(props.profile),
          reference_value: "",
          competence: "",
          amount: "",
          currency: "BRL",
          notes: "",
          record_state: "CURRENT",
        },
  );
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const editable = props.mode !== "view";
  const submit = async () => {
    setSaving(true);
    setError(null);
    try {
      const saved = props.record
        ? await updateBill(props.record.id, { ...values, version: props.record.version })
        : await createBill(values);
      await props.onSaved(
        saved,
        props.record ? "Conta/comprovante atualizado." : "Conta/comprovante criado.",
      );
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setSaving(false);
    }
  };
  if (props.mode !== "create" && !props.record) return <MissingRecord onClose={props.onClose} />;
  return (
    <Card className="record-editor" style={{ padding: "1rem" }}>
      <EditorHeader
        title={
          props.record
            ? `${props.record.type.label} · ${props.record.reference_value}`
            : "Nova conta ou comprovante"
        }
        onClose={props.onClose}
      />
      {error ? (
        <Alert message="Não foi possível salvar" type="error" description={<>{error}
        </>} />
      ) : null}
      <div className="record-form">
        <label>
          Tipo
          <select
            disabled={!editable}
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
          Estado
          <select
            disabled={!editable}
            value={values.record_state}
            onChange={(event) =>
              setValues({
                ...values,
                record_state: event.target.value as BillValuesRequest["record_state"],
              })
            }
          >
            {recordStates.map((value) => (
              <option key={value} value={value}>
                {recordStateLabel(value)}
              </option>
            ))}
          </select>
        </label>
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
      </div>
      <EditorActions
        editable={editable}
        saving={saving}
        record={props.record}
        onSave={submit}
        onEdit={props.onEdit}
        onDuplicate={props.onDuplicate}
      />
      {props.record && props.record.type.supports_current_use ? (
        <CurrentUseControls
          kind="bill"
          record={props.record}
          onChanged={async (message) => {
            await props.onSaved(props.record!, message);
          }}
        />
      ) : null}
      {props.record && props.canDelete ? (
        <DeleteBox
          confirmation={confirmation}
          pending={props.pending}
          label="registro"
          onConfirmation={setConfirmation}
          onDelete={() => props.onDelete(props.record!, confirmation)}
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
  const holders = useQuery({
    queryKey: ["profiles", "selection"],
    queryFn: ({ signal }) => listProfilesForSelection(signal),
  });
  const [holder, setHolder] = useState(props.record.current_use?.holder_profile_id ?? "");
  const holderAvailable = holders.data?.profiles.some((value) => value.id === holder) ?? false;
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
    <Card className="current-use" style={{ padding: "1rem" }}>
      <Flex vertical gap="0.75rem">
        <strong>Uso atual</strong>
        {error ? (
          <Alert message="Não foi possível alterar o uso" type="error" description={<>{error}
          </>} />
        ) : null}
        <label>
          Pessoa em uso
          <select value={holder} onChange={(event) => setHolder(event.target.value)}>
            <option value="">Selecione</option>
            {holder && !holderAvailable ? <option value={holder}>Pessoa atual</option> : null}
            {holders.data?.profiles.map((value) => (
              <option key={value.id} value={value.id}>
                {value.full_name}
              </option>
            ))}
          </select>
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
          <label className="record-checkbox">
            <input
              checked={values.date_required}
              type="checkbox"
              onChange={(event) => setValues({ ...values, date_required: event.target.checked })}
            />
            Data obrigatória
          </label>
          <label className="record-checkbox">
            <input
              checked={values.active}
              type="checkbox"
              onChange={(event) => setValues({ ...values, active: event.target.checked })}
            />
            Ativo
          </label>
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
            <DeleteType
              confirmation={confirmation}
              pending={remove.isPending}
              onConfirmation={setConfirmation}
              onDelete={() => remove.mutate()}
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
    supports_current_use: false,
  });
  const [confirmation, setConfirmation] = useState("");
  useEffect(() => {
    if (selected)
      setValues({
        technical_key: selected.technical_key,
        label: selected.label,
        active: selected.active,
        supports_current_use: selected.supports_current_use,
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
      setValues({ technical_key: "", label: "", active: true, supports_current_use: false });
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
          <span>
            {value.technical_key} ·{" "}
            {value.supports_current_use ? "Permite uso atual" : "Sem uso atual"}
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
          <label className="record-checkbox">
            <input
              checked={values.supports_current_use}
              type="checkbox"
              onChange={(event) =>
                setValues({ ...values, supports_current_use: event.target.checked })
              }
            />
            Permite uso atual
          </label>
          <label className="record-checkbox">
            <input
              checked={values.active}
              type="checkbox"
              onChange={(event) => setValues({ ...values, active: event.target.checked })}
            />
            Ativo
          </label>
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
                  supports_current_use: false,
                });
              }}
            >
              Novo
            </Button>
          </Flex>
          {selected ? (
            <DeleteType
              confirmation={confirmation}
              pending={remove.isPending}
              onConfirmation={setConfirmation}
              onDelete={() => remove.mutate()}
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
      <Alert message="Acesso restrito" type="error" description="Somente administradores podem alterar tipos." />
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
        <Card className="type-list" style={{ padding: "1rem" }}>
          {props.list}
        </Card>
        <Card className="type-form" style={{ padding: "1rem" }}>
          {props.form}
        </Card>
      </div>
    </Flex>
  );
}

function DeleteType(props: {
  confirmation: string;
  pending: boolean;
  onConfirmation: (value: string) => void;
  onDelete: () => void;
}) {
  return (
    <div className="type-delete">
      <span>Digite Confirmar para excluir o tipo não utilizado.</span>
      <input
        value={props.confirmation}
        onChange={(event) => props.onConfirmation(event.target.value)}
      />
      <Button
        disabled={props.confirmation !== "Confirmar" || props.pending}
        onClick={props.onDelete}
      >
        Excluir tipo
      </Button>
    </div>
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
function DeleteBox(props: {
  confirmation: string;
  pending: boolean;
  label: string;
  onConfirmation: (value: string) => void;
  onDelete: () => void;
}) {
  return (
    <Card className="record-delete" style={{ padding: "1rem" }}>
      <Flex vertical gap="0.75rem">
        <strong>Exclusão permanente</strong>
        <span>Digite Confirmar para excluir este {props.label}.</span>
        <input
          value={props.confirmation}
          onChange={(event) => props.onConfirmation(event.target.value)}
        />
        <Button
          disabled={props.confirmation !== "Confirmar" || props.pending}
          onClick={props.onDelete}
        >
          Excluir permanentemente
        </Button>
      </Flex>
    </Card>
  );
}
function MissingRecord({ onClose }: { onClose: () => void }) {
  return (
    <Alert message="Registro não encontrado" type="error" description={<><Flex vertical gap="0.75rem">
        <span>Atualize a lista e tente novamente.</span>
        <Button onClick={onClose}>Fechar</Button>
      </Flex>
    </>} />
  );
}
function RecordsError({ title, error }: { title: string; error: unknown }) {
  return (
    <Alert title={title} type="error" description={<>
      {errorMessage(error)}
    </>} />
  );
}
function RecordStatus({ value }: { value: "AVAILABLE" | "IN_USE" }) {
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
    documentColumn.accessor("record_state", {
      header: "Estado",
      cell: ({ getValue }) => recordStateLabel(getValue()),
    }),
    documentColumn.accessor("status", {
      header: "Status",
      cell: ({ getValue }) => <RecordStatus value={getValue()} />,
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
    billColumn.accessor("status", {
      header: "Status",
      cell: ({ getValue }) => <RecordStatus value={getValue()} />,
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
  status: "AVAILABLE" | "IN_USE";
  onOpen: () => void;
}) {
  return (
    <Card className="record-card" style={{ padding: "1rem" }}>
      <Flex vertical gap="0.5rem">
        <strong>{props.title}</strong>
        {props.lines.map((line) => (
          <span key={line}>{line}</span>
        ))}
        <RecordStatus value={props.status} />
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
    record_state: value.record_state,
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
    record_state: value.record_state,
  };
}
function documentSearch(search: ProfileListSearch) {
  return {
    document_page: search.document_page,
    document_limit: search.document_limit,
    document_sort: search.document_sort,
    document_order: search.document_order,
    document_identifier: search.document_identifier,
    document_status: search.document_status,
    document_state: search.document_state,
    document_type: search.document_type,
  };
}
function billSearch(search: ProfileListSearch) {
  return {
    bill_page: search.bill_page,
    bill_limit: search.bill_limit,
    bill_sort: search.bill_sort,
    bill_order: search.bill_order,
    bill_reference: search.bill_reference,
    bill_competence: search.bill_competence,
    bill_status: search.bill_status,
    bill_state: search.bill_state,
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
function recordStateLabel(value: string) {
  return value === "CURRENT"
    ? "Atual"
    : value === "REPLACED"
      ? "Substituído"
      : value === "EXPIRED"
        ? "Vencido"
        : "Arquivado";
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

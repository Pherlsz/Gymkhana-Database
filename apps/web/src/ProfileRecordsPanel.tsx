import { Button, Flex } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AttachmentsPanel } from "./AttachmentsPanel";
import { CadastroOcrSection } from "./lib/cadastro/CadastroPanel";
import { DataGrid, DataGridPagination } from "./DataGrid";
import { BillFilters, DocumentFilters } from "./lib/records/RecordFilters";
import { BillTypesAdmin, DocumentTypesAdmin } from "./lib/records/RecordTypesAdmin";
import { DocumentEditor } from "./lib/records/DocumentEditor";
import { BillEditor } from "./lib/records/BillEditor";
import {
  RecordCard,
  RecordsError,
  conflictMessage,
  createBillColumns,
  createDocumentColumns,
  mediumLabel,
  physicalCustody,
  recordSearch,
} from "./lib/records/RecordEditorCommon";
import { useI18n } from "./i18n";
import {
  deleteBill,
  deleteDocument,
  duplicateBill,
  duplicateDocument,
  listBillTypes,
  listBills,
  listDocumentTypes,
  listDocuments,
  updateBill,
  updateDocument,
  type BillRecord,
  type BillValuesRequest,
  type DocumentRecord,
  type DocumentValuesRequest,
  type Profile,
  type ProfileListSearch,
  type UserRole,
} from "./lib/api/client";

export { DocumentEditor } from "./lib/records/DocumentEditor";
export { BillEditor } from "./lib/records/BillEditor";

type Props = {
  profile: Profile;
  role: UserRole;
  section: "documents" | "bills";
  search: ProfileListSearch;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
  onNotice: (message: string) => void;
};

export function ProfileRecordsPanel(props: Props) {
  return props.section === "documents" ? (
    <DocumentsSection {...props} />
  ) : (
    <BillsSection {...props} />
  );
}

function DocumentsSection({ profile, role, search, onSearch, onNotice }: Props) {
  const { messages } = useI18n();
  const panel = messages.records.panel;
  const queryClient = useQueryClient();
  const types = useQuery({
    queryKey: ["document-types"],
    queryFn: ({ signal }) => listDocumentTypes(signal),
  });
  const records = useQuery({
    queryKey: ["documents", profile.id, recordSearch(search, "document")],
    queryFn: ({ signal }) => listDocuments(profile.id, recordSearch(search, "document"), signal),
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
      onNotice(panel.docDuplicatedNotice);
      onSearch({ document_selected: value.id, document_mode: "edit" });
    },
  });
  const deleteMutation = useMutation({
    mutationFn: ({ value, confirmation }: { value: DocumentRecord; confirmation: string }) =>
      deleteDocument(value.id, value.version, confirmation),
    onSuccess: async () => {
      await refresh();
      onNotice(panel.docDeletedNotice);
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
      onNotice(panel.docUpdatedNotice);
      await refresh();
    } catch (error) {
      await refresh();
      onNotice(conflictMessage(error, messages));
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
        title={panel.documentsTitle}
        description={panel.documentsDesc}
        onCreate={() => onSearch({ document_selected: undefined, document_mode: "create" })}
        {...(canAdministerTypes ? { onTypes: () => onSearch({ document_mode: "types" }) } : {})}
      />
      {records.isError ? <RecordsError title={panel.loadDocError} error={records.error} /> : null}
      <DocumentFilters search={search} types={types.data?.types ?? []} onSearch={onSearch} />
      <DataGrid
        caption={`${panel.captionDocs} ${profile.full_name}`}
        cardsClassName="records-cards"
        className="records-list"
        columns={createDocumentColumns(
          inlineUpdate,
          (value) => onSearch({ document_selected: value.id, document_mode: "view" }),
          messages,
        )}
        data={records.data?.documents ?? []}
        emptyLabel={panel.emptyDocs}
        getRowId={(value) => value.id}
        loading={records.isLoading}
        loadingLabel={panel.loadingDocs}
        renderCard={(value) => (
          <RecordCard
            key={value.id}
            title={`${value.type.label} · ${value.identifier_value}`}
            lines={[
              value.document_date || panel.dateNotProvided,
              mediumLabel(value.medium, messages),
            ]}
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
        label={panel.docCountLabel}
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
            title={panel.docAttachmentsTitle}
            description={panel.docAttachmentsDesc}
          />
          <CadastroOcrSection owner={{ owner_kind: "DOCUMENT", owner_id: selected.id }} />
        </>
      ) : null}
    </Flex>
  );
}

function BillsSection({ profile, role, search, onSearch, onNotice }: Props) {
  const { messages } = useI18n();
  const panel = messages.records.panel;
  const queryClient = useQueryClient();
  const types = useQuery({
    queryKey: ["bill-types"],
    queryFn: ({ signal }) => listBillTypes(signal),
  });
  const records = useQuery({
    queryKey: ["bills", profile.id, recordSearch(search, "bill")],
    queryFn: ({ signal }) => listBills(profile.id, recordSearch(search, "bill"), signal),
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
      onNotice(panel.billDuplicatedNotice);
      onSearch({ bill_selected: value.id, bill_mode: "edit" });
    },
  });
  const deleteMutation = useMutation({
    mutationFn: ({ value, confirmation }: { value: BillRecord; confirmation: string }) =>
      deleteBill(value.id, value.version, confirmation),
    onSuccess: async () => {
      await refresh();
      onNotice(panel.billDeletedNotice);
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
      onNotice(panel.billUpdatedNotice);
      await refresh();
    } catch (error) {
      await refresh();
      onNotice(conflictMessage(error, messages));
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
        title={panel.billsTitle}
        description={panel.billsDesc}
        onCreate={() => onSearch({ bill_selected: undefined, bill_mode: "create" })}
        {...(canAdministerTypes ? { onTypes: () => onSearch({ bill_mode: "types" }) } : {})}
      />
      {records.isError ? <RecordsError title={panel.loadBillError} error={records.error} /> : null}
      <BillFilters search={search} types={types.data?.types ?? []} onSearch={onSearch} />
      <DataGrid
        caption={`${panel.captionBills} ${profile.full_name}`}
        cardsClassName="records-cards"
        className="records-list"
        columns={createBillColumns(
          inlineUpdate,
          (value) => onSearch({ bill_selected: value.id, bill_mode: "view" }),
          messages,
        )}
        data={records.data?.bills ?? []}
        emptyLabel={panel.emptyBills}
        getRowId={(value) => value.id}
        loading={records.isLoading}
        loadingLabel={panel.loadingBills}
        renderCard={(value) => (
          <RecordCard
            key={value.id}
            title={`${value.type.label} · ${value.reference_value}`}
            lines={[
              mediumLabel(value.medium, messages),
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
        label={panel.billCountLabel}
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
            title={panel.billAttachmentsTitle}
            description={panel.billAttachmentsDesc}
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
  const { messages } = useI18n();
  const panel = messages.records.panel;
  return (
    <div className="records-section-header">
      <div>
        <h3>{props.title}</h3>
        <p>{props.description}</p>
      </div>
      <Flex>
        {props.onTypes ? <Button onClick={props.onTypes}>{panel.adminTypes}</Button> : null}
        <Button onClick={props.onCreate}>{panel.newRecord}</Button>
      </Flex>
    </div>
  );
}

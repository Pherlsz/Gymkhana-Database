import { QueryView } from "../../components/QueryView";
import { StateCard } from "../../components/StateCard";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
import { useI18n } from "../../i18n";
import {
  deleteBill,
  deleteDocument,
  duplicateBill,
  duplicateDocument,
  getProfile,
  listBillTypes,
  listBills,
  listDocumentTypes,
  listDocuments,
  type BillRecord,
  type DocumentRecord,
  type Profile,
  type ProfileListSearch,
  type UserRole,
} from "../api/client";
import { queryKeys } from "../api/queryKeys";
import { BillEditor } from "../records/BillEditor";
import { DocumentEditor } from "../records/DocumentEditor";
import { AttachmentsPanel } from "../../AttachmentsPanel";
import { CadastroOcrSection } from "../cadastro/CadastroOcrSection";
import { OWNER_BILL_SEARCH, OWNER_DOC_SEARCH } from "../cadastro/components/PersonOwnedCollections";
import { billSearch, documentSearch } from "./sheetQuery";

export function TableRecordEditorPanel({
  section,
  search,
  role,
  fallbackOwnerId,
  onSearch,
  onNotice,
}: {
  section: "documents" | "bills";
  search: ProfileListSearch;
  role: UserRole;
  fallbackOwnerId?: string | undefined;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
  onNotice: (message: string) => void;
}) {
  if (section === "documents") {
    return (
      <DocumentTableEditor
        fallbackOwnerId={fallbackOwnerId}
        role={role}
        search={search}
        onNotice={onNotice}
        onSearch={onSearch}
      />
    );
  }
  return (
    <BillTableEditor
      fallbackOwnerId={fallbackOwnerId}
      role={role}
      search={search}
      onNotice={onNotice}
      onSearch={onSearch}
    />
  );
}

function DocumentTableEditor({
  search,
  role,
  fallbackOwnerId,
  onSearch,
  onNotice,
}: {
  search: ProfileListSearch;
  role: UserRole;
  fallbackOwnerId?: string | undefined;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
  onNotice: (message: string) => void;
}) {
  const { messages } = useI18n();
  const queryClient = useQueryClient();
  const mode = search.document_mode;
  const selectedID = search.document_selected;
  if (!mode || mode === "types") return null;

  const listOwnerId = search.records_owner ?? fallbackOwnerId;
  const useOwnerList = Boolean(listOwnerId);
  const recordsQuery = useQuery({
    queryKey: useOwnerList
      ? queryKeys.records.documents(listOwnerId, OWNER_DOC_SEARCH)
      : queryKeys.tables.documents(documentSearch(search), search.records_owner),
    queryFn: ({ signal }) =>
      listDocuments(
        useOwnerList ? listOwnerId : search.records_owner,
        useOwnerList ? OWNER_DOC_SEARCH : documentSearch(search),
        signal,
      ),
    enabled: Boolean(mode === "create" ? listOwnerId : selectedID || listOwnerId),
  });
  const selected = recordsQuery.data?.documents.find((value) => value.id === selectedID);
  const ownerID = search.records_owner ?? selected?.owner_profile_id ?? fallbackOwnerId;
  const ownerQuery = useQuery({
    queryKey: queryKeys.profiles.detail(ownerID),
    queryFn: ({ signal }) => getProfile(ownerID!, signal),
    enabled: Boolean(ownerID && mode),
  });
  const typesQuery = useQuery({
    queryKey: queryKeys.types.documents,
    queryFn: ({ signal }) => listDocumentTypes(signal),
    enabled: Boolean(mode),
  });
  const openOwner = ownerID
    ? () =>
        onSearch({
          section: "profile",
          selected: ownerID,
          mode: "view",
          document_selected: undefined,
          document_mode: undefined,
          bill_selected: undefined,
          bill_mode: undefined,
        })
    : undefined;

  return (
    <RecordEditorShell<DocumentRecord>
      canDelete={role === "ADMIN" || role === "SUPERADMIN"}
      mode={mode}
      ownerID={ownerID}
      ownerQuery={ownerQuery}
      recordsLoading={Boolean(selectedID) && recordsQuery.isLoading}
      selected={selected}
      typesLoading={typesQuery.isLoading}
      onClose={() => onSearch({ document_selected: undefined, document_mode: undefined })}
      onDelete={async (value, confirmation) => {
        await deleteDocument(value.id, value.version, confirmation);
        await queryClient.invalidateQueries({ queryKey: queryKeys.tables.documents() });
        await queryClient.invalidateQueries({ queryKey: queryKeys.records.documents() });
        onNotice(messages.records.panel.docDeletedNotice);
        onSearch({ document_selected: undefined, document_mode: undefined });
      }}
      onDuplicate={async (id) => {
        const value = await duplicateDocument(id);
        await queryClient.invalidateQueries({ queryKey: queryKeys.tables.documents() });
        await queryClient.invalidateQueries({ queryKey: queryKeys.records.documents() });
        onNotice(messages.records.panel.docDuplicatedNotice);
        onSearch({ document_selected: value.id, document_mode: "edit" });
      }}
      onEdit={() => onSearch({ document_mode: "edit" })}
      onSaved={async (value, message) => {
        await queryClient.invalidateQueries({ queryKey: queryKeys.tables.documents() });
        await queryClient.invalidateQueries({ queryKey: queryKeys.records.documents() });
        onNotice(message);
        onSearch({ document_selected: value.id, document_mode: "view" });
      }}
      renderEditor={(profile, record, props) => (
        <DocumentEditor
          {...props}
          initialTypeId={search.document_type || undefined}
          lockType={Boolean(search.document_type)}
          profile={profile}
          record={record}
          types={typesQuery.data?.types ?? []}
          onOpenOwner={openOwner}
        >
          {record && (mode === "view" || mode === "edit") ? (
            <>
              <AttachmentsPanel
                description={messages.records.panel.docAttachmentsDesc}
                owner={{ owner_kind: "DOCUMENT", owner_id: record.id }}
                title={messages.records.panel.docAttachmentsTitle}
              />
              <CadastroOcrSection owner={{ owner_kind: "DOCUMENT", owner_id: record.id }} />
            </>
          ) : null}
        </DocumentEditor>
      )}
    />
  );
}

function BillTableEditor({
  search,
  role,
  fallbackOwnerId,
  onSearch,
  onNotice,
}: {
  search: ProfileListSearch;
  role: UserRole;
  fallbackOwnerId?: string | undefined;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
  onNotice: (message: string) => void;
}) {
  const { messages } = useI18n();
  const queryClient = useQueryClient();
  const mode = search.bill_mode;
  const selectedID = search.bill_selected;
  if (!mode || mode === "types") return null;

  const listOwnerId = search.records_owner ?? fallbackOwnerId;
  const useOwnerList = Boolean(listOwnerId);
  const recordsQuery = useQuery({
    queryKey: useOwnerList
      ? queryKeys.records.bills(listOwnerId, OWNER_BILL_SEARCH)
      : queryKeys.tables.bills(billSearch(search), search.records_owner),
    queryFn: ({ signal }) =>
      listBills(
        useOwnerList ? listOwnerId : search.records_owner,
        useOwnerList ? OWNER_BILL_SEARCH : billSearch(search),
        signal,
      ),
    enabled: Boolean(mode === "create" ? listOwnerId : selectedID || listOwnerId),
  });
  const selected = recordsQuery.data?.bills.find((value) => value.id === selectedID);
  const ownerID = search.records_owner ?? selected?.owner_profile_id ?? fallbackOwnerId;
  const ownerQuery = useQuery({
    queryKey: queryKeys.profiles.detail(ownerID),
    queryFn: ({ signal }) => getProfile(ownerID!, signal),
    enabled: Boolean(ownerID && mode),
  });
  const typesQuery = useQuery({
    queryKey: queryKeys.types.bills,
    queryFn: ({ signal }) => listBillTypes(signal),
    enabled: Boolean(mode),
  });
  const openOwner = ownerID
    ? () =>
        onSearch({
          section: "profile",
          selected: ownerID,
          mode: "view",
          document_selected: undefined,
          document_mode: undefined,
          bill_selected: undefined,
          bill_mode: undefined,
        })
    : undefined;

  return (
    <RecordEditorShell<BillRecord>
      canDelete={role === "ADMIN" || role === "SUPERADMIN"}
      mode={mode}
      ownerID={ownerID}
      ownerQuery={ownerQuery}
      recordsLoading={Boolean(selectedID) && recordsQuery.isLoading}
      selected={selected}
      typesLoading={typesQuery.isLoading}
      onClose={() => onSearch({ bill_selected: undefined, bill_mode: undefined })}
      onDelete={async (value, confirmation) => {
        await deleteBill(value.id, value.version, confirmation);
        await queryClient.invalidateQueries({ queryKey: queryKeys.tables.bills() });
        await queryClient.invalidateQueries({ queryKey: queryKeys.records.bills() });
        onNotice(messages.records.panel.billDeletedNotice);
        onSearch({ bill_selected: undefined, bill_mode: undefined });
      }}
      onDuplicate={async (id) => {
        const value = await duplicateBill(id);
        await queryClient.invalidateQueries({ queryKey: queryKeys.tables.bills() });
        await queryClient.invalidateQueries({ queryKey: queryKeys.records.bills() });
        onNotice(messages.records.panel.billDuplicatedNotice);
        onSearch({ bill_selected: value.id, bill_mode: "edit" });
      }}
      onEdit={() => onSearch({ bill_mode: "edit" })}
      onSaved={async (value, message) => {
        await queryClient.invalidateQueries({ queryKey: queryKeys.tables.bills() });
        await queryClient.invalidateQueries({ queryKey: queryKeys.records.bills() });
        onNotice(message);
        onSearch({ bill_selected: value.id, bill_mode: "view" });
      }}
      renderEditor={(profile, record, props) => (
        <BillEditor
          {...props}
          initialTypeId={search.bill_type || undefined}
          lockType={Boolean(search.bill_type)}
          profile={profile}
          record={record}
          types={typesQuery.data?.types ?? []}
          onOpenOwner={openOwner}
        >
          {record && (mode === "view" || mode === "edit") ? (
            <>
              <AttachmentsPanel
                description={messages.records.panel.billAttachmentsDesc}
                owner={{ owner_kind: "BILL", owner_id: record.id }}
                title={messages.records.panel.billAttachmentsTitle}
              />
              <CadastroOcrSection owner={{ owner_kind: "BILL", owner_id: record.id }} />
            </>
          ) : null}
        </BillEditor>
      )}
    />
  );
}

function RecordEditorShell<T extends DocumentRecord | BillRecord>({
  mode,
  ownerID,
  ownerQuery,
  selected,
  typesLoading,
  recordsLoading,
  canDelete,
  onClose,
  onEdit,
  onSaved,
  onDuplicate,
  onDelete,
  renderEditor,
}: {
  mode: "create" | "view" | "edit";
  ownerID: string | undefined;
  ownerQuery: {
    isLoading: boolean;
    error: Error | null;
    data: Profile | undefined;
    refetch: () => unknown;
  };
  selected: T | undefined;
  typesLoading: boolean;
  recordsLoading: boolean;
  canDelete: boolean;
  onClose: () => void;
  onEdit: () => void;
  onSaved: (value: T, message: string) => Promise<void>;
  onDuplicate: (id: string) => Promise<void>;
  onDelete: (value: T, confirmation: string) => Promise<void>;
  renderEditor: (
    profile: Profile,
    record: T | undefined,
    props: {
      mode: "create" | "view" | "edit";
      canDelete: boolean;
      pending: boolean;
      onClose: () => void;
      onEdit: () => void;
      onSaved: (value: T, message: string) => Promise<void>;
      onDuplicate: (value: T) => void;
      onDelete: (value: T, confirmation: string) => void;
    },
  ) => ReactNode;
}) {
  const { messages } = useI18n();
  const [pending, setPending] = useState(false);

  if (!ownerID) {
    return (
      <StateCard
        compact
        description={messages.tables.record.ownerRequiredDesc}
        kind="warning"
        title={messages.tables.record.ownerRequiredTitle}
      />
    );
  }
  if (
    recordsLoading ||
    ownerQuery.isLoading ||
    typesLoading ||
    ownerQuery.error ||
    !ownerQuery.data
  ) {
    return (
      <QueryView
        compact
        errorDescription={messages.tables.record.ownerNotFoundDesc}
        errorTitle={messages.tables.record.ownerNotFoundTitle}
        isError={Boolean(!recordsLoading && (ownerQuery.error || !ownerQuery.data))}
        isPending={recordsLoading || ownerQuery.isLoading || typesLoading}
        loadingTitle={messages.tables.record.formLoading}
        onRetry={ownerQuery.error ? () => void ownerQuery.refetch() : undefined}
      />
    );
  }

  const editorProps = {
    mode,
    canDelete,
    pending,
    onClose,
    onEdit,
    onSaved: async (value: T, message: string) => {
      setPending(true);
      try {
        await onSaved(value, message);
      } finally {
        setPending(false);
      }
    },
    onDuplicate: (value: T) => {
      setPending(true);
      void onDuplicate(value.id).finally(() => setPending(false));
    },
    onDelete: (value: T, confirmation: string) => {
      setPending(true);
      void onDelete(value, confirmation).finally(() => setPending(false));
    },
  };

  return (
    <div className="table-record-editor">
      {renderEditor(ownerQuery.data, selected, editorProps)}
    </div>
  );
}

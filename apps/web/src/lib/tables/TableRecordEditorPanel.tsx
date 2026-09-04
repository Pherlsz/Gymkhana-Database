import { StateCard } from "../../components/StateCard";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
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
import { BillEditor, DocumentEditor } from "../../ProfileRecordsPanel";
import { AttachmentsPanel } from "../../AttachmentsPanel";
import { CadastroOcrSection } from "../cadastro/CadastroPanel";
import { billSearch, documentSearch } from "./sheetQuery";

export function TableRecordEditorPanel({
  section,
  search,
  role,
  onSearch,
  onNotice,
}: {
  section: "documents" | "bills";
  search: ProfileListSearch;
  role: UserRole;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
  onNotice: (message: string) => void;
}) {
  if (section === "documents") {
    return (
      <DocumentTableEditor
        role={role}
        search={search}
        onNotice={onNotice}
        onSearch={onSearch}
      />
    );
  }
  return (
    <BillTableEditor role={role} search={search} onNotice={onNotice} onSearch={onSearch} />
  );
}

function DocumentTableEditor({
  search,
  role,
  onSearch,
  onNotice,
}: {
  search: ProfileListSearch;
  role: UserRole;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
  onNotice: (message: string) => void;
}) {
  const queryClient = useQueryClient();
  const mode = search.document_mode;
  const selectedID = search.document_selected;
  if (!mode || mode === "types") return null;

  const recordsQuery = useQuery({
    queryKey: ["table-record-editor", "documents", search.records_owner, documentSearch(search)],
    queryFn: ({ signal }) =>
      listDocuments(search.records_owner, documentSearch(search), signal),
    enabled: Boolean(mode === "create" ? search.records_owner : selectedID || search.records_owner),
  });
  const selected = recordsQuery.data?.documents.find((value) => value.id === selectedID);
  const ownerID = search.records_owner ?? selected?.owner_profile_id;
  const ownerQuery = useQuery({
    queryKey: ["profile", ownerID],
    queryFn: ({ signal }) => getProfile(ownerID!, signal),
    enabled: Boolean(ownerID && mode),
  });
  const typesQuery = useQuery({
    queryKey: ["document-types"],
    queryFn: ({ signal }) => listDocumentTypes(signal),
    enabled: Boolean(mode),
  });

  return (
    <RecordEditorShell<DocumentRecord>
      canDelete={role === "ADMIN" || role === "SUPERADMIN"}
      mode={mode}
      ownerID={ownerID}
      ownerQuery={ownerQuery}
      selected={selected}
      typesLoading={typesQuery.isLoading}
      onClose={() => onSearch({ document_selected: undefined, document_mode: undefined })}
      onDelete={async (value, confirmation) => {
        await deleteDocument(value.id, value.version, confirmation);
        await queryClient.invalidateQueries({ queryKey: ["tables", "documents"] });
        onNotice("Documento excluído permanentemente.");
        onSearch({ document_selected: undefined, document_mode: undefined });
      }}
      onDuplicate={async (id) => {
        const value = await duplicateDocument(id);
        await queryClient.invalidateQueries({ queryKey: ["tables", "documents"] });
        onNotice("Documento duplicado. Revise a cópia antes de continuar.");
        onSearch({ document_selected: value.id, document_mode: "edit" });
      }}
      onEdit={() => onSearch({ document_mode: "edit" })}
      onSaved={async (value, message) => {
        await queryClient.invalidateQueries({ queryKey: ["tables", "documents"] });
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
        />
      )}
      renderExtras={(record) =>
        record && (mode === "view" || mode === "edit") ? (
          <>
            <AttachmentsPanel
              description="Arquivos privados vinculados exclusivamente a este documento."
              owner={{ owner_kind: "DOCUMENT", owner_id: record.id }}
              title="Anexos do documento"
            />
            <CadastroOcrSection owner={{ owner_kind: "DOCUMENT", owner_id: record.id }} />
          </>
        ) : null
      }
    />
  );
}

function BillTableEditor({
  search,
  role,
  onSearch,
  onNotice,
}: {
  search: ProfileListSearch;
  role: UserRole;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
  onNotice: (message: string) => void;
}) {
  const queryClient = useQueryClient();
  const mode = search.bill_mode;
  const selectedID = search.bill_selected;
  if (!mode || mode === "types") return null;

  const recordsQuery = useQuery({
    queryKey: ["table-record-editor", "bills", search.records_owner, billSearch(search)],
    queryFn: ({ signal }) => listBills(search.records_owner, billSearch(search), signal),
    enabled: Boolean(mode === "create" ? search.records_owner : selectedID || search.records_owner),
  });
  const selected = recordsQuery.data?.bills.find((value) => value.id === selectedID);
  const ownerID = search.records_owner ?? selected?.owner_profile_id;
  const ownerQuery = useQuery({
    queryKey: ["profile", ownerID],
    queryFn: ({ signal }) => getProfile(ownerID!, signal),
    enabled: Boolean(ownerID && mode),
  });
  const typesQuery = useQuery({
    queryKey: ["bill-types"],
    queryFn: ({ signal }) => listBillTypes(signal),
    enabled: Boolean(mode),
  });

  return (
    <RecordEditorShell<BillRecord>
      canDelete={role === "ADMIN" || role === "SUPERADMIN"}
      mode={mode}
      ownerID={ownerID}
      ownerQuery={ownerQuery}
      selected={selected}
      typesLoading={typesQuery.isLoading}
      onClose={() => onSearch({ bill_selected: undefined, bill_mode: undefined })}
      onDelete={async (value, confirmation) => {
        await deleteBill(value.id, value.version, confirmation);
        await queryClient.invalidateQueries({ queryKey: ["tables", "bills"] });
        onNotice("Conta/comprovante excluído permanentemente.");
        onSearch({ bill_selected: undefined, bill_mode: undefined });
      }}
      onDuplicate={async (id) => {
        const value = await duplicateBill(id);
        await queryClient.invalidateQueries({ queryKey: ["tables", "bills"] });
        onNotice("Conta/comprovante duplicado. Revise a cópia antes de continuar.");
        onSearch({ bill_selected: value.id, bill_mode: "edit" });
      }}
      onEdit={() => onSearch({ bill_mode: "edit" })}
      onSaved={async (value, message) => {
        await queryClient.invalidateQueries({ queryKey: ["tables", "bills"] });
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
        />
      )}
      renderExtras={(record) =>
        record && (mode === "view" || mode === "edit") ? (
          <>
            <AttachmentsPanel
              description="Arquivos privados vinculados exclusivamente a este registro."
              owner={{ owner_kind: "BILL", owner_id: record.id }}
              title="Anexos da conta ou comprovante"
            />
            <CadastroOcrSection owner={{ owner_kind: "BILL", owner_id: record.id }} />
          </>
        ) : null
      }
    />
  );
}

function RecordEditorShell<T extends DocumentRecord | BillRecord>({
  mode,
  ownerID,
  ownerQuery,
  selected,
  typesLoading,
  canDelete,
  onClose,
  onEdit,
  onSaved,
  onDuplicate,
  onDelete,
  renderEditor,
  renderExtras,
}: {
  mode: "create" | "view" | "edit";
  ownerID: string | undefined;
  ownerQuery: { isLoading: boolean; error: Error | null; data: Profile | undefined };
  selected: T | undefined;
  typesLoading: boolean;
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
  renderExtras: (record: T | undefined) => ReactNode;
}) {
  const [pending, setPending] = useState(false);

  if (!ownerID) {
    return (
      <StateCard
        compact
        description="Selecione ou filtre a pessoa dona antes de cadastrar documento ou conta."
        kind="warning"
        title="Dono obrigatório"
      />
    );
  }
  if (ownerQuery.isLoading || typesLoading) {
    return <StateCard compact kind="loading" title="Carregando formulário" />;
  }
  if (ownerQuery.error || !ownerQuery.data) {
    return (
      <StateCard
        compact
        description="Não foi possível carregar o cadastro da pessoa dona."
        kind="error"
        title="Pessoa não encontrada"
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
      {renderExtras(selected)}
    </div>
  );
}

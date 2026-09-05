import { Drawer } from "antd";
import { type ReactNode } from "react";
import { ProfilePanel } from "../../ProfilePanel";
import { useI18n } from "../../i18n";
import { type Profile, type ProfileListSearch, type UserRole } from "../api/client";
import { RecordInspector, RecordInspectorState, recordInspectorFields } from "./RecordInspector";
import { TableRecordEditorPanel } from "./TableRecordEditorPanel";
import { DEFAULT_SHEET_PREFERENCES } from "./sheetPreferences";
import { columnLabel } from "./columnVisibility";
import type { SpreadsheetColumn } from "./SpreadsheetTable";
import type { TableRow } from "./tableRows";

const RECORD_CARD_OMITTED_KEYS: ReadonlySet<string> = new Set([
  "identifier",
  "reference",
  "owner",
  "owner_id",
  "type_id",
  "type_key",
  "current_holder_id",
  "document_badges",
]);

export interface TableInspectorDrawerProps {
  inspectorSheet: boolean;
  section: "profile" | "documents" | "bills";
  search: ProfileListSearch;
  selectedProfile?: Profile | undefined;
  selectedProfileLoading: boolean;
  selectedRecordRow?: TableRow | undefined;
  columns: SpreadsheetColumn<TableRow>[];
  loading: boolean;
  canDelete: boolean;
  deletePending: boolean;
  userRole: UserRole;
  onCloseInspector: () => void;
  onCloseRecord: () => void;
  onDeleteProfile: (value: Profile, confirmation: string) => void;
  onSaveProfile: (value: Profile, message: string) => Promise<void>;
  onEditProfile: () => void;
  onCancelEditProfile: () => void;
  onNotice: (message: string) => void;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
  onConfirmDiscardEdit: (apply: () => void) => void;
}

export function TableInspectorDrawer({
  inspectorSheet,
  section,
  search,
  selectedProfile,
  selectedProfileLoading,
  selectedRecordRow,
  columns,
  loading,
  canDelete,
  deletePending,
  userRole,
  onCloseInspector,
  onCloseRecord,
  onDeleteProfile,
  onSaveProfile,
  onEditProfile,
  onCancelEditProfile,
  onNotice,
  onSearch,
  onConfirmDiscardEdit,
}: TableInspectorDrawerProps) {
  const { messages } = useI18n();
  const copy = messages.tables;
  const recordCopy = copy.record;

  const selectedRecordId =
    section === "documents"
      ? search.document_selected
      : section === "bills"
        ? search.bill_selected
        : undefined;

  const recordInspector: ReactNode = selectedRecordId ? (
    selectedRecordRow ? (
      <RecordInspector
        ariaLabel={section === "bills" ? recordCopy.billAria : recordCopy.documentAria}
        closeLabel={copy.inspector.close}
        eyebrow={section === "bills" ? recordCopy.billEyebrow : recordCopy.documentEyebrow}
        fields={recordInspectorFields(
          selectedRecordRow,
          columns.map((column) => ({ key: column.key, label: columnLabel(column) })),
          RECORD_CARD_OMITTED_KEYS,
        )}
        onClose={onCloseRecord}
        onOpenOwner={
          selectedRecordRow.cells.owner_id
            ? () =>
                onSearch({
                  section: "profile",
                  selected: String(selectedRecordRow.cells.owner_id),
                  mode: "view",
                  document_selected: undefined,
                  document_mode: undefined,
                  bill_selected: undefined,
                  bill_mode: undefined,
                })
            : undefined
        }
        openOwnerLabel={recordCopy.openOwner}
        ownerLabel={recordCopy.owner}
        ownerName={String(selectedRecordRow.cells.owner ?? "")}
        title={
          String(
            selectedRecordRow.cells[section === "bills" ? "reference" : "identifier"] ?? "",
          ).trim() || recordCopy.untitled
        }
      />
    ) : (
      <RecordInspectorState
        ariaLabel={section === "bills" ? recordCopy.billAria : recordCopy.documentAria}
        closeLabel={copy.inspector.close}
        description={loading ? undefined : recordCopy.notFoundHint}
        eyebrow={section === "bills" ? recordCopy.billEyebrow : recordCopy.documentEyebrow}
        kind={loading ? "loading" : "error"}
        onClose={onCloseRecord}
        title={loading ? recordCopy.loading : recordCopy.notFound}
      />
    )
  ) : null;

  const recordFormOpen =
    (section === "documents" && search.document_mode && search.document_mode !== "view") ||
    (section === "bills" && search.bill_mode && search.bill_mode !== "view");

  const recordForm: ReactNode = recordFormOpen ? (
    <TableRecordEditorPanel
      role={userRole}
      search={search}
      section={section === "bills" ? "bills" : "documents"}
      onNotice={onNotice}
      onSearch={onSearch}
    />
  ) : null;

  const inspector: ReactNode = search.mode ? (
    <ProfilePanel
      key={search.mode === "create" ? "create" : "inspector"}
      canDelete={canDelete}
      hideSections
      recordLinks={Boolean(selectedProfile)}
      mode={search.mode}
      pending={deletePending}
      loading={!selectedProfile && selectedProfileLoading}
      profile={selectedProfile}
      role={userRole}
      search={search}
      section={section === "bills" ? "bills" : section === "documents" ? "documents" : "profile"}
      onClose={onCloseInspector}
      onDelete={onDeleteProfile}
      onEdit={onEditProfile}
      onCancelEdit={onCancelEditProfile}
      onNotice={onNotice}
      onOpenDocuments={(value) => {
        const apply = () =>
          onSearch({
            section: "documents",
            selected: value.id,
            mode: "view",
            records_owner: value.id,
            document_page: 1,
            document_selected: undefined,
            document_mode: undefined,
          });
        if (search.mode === "edit") onConfirmDiscardEdit(apply);
        else apply();
      }}
      onOpenBills={(value) => {
        const apply = () =>
          onSearch({
            section: "bills",
            selected: value.id,
            mode: "view",
            records_owner: value.id,
            bill_page: 1,
            bill_selected: undefined,
            bill_mode: undefined,
          });
        if (search.mode === "edit") onConfirmDiscardEdit(apply);
        else apply();
      }}
      onSaved={onSaveProfile}
      onSearch={onSearch}
    />
  ) : null;

  const content = recordForm ?? recordInspector ?? inspector;

  if (inspectorSheet) {
    return (
      <Drawer
        className="tables-inspector-sheet"
        rootClassName="tables-inspector-sheet"
        closable={false}
        destroyOnHidden
        getContainer={false}
        size={DEFAULT_SHEET_PREFERENCES.inspectorSheetSize}
        mask={false}
        open={Boolean(search.mode) || Boolean(recordInspector) || Boolean(recordForm)}
        placement="bottom"
        styles={{
          body: { display: "flex", height: "100%", overflow: "hidden", padding: 0 },
          wrapper: { pointerEvents: "auto" },
        }}
        onClose={
          recordForm
            ? () =>
                onSearch({
                  document_selected: undefined,
                  document_mode: undefined,
                  bill_selected: undefined,
                  bill_mode: undefined,
                })
            : recordInspector
              ? onCloseRecord
              : onCloseInspector
        }
      >
        {content}
      </Drawer>
    );
  }

  return <>{content}</>;
}

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
  "status_key",
  "medium_key",
]);

export interface TableInspectorDrawerProps {
  inspectorSheet: boolean;
  section: "profile" | "documents" | "bills";
  search: ProfileListSearch;
  selectedProfile?: Profile | undefined;
  selectedProfileLoading: boolean;
  selectedRecordRow?: TableRow | undefined;
  assistantRecord?:
    | { kind: "document" | "bill"; row: TableRow; columns: { key: string; label: string }[] }
    | undefined;
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
  assistantRecord,
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
  const assistantInspector: ReactNode = assistantRecord ? (
    <RecordInspector
      ariaLabel={assistantRecord.kind === "bill" ? recordCopy.billAria : recordCopy.documentAria}
      closeLabel={copy.inspector.close}
      eyebrow={
        assistantRecord.kind === "bill" ? recordCopy.billEyebrow : recordCopy.documentEyebrow
      }
      fields={recordInspectorFields(assistantRecord.row, assistantRecord.columns, new Set())}
      onClose={onCloseRecord}
      onOpenOwner={undefined}
      openOwnerLabel={recordCopy.openOwner}
      ownerLabel={recordCopy.owner}
      ownerName=""
      title={String(assistantRecord.row.cells.entityLabel ?? "").trim() || recordCopy.untitled}
    />
  ) : null;

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

  const documentFormOpen = Boolean(search.document_mode && search.document_mode !== "types");
  const billFormOpen = Boolean(search.bill_mode && search.bill_mode !== "types");
  const recordFormOpen = documentFormOpen || billFormOpen;

  const recordForm: ReactNode = recordFormOpen ? (
    <TableRecordEditorPanel
      fallbackOwnerId={
        selectedRecordRow?.cells.owner_id
          ? String(selectedRecordRow.cells.owner_id)
          : selectedProfile?.id
      }
      role={userRole}
      search={search}
      section={billFormOpen ? "bills" : "documents"}
      onNotice={onNotice}
      onSearch={onSearch}
    />
  ) : null;

  const inspector: ReactNode = search.mode ? (
    <ProfilePanel
      key={search.mode === "create" ? "create" : "inspector"}
      canDelete={canDelete}
      hideSections
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
      {...(selectedProfile
        ? {
            onOpenOwnedDocument: (documentId: string) => {
              const apply = () =>
                onSearch({
                  selected: selectedProfile.id,
                  mode: "view" as const,
                  document_selected: documentId,
                  document_mode: "view" as const,
                  bill_selected: undefined,
                  bill_mode: undefined,
                });
              if (search.mode === "edit") onConfirmDiscardEdit(apply);
              else apply();
            },
            onOpenOwnedBill: (billId: string) => {
              const apply = () =>
                onSearch({
                  selected: selectedProfile.id,
                  mode: "view" as const,
                  bill_selected: billId,
                  bill_mode: "view" as const,
                  document_selected: undefined,
                  document_mode: undefined,
                });
              if (search.mode === "edit") onConfirmDiscardEdit(apply);
              else apply();
            },
          }
        : {})}
      onSaved={onSaveProfile}
      onSearch={onSearch}
    />
  ) : null;

  const content = recordForm ?? assistantInspector ?? recordInspector ?? inspector;

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
        open={
          Boolean(search.mode) ||
          Boolean(assistantInspector) ||
          Boolean(recordInspector) ||
          Boolean(recordForm)
        }
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
            : assistantInspector
              ? onCloseRecord
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

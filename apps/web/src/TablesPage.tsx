import { Alert, Modal } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { memo, startTransition, useCallback, useMemo, useState } from "react";
import { getRouteApi } from "@tanstack/react-router";
import { useApplicationSession } from "./session";
import {
  isTableKind,
  normalizeTableSearch,
  sectionFromTable,
  tableFromSection,
} from "./lib/tables/tableRoutes";
import {
  deleteProfile,
  type Profile,
  type ProfileListSearch,
} from "./lib/api/client";
import { queryKeys } from "./lib/api/queryKeys";
import { useI18n } from "./i18n";
import { PageHeader } from "./components/PageHeader";
import { StateCard } from "./components/StateCard";
import "./tables.css";
import { SpreadsheetTable } from "./lib/tables/SpreadsheetTable";
import { TablesToolbar } from "./lib/tables/TablesToolbar";
import { cellKeyForFilter, localDateMatches } from "./lib/tables/tableFilters";
import {
  columnGroup,
  columnLabel,
  formatColumnCols,
  isColumnVisible,
  parseColumnCols,
  setColumnVisible,
  showAllColumns,
  withColumnLayout,
  type TableSheetScope,
} from "./lib/tables/columnVisibility";
import { buildPeopleColumns } from "./lib/tables/peopleColumns";
import { buildBillColumns, buildDocumentColumns } from "./lib/tables/recordColumns";
import { activeFilterChips } from "./lib/tables/sheetFilters";
import { TableInspectorDrawer } from "./lib/tables/TableInspectorDrawer";
import {
  clearFilters,
  pagePatch,
  resetPage,
} from "./lib/tables/sheetQuery";
import { useInspectorSheet } from "./lib/tables/useInspectorSheet";
import {
  cellText,
  paginationRange,
  type TableRow,
} from "./lib/tables/tableRows";
import { DEFAULT_SHEET_PREFERENCES } from "./lib/tables/sheetPreferences";
import { clampSpreadsheetPageSize } from "./lib/tables/spreadsheetViewport";
import { useTableSheetData } from "./lib/tables/useTableSheetData";

const tablesRoute = getRouteApi("/tables/$table");

const MemoSpreadsheetTable = memo(SpreadsheetTable) as typeof SpreadsheetTable;

export function TablesPage() {
  const { messages, t } = useI18n();
  const copy = messages.tables;
  const session = useApplicationSession();
  const { table } = tablesRoute.useParams();
  const routeSearch = tablesRoute.useSearch();
  const navigate = tablesRoute.useNavigate();
  const queryClient = useQueryClient();
  const section = sectionFromTable(isTableKind(table) ? table : "people");
  const search = { ...routeSearch, section };
  const scope: TableSheetScope =
    section === "documents" ? "documents" : section === "bills" ? "bills" : "profiles";
  const columnOverrides = useMemo(() => parseColumnCols(search.cols), [search.cols]);
  const [localFilters, setLocalFilters] = useState<Record<string, string>>({});
  const [notice, setNotice] = useState<string | null>(null);
  const inspectorSheet = useInspectorSheet(DEFAULT_SHEET_PREFERENCES.inspectorMediaQuery);

  const sectionCopy =
    section === "documents" ? copy.documents : section === "bills" ? copy.bills : copy.people;

  const updateSearch = useCallback(
    (patch: Partial<ProfileListSearch>) => {
      const nextTable = patch.section
        ? tableFromSection(patch.section)
        : isTableKind(table)
          ? table
          : "people";
      const { section: _section, ...rest } = patch;
      void navigate({
        params: { table: nextTable },
        search: (current) => normalizeTableSearch({ ...current, ...rest }),
        to: "/tables/$table",
      });
    },
    [navigate, table],
  );

  const [searchInput, setSearchInput] = useState(search.q);
  const [prevSearchQ, setPrevSearchQ] = useState(search.q);
  if (search.q !== prevSearchQ) {
    setPrevSearchQ(search.q);
    setSearchInput(search.q);
  }

  const setLocal = useCallback((key: string, value: string) => {
    setLocalFilters((current) => ({ ...current, [key]: value }));
  }, []);

  const {
    extraFields,
    baseRows,
    allFilters,
    loading,
    errorDescription,
    total,
    page,
    pageSize,
    sortField,
    sortOrder,
    selectedProfile,
    selectedProfileLoading,
    recordsOwnerName,
    columnMetadataLoading,
  } = useTableSheetData({
    section,
    search,
    copy,
    messages,
    localFilters,
    setLocal,
    updateSearch,
  });

  const onTablePage = useCallback(
    (nextPage: number, nextSize: number) => {
      updateSearch(pagePatch(section, nextPage, clampSpreadsheetPageSize(nextSize)));
    },
    [section, updateSearch],
  );

  const onTableSort = useCallback(
    (field: string, order: "asc" | "desc") => {
      if (section === "documents") {
        updateSearch({
          document_sort: field as ProfileListSearch["document_sort"],
          document_order: order,
        });
        return;
      }
      if (section === "bills") {
        updateSearch({
          bill_sort: field as ProfileListSearch["bill_sort"],
          bill_order: order,
        });
        return;
      }
      updateSearch({ sort: field as ProfileListSearch["sort"], order });
    },
    [section, updateSearch],
  );

  const closeInspector = useCallback(() => {
    updateSearch({
      selected: undefined,
      mode: undefined,
      document_selected: undefined,
      document_mode: undefined,
      bill_selected: undefined,
      bill_mode: undefined,
    });
  }, [updateSearch]);

  const confirmDiscardEdit = (apply: () => void) => {
    Modal.confirm({
      title: copy.inspector.discardTitle,
      content: copy.inspector.discardBody,
      okText: copy.inspector.discardOk,
      cancelText: copy.inspector.discardCancel,
      onOk: apply,
    });
  };

  const onTableRowClick = useCallback(
    (row: TableRow) => {
      const applyPeople = () =>
        startTransition(() => {
          updateSearch({ selected: row.id, mode: "view" });
        });
      const confirmIfEditing = (nextId: string | undefined, apply: () => void) => {
        if (search.mode === "edit" && search.selected && search.selected !== nextId) {
          confirmDiscardEdit(apply);
          return;
        }
        apply();
      };
      if (section === "documents") {
        confirmIfEditing(undefined, () =>
          startTransition(() => {
            updateSearch({
              selected: undefined,
              mode: undefined,
              document_selected: row.id,
              document_mode: "view",
            });
          }),
        );
        return;
      }
      if (section === "bills") {
        confirmIfEditing(undefined, () =>
          startTransition(() => {
            updateSearch({
              selected: undefined,
              mode: undefined,
              bill_selected: row.id,
              bill_mode: "view",
            });
          }),
        );
        return;
      }
      confirmIfEditing(row.id, applyPeople);
    },
    [confirmDiscardEdit, section, search.mode, search.selected, updateSearch],
  );

  const dataColumns = useMemo(() => {
    if (section === "documents") return buildDocumentColumns(copy, extraFields);
    if (section === "bills") return buildBillColumns(copy, extraFields);
    return buildPeopleColumns(copy.columns, copy.badges, extraFields);
  }, [copy, extraFields, section]);

  const columns = useMemo(
    () => dataColumns.map((column) => withColumnLayout(column, scope)),
    [dataColumns, scope],
  );
  const visibleColumns = useMemo(
    () => columns.filter((column) => isColumnVisible(column, columnOverrides)),
    [columnOverrides, columns],
  );
  const rows = useMemo(() => {
    const localExactKeys = new Set(
      allFilters.filter((field) => field.local && field.kind === "select").map(cellKeyForFilter),
    );
    const localDateKeys = new Set(
      allFilters.filter((field) => field.local && field.kind === "date").map(cellKeyForFilter),
    );
    return baseRows.filter((row) =>
      Object.entries(localFilters).every(([key, value]) => {
        if (!value) return true;
        const text = cellText(row.cells[key]).toLowerCase();
        const needle = value.toLowerCase();
        if (localExactKeys.has(key)) return text === needle;
        if (localDateKeys.has(key)) return localDateMatches(cellText(row.cells[key]), value);
        return text.includes(needle);
      }),
    );
  }, [allFilters, baseRows, localFilters]);
  const canDelete = session.user.role === "ADMIN" || session.user.role === "SUPERADMIN";

  const activeChips = [
    ...(search.records_owner
      ? [
          {
            key: "records_owner",
            field: copy.inspector.ownerChip,
            value: recordsOwnerName,
            onClear: () =>
              updateSearch({ records_owner: undefined, document_page: 1, bill_page: 1 }),
          },
        ]
      : []),
    ...activeFilterChips(allFilters),
  ];

  const refreshPeople = async () => {
    await queryClient.invalidateQueries({ queryKey: queryKeys.tables.profiles() });
    await queryClient.invalidateQueries({ queryKey: queryKeys.profiles.all });
  };
  const deleteMutation = useMutation({
    mutationFn: ({ value, confirmation }: { value: Profile; confirmation: string }) =>
      deleteProfile(value.id, value.version, confirmation),
    onSuccess: async () => {
      await refreshPeople();
      setNotice(copy.notices.personDeleted);
      updateSearch({ selected: undefined, mode: undefined });
    },
  });

  const updateCols = useCallback(
    (cols: string) => {
      startTransition(() => {
        updateSearch({ cols });
      });
    },
    [updateSearch],
  );

  const selectedRecordId =
    section === "documents"
      ? search.document_selected
      : section === "bills"
        ? search.bill_selected
        : undefined;
  const selectedRecordRow = selectedRecordId
    ? baseRows.find((row) => row.id === selectedRecordId)
    : undefined;
  const closeRecord = useCallback(() => {
    updateSearch({
      document_selected: undefined,
      document_mode: undefined,
      bill_selected: undefined,
      bill_mode: undefined,
    });
  }, [updateSearch]);

  const columnPickerItems = useMemo(
    () =>
      columns.map((column) => ({
        key: column.key,
        label: columnLabel(column),
        group: copy.columnPicker.groups[columnGroup(column.key, scope)],
        visible: isColumnVisible(column, columnOverrides),
        locked: Boolean(column.locked),
      })),
    [columns, copy.columnPicker.groups, columnOverrides, scope],
  );

  const handleColumnToggle = useCallback(
    (key: string, visible: boolean) => {
      const column = columns.find((item) => item.key === key);
      if (!column) return;
      updateCols(formatColumnCols(setColumnVisible(column, visible, columnOverrides)));
    },
    [columns, columnOverrides, updateCols],
  );

  const handleColumnShowAll = () => {
    updateCols(formatColumnCols(showAllColumns(columns, columnOverrides)));
  };

  const handleColumnReset = () => {
    updateCols("");
  };

  return (
    <div className="tables-page">
      <PageHeader description={sectionCopy.description} title={sectionCopy.title} />

      {notice ? <Alert showIcon type="success" title={notice} /> : null}
      {errorDescription ? (
        <StateCard description={errorDescription} kind="error" title={copy.error} />
      ) : null}

      <TablesToolbar
        searchLabel={copy.filters.search}
        searchPlaceholder={copy.filters.searchAll}
        searchValue={searchInput}
        searchGrain={
          section === "documents" ? "documents" : section === "bills" ? "bills" : "profiles"
        }
        onSearchChange={setSearchInput}
        onSearchSubmit={(value) => {
          if (value === search.q) return;
          updateSearch({ q: value, ...resetPage(section) });
        }}
        fieldFiltersLabel={copy.filters.byField}
        addFilterLabel={copy.filters.addFilter}
        chooseFieldLabel={copy.filters.chooseField}
        chooseValueLabel={copy.filters.chooseValue}
        removeFilterLabel={copy.filters.removeFilter}
        appliedFiltersLabel={copy.filters.appliedFilters}
        noFieldsLabel={copy.filters.noFields}
        moreChipsLabel={(count) => t(copy.filters.moreChips, { count })}
        moreChipsCollapseLabel={copy.filters.moreChipsCollapse}
        chips={activeChips}
        clearLabel={copy.filters.clear}
        onClearAll={() => {
          setLocalFilters({});
          updateSearch(clearFilters(section));
        }}
        localHint={copy.filters.localOnly}
        localScopeBadge={copy.filters.localScopeBadge}
        filters={allFilters}
        columnPicker={{
          label: copy.columnPicker.button,
          title: copy.columnPicker.title,
          searchLabel: copy.columnPicker.search,
          showAllLabel: copy.columnPicker.showAll,
          resetLabel: copy.columnPicker.reset,
          lockedLabel: copy.columnPicker.locked,
          emptyLabel: copy.columnPicker.empty,
          visibleCountLabel: (visible, totalCount) =>
            t(copy.columnPicker.visibleCount, { visible, total: totalCount }),
          hiddenCount: columnMetadataLoading ? 0 : columns.length - visibleColumns.length,
          items: columnPickerItems,
          onToggle: handleColumnToggle,
          onShowAll: handleColumnShowAll,
          onReset: handleColumnReset,
        }}
      />

      <div className="tables-page__workspace">
        <MemoSpreadsheetTable
          caption={sectionCopy.caption}
          columns={visibleColumns}
          emptyLabel={sectionCopy.empty}
          loading={loading}
          loadingLabel={sectionCopy.loading}
          page={page}
          pageSize={pageSize}
          rangeLabel={paginationRange(page, pageSize, total, copy.pagination.range)}
          pageSizeAriaLabel={copy.pagination.pageSizeAria}
          pageSizeOptionLabel={copy.pagination.pageSize}
          rows={rows}
          selectedRowId={
            section === "documents"
              ? search.document_selected
              : section === "bills"
                ? search.bill_selected
                : search.selected
          }
          sortField={sortField}
          sortOrder={sortOrder}
          total={total}
          onPage={onTablePage}
          onRowClick={onTableRowClick}
          onSort={onTableSort}
        />

        <TableInspectorDrawer
          canDelete={canDelete}
          columns={columns}
          deletePending={deleteMutation.isPending}
          inspectorSheet={inspectorSheet}
          loading={loading}
          search={search}
          section={section}
          selectedProfile={selectedProfile}
          selectedProfileLoading={selectedProfileLoading}
          selectedRecordRow={selectedRecordRow}
          userRole={session.user.role}
          onCancelEditProfile={() => updateSearch({ mode: "view" })}
          onCloseInspector={closeInspector}
          onCloseRecord={closeRecord}
          onConfirmDiscardEdit={confirmDiscardEdit}
          onDeleteProfile={(value, confirmation) => deleteMutation.mutate({ value, confirmation })}
          onEditProfile={() => updateSearch({ mode: "edit" })}
          onNotice={setNotice}
          onSaveProfile={async (value, message) => {
            await refreshPeople();
            setNotice(message);
            updateSearch({ selected: value.id, mode: "view" });
          }}
          onSearch={updateSearch}
        />
      </div>
    </div>
  );
}

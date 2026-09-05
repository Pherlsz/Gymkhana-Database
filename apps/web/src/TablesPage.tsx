import { Alert, Modal } from "antd";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
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
  APIRequestError,
  deleteProfile,
  getProfile,
  listBillTypes,
  listBills,
  listCustomFields,
  listCustomOptions,
  listDistinctCities,
  listDocumentTypes,
  listDocuments,
  listProfiles,
  type BillRecord,
  type DocumentRecord,
  type Profile,
  type ProfileListSearch,
} from "./lib/api/client";
import { useI18n } from "./i18n";
import { PageHeader } from "./components/PageHeader";
import { StateCard } from "./components/StateCard";
import { SpreadsheetTable } from "./lib/tables/SpreadsheetTable";
import { TablesToolbar } from "./lib/tables/TablesToolbar";
import { cellKeyForFilter, distinctValues, localDateMatches } from "./lib/tables/tableFilters";
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
import { buildPeopleColumns, PEOPLE_DOC_KEY_SET } from "./lib/tables/peopleColumns";
import { buildBillColumns, buildDocumentColumns } from "./lib/tables/recordColumns";
import { activeFilterChips, buildSheetFilters } from "./lib/tables/sheetFilters";
import { TableInspectorDrawer } from "./lib/tables/TableInspectorDrawer";
import {
  billSearch,
  clearFilters,
  documentSearch,
  pagePatch,
  profileListKey,
  resetPage,
} from "./lib/tables/sheetQuery";
import { useInspectorSheet } from "./lib/tables/useInspectorSheet";
import {
  billRow,
  cellText,
  documentRow,
  paginationRange,
  personRow,
  uniqueCustomFields,
  type RecordSheetLabels,
  type TableRow,
} from "./lib/tables/tableRows";
import { groupedTypeFilterOptions } from "./lib/tables/typeFilterOptions";
import { DEFAULT_SHEET_PREFERENCES } from "./lib/tables/sheetPreferences";
import { clampSpreadsheetPageSize } from "./lib/tables/spreadsheetViewport";

const tablesRoute = getRouteApi("/tables/$table");

const MemoSpreadsheetTable = memo(SpreadsheetTable) as typeof SpreadsheetTable;

export function TablesPage() {
  const { messages } = useI18n();
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
  const tableSearch = {
    ...search,
    limit: clampSpreadsheetPageSize(search.limit),
    document_limit: clampSpreadsheetPageSize(search.document_limit),
    bill_limit: clampSpreadsheetPageSize(search.bill_limit),
  };

  const [searchInput, setSearchInput] = useState(search.q);
  const [prevSearchQ, setPrevSearchQ] = useState(search.q);
  if (search.q !== prevSearchQ) {
    setPrevSearchQ(search.q);
    setSearchInput(search.q);
  }
  const peopleQuery = useQuery({
    queryKey: ["tables", "profiles", profileListKey(tableSearch)],
    queryFn: ({ signal }) => listProfiles(tableSearch, signal),
    enabled: section === "profile",
  });
  const citiesQuery = useQuery({
    queryKey: ["profile-city-options", search.full_name, search.cpf, search.email, search.state],
    queryFn: ({ signal }) =>
      listDistinctCities(
        {
          full_name: search.full_name,
          cpf: search.cpf,
          email: search.email,
          state: search.state,
        },
        signal,
      ),
    enabled: section === "profile",
    staleTime: 5 * 60 * 1000,
  });
  const documentQuery = useQuery({
    queryKey: ["tables", "documents", documentSearch(tableSearch), tableSearch.records_owner],
    queryFn: ({ signal }) =>
      listDocuments(tableSearch.records_owner, documentSearch(tableSearch), signal),
    enabled: section === "documents",
  });
  const billQuery = useQuery({
    queryKey: ["tables", "bills", billSearch(tableSearch), tableSearch.records_owner],
    queryFn: ({ signal }) => listBills(tableSearch.records_owner, billSearch(tableSearch), signal),
    enabled: section === "bills",
  });
  const documentTypes = useQuery({
    queryKey: ["document-types"],
    queryFn: ({ signal }) => listDocumentTypes(signal),
    enabled: section === "documents" || section === "profile",
  });
  const billTypes = useQuery({
    queryKey: ["bill-types"],
    queryFn: ({ signal }) => listBillTypes(signal),
    enabled: section === "bills",
  });
  const selectedProfileQuery = useQuery({
    queryKey: ["profile", search.selected],
    queryFn: ({ signal }) => getProfile(search.selected!, signal),
    enabled: Boolean(search.mode && search.selected),
  });
  const profileFieldsQuery = useQuery({
    queryKey: ["custom-fields", "PROFILE"],
    queryFn: () => listCustomFields("PROFILE"),
    enabled: section === "profile",
  });
  const documentTypeList = documentTypes.data?.types ?? [];
  const billTypeList = billTypes.data?.types ?? [];
  const documentFieldQueries = useQueries({
    queries: documentTypeList.map((type) => ({
      queryKey: ["custom-fields", "DOCUMENT_TYPE", type.id],
      queryFn: () => listCustomFields("DOCUMENT_TYPE", type.id),
      enabled: section === "documents",
    })),
  });
  const billFieldQueries = useQueries({
    queries: billTypeList.map((type) => ({
      queryKey: ["custom-fields", "BILL_TYPE", type.id],
      queryFn: () => listCustomFields("BILL_TYPE", type.id),
      enabled: section === "bills",
    })),
  });
  const documentFieldsStamp = queriesStamp(documentFieldQueries);
  const billFieldsStamp = queriesStamp(billFieldQueries);
  const columnMetadataLoading =
    section === "profile"
      ? profileFieldsQuery.isPending
      : section === "documents"
        ? documentTypes.isPending || documentFieldQueries.some((query) => query.isPending)
        : billTypes.isPending || billFieldQueries.some((query) => query.isPending);

  const recordLabels: RecordSheetLabels = {
    boolean: copy.boolean,
    status: copy.status,
    medium: copy.medium,
    idleCustody: copy.idleCustody,
  };

  const typeGroups = useMemo(() => {
    if (section === "documents") {
      return groupedTypeFilterOptions(
        documentTypeList.map((type) => ({
          id: type.id,
          label: type.label,
          technicalKey: type.technical_key,
        })),
        "document",
        messages.home.tables,
      );
    }
    if (section === "bills") {
      return groupedTypeFilterOptions(
        billTypeList.map((type) => ({
          id: type.id,
          label: type.label,
          technicalKey: type.technical_key,
        })),
        "bill",
        messages.home.tables,
      );
    }
    return [];
  }, [billTypeList, documentTypeList, messages.home.tables, section]);

  const extraFields = useMemo(() => {
    if (section === "profile") return uniqueCustomFields(profileFieldsQuery.data?.fields ?? []);
    if (section === "documents") {
      const selectedType = search.document_type;
      const fields = documentTypeList.flatMap((type, index) => {
        if (selectedType && type.id !== selectedType) return [];
        return documentFieldQueries[index]?.data?.fields ?? [];
      });
      return uniqueCustomFields(fields);
    }
    const selectedType = search.bill_type;
    const fields = billTypeList.flatMap((type, index) => {
      if (selectedType && type.id !== selectedType) return [];
      return billFieldQueries[index]?.data?.fields ?? [];
    });
    return uniqueCustomFields(fields);
    // Stamps replace the useQueries array identity, which changes every render.
  }, [
    billFieldsStamp,
    billTypeList,
    documentFieldsStamp,
    documentTypeList,
    profileFieldsQuery.data,
    search.bill_type,
    search.document_type,
    section,
  ]);

  const identifierTypes = useMemo(() => {
    if (section !== "profile") return [];
    const grouped = groupedTypeFilterOptions(
      documentTypeList.map((type) => ({
        id: type.id,
        label: type.label,
        technicalKey: type.technical_key,
      })),
      "document",
      messages.home.tables,
    );
    const ordered = grouped.flatMap((group) =>
      group.options.map((option) => documentTypeList.find((type) => type.id === option.value)),
    );
    return ordered.filter(
      (type): type is NonNullable<typeof type> =>
        type != null && PEOPLE_DOC_KEY_SET.has(type.technical_key),
    );
  }, [documentTypeList, messages.home.tables, section]);

  const baseRows: TableRow[] = useMemo(() => {
    if (section === "documents") {
      return (documentQuery.data?.documents ?? []).map((record: DocumentRecord) =>
        documentRow(record, recordLabels, extraFields),
      );
    }
    if (section === "bills") {
      return (billQuery.data?.bills ?? []).map((record: BillRecord) =>
        billRow(record, recordLabels, extraFields),
      );
    }
    return (peopleQuery.data?.profiles ?? []).map((profile) =>
      personRow(profile, extraFields, copy.boolean),
    );
  }, [
    billQuery.data,
    copy.boolean,
    documentQuery.data,
    extraFields,
    peopleQuery.data,
    recordLabels,
    section,
  ]);

  const selectCustomFields = useMemo(
    () =>
      extraFields.filter(
        (field) => field.field_kind === "SINGLE_SELECT" || field.field_kind === "MULTI_SELECT",
      ),
    [extraFields],
  );
  const optionQueries = useQueries({
    queries: selectCustomFields.map((field) => ({
      queryKey: ["custom-options", field.id],
      queryFn: ({ signal }) => listCustomOptions(field.id, signal),
    })),
  });
  const optionStamp = queriesStamp(optionQueries);
  const optionsByFieldId = useMemo(() => {
    const map = new Map<string, { value: string; label: string }[]>();
    selectCustomFields.forEach((field, index) => {
      const options = (optionQueries[index]?.data?.options ?? [])
        .filter((option) => option.active)
        .map((option) => ({ value: option.label, label: option.label }));
      if (options.length) map.set(field.id, options);
    });
    return map;
  }, [optionStamp, selectCustomFields]);

  const distinctByKey = useMemo(() => {
    const result: Record<string, string[]> = {};
    for (const field of extraFields)
      result[field.technical_key] = distinctValues(baseRows, field.technical_key);
    return result;
  }, [baseRows, extraFields]);

  const setLocal = useCallback((key: string, value: string) => {
    setLocalFilters((current) => ({ ...current, [key]: value }));
  }, []);

  const cityOptions = useMemo(
    () =>
      (citiesQuery.data?.values ?? []).map((city) => ({
        value: city,
        label: city,
      })),
    [citiesQuery.data],
  );

  const allFilters = useMemo(
    () =>
      buildSheetFilters({
        section,
        search,
        copy,
        localFilters,
        extraFields,
        optionsByFieldId,
        distinctByKey,
        typeGroups,
        identifierTypes,
        cityOptions,
        setLocal,
        updateSearch,
      }),
    [
      cityOptions,
      copy,
      distinctByKey,
      extraFields,
      identifierTypes,
      localFilters,
      optionsByFieldId,
      search,
      section,
      setLocal,
      typeGroups,
      updateSearch,
    ],
  );
  const loading =
    section === "documents"
      ? documentQuery.isLoading
      : section === "bills"
        ? billQuery.isLoading
        : peopleQuery.isLoading;
  const error =
    section === "documents"
      ? documentQuery.error
      : section === "bills"
        ? billQuery.error
        : peopleQuery.error;
  const errorDescription = tableErrorDescription(error);
  const total =
    section === "documents"
      ? (documentQuery.data?.page.total ?? 0)
      : section === "bills"
        ? (billQuery.data?.page.total ?? 0)
        : (peopleQuery.data?.page.total ?? 0);
  const page =
    section === "documents"
      ? search.document_page
      : section === "bills"
        ? search.bill_page
        : search.page;
  const pageSize =
    section === "documents"
      ? tableSearch.document_limit
      : section === "bills"
        ? tableSearch.bill_limit
        : tableSearch.limit;
  const sortField =
    section === "documents"
      ? search.document_sort
      : section === "bills"
        ? search.bill_sort
        : search.sort;
  const sortOrder =
    section === "documents"
      ? search.document_order
      : section === "bills"
        ? search.bill_order
        : search.order;

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
      // Each table opens its own record. Documents and bills used to set
      // `selected` to the owner, so clicking a document showed the person.
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
  const selectedProfile =
    peopleQuery.data?.profiles.find((value) => value.id === search.selected) ??
    selectedProfileQuery.data;
  const recordsOwnerName =
    (section === "documents"
      ? documentQuery.data?.documents.find(
          (value) => value.owner_profile_id === search.records_owner,
        )?.owner_full_name
      : billQuery.data?.bills.find((value) => value.owner_profile_id === search.records_owner)
          ?.owner_full_name) ??
    (search.records_owner && selectedProfile?.id === search.records_owner
      ? selectedProfile?.full_name
      : undefined) ??
    copy.inspector.ownerChip;
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
    await queryClient.invalidateQueries({ queryKey: ["tables", "profiles"] });
    await queryClient.invalidateQueries({ queryKey: ["profile"] });
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
        moreChipsLabel={(count) => copy.filters.moreChips.replace("{count}", String(count))}
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
            copy.columnPicker.visibleCount
              .replace("{visible}", String(visible))
              .replace("{total}", String(totalCount)),
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
          selectedProfileLoading={selectedProfileQuery.isLoading}
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

function queriesStamp(queries: { dataUpdatedAt: number }[]): string {
  return queries.map((query) => String(query.dataUpdatedAt)).join(",");
}

function tableErrorDescription(error: unknown) {
  if (!error || isAbortError(error)) return undefined;
  if (error instanceof APIRequestError) {
    const fields = error.fieldErrors
      .map((field) => (field.field ? `${field.field}: ${field.message}` : field.message))
      .filter(Boolean)
      .join(" · ");
    return fields ? `${error.message} (${fields})` : error.message;
  }
  return error instanceof Error ? error.message : undefined;
}

function isAbortError(error: unknown) {
  return error instanceof Error && error.name === "AbortError";
}

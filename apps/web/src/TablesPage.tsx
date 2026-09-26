import { Modal, Tag } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Sparkles } from "lucide-react";
import { memo, startTransition, useCallback, useMemo, useState } from "react";
import { ICON, ICON_STROKE } from "./components/icons";
import { getRouteApi } from "@tanstack/react-router";
import { useApplicationSession } from "./session";
import {
  isTableKind,
  normalizeTableSearch,
  sectionFromTable,
  tableFromSection,
} from "./lib/tables/tableRoutes";
import { deleteProfile, type Profile, type ProfileListSearch } from "./lib/api/client";
import { queryKeys } from "./lib/api/queryKeys";
import { useI18n } from "./i18n";
import { PageShell } from "./components/PageShell";
import { StateCard } from "./components/StateCard";
import { StatusBanner } from "./components/StatusBanner";
import "./tables.css";
import { SpreadsheetTable } from "./lib/tables/SpreadsheetTable";
import { TablesToolbar } from "./lib/tables/TablesToolbar";
import { cellKeyForFilter, textFilter } from "./lib/tables/tableFilters";
import { bindPredicates, fieldFilterActive, fieldPredicate } from "./lib/tables/bindPredicates";
import { cellMatches, formulaColumnKey } from "./lib/tables/columnPredicate";
import type { ColumnPredicate } from "./lib/tables/columnPredicate";
import { matchFormula } from "./lib/tables/formulas/catalog";
import { evalFormula } from "./lib/tables/formulas/eval";
import { useSheetFormulas } from "./lib/tables/useSheetFormulas";
import { useSheetColumnWidths } from "./lib/tables/useSheetColumnWidths";
import { resultColumnWidth } from "./lib/tables/resultColumnWidth";
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
import { pagePatch, resetPage } from "./lib/tables/sheetQuery";
import { useInspectorSheet } from "./lib/tables/useInspectorSheet";
import { paginationRange, type TableRow } from "./lib/tables/tableRows";
import { DEFAULT_SHEET_PREFERENCES } from "./lib/tables/sheetPreferences";
import { clampSpreadsheetPageSize } from "./lib/tables/spreadsheetViewport";
import { useTableSheetData } from "./lib/tables/useTableSheetData";
import { TableRowActions } from "./lib/tables/TableRowActions";
import {
  tableRowActionSubject,
  tableRowEntityId,
  tableRowEntityKind,
  tableRowSearchPatch,
} from "./lib/tables/tableRowNavigation";
import {
  AssistantSessionFloat,
  type AssistantWindowSize,
} from "./lib/assistant/AssistantSessionFloat";
import { getChatCapability, getChatResultPage } from "./lib/assistant/assistantApi";
import {
  ASSISTANT_CAPABILITY_KEY,
  ASSISTANT_CAPABILITY_STALE_MS,
} from "./lib/assistant/useAssistantChat";
import { downloadChatResultXlsx } from "./lib/assistant/recorteExport";
import {
  RECORTE_GRID_LIMIT,
  filterRecorteRows,
  pageRecorteRows,
  sortRecorteRows,
} from "./lib/assistant/recorteSheet";
import { clearRecortePatch, recorteFilterParts } from "./lib/assistant/tableRecorte";

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
  const [extraPredicates, setExtraPredicates] = useState<Record<string, ColumnPredicate>>({});
  const { formulas, setFormula } = useSheetFormulas(scope);
  const widthScope = search.result ? `${scope}:recorte` : scope;
  const { widths: columnWidths, setWidth: setColumnWidth } = useSheetColumnWidths(widthScope);
  const [recorteQuery, setRecorteQuery] = useState("");
  const [recorteFilters, setRecorteFilters] = useState<Record<string, string>>({});
  const [recorteSort, setRecorteSort] = useState<{ field: string; order: "asc" | "desc" }>({
    field: "",
    order: "asc",
  });
  const [notice, setNotice] = useState<string | null>(null);
  const [assistantSize, setAssistantSize] = useState<AssistantWindowSize>("min");
  const [resultRowId, setResultRowId] = useState<string | undefined>();
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
  const [prevResult, setPrevResult] = useState(search.result);
  if (search.result !== prevResult) {
    setPrevResult(search.result);
    setRecorteQuery("");
    setRecorteFilters({});
    setRecorteSort({ field: "", order: "asc" });
  }

  const setLocal = useCallback(
    (key: string, value: string) => {
      setLocalFilters((current) => ({ ...current, [key]: value }));
      updateSearch(resetPage(section));
    },
    [section, updateSearch],
  );

  const recorteSummary = useMemo(() => {
    const labels: Record<string, string> = {
      full_name: copy.columns.fullName,
      cpf: copy.columns.cpf,
      email: copy.columns.email,
      city: copy.columns.city,
      state: copy.columns.state,
      document_identifier: copy.columns.identifier,
      document_medium: copy.columns.medium,
      bill_reference: copy.columns.reference,
      bill_competence: copy.columns.competence,
      bill_medium: copy.columns.medium,
    };
    const text = recorteFilterParts(search)
      .map((part) => `${labels[part.key] ?? part.key}: ${part.value}`)
      .join(" · ");
    return text || copy.assistant.recorte;
  }, [copy, search]);

  const showAssistantResult = useCallback(
    (referenceId: string) => {
      setResultRowId(undefined);
      updateSearch({ result: referenceId, ...resetPage(section) });
    },
    [section, updateSearch],
  );

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

  const resultQuery = useQuery({
    queryKey: ["chat-result-grid", search.result],
    queryFn: ({ signal }) => getChatResultPage(search.result, RECORTE_GRID_LIMIT, 0, signal),
    enabled: Boolean(search.result),
    staleTime: 30_000,
  });
  // Warm Assistente capability so opening the float does not wait on the first fetch.
  useQuery({
    queryKey: ASSISTANT_CAPABILITY_KEY,
    queryFn: ({ signal }) => getChatCapability(signal),
    staleTime: ASSISTANT_CAPABILITY_STALE_MS,
    retry: 1,
  });
  const exportRecorte = useMutation({
    mutationFn: () => downloadChatResultXlsx(search.result),
  });
  const resultRows = useMemo<TableRow[]>(() => {
    return (resultQuery.data?.rows ?? []).map((row) => ({
      id: row.id,
      cells: {
        ...row.cells,
        entityKind: row.entity_kind,
        entityId: row.entity_id,
        entityLabel: row.entity_label,
      },
    }));
  }, [resultQuery.data]);
  const resultColumns = useMemo(
    () =>
      (resultQuery.data?.columns ?? []).map((column) =>
        withColumnLayout(
          {
            key: column.key,
            title: column.label,
            label: column.label,
            sortField: column.key,
            width: resultColumnWidth(column.key, column.label),
            defaultVisible: true,
            locked: true,
          },
          scope,
        ),
      ),
    [resultQuery.data, scope],
  );

  const onTablePage = useCallback(
    (nextPage: number, nextSize: number) => {
      updateSearch(pagePatch(section, nextPage, clampSpreadsheetPageSize(nextSize)));
    },
    [section, updateSearch],
  );

  const onTableSort = useCallback(
    (field: string, order: "asc" | "desc") => {
      if (search.result) {
        setRecorteSort({ field, order });
        return;
      }
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
    [search.result, section, updateSearch],
  );

  const closeInspector = useCallback(() => {
    setResultRowId(undefined);
    updateSearch({
      selected: undefined,
      mode: undefined,
      document_selected: undefined,
      document_mode: undefined,
      bill_selected: undefined,
      bill_mode: undefined,
    });
  }, [updateSearch]);

  const confirmDiscardEdit = useCallback(
    (apply: () => void) => {
      Modal.confirm({
        title: copy.inspector.discardTitle,
        content: copy.inspector.discardBody,
        okText: copy.inspector.discardOk,
        cancelText: copy.inspector.discardCancel,
        onOk: apply,
      });
    },
    [copy.inspector],
  );

  const openTableRow = useCallback(
    (row: TableRow, intent: "view" | "edit") => {
      const isResult = Boolean(search.result);
      const apply = () => {
        if (isResult) setResultRowId(row.id);
        startTransition(() =>
          updateSearch(tableRowSearchPatch(row, { section, isResult, intent })),
        );
      };
      const nextPersonId =
        tableRowEntityKind(row, { section, isResult }) === "person"
          ? tableRowEntityId(row, isResult)
          : undefined;
      if (search.mode === "edit" && search.selected && search.selected !== nextPersonId) {
        confirmDiscardEdit(apply);
        return;
      }
      apply();
    },
    [confirmDiscardEdit, search.mode, search.result, search.selected, section, updateSearch],
  );

  const onTableRowClick = useCallback((row: TableRow) => openTableRow(row, "view"), [openTableRow]);

  const onTableRowEdit = useCallback((row: TableRow) => openTableRow(row, "edit"), [openTableRow]);

  const isResult = Boolean(search.result);
  const renderRowActions = useCallback(
    (row: TableRow) => (
      <TableRowActions
        isResult={isResult}
        row={row}
        section={section}
        onEdit={onTableRowEdit}
        onNotice={setNotice}
      />
    ),
    [isResult, onTableRowEdit, section],
  );
  const rowActions = useMemo(
    () => ({ label: copy.rowActions.column, render: renderRowActions }),
    [copy.rowActions.column, renderRowActions],
  );
  const getRowLabel = useCallback(
    (row: TableRow) =>
      tableRowActionSubject(row, tableRowEntityKind(row, { section, isResult })) ||
      copy.rowActions.untitled,
    [copy.rowActions.untitled, isResult, section],
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
  const canDelete = session.user.role === "ADMIN" || session.user.role === "SUPERADMIN";

  const patchRecorteFilter = useCallback(
    (key: string, value: string) => {
      setRecorteFilters((current) => ({ ...current, [key]: value }));
      updateSearch(resetPage(section));
    },
    [section, updateSearch],
  );
  const recorteToolbarFilters = useMemo(
    () =>
      resultColumns.map((column) =>
        textFilter(
          column.key,
          column.label ?? String(column.title ?? column.key),
          recorteFilters[column.key] ?? "",
          (value) => patchRecorteFilter(column.key, value),
        ),
      ),
    [patchRecorteFilter, recorteFilters, resultColumns],
  );
  const formulaFields = useMemo(
    () =>
      Object.entries(formulas).map(([sourceKey, expression]) =>
        textFilter(
          formulaColumnKey(sourceKey),
          matchFormula(expression)?.name ?? "fx",
          "",
          () => undefined,
          true,
        ),
      ),
    [formulas],
  );
  // Columns without an entry in buildSheetFilters (team, sector, idle_custody, …) get a local
  // text funnel so every visible data header can filter the loaded page.
  // Split into two memos: structure (gapColumnKeys) only changes when columns/filters change;
  // values (gapLocalFilters) change on every keystroke. This prevents localFilters from
  // invalidating the expensive visibleColumns + allFilters iteration.
  const gapColumnKeys = useMemo(() => {
    if (search.result) return [];
    const covered = new Set(allFilters.flatMap((field) => [field.key, cellKeyForFilter(field)]));
    return visibleColumns
      .filter(
        (column) =>
          !column.render &&
          !covered.has(column.key) &&
          !covered.has(column.dataIndex ?? column.key),
      )
      .map((column) => ({
        key: column.key,
        label: column.label ?? String(column.title ?? column.key),
      }));
  }, [allFilters, search.result, visibleColumns]);
  const gapLocalFilters = useMemo(
    () =>
      gapColumnKeys.map(({ key, label }) =>
        textFilter(key, label, localFilters[key] ?? "", (value) => setLocal(key, value), true),
      ),
    [gapColumnKeys, localFilters, setLocal],
  );
  const sheetFilters = useMemo(
    () =>
      bindPredicates(
        [
          ...(search.result ? recorteToolbarFilters : [...allFilters, ...gapLocalFilters]),
          ...formulaFields,
        ],
        extraPredicates,
        setExtraPredicates,
      ),
    [
      allFilters,
      extraPredicates,
      formulaFields,
      gapLocalFilters,
      recorteToolbarFilters,
      search.result,
    ],
  );
  const formulaColumns = useMemo(
    () =>
      (search.result ? resultColumns : visibleColumns).map((column) => ({
        key: column.dataIndex ?? column.key,
        label: column.label ?? String(column.title ?? column.key),
      })),
    [resultColumns, search.result, visibleColumns],
  );

  const recorteWorkingRows = useMemo(() => {
    const filtered = filterRecorteRows(resultRows, recorteFilters, recorteQuery);
    return sortRecorteRows(filtered, recorteSort.field, recorteSort.order);
  }, [recorteFilters, recorteQuery, recorteSort.field, recorteSort.order, resultRows]);
  const matchedRows = useMemo(() => {
    const source = search.result ? recorteWorkingRows : baseRows;
    const withFx =
      Object.keys(formulas).length === 0
        ? source
        : source.map((row) => {
            const cells = { ...row.cells };
            for (const [sourceKey, expression] of Object.entries(formulas)) {
              try {
                cells[formulaColumnKey(sourceKey)] = evalFormula(expression, cells, formulaColumns);
              } catch {
                cells[formulaColumnKey(sourceKey)] = "";
              }
            }
            return { ...row, cells };
          });
    return withFx.filter((row) =>
      sheetFilters.every((field) => {
        if (!fieldFilterActive(field)) return true;
        if (search.result || field.local) {
          return cellMatches(row.cells[cellKeyForFilter(field)], fieldPredicate(field));
        }
        return true;
      }),
    );
  }, [baseRows, formulaColumns, formulas, recorteWorkingRows, search.result, sheetFilters]);
  const rows = useMemo(() => {
    if (search.result) return pageRecorteRows(matchedRows, page, pageSize);
    return matchedRows;
  }, [matchedRows, page, pageSize, search.result]);
  const activeChips = [
    ...(!search.result && search.records_owner
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
    ...activeFilterChips(sheetFilters),
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
  const assistantRecord = useMemo(() => {
    if (!search.result || !resultRowId) return undefined;
    const row = resultRows.find((value) => value.id === resultRowId);
    const kind = String(row?.cells.entityKind ?? "");
    if (!row || (kind !== "document" && kind !== "bill")) return undefined;
    return {
      kind: kind as "document" | "bill",
      row,
      columns: (resultQuery.data?.columns ?? []).map((column) => ({
        key: column.key,
        label: column.label,
      })),
    };
  }, [resultQuery.data, resultRowId, resultRows, search.result]);
  const closeRecord = useCallback(() => {
    setResultRowId(undefined);
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

  const applyFormula = useCallback(
    (sourceKey: string, expression: string) => setFormula(sourceKey, expression),
    [setFormula],
  );
  const removeFormula = useCallback((sourceKey: string) => setFormula(sourceKey, ""), [setFormula]);

  const sheetColumns = useMemo(() => {
    const source = search.result ? resultColumns : visibleColumns;
    // Pre-index filters by key for O(1) lookup instead of O(N) find() per column.
    const filterByKey = new Map(sheetFilters.map((f) => [f.key, f]));
    const filterByCellKey = new Map(sheetFilters.map((f) => [cellKeyForFilter(f), f]));
    const out = [];
    for (const column of source) {
      const sourceKey = column.dataIndex ?? column.key;
      const field = filterByKey.get(column.key) ?? filterByCellKey.get(sourceKey);
      const expression = formulas[sourceKey] ?? "";
      out.push({
        ...column,
        filterField: column.render ? undefined : field,
        formula: column.render
          ? undefined
          : {
              expression,
              onApply: (next: string) => applyFormula(sourceKey, next),
              onRemove: () => removeFormula(sourceKey),
            },
      });
      if (!expression) continue;
      const spec = matchFormula(expression);
      const key = formulaColumnKey(sourceKey);
      out.push({
        key,
        title: spec?.name ?? "fx",
        label: spec?.name ?? "fx",
        dataIndex: key,
        className: spec?.numeric ? "spreadsheet-table__numeric" : undefined,
        filterField: filterByKey.get(key),
      });
    }
    return out;
  }, [
    applyFormula,
    formulas,
    removeFormula,
    resultColumns,
    search.result,
    sheetFilters,
    visibleColumns,
  ]);

  const funnel = useMemo(
    () => ({
      copy: {
        filter: copy.filters.funnel,
        operator: copy.filters.operator,
        value: copy.filters.chooseValue,
        from: copy.filters.from,
        to: copy.filters.to,
        function: copy.filters.function,
        formula: copy.filters.formula,
        insert: copy.filters.insertFunction,
        removeFormula: copy.filters.removeFormula,
        ops: copy.filters.ops,
      },
      columns: formulaColumns,
    }),
    [copy.filters, formulaColumns],
  );

  return (
    <PageShell
      className="tables-page"
      description={sectionCopy.description}
      title={sectionCopy.title}
    >
      {notice ? <StatusBanner title={notice} tone="success" /> : null}
      {search.result && resultQuery.isError ? (
        <StateCard
          description={copy.assistant.errors.tool_failed}
          kind="error"
          title={copy.error}
        />
      ) : errorDescription && !search.result ? (
        <StateCard description={errorDescription} kind="error" title={copy.error} />
      ) : null}

      <TablesToolbar
        searchLabel={copy.filters.search}
        searchPlaceholder={search.result ? copy.filters.searchRecorte : copy.filters.searchAll}
        searchValue={search.result ? recorteQuery : searchInput}
        searchGrain={
          section === "documents" ? "documents" : section === "bills" ? "bills" : "profiles"
        }
        searchMode={search.result ? "local" : "suggest"}
        onSearchChange={
          search.result
            ? (value) => {
                setRecorteQuery(value);
                updateSearch(resetPage(section));
              }
            : setSearchInput
        }
        onSearchSubmit={
          search.result
            ? (value) => {
                setRecorteQuery(value);
                updateSearch(resetPage(section));
              }
            : (value) => {
                if (value === search.q) return;
                updateSearch({ q: value, ...resetPage(section) });
              }
        }
        appliedFiltersLabel={copy.filters.appliedFilters}
        moreChipsLabel={(count) => t(copy.filters.moreChips, { count })}
        moreChipsCollapseLabel={copy.filters.moreChipsCollapse}
        chips={activeChips}
        clearLabel={copy.filters.clear}
        onClearAll={() => {
          if (search.result) {
            setRecorteQuery("");
            setRecorteFilters({});
            setExtraPredicates({});
            setRecorteSort({ field: "", order: "asc" });
            updateSearch(resetPage(section));
            return;
          }
          setLocalFilters({});
          setExtraPredicates({});
          updateSearch(clearRecortePatch(search));
        }}
        localHint={copy.filters.localOnly}
        localScopeBadge={copy.filters.localScopeBadge}
        columnPicker={
          search.result
            ? undefined
            : {
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
              }
        }
        assistant={{
          active: assistantSize !== "min",
          label: copy.assistant.assistant,
          onClick: () => setAssistantSize("float"),
        }}
      />

      {search.result ? (
        <div className="assistant-banner">
          <Sparkles
            aria-hidden
            className="assistant-banner__mark"
            size={ICON.sm}
            strokeWidth={ICON_STROKE}
          />
          <span className="assistant-banner__text">
            {copy.assistant.showingResult}
            {resultQuery.data?.summary ? ` ${resultQuery.data.summary}` : ""}
          </span>
          <div className="assistant-banner__actions">
            <button
              className="assistant-banner__export"
              disabled={exportRecorte.isPending}
              type="button"
              onClick={() => exportRecorte.mutate()}
            >
              {exportRecorte.isPending
                ? copy.assistant.exportingRecorte
                : copy.assistant.exportRecorte}
            </button>
            <button
              className="assistant-banner__clear"
              type="button"
              onClick={() => {
                setResultRowId(undefined);
                updateSearch({ result: "" });
              }}
            >
              {copy.assistant.clearResult}
            </button>
            {exportRecorte.isError ? (
              <span className="assistant-banner__error">{copy.assistant.exportRecorteError}</span>
            ) : null}
          </div>
        </div>
      ) : search.recorte ? (
        <div className="assistant-banner">
          <Sparkles
            aria-hidden
            className="assistant-banner__mark"
            size={ICON.sm}
            strokeWidth={ICON_STROKE}
          />
          <span className="assistant-banner__text">{recorteSummary}</span>
          <div className="assistant-banner__actions">
            <button
              className="assistant-banner__clear"
              type="button"
              onClick={() => updateSearch(clearRecortePatch(search))}
            >
              {copy.assistant.clearRecorte}
            </button>
          </div>
          <span className="assistant-banner__chips">
            <Tag className="tables-toolbar__chip">{sectionCopy.title}</Tag>
          </span>
        </div>
      ) : null}

      <div className="tables-page__workspace">
        <MemoSpreadsheetTable
          caption={sectionCopy.caption}
          columns={sheetColumns}
          emptyLabel={sectionCopy.empty}
          loading={search.result ? resultQuery.isFetching : loading}
          loadingLabel={sectionCopy.loading}
          page={page}
          pageSize={pageSize}
          rangeLabel={paginationRange(
            page,
            pageSize,
            search.result ? matchedRows.length : total,
            copy.pagination.range,
          )}
          pageSizeAriaLabel={copy.pagination.pageSizeAria}
          pageSizeOptionLabel={copy.pagination.pageSize}
          rows={rows}
          selectedRowId={
            search.result
              ? resultRowId
              : section === "documents"
                ? search.document_selected
                : section === "bills"
                  ? search.bill_selected
                  : search.selected
          }
          sortField={search.result ? recorteSort.field : sortField}
          sortOrder={search.result ? recorteSort.order : sortOrder}
          total={search.result ? matchedRows.length : total}
          onPage={onTablePage}
          onRowClick={onTableRowClick}
          onSort={onTableSort}
          rowActions={search.result ? undefined : rowActions}
          getRowLabel={getRowLabel}
          funnel={funnel}
          columnWidths={columnWidths}
          onColumnWidth={setColumnWidth}
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
          assistantRecord={assistantRecord}
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
        <AssistantSessionFloat
          size={assistantSize}
          tableLabel={sectionCopy.title}
          activeResultId={search.result || undefined}
          onShowResult={showAssistantResult}
          onSizeChange={setAssistantSize}
        />
      </div>
    </PageShell>
  );
}

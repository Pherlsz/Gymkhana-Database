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
import {
  bindPredicates,
  clientPredicate,
  fieldFilterActive,
  serverAppliesPredicate,
} from "./lib/tables/bindPredicates";
import { cellMatches, formulaColumnKey } from "./lib/tables/columnPredicate";
import type { ColumnPredicate } from "./lib/tables/columnPredicate";
import { matchFormula } from "./lib/tables/formulas/catalog";
import { evalFormula } from "./lib/tables/formulas/eval";
import { useSheetFormulas } from "./lib/tables/useSheetFormulas";
import { useSheetColumnWidths } from "./lib/tables/useSheetColumnWidths";
import { useSheetColumnVisibility } from "./lib/tables/useSheetColumnVisibility";
import { useAssistantResultSheet } from "./lib/tables/useAssistantResultSheet";
import {
  columnGroup,
  columnLabel,
  formatColumnCols,
  hideAllColumns,
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
import { getChatCapability } from "./lib/assistant/assistantApi";
import {
  ASSISTANT_CAPABILITY_KEY,
  ASSISTANT_CAPABILITY_STALE_MS,
} from "./lib/assistant/useAssistantChat";
import { pageRecorteRows } from "./lib/assistant/recorteSheet";
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
  const [localFilters, setLocalFilters] = useState<Record<string, string>>({});
  const [extraPredicates, setExtraPredicates] = useState<Record<string, ColumnPredicate>>({});
  const { formulas, setFormula } = useSheetFormulas(scope);
  const widthScope = search.result ? `${scope}:recorte` : scope;
  const { widths: columnWidths, setWidth: setColumnWidth } = useSheetColumnWidths(widthScope);
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
        search: (current) => normalizeTableSearch({ ...current, ...rest, cols: "" }),
        to: "/tables/$table",
      });
    },
    [navigate, table],
  );

  const clearUrlCols = useCallback(() => {
    updateSearch({ cols: "" });
  }, [updateSearch]);
  const { overrides: columnOverrides, replace: replaceColumnOverrides } = useSheetColumnVisibility(
    tableFromSection(section),
    search.cols,
    clearUrlCols,
  );

  const [searchInput, setSearchInput] = useState(search.q);
  const [prevSearchQ, setPrevSearchQ] = useState(search.q);
  if (search.q !== prevSearchQ) {
    setPrevSearchQ(search.q);
    setSearchInput(search.q);
  }
  // Funnel and page-only filters belong to one grid. Keys repeat across tables
  // and assistant results (owner, notes, full_name…), so a leftover predicate
  // would silently narrow the next grid, often to nothing.
  const filterScope = `${section}|${search.result}`;
  const [prevFilterScope, setPrevFilterScope] = useState(filterScope);
  if (filterScope !== prevFilterScope) {
    setPrevFilterScope(filterScope);
    setLocalFilters({});
    setExtraPredicates({});
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
    sheetPlaceholder,
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

  const resetSectionPage = useCallback(
    () => updateSearch(resetPage(section)),
    [section, updateSearch],
  );
  const result = useAssistantResultSheet({
    resultId: search.result,
    scope,
    onFilterChange: resetSectionPage,
  });
  const { setQuery: setResultQuery, setSort: setResultSort, reset: resetResult } = result;
  // Warm Assistente capability so opening the float does not wait on the first fetch.
  const assistantCapability = useQuery({
    queryKey: ASSISTANT_CAPABILITY_KEY,
    queryFn: ({ signal }) => getChatCapability(signal),
    staleTime: ASSISTANT_CAPABILITY_STALE_MS,
    retry: 1,
  });
  const resultRows = result.rows;
  const resultColumns = result.columns;

  const onTablePage = useCallback(
    (nextPage: number, nextSize: number) => {
      updateSearch(pagePatch(section, nextPage, clampSpreadsheetPageSize(nextSize)));
    },
    [section, updateSearch],
  );

  const onTableSort = useCallback(
    (field: string, order: "asc" | "desc") => {
      if (search.result) {
        setResultSort({ field, order });
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
    [search.result, section, setResultSort, updateSearch],
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

  const recorteToolbarFilters = result.toolbarFilters;
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

  const matchedRows = useMemo(() => {
    const source = search.result ? result.workingRows : baseRows;
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
    const browserFilters = sheetFilters
      .filter(
        (field) => fieldFilterActive(field) && (search.result || !serverAppliesPredicate(field)),
      )
      .map((field) => ({ key: cellKeyForFilter(field), predicate: clientPredicate(field) }));
    if (browserFilters.length === 0) return withFx;
    return withFx.filter((row) =>
      browserFilters.every(({ key, predicate }) => cellMatches(row.cells[key], predicate)),
    );
  }, [baseRows, formulaColumns, formulas, result.workingRows, search.result, sheetFilters]);
  const rows = useMemo(() => {
    if (search.result) return pageRecorteRows(matchedRows, page, pageSize);
    return matchedRows;
  }, [matchedRows, page, pageSize, search.result]);
  const activeChips = [
    ...(!search.result && search.q.trim()
      ? [
          {
            key: "q",
            field: copy.filters.searchChip,
            value: search.q.trim(),
            onClear: () => updateSearch({ q: "", ...resetPage(section) }),
          },
        ]
      : []),
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
    ...activeFilterChips(sheetFilters, {
      markLocal: !search.result,
      opLabels: copy.filters.ops,
    }),
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
      replaceColumnOverrides(parseColumnCols(cols));
    },
    [replaceColumnOverrides],
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
      columns: (result.data?.columns ?? []).map((column) => ({
        key: column.key,
        label: column.label,
      })),
    };
  }, [result.data, resultRowId, resultRows, search.result]);
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

  const columnsForKeys = (keys: string[]) => {
    const selected = new Set(keys);
    return columns.filter((column) => selected.has(column.key));
  };

  const handleColumnShowAll = (keys: string[]) => {
    updateCols(formatColumnCols(showAllColumns(columnsForKeys(keys), columnOverrides)));
  };

  const handleColumnHideAll = (keys: string[]) => {
    updateCols(formatColumnCols(hideAllColumns(columnsForKeys(keys), columnOverrides)));
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

  const exportRecorte = result.exportResult;
  const leaveResult = () => {
    setResultRowId(undefined);
    updateSearch({ result: "" });
  };
  const sheetError = search.result
    ? result.errorKind
      ? {
          title: copy.assistant.resultError,
          description:
            result.errorKind === "expired"
              ? copy.assistant.resultExpired
              : result.errorKind === "missing"
                ? copy.assistant.resultMissing
                : copy.assistant.errors.tool_failed,
          retry: result.errorKind === "failed" ? () => void result.refetch() : undefined,
        }
      : undefined
    : errorDescription
      ? {
          title: copy.error,
          description: errorDescription,
          retry: () => void queryClient.invalidateQueries({ queryKey: queryKeys.tables.all }),
        }
      : undefined;
  const narrowed = search.result
    ? Boolean(result.query.trim()) || activeChips.length > 0
    : activeChips.length > 0;
  const emptyLabel =
    search.result && result.rows.length === 0
      ? copy.assistant.resultEmpty
      : narrowed
        ? copy.filters.noMatches
        : sectionCopy.empty;

  return (
    <PageShell
      className={assistantSize === "min" ? "tables-page" : "tables-page is-assistant-open"}
      description={sectionCopy.description}
      title={sectionCopy.title}
    >
      {notice ? <StatusBanner title={notice} tone="success" /> : null}

      <TablesToolbar
        searchLabel={copy.filters.search}
        searchPlaceholder={search.result ? copy.filters.searchRecorte : copy.filters.searchAll}
        searchValue={search.result ? result.query : searchInput}
        searchGrain={
          section === "documents" ? "documents" : section === "bills" ? "bills" : "profiles"
        }
        searchMode={search.result ? "local" : "suggest"}
        onSearchChange={
          search.result
            ? (value) => {
                setResultQuery(value);
                resetSectionPage();
              }
            : setSearchInput
        }
        onSearchSubmit={
          search.result
            ? (value) => {
                if (value === result.query) return;
                setResultQuery(value);
                resetSectionPage();
              }
            : (value) => {
                const next = value.trim();
                if (next === search.q.trim()) return;
                updateSearch({ q: next, ...resetPage(section) });
              }
        }
        appliedFiltersLabel={copy.filters.appliedFilters}
        moreChipsLabel={(count) => t(copy.filters.moreChips, { count })}
        moreChipsCollapseLabel={copy.filters.moreChipsCollapse}
        chips={activeChips}
        clearLabel={copy.filters.clear}
        onClearAll={() => {
          if (search.result) {
            resetResult();
            setExtraPredicates({});
            resetSectionPage();
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
                hideAllLabel: copy.columnPicker.hideAll,
                resetLabel: copy.columnPicker.reset,
                lockedLabel: copy.columnPicker.locked,
                emptyLabel: copy.columnPicker.empty,
                visibleCountLabel: (visible, totalCount) =>
                  t(copy.columnPicker.visibleCount, { visible, total: totalCount }),
                hiddenCount: columnMetadataLoading ? 0 : columns.length - visibleColumns.length,
                items: columnPickerItems,
                onToggle: handleColumnToggle,
                onShowAll: handleColumnShowAll,
                onHideAll: handleColumnHideAll,
                onReset: handleColumnReset,
              }
        }
        assistant={
          assistantCapability.data?.enabled
            ? {
                active: assistantSize !== "min",
                label: copy.assistant.assistant,
                onClick: () => setAssistantSize("float"),
              }
            : undefined
        }
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
            {result.data?.summary ? ` ${result.data.summary}` : ""}
          </span>
          <div className="assistant-banner__actions">
            <button
              className="assistant-banner__export"
              disabled={exportRecorte.isPending || !result.data || result.rows.length === 0}
              type="button"
              onClick={() => exportRecorte.mutate()}
            >
              {exportRecorte.isPending
                ? copy.assistant.exportingRecorte
                : copy.assistant.exportRecorte}
            </button>
            <button className="assistant-banner__clear" type="button" onClick={leaveResult}>
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
        {sheetError ? (
          <StateCard
            className="tables-page__state"
            compact
            description={sheetError.description}
            kind="error"
            title={sheetError.title}
            onRetry={sheetError.retry}
            {...(search.result
              ? { onBack: leaveResult, backLabel: copy.assistant.clearResult }
              : {})}
          />
        ) : (
          <MemoSpreadsheetTable
            caption={sectionCopy.caption}
            columns={sheetColumns}
            emptyLabel={emptyLabel}
            loading={search.result ? result.fetching : loading || sheetPlaceholder}
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
            sortField={search.result ? result.sort.field : search.q.trim() ? "" : sortField}
            sortOrder={search.result ? result.sort.order : search.q.trim() ? "asc" : sortOrder}
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
        )}

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

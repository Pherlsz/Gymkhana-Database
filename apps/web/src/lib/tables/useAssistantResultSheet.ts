import { useMutation, useQuery } from "@tanstack/react-query";
import { useCallback, useMemo, useState } from "react";
import { APIRequestError } from "../api/client";
import { getChatResultPage } from "../assistant/assistantApi";
import { downloadChatResultXlsx } from "../assistant/recorteExport";
import { RECORTE_GRID_LIMIT, filterRecorteRows, sortRecorteRows } from "../assistant/recorteSheet";
import { withColumnLayout, type TableSheetScope } from "./columnVisibility";
import { resultColumnWidth } from "./resultColumnWidth";
import { textFilter } from "./tableFilters";
import type { TableRow } from "./tableRows";

export type AssistantResultErrorKind = "expired" | "missing" | "failed";

export function assistantResultErrorKind(error: unknown): AssistantResultErrorKind | undefined {
  if (!error) return undefined;
  if (error instanceof APIRequestError) {
    if (error.status === 410) return "expired";
    if (error.status === 404 || error.status === 403) return "missing";
  }
  return "failed";
}

/**
 * The grid while it shows an assistant result: one request loads the result,
 * and search, column funnels and sort run on those rows in the browser.
 */
export function useAssistantResultSheet({
  resultId,
  scope,
  onFilterChange,
}: {
  resultId: string;
  scope: TableSheetScope;
  onFilterChange: () => void;
}) {
  const [query, setQuery] = useState("");
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [sort, setSort] = useState<{ field: string; order: "asc" | "desc" }>({
    field: "",
    order: "asc",
  });
  const [prevResult, setPrevResult] = useState(resultId);
  if (resultId !== prevResult) {
    setPrevResult(resultId);
    setQuery("");
    setFilters({});
    setSort({ field: "", order: "asc" });
  }

  const resultQuery = useQuery({
    queryKey: ["chat-result-grid", resultId],
    queryFn: ({ signal }) => getChatResultPage(resultId, RECORTE_GRID_LIMIT, 0, signal),
    enabled: Boolean(resultId),
    staleTime: 30_000,
    retry: (count, error) => count < 1 && !(error instanceof APIRequestError && error.status < 500),
  });
  const exportResult = useMutation({
    mutationFn: () => downloadChatResultXlsx(resultId),
  });

  const rows = useMemo<TableRow[]>(
    () =>
      (resultQuery.data?.rows ?? []).map((row) => ({
        id: row.id,
        cells: {
          ...row.cells,
          entityKind: row.entity_kind,
          entityId: row.entity_id,
          entityLabel: row.entity_label,
        },
      })),
    [resultQuery.data],
  );
  const columns = useMemo(
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

  const patchFilter = useCallback(
    (key: string, value: string) => {
      setFilters((current) => ({ ...current, [key]: value }));
      onFilterChange();
    },
    [onFilterChange],
  );
  const toolbarFilters = useMemo(
    () =>
      columns.map((column) =>
        textFilter(
          column.key,
          column.label ?? String(column.title ?? column.key),
          filters[column.key] ?? "",
          (value) => patchFilter(column.key, value),
        ),
      ),
    [columns, filters, patchFilter],
  );

  // Column funnels are matched by their predicate in the page; only the free
  // text search narrows here, so `^x` / `=x` funnel values are never read as text.
  const workingRows = useMemo(
    () => sortRecorteRows(filterRecorteRows(rows, {}, query), sort.field, sort.order),
    [query, rows, sort.field, sort.order],
  );

  const reset = useCallback(() => {
    setQuery("");
    setFilters({});
    setSort({ field: "", order: "asc" });
  }, []);

  return {
    query,
    setQuery,
    sort,
    setSort,
    reset,
    rows,
    columns,
    toolbarFilters,
    workingRows,
    data: resultQuery.data,
    fetching: resultQuery.isFetching,
    errorKind: resultId ? assistantResultErrorKind(resultQuery.error) : undefined,
    refetch: resultQuery.refetch,
    exportResult,
  };
}

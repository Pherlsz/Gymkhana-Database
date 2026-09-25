import { Card, Empty, Pagination, Select, Table, type TableProps } from "antd";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type Key,
  type PointerEvent as ReactPointerEvent,
  type ReactElement,
  type ReactNode,
} from "react";
import { DEFAULT_SHEET_PREFERENCES } from "./sheetPreferences";
import { SHEET_COLUMN_WIDTH } from "./sheetDefaults";
import { spreadsheetPageSizeOptions } from "./spreadsheetViewport";
import { t } from "../../i18n";
import { ColumnFunnel, type ColumnFunnelCopy } from "./ColumnFunnel";
import type { ToolbarFilterField } from "./FilterControl";
import { clampColumnWidth } from "./useSheetColumnWidths";

export type SpreadsheetColumn<T> = {
  key: string;
  title: ReactNode;
  label?: string | undefined;
  width?: number | undefined;
  sortField?: string | undefined;
  dataIndex?: string | undefined;
  render?: ((row: T) => ReactNode) | undefined;
  className?: string | undefined;
  defaultVisible?: boolean | undefined;
  locked?: boolean | undefined;
  filterField?: ToolbarFilterField | undefined;
  formula?:
    | {
        expression: string;
        onApply: (expression: string) => void;
        onRemove: () => void;
      }
    | undefined;
};

export type SpreadsheetFunnel = {
  copy: ColumnFunnelCopy;
  columns: { key: string; label: string }[];
};

type SpreadsheetRow = { id: string };

const RESIZE_EDGE_PX = 12;

function paintCellWidth(el: HTMLElement, width: number) {
  const px = `${width}px`;
  el.style.setProperty("width", px);
  el.style.setProperty("min-width", px);
  el.style.setProperty("max-width", px);
  el.style.setProperty("flex", `0 0 ${px}`);
}

/** Keep header + virtual body on the same pixel widths while dragging. */
function applyLiveColumnWidth(th: HTMLElement, width: number, tableWidth: number) {
  const root = th.closest(".spreadsheet-table");
  if (!(root instanceof HTMLElement)) return;
  const headerRow = th.parentElement;
  if (!headerRow) return;
  const index = Array.prototype.indexOf.call(headerRow.children, th);
  if (index < 0) return;

  const px = `${width}px`;
  const pxTotal = `${tableWidth}px`;

  // Header uses table-layout:fixed + colgroup — cell style alone does not move it.
  root.querySelectorAll(".ant-table-header colgroup").forEach((group) => {
    const col = group.children[index];
    if (col instanceof HTMLElement) col.style.width = px;
  });

  root.querySelectorAll<HTMLElement>(".ant-table-thead > tr").forEach((row) => {
    const cell = row.children[index];
    if (cell instanceof HTMLElement) paintCellWidth(cell, width);
  });

  const resizing = document.body.classList.contains("spreadsheet-col-resizing");
  root.querySelectorAll<HTMLElement>(".ant-table-tbody-virtual-holder-inner > div").forEach((row) => {
    const cell = row.children[index];
    if (cell instanceof HTMLElement) {
      paintCellWidth(cell, width);
      cell.classList.toggle("is-resizing", resizing);
    }
    row.style.width = pxTotal;
    row.style.minWidth = pxTotal;
  });

  root.querySelectorAll<HTMLElement>(".ant-table-header table").forEach((table) => {
    table.style.width = pxTotal;
    table.style.minWidth = pxTotal;
  });
  root.querySelectorAll<HTMLElement>(".ant-table-tbody-virtual-holder-inner").forEach((inner) => {
    inner.style.width = pxTotal;
    inner.style.minWidth = pxTotal;
  });
}

type VirtualTableHandle = {
  nativeElement: HTMLDivElement;
  scrollTo: (config: { index?: number; key?: Key; top?: number }) => void;
};

type VirtualAntdTableProps<T> = TableProps<T> & {
  listItemHeight?: number;
  ref?: { current: VirtualTableHandle | null };
};

const VirtualAntdTable = Table as <T>(props: VirtualAntdTableProps<T>) => ReactElement;

export function SpreadsheetTable<T extends SpreadsheetRow>({
  caption,
  columns,
  rows,
  loading,
  emptyLabel,
  loadingLabel,
  page,
  pageSize,
  total,
  sortField,
  sortOrder,
  selectedRowId,
  rangeLabel,
  pageSizeAriaLabel,
  pageSizeOptionLabel,
  onRowClick,
  onPage,
  onSort,
  rowActions,
  getRowLabel,
  funnel,
  columnWidths,
  onColumnWidth,
}: {
  caption: string;
  columns: SpreadsheetColumn<T>[];
  rows: T[];
  loading: boolean;
  emptyLabel: string;
  loadingLabel?: string | undefined;
  page: number;
  pageSize: number;
  total: number;
  sortField: string;
  sortOrder: "asc" | "desc";
  selectedRowId?: string | undefined;
  rangeLabel: string;
  pageSizeAriaLabel: string;
  pageSizeOptionLabel: string;
  onRowClick?: ((row: T) => void) | undefined;
  onPage: (page: number, pageSize: number) => void;
  onSort: (field: string, order: "asc" | "desc") => void;
  rowActions?:
    | {
        label: string;
        render: (row: T) => ReactNode;
      }
    | undefined;
  getRowLabel?: ((row: T) => string) | undefined;
  funnel?: SpreadsheetFunnel | undefined;
  columnWidths?: Record<string, number> | undefined;
  onColumnWidth?: ((key: string, width: number) => void) | undefined;
}) {
  const suppressSortClick = useRef(false);

  const resolvedWidth = useCallback(
    (column: SpreadsheetColumn<T>) =>
      columnWidths?.[column.key] ?? column.width ?? DEFAULT_SHEET_PREFERENCES.columnWidth,
    [columnWidths],
  );

  const tableWidth = Math.max(
    1,
    (rowActions ? SHEET_COLUMN_WIDTH.actions : 0) +
      columns.reduce((sum, column) => sum + resolvedWidth(column), 0),
  );

  const startResize = useCallback(
    (event: ReactPointerEvent<HTMLElement>, columnKey: string, startWidth: number) => {
      if (!onColumnWidth || event.button !== 0) return;
      if (
        event.target instanceof Element &&
        event.target.closest(".column-funnel__anchor, .column-funnel__trigger, .column-funnel")
      ) {
        return;
      }
      const th = event.currentTarget;
      const rect = th.getBoundingClientRect();
      if (rect.right - event.clientX > RESIZE_EDGE_PX) return;

      event.preventDefault();
      event.stopPropagation();

      const startX = event.clientX;
      const startTotal = tableWidth;
      let current = startWidth;
      let frame = 0;

      const root = th.closest(".spreadsheet-table");
      const columnIndex = Array.prototype.indexOf.call(th.parentElement?.children ?? [], th);

      const setResizingClass = (on: boolean) => {
        th.classList.toggle("is-resizing", on);
        if (!(root instanceof HTMLElement) || columnIndex < 0) return;
        root.querySelectorAll<HTMLElement>(".ant-table-tbody-virtual-holder-inner > div").forEach(
          (row) => {
            row.children[columnIndex]?.classList.toggle("is-resizing", on);
          },
        );
      };

      setResizingClass(true);
      document.body.classList.add("spreadsheet-col-resizing");
      th.setPointerCapture(event.pointerId);

      const paint = (next: number) => {
        applyLiveColumnWidth(th, next, startTotal + (next - startWidth));
      };

      const onMove = (move: PointerEvent) => {
        current = clampColumnWidth(startWidth + (move.clientX - startX));
        if (frame) return;
        frame = requestAnimationFrame(() => {
          frame = 0;
          paint(current);
        });
      };

      const onUp = (up: PointerEvent) => {
        if (frame) cancelAnimationFrame(frame);
        th.releasePointerCapture(up.pointerId);
        th.removeEventListener("pointermove", onMove);
        th.removeEventListener("pointerup", onUp);
        th.removeEventListener("pointercancel", onUp);
        paint(current);
        setResizingClass(false);
        document.body.classList.remove("spreadsheet-col-resizing");
        // Any pointer on the resize edge must not sort — including click / dblclick with no drag.
        suppressSortClick.current = true;
        if (current !== startWidth) onColumnWidth(columnKey, current);
      };

      th.addEventListener("pointermove", onMove);
      th.addEventListener("pointerup", onUp);
      th.addEventListener("pointercancel", onUp);
    },
    [onColumnWidth, tableWidth],
  );

  const pageSizeChoices = useMemo(
    () =>
      spreadsheetPageSizeOptions(pageSize).map((size) => ({
        value: size,
        label: t(pageSizeOptionLabel, { n: size }),
      })),
    [pageSize, pageSizeOptionLabel],
  );

  const antdColumns = useMemo(() => {
    const dataColumns = columns.map((column): NonNullable<TableProps<T>["columns"]>[number] => {
      const width = resolvedWidth(column);
      const sorted = Boolean(column.sortField && column.sortField === sortField);
      const cellKey = column.dataIndex ?? column.key;
      const cellStyle = { width, minWidth: width, flex: `0 0 ${width}px` };
      const field = column.sortField;
      return {
        key: column.key,
        title: (
          <span className="spreadsheet-table__head">
            <span className="spreadsheet-table__head-label">{column.title}</span>
            {funnel && column.filterField ? (
              <span className="column-funnel__anchor">
                <ColumnFunnel
                  columns={funnel.columns}
                  copy={funnel.copy}
                  field={column.filterField}
                  formula={column.formula}
                />
              </span>
            ) : null}
          </span>
        ),
        dataIndex: ["cells", cellKey],
        width,
        ellipsis: false,
        ...(column.className ? { className: column.className } : {}),
        sorter: Boolean(column.sortField),
        sortDirections: ["ascend", "descend"],
        sortOrder: sorted ? (sortOrder === "asc" ? "ascend" : "descend") : null,
        showSorterTooltip: false,
        onHeaderCell: () => ({
          style: { ...cellStyle, position: "relative" as const },
          ...(onColumnWidth
            ? {
                onPointerDown: (event: ReactPointerEvent<HTMLElement>) => {
                  startResize(event, column.key, width);
                },
              }
            : {}),
          ...(field
            ? {
                onClick: (event: {
                  target: EventTarget | null;
                  clientX?: number;
                  currentTarget?: EventTarget | null;
                }) => {
                  if (suppressSortClick.current) {
                    suppressSortClick.current = false;
                    return;
                  }
                  if (
                    event.target instanceof Element &&
                    event.target.closest(
                      ".column-funnel__anchor, .column-funnel__trigger, .column-funnel",
                    )
                  ) {
                    return;
                  }
                  if (document.body.classList.contains("spreadsheet-col-resizing")) return;
                  // Keep sort/reorder clear of the resize strip on the right edge.
                  if (
                    onColumnWidth &&
                    typeof event.clientX === "number" &&
                    event.currentTarget instanceof HTMLElement
                  ) {
                    const rect = event.currentTarget.getBoundingClientRect();
                    if (rect.right - event.clientX <= RESIZE_EDGE_PX) return;
                  }
                  if (field === sortField) {
                    onSort(field, sortOrder === "asc" ? "desc" : "asc");
                    return;
                  }
                  onSort(field, "asc");
                },
              }
            : {}),
        }),
        onCell: () => ({
          style: {
            ...cellStyle,
            display: "flex",
            alignItems: "center",
            padding: "0 0.55rem",
          },
        }),
        ...(column.render ? { render: (_value: unknown, row: T) => column.render?.(row) } : {}),
      };
    });
    if (!rowActions) return dataColumns;
    const width = SHEET_COLUMN_WIDTH.actions;
    const cellStyle = { width, minWidth: width, flex: `0 0 ${width}px` };
    const actionsColumn: NonNullable<TableProps<T>["columns"]>[number] = {
      key: "__actions",
      title: <span className="visually-hidden">{rowActions.label}</span>,
      width,
      ellipsis: false,
      className: "spreadsheet-table__actions",
      onHeaderCell: () => ({ style: { ...cellStyle, padding: 0 } }),
      onCell: () => ({
        style: {
          ...cellStyle,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          padding: 0,
        },
        onClick: (event: { stopPropagation: () => void }) => {
          event.stopPropagation();
        },
      }),
      render: (_value: unknown, row: T) => rowActions.render(row),
    };
    return [actionsColumn, ...dataColumns];
  }, [
    columns,
    funnel,
    onColumnWidth,
    onSort,
    resolvedWidth,
    rowActions,
    sortField,
    sortOrder,
    startResize,
  ]);

  const sheetRef = useRef<HTMLDivElement>(null);
  const tableRef = useRef<VirtualTableHandle>(null);
  const [bodyHeight, setBodyHeight] = useState(DEFAULT_SHEET_PREFERENCES.fallbackBodyHeightPx);
  const showEmpty = !loading && rows.length === 0;
  const tableLoading = loading
    ? {
        spinning: true,
        size: "large" as const,
        ...(loadingLabel ? { description: loadingLabel } : {}),
      }
    : false;

  useLayoutEffect(() => {
    const sheet = sheetRef.current;
    if (!sheet || typeof ResizeObserver === "undefined") return;
    const measure = () => {
      const header = sheet.querySelector(".ant-table-header");
      const headerHeight =
        header instanceof HTMLElement
          ? header.getBoundingClientRect().height
          : DEFAULT_SHEET_PREFERENCES.headerFallbackHeightPx;
      const next = Math.max(
        DEFAULT_SHEET_PREFERENCES.minBodyHeightPx,
        Math.floor(sheet.clientHeight - headerHeight),
      );
      setBodyHeight((current) => (Math.abs(current - next) <= 1 ? current : next));
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(sheet);
    return () => observer.disconnect();
  }, [columns.length, rowActions, showEmpty]);

  useEffect(() => {
    if (!onRowClick || !selectedRowId) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
      if (event.altKey || event.ctrlKey || event.metaKey) return;
      if (document.querySelector(".ant-modal-wrap")) return;
      const target = event.target;
      if (target instanceof HTMLElement) {
        const tag = target.tagName;
        if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || target.isContentEditable)
          return;
        if (
          target.closest(
            ".ant-modal, .ant-select-dropdown, .ant-picker-dropdown, .ant-popover, .column-funnel",
          )
        )
          return;
        const drawer = target.closest(".ant-drawer");
        if (drawer && !drawer.classList.contains("tables-inspector-sheet")) return;
      }
      if (rows.length === 0) return;
      const current = selectedRowId ? rows.findIndex((row) => row.id === selectedRowId) : -1;
      const delta = event.key === "ArrowDown" ? 1 : -1;
      const nextIndex =
        current === -1
          ? delta > 0
            ? 0
            : rows.length - 1
          : Math.min(rows.length - 1, Math.max(0, current + delta));
      const next = rows[nextIndex];
      if (!next || next.id === selectedRowId) return;
      event.preventDefault();
      onRowClick(next);
      tableRef.current?.scrollTo({ key: next.id });
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [onRowClick, rows, selectedRowId]);

  if (showEmpty) {
    return (
      <Card
        className="spreadsheet-table-card spreadsheet-table-card--empty"
        size="small"
        styles={{
          body: {
            display: "grid",
            height: "100%",
            minHeight: 0,
            minWidth: 0,
            padding: 0,
          },
          root: {
            display: "grid",
            gridTemplateRows: "minmax(0, 1fr)",
            height: "100%",
            width: "100%",
            minHeight: 0,
            minWidth: 0,
            overflow: "hidden",
          },
        }}
      >
        <div className="spreadsheet-table__empty" role="status">
          <span className="visually-hidden">{caption}</span>
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyLabel} />
        </div>
      </Card>
    );
  }

  return (
    <Card
      className="spreadsheet-table-card"
      size="small"
      styles={{
        body: {
          display: "grid",
          gridTemplateRows: "minmax(0, 1fr) auto",
          height: "100%",
          minHeight: 0,
          minWidth: 0,
          overflow: "hidden",
          padding: 0,
        },
        root: {
          display: "grid",
          gridTemplateRows: "minmax(0, 1fr)",
          height: "100%",
          width: "100%",
          minHeight: 0,
          minWidth: 0,
          overflow: "hidden",
        },
      }}
    >
      <div ref={sheetRef} aria-busy={loading} className="spreadsheet-table-card__sheet">
        <VirtualAntdTable<T>
          caption={<span className="visually-hidden">{caption}</span>}
          className="spreadsheet-table"
          columns={antdColumns}
          dataSource={loading && rows.length === 0 ? undefined : rows}
          loading={tableLoading}
          locale={{ emptyText: emptyLabel }}
          pagination={false}
          ref={tableRef}
          rowClassName={(row) => (row.id === selectedRowId ? "spreadsheet-table__row--active" : "")}
          rowKey="id"
          scroll={{ x: tableWidth, y: bodyHeight }}
          showSorterTooltip={false}
          size="small"
          tableLayout="fixed"
          virtual
          listItemHeight={DEFAULT_SHEET_PREFERENCES.rowHeightPx}
          onRow={
            onRowClick
              ? (row, index) => {
                  const label = getRowLabel?.(row);
                  return {
                    "aria-selected": row.id === selectedRowId,
                    ...(label ? { "aria-label": label } : {}),
                    role: "button",
                    tabIndex: row.id === selectedRowId || (!selectedRowId && index === 0) ? 0 : -1,
                    onClick: () => onRowClick(row),
                    onKeyDown: (event) => {
                      if (event.key !== "Enter" && event.key !== " ") return;
                      event.preventDefault();
                      onRowClick(row);
                    },
                  };
                }
              : (row) => {
                  const label = getRowLabel?.(row);
                  return {
                    "aria-selected": row.id === selectedRowId,
                    ...(label ? { "aria-label": label } : {}),
                  };
                }
          }
        />
      </div>
      <div className="spreadsheet-table__footer">
        <span className="spreadsheet-table__range">{rangeLabel}</span>
        <div className="spreadsheet-table__pager">
          <div className="spreadsheet-table__page-size">
            <Select
              aria-label={pageSizeAriaLabel}
              options={pageSizeChoices}
              popupMatchSelectWidth={false}
              showSearch={false}
              size="small"
              value={pageSize}
              onChange={(value) => onPage(page, value)}
            />
          </div>
          <Pagination
            current={page}
            pageSize={pageSize}
            total={total}
            showSizeChanger={false}
            size="small"
            showLessItems
            onChange={(nextPage) => onPage(nextPage, pageSize)}
          />
        </div>
      </div>
    </Card>
  );
}

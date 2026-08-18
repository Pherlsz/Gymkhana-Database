import { Card, Empty, Pagination, Select, Table, type TableProps } from "antd";
import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type Key,
  type ReactElement,
  type ReactNode,
} from "react";
import { DEFAULT_SHEET_PREFERENCES } from "./sheetPreferences";
import { spreadsheetPageSizeOptions } from "./spreadsheetViewport";

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
};

type SpreadsheetRow = { id: string };

function sheetCell<T extends SpreadsheetRow>(row: T, key: string): unknown {
  const cells = (row as T & { cells?: Record<string, unknown> }).cells;
  return cells?.[key];
}

type VirtualTableHandle = {
  nativeElement: HTMLDivElement;
  scrollTo: (config: { index?: number; key?: Key; top?: number }) => void;
};

/** antd 6 computes a default row height and omits listItemHeight from TableProps, but still forwards it to RcVirtualTable. */
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
}) {
  const tableWidth = Math.max(
    1,
    columns.reduce(
      (sum, column) => sum + (column.width ?? DEFAULT_SHEET_PREFERENCES.columnWidth),
      0,
    ),
  );

  const pageSizeChoices = useMemo(
    () =>
      spreadsheetPageSizeOptions(pageSize).map((size) => ({
        value: size,
        label: pageSizeOptionLabel.replace("{n}", String(size)),
      })),
    [pageSize, pageSizeOptionLabel],
  );

  const antdColumns = useMemo(
    () =>
      columns.map((column): NonNullable<TableProps<T>["columns"]>[number] => {
        const width = column.width ?? DEFAULT_SHEET_PREFERENCES.columnWidth;
        const sorted = Boolean(column.sortField && column.sortField === sortField);
        const cellKey = column.dataIndex ?? column.key;
        const cellStyle = { width, minWidth: width, flex: `0 0 ${width}px` };
        const field = column.sortField;
        return {
          key: column.key,
          title: column.title,
          dataIndex: ["cells", cellKey],
          width,
          ellipsis: false,
          ...(column.className ? { className: column.className } : {}),
          sorter: Boolean(column.sortField),
          sortDirections: ["ascend", "descend"],
          sortOrder: sorted ? (sortOrder === "asc" ? "ascend" : "descend") : null,
          showSorterTooltip: false,
          shouldCellUpdate: (current, previous) =>
            current.id !== previous.id ||
            sheetCell(current, cellKey) !== sheetCell(previous, cellKey),
          onHeaderCell: () => ({
            style: cellStyle,
            ...(field
              ? {
                  onClick: () => {
                    if (field === sortField) {
                      onSort(field, sortOrder === "asc" ? "desc" : "asc");
                      return;
                    }
                    onSort(field, "asc");
                  },
                }
              : {}),
          }),
          onCell: () => ({ style: cellStyle }),
          ...(column.render ? { render: (_value: unknown, row: T) => column.render?.(row) } : {}),
        };
      }),
    [columns, onSort, sortField, sortOrder],
  );

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
  }, [columns.length, showEmpty]);

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
        if (target.closest(".ant-modal, .ant-select-dropdown, .ant-picker-dropdown")) return;
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
              ? (row) => ({
                  "aria-selected": row.id === selectedRowId,
                  onClick: () => onRowClick(row),
                })
              : (row) => ({ "aria-selected": row.id === selectedRowId })
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

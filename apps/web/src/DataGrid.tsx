import {
  Button,
  Card,
  Checkbox,
  Flex,
  Table,
  type TablePaginationConfig,
  type TableProps,
} from "antd";
import type { ColumnDef } from "@tanstack/react-table";
import { useMemo, type ReactNode } from "react";
import { InlineStatus } from "./components/InlineStatus";
import { useI18n } from "./i18n";

type GridColumn<TData> = ColumnDef<TData, any>;

type LooseColumn<TData> = {
  id?: string;
  accessorKey?: string;
  accessorFn?: (row: TData, index: number) => unknown;
  header?: unknown;
  cell?: (context: { row: { original: TData }; getValue: () => unknown }) => ReactNode;
};

export function DataGrid<TData>({
  data,
  columns,
  getRowId,
  caption,
  loading,
  loadingLabel,
  emptyLabel,
  selectedRowId,
  renderCard,
  className,
  tableClassName,
  tableWrapClassName,
  cardsClassName,
  selection,
}: {
  data: TData[];
  columns: GridColumn<TData>[];
  getRowId: (row: TData) => string;
  caption: string;
  loading: boolean;
  loadingLabel: string;
  emptyLabel: string;
  selectedRowId?: string | undefined;
  renderCard?: ((row: TData) => ReactNode) | undefined;
  className?: string | undefined;
  tableClassName?: string | undefined;
  tableWrapClassName?: string | undefined;
  cardsClassName?: string | undefined;
  selection?:
    | {
        selectedIds: ReadonlySet<string>;
        onChange: (selectedIds: Set<string>) => void;
        rowLabel: (row: TData) => string;
      }
    | undefined;
}) {
  const { messages, t } = useI18n();
  const grid = messages.tables.grid;
  const surfaceClassName = ["data-grid", className].filter(Boolean).join(" ");
  const shellClassName = ["data-grid__table-shell", tableWrapClassName].filter(Boolean).join(" ");
  const cardsClasses = ["data-grid__cards", cardsClassName].filter(Boolean).join(" ");

  const antdColumns = useMemo(
    () =>
      columns.map((columnDef, index): NonNullable<TableProps<TData>["columns"]>[number] => {
        const column = columnDef as LooseColumn<TData>;
        const id =
          column.id ??
          (typeof column.accessorKey === "string" ? column.accessorKey : `col-${index}`);
        const read = (row: TData): unknown => {
          if (typeof column.accessorFn === "function") return column.accessorFn(row, index);
          if (typeof column.accessorKey === "string") {
            return (row as Record<string, unknown>)[column.accessorKey];
          }
          return undefined;
        };
        const header =
          typeof column.header === "string"
            ? column.header
            : column.header
              ? String(column.header)
              : "";
        const cell = typeof column.cell === "function" ? column.cell : undefined;
        const hasValue =
          typeof column.accessorFn === "function" || typeof column.accessorKey === "string";
        return {
          key: id,
          title: header,
          render: (_: unknown, row: TData) =>
            cell
              ? cell({
                  row: { original: row },
                  getValue: () => read(row),
                } as any)
              : renderText(read(row)),
          ...(hasValue
            ? {
                sorter: (a: TData, b: TData) => compareValues(read(a), read(b)),
              }
            : {}),
        };
      }),
    [columns, data],
  );

  const pagination: TablePaginationConfig | false =
    data.length > 10 ? { defaultPageSize: 10, showSizeChanger: true } : false;

  return (
    <Card aria-busy={loading} className={surfaceClassName}>
      {loading ? (
        <InlineStatus className="data-grid__status" kind="loading" label={loadingLabel} />
      ) : null}
      {!loading && data.length === 0 ? (
        <InlineStatus className="data-grid__status" kind="empty" label={emptyLabel} />
      ) : null}
      {!loading && data.length > 0 ? (
        <>
          <div className={shellClassName}>
            <Table<TData>
              {...(tableClassName ? { className: tableClassName } : {})}
              columns={antdColumns}
              dataSource={data}
              locale={{ emptyText: emptyLabel }}
              pagination={pagination}
              rowKey={getRowId}
              {...(selection
                ? {
                    rowSelection: {
                      selectedRowKeys: [...selection.selectedIds],
                      onChange: (keys) =>
                        selection.onChange(new Set(keys.map((key) => String(key)))),
                      columnTitle: (
                        <span className="visually-hidden">{t(grid.selectAll, { caption })}</span>
                      ),
                      getCheckboxProps: (row: TData) => ({
                        "aria-label": t(grid.selectRow, { label: selection.rowLabel(row) }),
                      }),
                    },
                  }
                : {})}
              onRow={(row) => ({
                "aria-selected":
                  getRowId(row) === selectedRowId || selection?.selectedIds.has(getRowId(row)),
              })}
              scroll={{ x: "max-content" }}
              size="middle"
              caption={<span className="visually-hidden">{caption}</span>}
            />
          </div>
          {renderCard ? (
            <div className={cardsClasses}>
              {data.map((row) => {
                const id = getRowId(row);
                return (
                  <div className="data-grid__card-shell" key={id}>
                    {selection ? (
                      <div className="data-grid__card-selection">
                        <Checkbox
                          aria-label={grid.selectRowCard.replace(
                            "{label}",
                            selection.rowLabel(row),
                          )}
                          checked={selection.selectedIds.has(id)}
                          onChange={(event) => {
                            const next = new Set(selection.selectedIds);
                            if (event.target.checked) next.add(id);
                            else next.delete(id);
                            selection.onChange(next);
                          }}
                        />
                      </div>
                    ) : null}
                    {renderCard(row)}
                  </div>
                );
              })}
            </div>
          ) : null}
        </>
      ) : null}
    </Card>
  );
}

function renderText(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return "";
}

function rankValue(value: unknown): number {
  return value === null || value === undefined ? 1 : 0;
}

function compareValues(a: unknown, b: unknown): number {
  const order = rankValue(a) - rankValue(b);
  if (order !== 0) return order;
  if (typeof a === "number" && typeof b === "number") return a - b;
  if (typeof a === "boolean" && typeof b === "boolean") return Number(a) - Number(b);
  return String(a ?? "").localeCompare(String(b ?? ""), "pt-BR");
}

export function DataGridPagination({
  page,
  totalPages,
  total,
  label,
  onPage,
}: {
  page: number;
  totalPages: number;
  total: number;
  label: string;
  onPage: (page: number) => void;
}) {
  const { messages, t } = useI18n();
  const grid = messages.tables.grid;
  return (
    <Flex align="center" className="data-grid__pagination">
      <Button disabled={page <= 1} onClick={() => onPage(page - 1)}>
        {grid.previousPage}
      </Button>
      <span aria-live="polite">{t(grid.pageStatus, { page, totalPages, total, label })}</span>
      <Button disabled={page >= totalPages} onClick={() => onPage(page + 1)}>
        {grid.nextPage}
      </Button>
    </Flex>
  );
}

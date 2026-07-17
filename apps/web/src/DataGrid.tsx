import { Button, Inline, Surface } from "@pherlsz/gymkhana-ui";
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from "@tanstack/react-table";
import type { ReactNode } from "react";

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
}: {
  data: TData[];
  columns: ColumnDef<TData, any>[];
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
}) {
  const table = useReactTable({ data, columns, getCoreRowModel: getCoreRowModel(), getRowId });
  const surfaceClassName = ["data-grid", className].filter(Boolean).join(" ");
  const wrapClassName = ["data-grid__table-wrap", tableWrapClassName].filter(Boolean).join(" ");
  const tableClasses = ["data-grid__table", tableClassName].filter(Boolean).join(" ");
  const cardsClasses = ["data-grid__cards", cardsClassName].filter(Boolean).join(" ");

  return (
    <Surface aria-busy={loading} className={surfaceClassName} tone="raised">
      {loading ? (
        <p className="data-grid__status" role="status">
          {loadingLabel}
        </p>
      ) : null}
      {!loading && data.length === 0 ? <p className="data-grid__status">{emptyLabel}</p> : null}
      {!loading && data.length > 0 ? (
        <>
          <div className={wrapClassName}>
            <table className={tableClasses}>
              <caption className="visually-hidden">{caption}</caption>
              <thead>
                {table.getHeaderGroups().map((group) => (
                  <tr key={group.id}>
                    {group.headers.map((header) => (
                      <th key={header.id} scope="col">
                        {header.isPlaceholder
                          ? null
                          : flexRender(header.column.columnDef.header, header.getContext())}
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody>
                {table.getRowModel().rows.map((row) => (
                  <tr aria-selected={row.id === selectedRowId} key={row.id}>
                    {row.getVisibleCells().map((cell) => (
                      <td key={cell.id}>
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {renderCard ? (
            <div className={cardsClasses}>{data.map((row) => renderCard(row))}</div>
          ) : null}
        </>
      ) : null}
    </Surface>
  );
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
  return (
    <Inline align="center" className="data-grid__pagination">
      <Button disabled={page <= 1} onClick={() => onPage(page - 1)}>
        Anterior
      </Button>
      <span aria-live="polite">
        Página {page} de {totalPages} · {total} {label}
      </span>
      <Button disabled={page >= totalPages} onClick={() => onPage(page + 1)}>
        Próxima
      </Button>
    </Inline>
  );
}

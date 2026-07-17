import { Button, Inline, Surface } from "@pherlsz/gymkhana-ui";
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from "@tanstack/react-table";
import { useEffect, useRef, type ReactNode } from "react";

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
  selection?:
    | {
        selectedIds: ReadonlySet<string>;
        onChange: (selectedIds: Set<string>) => void;
        rowLabel: (row: TData) => string;
      }
    | undefined;
}) {
  const table = useReactTable({ data, columns, getCoreRowModel: getCoreRowModel(), getRowId });
  const surfaceClassName = ["data-grid", className].filter(Boolean).join(" ");
  const wrapClassName = ["data-grid__table-wrap", tableWrapClassName].filter(Boolean).join(" ");
  const tableClasses = ["data-grid__table", tableClassName].filter(Boolean).join(" ");
  const cardsClasses = ["data-grid__cards", cardsClassName].filter(Boolean).join(" ");
  const pageIDs = data.map(getRowId);
  const selectedOnPage = pageIDs.filter((id) => selection?.selectedIds.has(id)).length;
  const allSelected = pageIDs.length > 0 && selectedOnPage === pageIDs.length;
  const partlySelected = selectedOnPage > 0 && !allSelected;
  const togglePage = (checked: boolean) => {
    if (!selection) return;
    const next = new Set(selection.selectedIds);
    for (const id of pageIDs) {
      if (checked) next.add(id);
      else next.delete(id);
    }
    selection.onChange(next);
  };
  const toggleRow = (id: string, checked: boolean) => {
    if (!selection) return;
    const next = new Set(selection.selectedIds);
    if (checked) next.add(id);
    else next.delete(id);
    selection.onChange(next);
  };

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
                    {selection ? (
                      <th className="data-grid__selection" scope="col">
                        <SelectionCheckbox
                          checked={allSelected}
                          indeterminate={partlySelected}
                          label={`Selecionar todas as linhas de ${caption}`}
                          onChange={togglePage}
                        />
                      </th>
                    ) : null}
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
                  <tr
                    aria-selected={row.id === selectedRowId || selection?.selectedIds.has(row.id)}
                    key={row.id}
                  >
                    {selection ? (
                      <td className="data-grid__selection">
                        <SelectionCheckbox
                          checked={selection.selectedIds.has(row.id)}
                          indeterminate={false}
                          label={`Selecionar ${selection.rowLabel(row.original)}`}
                          onChange={(checked) => toggleRow(row.id, checked)}
                        />
                      </td>
                    ) : null}
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
            <div className={cardsClasses}>
              {data.map((row) => {
                const id = getRowId(row);
                return (
                  <div className="data-grid__card-shell" key={id}>
                    {selection ? (
                      <div className="data-grid__card-selection">
                        <SelectionCheckbox
                          checked={selection.selectedIds.has(id)}
                          indeterminate={false}
                          label={`Selecionar ${selection.rowLabel(row)} no cartão`}
                          onChange={(checked) => toggleRow(id, checked)}
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
    </Surface>
  );
}

function SelectionCheckbox({
  checked,
  indeterminate,
  label,
  onChange,
}: {
  checked: boolean;
  indeterminate: boolean;
  label: string;
  onChange: (checked: boolean) => void;
}) {
  const reference = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (reference.current) reference.current.indeterminate = indeterminate;
  }, [indeterminate]);
  return (
    <input
      aria-label={label}
      checked={checked}
      ref={reference}
      type="checkbox"
      onChange={(event) => onChange(event.currentTarget.checked)}
    />
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

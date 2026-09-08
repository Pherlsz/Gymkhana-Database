import { fireEvent, render, screen } from "@testing-library/react";
import { createColumnHelper } from "@tanstack/react-table";
import { describe, expect, it, vi } from "vitest";
import { DataGrid, DataGridPagination } from "./DataGrid";

type Row = { id: string; name: string };
const helper = createColumnHelper<Row>();
const columns = [helper.accessor("name", { header: "Nome", cell: (cell) => cell.getValue() })];

describe("DataGrid", () => {
  it("renders accessible loading and empty states", () => {
    const { rerender } = render(
      <DataGrid
        caption="Pessoas"
        columns={columns}
        data={[]}
        emptyLabel="Nenhuma pessoa"
        getRowId={(row) => row.id}
        loading
        loadingLabel="Carregando pessoas"
      />,
    );
    expect(screen.getByRole("status")).toHaveTextContent("Carregando pessoas");
    rerender(
      <DataGrid
        caption="Pessoas"
        columns={columns}
        data={[]}
        emptyLabel="Nenhuma pessoa"
        getRowId={(row) => row.id}
        loading={false}
        loadingLabel="Carregando pessoas"
      />,
    );
    expect(screen.getByText("Nenhuma pessoa")).toBeInTheDocument();
  });

  it("renders desktop and mobile representations with selected row state", () => {
    render(
      <DataGrid
        caption="Pessoas"
        columns={columns}
        data={[{ id: "profile-1", name: "Ana" }]}
        emptyLabel="Nenhuma pessoa"
        getRowId={(row) => row.id}
        loading={false}
        loadingLabel="Carregando pessoas"
        renderCard={(row) => <article key={row.id}>Cartão de {row.name}</article>}
        selectedRowId="profile-1"
      />,
    );
    expect(screen.getByRole("table", { name: "Pessoas" })).toBeInTheDocument();
    expect(screen.getByRole("row", { name: /Ana/ })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("Cartão de Ana")).toBeInTheDocument();
  });

  it("keeps row selection explicit and controlled by the parent", () => {
    const onChange = vi.fn();
    const data = [
      { id: "profile-1", name: "Ana" },
      { id: "profile-2", name: "Bia" },
    ];
    const { rerender } = render(
      <DataGrid
        caption="Pessoas"
        columns={columns}
        data={data}
        emptyLabel="Nenhuma pessoa"
        getRowId={(row) => row.id}
        loading={false}
        loadingLabel="Carregando pessoas"
        renderCard={(row) => <article>Cartão de {row.name}</article>}
        selection={{ selectedIds: new Set(), onChange, rowLabel: (row) => row.name }}
      />,
    );
    fireEvent.click(screen.getByRole("checkbox", { name: "Selecionar Ana" }));
    expect(onChange).toHaveBeenLastCalledWith(new Set(["profile-1"]));
    fireEvent.click(screen.getByRole("checkbox", { name: "Selecionar Bia no cartão" }));
    expect(onChange).toHaveBeenLastCalledWith(new Set(["profile-2"]));

    rerender(
      <DataGrid
        caption="Pessoas"
        columns={columns}
        data={data}
        emptyLabel="Nenhuma pessoa"
        getRowId={(row) => row.id}
        loading={false}
        loadingLabel="Carregando pessoas"
        renderCard={(row) => <article>Cartão de {row.name}</article>}
        selection={{
          selectedIds: new Set(["profile-1"]),
          onChange,
          rowLabel: (row) => row.name,
        }}
      />,
    );
    // antd's select-all header checkbox exposes no accessible name; the row
    // checkbox keeps the selection explicit and parent-controlled.
    fireEvent.click(screen.getByRole("checkbox", { name: "Selecionar Bia" }));
    expect(onChange).toHaveBeenLastCalledWith(new Set(["profile-1", "profile-2"]));
  });
});

describe("DataGridPagination", () => {
  it("keeps navigation controlled by the URL-owning parent", () => {
    const onPage = vi.fn();
    render(
      <DataGridPagination label="resultados" onPage={onPage} page={2} total={250} totalPages={3} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Anterior" }));
    fireEvent.click(screen.getByRole("button", { name: "Próxima" }));
    expect(onPage).toHaveBeenNthCalledWith(1, 1);
    expect(onPage).toHaveBeenNthCalledWith(2, 3);
  });
});

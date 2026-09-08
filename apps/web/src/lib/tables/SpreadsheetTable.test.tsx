import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SpreadsheetTable } from "./SpreadsheetTable";

const columns = [
  {
    key: "name",
    title: "Nome",
    width: 160,
    sortField: "full_name",
    render: (row: { id: string; name: string }) => row.name,
  },
  {
    key: "city",
    title: "Cidade",
    width: 120,
    render: (row: { id: string; name: string; city: string }) => row.city,
  },
];

function renderTable(
  rowCount: number,
  onPage = vi.fn(),
  onSort = vi.fn(),
  extras: {
    onRowClick?: (row: { id: string; name: string; city: string }) => void;
    selectedRowId?: string;
  } = {},
) {
  const rows = Array.from({ length: rowCount }, (_, index) => ({
    id: `id-${index}`,
    name: `Pessoa ${index}`,
    city: "Porto Alegre",
  }));
  return render(
    <SpreadsheetTable
      caption="Pessoas"
      columns={columns}
      emptyLabel="Vazio"
      loading={false}
      page={1}
      pageSize={100}
      pageSizeAriaLabel="Linhas por página"
      pageSizeOptionLabel="{n} / página"
      rangeLabel="1–1 de 1"
      rows={rows}
      selectedRowId={extras.selectedRowId}
      sortField="full_name"
      sortOrder="asc"
      total={rowCount}
      onPage={onPage}
      onRowClick={extras.onRowClick}
      onSort={onSort}
    />,
  );
}

describe("SpreadsheetTable", () => {
  afterEach(() => {
    cleanup();
  });

  it("keeps a one-row sheet in the DOM so cells stay findable", () => {
    renderTable(1);
    expect(screen.getByText("Pessoa 0")).toBeInTheDocument();
  });

  it("uses Ant Design Select without a search field for page size", async () => {
    const onPage = vi.fn();
    renderTable(3, onPage);
    const pageSize = screen.getByLabelText("Linhas por página");
    expect(pageSize.closest(".ant-select")).toBeTruthy();
    expect(document.querySelector(".spreadsheet-table__footer select")).toBeNull();
    fireEvent.mouseDown(pageSize);
    expect(await screen.findByTitle("50 / página")).toBeInTheDocument();
    expect(document.querySelectorAll(".ant-select-item-option").length).toBe(4);
    expect(document.querySelector(".ant-select-dropdown input")).toBeNull();
    fireEvent.click(screen.getByTitle("50 / página"));
    expect(onPage).toHaveBeenCalledWith(1, 50);
  });

  it("toggles sort when the already sorted column header is clicked again", () => {
    const onSort = vi.fn();
    function Harness() {
      const [sortField, setSortField] = useState("full_name");
      const [sortOrder, setSortOrder] = useState<"asc" | "desc">("asc");
      return (
        <SpreadsheetTable
          caption="Pessoas"
          columns={columns}
          emptyLabel="Vazio"
          loading={false}
          page={1}
          pageSize={100}
          pageSizeAriaLabel="Linhas por página"
          pageSizeOptionLabel="{n} / página"
          rangeLabel="1–3 / 3"
          rows={[
            { id: "id-0", name: "Pessoa 0", city: "Porto Alegre" },
            { id: "id-1", name: "Pessoa 1", city: "Porto Alegre" },
            { id: "id-2", name: "Pessoa 2", city: "Porto Alegre" },
          ]}
          sortField={sortField}
          sortOrder={sortOrder}
          total={3}
          onPage={vi.fn()}
          onSort={(field, order) => {
            onSort(field, order);
            setSortField(field);
            setSortOrder(order);
          }}
        />
      );
    }
    render(<Harness />);
    const nameHeader = document.querySelector(".ant-table-header th.ant-table-column-has-sorters");
    expect(nameHeader).toBeTruthy();
    fireEvent.click(nameHeader as HTMLElement);
    expect(onSort).toHaveBeenLastCalledWith("full_name", "desc");
    fireEvent.click(nameHeader as HTMLElement);
    expect(onSort).toHaveBeenLastCalledWith("full_name", "asc");
    expect(onSort).toHaveBeenCalledTimes(2);
  });

  it("does not wrap sheet cells in Ant Design ellipsis tooltips", () => {
    renderTable(1);
    expect(document.querySelector(".ant-table-cell-ellipsis")).toBeNull();
  });

  it("renders the Ant Design virtual table with numeric scroll", () => {
    renderTable(3);
    expect(document.querySelector(".spreadsheet-table.ant-table-wrapper")).toBeTruthy();
    expect(document.querySelector(".ant-table-virtual")).toBeTruthy();
    expect(document.querySelector(".ant-table-header")).toBeTruthy();
    expect(
      document
        .querySelector(".spreadsheet-table-card")
        ?.contains(document.querySelector(".spreadsheet-table__footer")),
    ).toBe(true);
    expect(
      document
        .querySelector(".spreadsheet-table")
        ?.contains(document.querySelector(".spreadsheet-table__footer")),
    ).toBe(false);
  });

  it("stretches the sheet card across the workspace instead of shrinking to two columns", () => {
    renderTable(3);
    const card = document.querySelector(".spreadsheet-table-card");
    expect(card).toBeInstanceOf(HTMLElement);
    expect((card as HTMLElement).style.width).toBe("100%");
  });

  it("does not mount every row of a 100-row page", () => {
    renderTable(100);
    expect(document.querySelector(".ant-table-virtual")).toBeTruthy();
    expect(document.querySelectorAll(".ant-table-row").length).toBeLessThan(100);
  });

  it("keeps a loading indicator on the Ant Design table", () => {
    render(
      <SpreadsheetTable
        caption="Pessoas"
        columns={columns}
        emptyLabel="Vazio"
        loading
        loadingLabel="Carregando pessoas…"
        page={1}
        pageSize={100}
        pageSizeAriaLabel="Linhas por página"
        pageSizeOptionLabel="{n} / página"
        rangeLabel="0–0 / 0"
        rows={[]}
        sortField="full_name"
        sortOrder="asc"
        total={0}
        onPage={vi.fn()}
        onSort={vi.fn()}
      />,
    );
    expect(
      document.querySelector(".spreadsheet-table-card__sheet")?.getAttribute("aria-busy"),
    ).toBe("true");
    expect(document.querySelector(".ant-spin-spinning")).toBeTruthy();
    expect(
      document.querySelector(".spreadsheet-table.ant-table-wrapper > .ant-spin-spinning"),
    ).toBeTruthy();
    expect(
      document.querySelector(
        ".spreadsheet-table.ant-table-wrapper > .ant-spin-spinning > .ant-spin-section",
      ),
    ).toBeTruthy();
    expect(screen.queryByText("Vazio")).not.toBeInTheDocument();
    expect(screen.getByText("Carregando pessoas…")).toBeInTheDocument();
    const body = document.querySelector(".ant-table-body");
    expect(body).toBeInstanceOf(HTMLElement);
    expect((body as HTMLElement).style.maxHeight).toMatch(/^\d+px$/);
  });

  it("shows only an empty indicator with no columns or pager", () => {
    render(
      <SpreadsheetTable
        caption="Pessoas"
        columns={columns}
        emptyLabel="Vazio"
        loading={false}
        page={1}
        pageSize={100}
        pageSizeAriaLabel="Linhas por página"
        pageSizeOptionLabel="{n} / página"
        rangeLabel="0–0 / 0"
        rows={[]}
        sortField="full_name"
        sortOrder="asc"
        total={0}
        onPage={vi.fn()}
        onSort={vi.fn()}
      />,
    );
    expect(screen.getByText("Vazio")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveClass("spreadsheet-table__empty");
    expect(document.querySelector(".spreadsheet-table-card--empty")).toBeTruthy();
    expect(document.querySelector(".spreadsheet-table")).toBeNull();
    expect(document.querySelector("table")).toBeNull();
    expect(screen.queryByText("Nome")).not.toBeInTheDocument();
    expect(document.querySelector(".spreadsheet-table__footer")).toBeNull();
    expect(screen.queryByLabelText("Linhas por página")).not.toBeInTheDocument();
    expect(screen.queryByText("0–0 / 0")).not.toBeInTheDocument();
  });

  it("moves the selected row with arrow keys when a row is already selected", () => {
    const onRowClick = vi.fn();
    renderTable(3, vi.fn(), vi.fn(), { onRowClick, selectedRowId: "id-0" });
    fireEvent.keyDown(window, { key: "ArrowDown" });
    expect(onRowClick).toHaveBeenCalledWith(expect.objectContaining({ id: "id-1" }));
    cleanup();
    onRowClick.mockClear();
    renderTable(3, vi.fn(), vi.fn(), { onRowClick, selectedRowId: "id-1" });
    fireEvent.keyDown(window, { key: "ArrowUp" });
    expect(onRowClick).toHaveBeenCalledWith(expect.objectContaining({ id: "id-0" }));
  });

  it("does not start row navigation with arrows before a row is selected", () => {
    const onRowClick = vi.fn();
    renderTable(3, vi.fn(), vi.fn(), { onRowClick });
    fireEvent.keyDown(window, { key: "ArrowDown" });
    expect(onRowClick).not.toHaveBeenCalled();
  });
});

import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { FilterSurface } from "./FilterSurface";
import type { ToolbarFilterField } from "./FilterControl";

function textField(
  key: string,
  label: string,
  value: string,
  onChange: (value: string) => void,
): ToolbarFilterField {
  return { key, kind: "text", label, value, onChange };
}

function selectField(
  key: string,
  label: string,
  value: string,
  options: { value: string; label: string }[],
  allLabel: string,
  onChange: (value: string) => void,
): ToolbarFilterField {
  return { key, kind: "select", label, value, options, allLabel, onChange };
}

async function chooseTitle(combobox: HTMLElement, label: string) {
  fireEvent.mouseDown(combobox);
  const options = await screen.findAllByTitle(label);
  fireEvent.click(options[options.length - 1] as HTMLElement);
}

function DocumentFilterHarness({ initialStatus = "" }: { initialStatus?: string }) {
  const [values, setValues] = useState({
    type: "",
    status: initialStatus,
    medium: "",
  });
  const set = (key: keyof typeof values) => (value: string) => {
    setValues((current) => ({ ...current, [key]: value }));
  };
  return (
    <FilterSurface
      addFilterLabel="Adicionar filtro"
      appliedLabel="Filtros aplicados"
      chooseFieldLabel="Campo"
      chooseValueLabel="Valor"
      clearLabel="Limpar filtros"
      fields={[
        selectField(
          "type",
          "Tipo",
          values.type,
          [{ value: "type-rg", label: "RG" }],
          "Todos os tipos",
          set("type"),
        ),
        selectField(
          "status",
          "Status",
          values.status,
          [
            { value: "AVAILABLE", label: "Disponível" },
            { value: "IN_USE", label: "Em uso" },
          ],
          "Todos os status",
          set("status"),
        ),
        selectField(
          "medium",
          "Suporte",
          values.medium,
          [
            { value: "PHYSICAL", label: "Físico" },
            { value: "DIGITAL", label: "Digital" },
          ],
          "Físico e digital",
          set("medium"),
        ),
      ]}
      localHint="Filtro só nesta página"
      noFieldsLabel="Nenhum campo encontrado."
      removeFilterLabel="Remover"
    />
  );
}

describe("FilterSurface", () => {
  afterEach(() => {
    cleanup();
  });
  it("picks a field from the builder row", async () => {
    let city = "";
    render(
      <FilterSurface
        addFilterLabel="Adicionar filtro"
        appliedLabel="Filtros aplicados"
        chooseFieldLabel="Campo"
        chooseValueLabel="Valor"
        clearLabel="Limpar filtros"
        fields={[
          textField("full_name", "Nome", "", () => {}),
          textField("city", "Cidade", city, (value) => {
            city = value;
          }),
        ]}
        localHint="Filtro só nesta página"
        noFieldsLabel="Nenhum campo encontrado."
        removeFilterLabel="Remover"
      />,
    );

    await chooseTitle(screen.getByRole("combobox", { name: "Campo" }), "Cidade");
    expect(await screen.findByRole("textbox", { name: "Cidade" })).toBeInTheDocument();
  });

  it("keeps applied rows in catalog order when a later field was set first", async () => {
    render(<DocumentFilterHarness initialStatus="IN_USE" />);

    expect(screen.getByRole("combobox", { name: "Status" })).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "Tipo" })).not.toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Campo" }));
    const fieldNames = [...document.querySelectorAll(".ant-select-item-option-content")].map(
      (node) => node.textContent,
    );
    expect(fieldNames[0]).toBe("Tipo");

    fireEvent.click(screen.getByRole("button", { name: "Adicionar filtro" }));
    const rows = document.querySelectorAll(".filter-surface__row");
    expect(rows).toHaveLength(2);
    await chooseTitle(
      within(rows[1] as HTMLElement).getByRole("combobox", { name: "Campo" }),
      "Tipo",
    );
    await chooseTitle(screen.getByRole("combobox", { name: "Tipo" }), "RG");

    const applied = [...document.querySelectorAll(".filter-surface__row.is-applied")];
    expect(applied).toHaveLength(2);
    expect(within(applied[0] as HTMLElement).getByRole("combobox", { name: "Tipo" })).toBeTruthy();
    expect(
      within(applied[1] as HTMLElement).getByRole("combobox", { name: "Status" }),
    ).toBeTruthy();
  });
});

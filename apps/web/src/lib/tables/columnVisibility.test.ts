import { describe, expect, it } from "vitest";
import {
  defaultColumnVisible,
  formatColumnCols,
  isColumnVisible,
  parseColumnCols,
  setColumnVisible,
  hideAllColumns,
  showAllColumns,
  withColumnLayout,
} from "./columnVisibility";
import type { SpreadsheetColumn } from "./SpreadsheetTable";

function column(key: string, className?: string): SpreadsheetColumn<{ id: string }> {
  return {
    key,
    title: key,
    ...(className ? { className } : {}),
    render: () => key,
  };
}

describe("column visibility", () => {
  it("keeps gymkhana scan columns visible and tucks the long tail away", () => {
    expect(defaultColumnVisible(column("full_name"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("team"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("street"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("number"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("city"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("birth_date"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("email"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("mobile"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("documents"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("cpf"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("rg"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("postal_code"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("cnh"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("passport"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("team"), "profiles")).toBe(true);
    expect(defaultColumnVisible(column("age"), "profiles")).toBe(false);
    expect(defaultColumnVisible(column("neighborhood"), "profiles")).toBe(false);
    expect(defaultColumnVisible(column("father_name"), "profiles")).toBe(false);
    expect(defaultColumnVisible(column("digit_sum_cpf"), "profiles")).toBe(false);
    expect(defaultColumnVisible(column("pet"), "profiles")).toBe(false);
  });

  it("does not revive formula columns as a default-visible class", () => {
    expect(
      defaultColumnVisible(
        column("digit_sum_identifier", "spreadsheet-table__formula"),
        "documents",
      ),
    ).toBe(false);
    expect(
      defaultColumnVisible(column("my-formula", "spreadsheet-table__formula"), "documents"),
    ).toBe(false);
  });

  it("locks only the name column and ignores hide overrides on it", () => {
    const name = withColumnLayout(column("full_name"), "profiles");
    const city = withColumnLayout(column("city"), "profiles");
    expect(name.locked).toBe(true);
    expect(city.locked).toBe(false);
    expect(city.defaultVisible).toBe(true);
    expect(isColumnVisible(name, { full_name: false })).toBe(true);
    expect(setColumnVisible(city, false, {})).toEqual({ city: false });
  });

  it("stores only overrides that differ from the default", () => {
    const neighborhood = withColumnLayout(column("neighborhood"), "profiles");
    const pet = withColumnLayout(column("pet"), "profiles");
    expect(setColumnVisible(neighborhood, true, {})).toEqual({ neighborhood: true });
    expect(setColumnVisible(pet, true, { pet: true })).toEqual({ pet: true });
    expect(setColumnVisible(pet, false, { pet: true })).toEqual({});
  });

  it("can reveal every column without unlocking identity", () => {
    const columns = [
      withColumnLayout(column("full_name"), "profiles"),
      withColumnLayout(column("pet"), "profiles"),
    ];
    expect(showAllColumns(columns, {}).pet).toBe(true);
    expect(showAllColumns(columns, {}).full_name).toBeUndefined();
  });

  it("hides every unlocked column and leaves the name visible", () => {
    const columns = [
      withColumnLayout(column("full_name"), "profiles"),
      withColumnLayout(column("city"), "profiles"),
      withColumnLayout(column("pet"), "profiles"),
    ];
    expect(hideAllColumns(columns, { pet: true })).toEqual({ city: false });
  });

  it("encodes only column overrides into a compact cols token", () => {
    expect(formatColumnCols({ city: false, father_name: true })).toBe("-city,father_name");
    expect(parseColumnCols("-city,father_name")).toEqual({ city: false, father_name: true });
    expect(parseColumnCols("-full_name,city")).toEqual({ city: true });
    expect(formatColumnCols(parseColumnCols(""))).toBe("");
  });
});

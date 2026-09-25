import { describe, expect, it } from "vitest";
import {
  tableRowActionSubject,
  tableRowAllowsCurrentUse,
  tableRowEntityId,
  tableRowEntityKind,
  tableRowInUse,
  tableRowSearchPatch,
} from "./tableRowNavigation";
import type { TableRow } from "./tableRows";

function row(cells: Record<string, unknown>, id = "row-1"): TableRow {
  return { id, cells };
}

describe("tableRowNavigation", () => {
  it("resolves entity kind from the sheet section or assistant result", () => {
    const person = row({ full_name: "Ana" });
    expect(tableRowEntityKind(person, { section: "profile", isResult: false })).toBe("person");
    expect(tableRowEntityKind(person, { section: "documents", isResult: false })).toBe("document");
    expect(
      tableRowEntityKind(row({ entityKind: "bill", entityId: "bill-1" }), {
        section: "profile",
        isResult: true,
      }),
    ).toBe("bill");
  });

  it("opens the editor in edit mode without changing people patches on the people sheet", () => {
    const person = row({ full_name: "Ana" }, "p-1");
    expect(
      tableRowSearchPatch(person, { section: "profile", isResult: false, intent: "edit" }),
    ).toEqual({ selected: "p-1", mode: "edit" });
    expect(
      tableRowSearchPatch(row({ identifier: "123" }, "d-1"), {
        section: "documents",
        isResult: false,
        intent: "edit",
      }),
    ).toMatchObject({ document_selected: "d-1", document_mode: "edit" });
  });

  it("allows current use only for physical documents and bills with a known status", () => {
    const physical = row({ medium_key: "PHYSICAL", status_key: "AVAILABLE", identifier: "123" });
    const digital = row({ medium_key: "DIGITAL", status_key: "AVAILABLE", identifier: "DIG" });
    expect(tableRowAllowsCurrentUse(physical, "document")).toBe(true);
    expect(tableRowAllowsCurrentUse(digital, "document")).toBe(false);
    expect(tableRowAllowsCurrentUse(physical, "person")).toBe(false);
    expect(tableRowInUse(row({ status_key: "IN_USE" }))).toBe(true);
  });

  it("prefers the visible identifier for the actions label", () => {
    expect(tableRowActionSubject(row({ identifier: "1234567890" }), "document")).toBe("1234567890");
    expect(tableRowActionSubject(row({ full_name: "—" }), "person")).toBe("");
    expect(tableRowEntityId(row({ entityId: "ext-1" }, "row-9"), true)).toBe("ext-1");
  });
});

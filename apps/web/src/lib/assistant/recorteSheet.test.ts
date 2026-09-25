import { describe, expect, it } from "vitest";
import {
  compareRecorteCells,
  filterRecorteRows,
  pageRecorteRows,
  sortRecorteRows,
} from "./recorteSheet";
import type { TableRow } from "../tables/tableRows";

function row(id: string, cells: Record<string, unknown>): TableRow {
  return { id, cells };
}

const ana = row("1", { full_name: "Ana", city: "Porto Alegre", entityLabel: "Ana" });
const bia = row("2", { full_name: "Bia", city: "Canoas", entityLabel: "Bia" });
const caio = row("3", { full_name: "Caio", city: "Porto Alegre", entityLabel: "Caio" });

describe("recorte sheet", () => {
  it("filters by column and by the recorte search", () => {
    expect(filterRecorteRows([ana, bia, caio], { city: "Porto" }, "")).toEqual([ana, caio]);
    expect(filterRecorteRows([ana, bia, caio], {}, "bia")).toEqual([bia]);
    expect(filterRecorteRows([ana, bia, caio], { city: "Porto" }, "caio")).toEqual([caio]);
  });

  it("sorts the recorte without mixing in the rest of the table", () => {
    const desc = sortRecorteRows([ana, bia, caio], "full_name", "desc");
    expect(desc.map((item) => item.id)).toEqual(["3", "2", "1"]);
    expect(compareRecorteCells("10", "2")).toBeGreaterThan(0);
  });

  it("pages the already filtered recorte", () => {
    expect(pageRecorteRows([ana, bia, caio], 2, 2).map((item) => item.id)).toEqual(["3"]);
  });
});

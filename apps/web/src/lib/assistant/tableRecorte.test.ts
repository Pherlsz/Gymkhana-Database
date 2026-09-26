import { describe, expect, it } from "vitest";
import { normalizeProfileSearch } from "../profile/profileSearch";
import { applyRecortePatch, clearRecortePatch, recorteFilterParts } from "./tableRecorte";

const current = normalizeProfileSearch({ cols: "-city", city: "Canoas" });

describe("assistant recorte on the table link", () => {
  it("compiles filters and a hidden column into the shareable search", () => {
    expect(
      applyRecortePatch(current, {
        table: "people",
        filters: { full_name: "Pedro", ignored: "x" },
        columns: ["full_name", "father_name"],
        sort: "full_name",
        order: "desc",
      }),
    ).toMatchObject({
      section: "profile",
      full_name: "Pedro",
      city: "",
      sort: "full_name",
      order: "desc",
      cols: "-city,father_name",
      recorte: "father_name",
    });
  });

  it("keeps the marker when the router parses 1 as a number", () => {
    expect(normalizeProfileSearch({ recorte: 1 }).recorte).toBe("on");
    expect(normalizeProfileSearch({ recorte: "on" }).recorte).toBe("on");
  });

  it("survives a refresh and leaves with Limpar", () => {
    const applied = normalizeProfileSearch(
      applyRecortePatch(current, {
        table: "people",
        filters: { full_name: "Pedro" },
        columns: ["father_name"],
      }),
    );
    expect(recorteFilterParts(applied)).toEqual([{ key: "full_name", value: "Pedro" }]);
    expect(clearRecortePatch(applied)).toMatchObject({
      full_name: "",
      cols: "-city",
      recorte: "",
    });
  });
});

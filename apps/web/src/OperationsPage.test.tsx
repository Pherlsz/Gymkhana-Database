import { describe, expect, it } from "vitest";
import { normalizeOperationsSearch, parseBulkSelection } from "./OperationsPage";

const importID = "019bf789-4400-7f12-9abc-123456789abc";

describe("OperationsPage route helpers", () => {
  it("normalizes URL state and validates reinforced bulk selection locally", () => {
    expect(normalizeOperationsSearch({ selected: importID })).toEqual({ selected: importID });
    expect(
      normalizeOperationsSearch({ selected: "https://storage.invalid/private?signed=secret" }),
    ).toEqual({});
    expect(parseBulkSelection(`${importID},7`)).toEqual([{ id: importID, version: 7 }]);
    expect(() => parseBulkSelection(`${importID},0`)).toThrow(/UUID,versão/);
    expect(() => parseBulkSelection("not-an-id,1")).toThrow(/UUID,versão/);
  });
});

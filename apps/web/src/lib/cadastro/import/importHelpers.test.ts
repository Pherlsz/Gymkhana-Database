import { describe, expect, it } from "vitest";
import { operationActive, operationTerminal } from "./importHelpers";

describe("importHelpers", () => {
  it("tracks active and terminal import states", () => {
    expect(operationActive("PARSING")).toBe(true);
    expect(operationActive("READY")).toBe(false);
    expect(operationTerminal("COMPLETED")).toBe(true);
    expect(operationTerminal("MAPPING")).toBe(false);
  });
});

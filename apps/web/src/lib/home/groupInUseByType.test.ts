import { describe, expect, it } from "vitest";
import { groupInUseByType, type HomeInUseItem } from "./useHomeOverview";

const doc = (id: string, typeId: string, typeLabel: string): HomeInUseItem => ({
  id,
  kind: "document",
  label: `DOC-${id}`,
  typeId,
  typeLabel,
  ownerProfileId: "profile-1",
  notes: "",
  assignedAt: null,
});

describe("groupInUseByType", () => {
  it("groups in-use items by entity + type and keeps the larger counts first", () => {
    const chips = groupInUseByType([
      doc("doc-1", "type-rg", "RG"),
      doc("doc-2", "type-rg", "RG"),
      {
        ...doc("bill-1", "type-energy", "Energia"),
        kind: "bill" as const,
        label: "NF-9",
      },
    ]);
    expect(chips).toEqual([
      { key: "document:type-rg", kind: "document", typeId: "type-rg", typeLabel: "RG", count: 2 },
      {
        key: "bill:type-energy",
        kind: "bill",
        typeId: "type-energy",
        typeLabel: "Energia",
        count: 1,
      },
    ]);
  });
});

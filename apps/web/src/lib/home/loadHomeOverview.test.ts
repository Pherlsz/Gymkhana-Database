import { describe, expect, it, vi } from "vitest";
import { daysInUse, groupInUseByType, loadHomeOverview, mapInUseItems } from "./loadHomeOverview";

vi.mock("../api/client", () => ({
  getProfileListTotals: vi.fn(async () => ({ page: { total: 10 } })),
  getDocumentListTotals: vi.fn(async (_status?: string) => ({
    documents: [],
    page: { total: _status === "IN_USE" ? 2 : 4 },
  })),
  getBillListTotals: vi.fn(async (_status?: string) => ({
    bills: [],
    page: { total: _status === "IN_USE" ? 1 : 3 },
  })),
  listDocumentsInUse: vi.fn(async () => ({
    documents: [
      {
        id: "doc-1",
        owner_profile_id: "profile-1",
        document_type_id: "type-rg",
        identifier_value: "RG-1",
        notes: "Gincana",
        type: { id: "type-rg", label: "RG" },
        current_use: { assigned_at: "2026-08-01T12:00:00Z" },
      },
      {
        id: "doc-2",
        owner_profile_id: "profile-2",
        document_type_id: "type-rg",
        identifier_value: "RG-2",
        notes: "",
        type: { id: "type-rg", label: "RG" },
        current_use: { assigned_at: "2026-08-13T12:00:00Z" },
      },
    ],
    page: { total: 2 },
  })),
  listBillsInUse: vi.fn(async () => ({
    bills: [
      {
        id: "bill-1",
        owner_profile_id: "profile-3",
        bill_type_id: "type-energy",
        reference_value: "NF-9",
        notes: "",
        type: { id: "type-energy", label: "Energia" },
        current_use: { assigned_at: "2026-08-02T12:00:00Z" },
      },
    ],
    page: { total: 1 },
  })),
}));

describe("loadHomeOverview", () => {
  it("aggregates backend totals and in-use items", async () => {
    const overview = await loadHomeOverview();
    expect(overview.failed).toBe(false);
    expect(overview.profileTotal).toBe(10);
    expect(overview.documentTotal).toBe(4);
    expect(overview.billTotal).toBe(3);
    expect(overview.documentsInUse).toBe(2);
    expect(overview.billsInUse).toBe(1);
    expect(overview.inUseItems).toHaveLength(3);
    expect(overview.inUseItems[0]?.kind).toBe("document");
    expect(overview.inUseItems[0]?.typeId).toBe("type-rg");
    expect(overview.inUseItems[2]?.kind).toBe("bill");
  });
});

describe("groupInUseByType", () => {
  it("groups in-use items by type and keeps the larger counts first", () => {
    const chips = groupInUseByType(
      mapInUseItems(
        {
          documents: [
            {
              id: "doc-1",
              owner_profile_id: "p1",
              document_type_id: "type-rg",
              identifier_value: "RG-1",
              notes: "",
              type: { id: "type-rg", label: "RG" },
              current_use: { assigned_at: "2026-08-01T12:00:00Z" },
            },
            {
              id: "doc-2",
              owner_profile_id: "p2",
              document_type_id: "type-rg",
              identifier_value: "RG-2",
              notes: "",
              type: { id: "type-rg", label: "RG" },
              current_use: { assigned_at: "2026-08-01T12:00:00Z" },
            },
          ],
          page: { total: 2, limit: 50, offset: 0, sort_field: "updated_at", sort_order: "desc" },
        } as never,
        {
          bills: [
            {
              id: "bill-1",
              owner_profile_id: "p3",
              bill_type_id: "type-energy",
              reference_value: "NF-9",
              notes: "",
              type: { id: "type-energy", label: "Energia" },
              current_use: { assigned_at: "2026-08-02T12:00:00Z" },
            },
          ],
          page: { total: 1, limit: 50, offset: 0, sort_field: "updated_at", sort_order: "desc" },
        } as never,
      ),
    );
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

describe("daysInUse", () => {
  it("counts whole days from the assignment date", () => {
    expect(daysInUse("2026-08-12T18:00:00Z", new Date("2026-08-14T09:00:00Z"))).toBe(2);
    expect(daysInUse("2026-08-14T23:00:00Z", new Date("2026-08-14T09:00:00Z"))).toBe(0);
    expect(daysInUse(null)).toBeNull();
  });
});

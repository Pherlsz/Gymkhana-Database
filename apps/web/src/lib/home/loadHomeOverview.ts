import {
  getBillListTotals,
  getDocumentListTotals,
  getProfileListTotals,
  listBillsInUse,
  listDocumentsInUse,
  type BillPageResponse,
  type DocumentPageResponse,
} from "../api/client";

export type HomeInUseItem = {
  id: string;
  kind: "document" | "bill";
  label: string;
  typeId: string;
  typeLabel: string;
  ownerProfileId: string;
  notes: string;
  assignedAt: string | null;
};

export type HomeInUseTypeChip = {
  key: string;
  kind: "document" | "bill";
  typeId: string;
  typeLabel: string;
  count: number;
};

export type HomeCatalogType = {
  id: string;
  label: string;
  technicalKey: string;
  count: number | null;
  loading: boolean;
};

export type HomeOverview = {
  profileTotal: number | null;
  documentTotal: number | null;
  billTotal: number | null;
  documentsInUse: number | null;
  billsInUse: number | null;
  inUseItems: HomeInUseItem[];
  failed: boolean;
};

export function mapInUseItems(
  documents?: DocumentPageResponse,
  bills?: BillPageResponse,
): HomeInUseItem[] {
  const items: HomeInUseItem[] = [];
  for (const document of documents?.documents ?? []) {
    items.push({
      id: document.id,
      kind: "document",
      label: document.identifier_value,
      typeId: document.document_type_id || document.type.id,
      typeLabel: document.type.label,
      ownerProfileId: document.owner_profile_id,
      notes: document.notes,
      assignedAt: document.current_use?.assigned_at ?? null,
    });
  }
  for (const bill of bills?.bills ?? []) {
    items.push({
      id: bill.id,
      kind: "bill",
      label: bill.reference_value,
      typeId: bill.bill_type_id || bill.type.id,
      typeLabel: bill.type.label,
      ownerProfileId: bill.owner_profile_id,
      notes: bill.notes,
      assignedAt: bill.current_use?.assigned_at ?? null,
    });
  }
  return items;
}

export function groupInUseByType(items: HomeInUseItem[]): HomeInUseTypeChip[] {
  const grouped = new Map<string, HomeInUseTypeChip>();
  for (const item of items) {
    const key = `${item.kind}:${item.typeId || item.typeLabel}`;
    const existing = grouped.get(key);
    if (existing) {
      existing.count += 1;
      continue;
    }
    grouped.set(key, {
      key,
      kind: item.kind,
      typeId: item.typeId,
      typeLabel: item.typeLabel,
      count: 1,
    });
  }
  return [...grouped.values()].toSorted((left, right) => {
    if (right.count !== left.count) return right.count - left.count;
    return left.typeLabel.localeCompare(right.typeLabel, "pt-BR");
  });
}

export function daysInUse(assignedAt: string | null, now = new Date()): number | null {
  if (!assignedAt) return null;
  const assigned = new Date(assignedAt);
  if (Number.isNaN(assigned.getTime())) return null;
  const start = Date.UTC(assigned.getUTCFullYear(), assigned.getUTCMonth(), assigned.getUTCDate());
  const today = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
  return Math.max(0, Math.round((today - start) / 86_400_000));
}

function isAbort(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

async function settled<T>(promise: Promise<T>): Promise<{ ok: true; value: T } | { ok: false }> {
  try {
    return { ok: true, value: await promise };
  } catch (error: unknown) {
    if (isAbort(error)) throw error;
    return { ok: false };
  }
}

export async function loadHomeOverview(signal?: AbortSignal): Promise<HomeOverview> {
  const [profiles, documents, bills, inUseDocuments, inUseBills] = await Promise.all([
    settled(getProfileListTotals(signal)),
    settled(getDocumentListTotals(undefined, signal)),
    settled(getBillListTotals(undefined, signal)),
    settled(listDocumentsInUse(signal)),
    settled(listBillsInUse(signal)),
  ]);

  return {
    profileTotal: profiles.ok ? profiles.value.page.total : null,
    documentTotal: documents.ok ? documents.value.page.total : null,
    billTotal: bills.ok ? bills.value.page.total : null,
    documentsInUse: inUseDocuments.ok ? inUseDocuments.value.page.total : null,
    billsInUse: inUseBills.ok ? inUseBills.value.page.total : null,
    inUseItems: mapInUseItems(
      inUseDocuments.ok ? inUseDocuments.value : undefined,
      inUseBills.ok ? inUseBills.value : undefined,
    ),
    failed: !profiles.ok || !documents.ok || !bills.ok || !inUseDocuments.ok || !inUseBills.ok,
  };
}

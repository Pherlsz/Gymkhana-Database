import { useQuery } from "@tanstack/react-query";
import {
  getProfileListTotals,
  listBillTypes,
  listBillsInUse,
  listDocumentTypes,
  listDocumentsInUse,
  type BillPageResponse,
  type DocumentPageResponse,
} from "../api/client";

const homeQuery = {
  retry: 1,
  staleTime: 30_000,
} as const;

const catalogCountQuery = {
  retry: 1,
  staleTime: 0,
  refetchOnMount: "always" as const,
};

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

/**
 * Groups the fetched in-use page by entity + type. The page endpoint caps at 50
 * items, so past that point chip counts understate `page.total`; callers show a
 * truncation note when the total exceeds the fetched length.
 */
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

function mapInUseItems(
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

function useHomeCatalogTypes() {
  const documentTypes = useQuery({
    ...catalogCountQuery,
    queryKey: ["home", "overview", "document-types"],
    queryFn: ({ signal }) => listDocumentTypes(signal),
  });
  const billTypes = useQuery({
    ...catalogCountQuery,
    queryKey: ["home", "overview", "bill-types"],
    queryFn: ({ signal }) => listBillTypes(signal),
  });

  return {
    documentTypes: documentTypes.data?.types ?? [],
    billTypes: billTypes.data?.types ?? [],
    loading: {
      documentTypes: documentTypes.isPending,
      billTypes: billTypes.isPending,
    },
    failed: documentTypes.isError || billTypes.isError,
    refetch: () => Promise.all([documentTypes.refetch(), billTypes.refetch()]),
  };
}

export function useHomeOverview() {
  const profiles = useQuery({
    ...homeQuery,
    queryKey: ["home", "overview", "profiles"],
    queryFn: ({ signal }) => getProfileListTotals(signal),
  });
  const documentsInUse = useQuery({
    ...homeQuery,
    queryKey: ["home", "overview", "documents-in-use"],
    queryFn: ({ signal }) => listDocumentsInUse(signal),
  });
  const billsInUse = useQuery({
    ...homeQuery,
    queryKey: ["home", "overview", "bills-in-use"],
    queryFn: ({ signal }) => listBillsInUse(signal),
  });
  const catalog = useHomeCatalogTypes();
  const inUseFetched =
    (documentsInUse.data?.documents.length ?? 0) + (billsInUse.data?.bills.length ?? 0);
  const inUseTypeChips = groupInUseByType(mapInUseItems(documentsInUse.data, billsInUse.data));

  return {
    profileTotal: profiles.data?.page.total ?? null,
    documentsInUse: documentsInUse.data?.page.total ?? null,
    billsInUse: billsInUse.data?.page.total ?? null,
    /** How many in-use items the fixed-size pages actually returned. */
    inUseFetched,
    inUseTypeChips,
    documentTypes: catalog.documentTypes.map((type) => ({
      id: type.id,
      label: type.label,
      technicalKey: type.technical_key,
      count: type.count ?? null,
      loading: catalog.loading.documentTypes,
    })),
    billTypes: catalog.billTypes.map((type) => ({
      id: type.id,
      label: type.label,
      technicalKey: type.technical_key,
      count: type.count ?? null,
      loading: catalog.loading.billTypes,
    })),
    loading: {
      profiles: profiles.isPending,
      inUse: documentsInUse.isPending || billsInUse.isPending,
      documentTypes: catalog.loading.documentTypes,
      billTypes: catalog.loading.billTypes,
    },
    failed: profiles.isError || catalog.failed,
    inUseFailed: documentsInUse.isError || billsInUse.isError,
    refetch: () =>
      Promise.all([
        profiles.refetch(),
        documentsInUse.refetch(),
        billsInUse.refetch(),
        catalog.refetch(),
      ]),
  };
}

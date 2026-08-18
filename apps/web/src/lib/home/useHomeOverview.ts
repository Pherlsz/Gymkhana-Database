import { useQuery } from "@tanstack/react-query";
import {
  getProfileListTotals,
  listBillTypes,
  listBillsInUse,
  listDocumentTypes,
  listDocumentsInUse,
} from "../api/client";
import { groupInUseByType, mapInUseItems } from "./loadHomeOverview";

const homeQuery = {
  retry: 1,
  staleTime: 30_000,
} as const;

const catalogCountQuery = {
  retry: 1,
  staleTime: 0,
  refetchOnMount: "always" as const,
};

export function useHomeCatalogTypes() {
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
  const inUseItems = mapInUseItems(documentsInUse.data, billsInUse.data);

  return {
    profileTotal: profiles.data?.page.total ?? null,
    documentsInUse: documentsInUse.data?.page.total ?? null,
    billsInUse: billsInUse.data?.page.total ?? null,
    inUseItems,
    inUseTypeChips: groupInUseByType(inUseItems),
    documentTypes: catalog.documentTypes.map((type) => ({
      id: type.id,
      label: type.label,
      technicalKey: type.technical_key,
      count: type.count ?? 0,
      loading: catalog.loading.documentTypes,
    })),
    billTypes: catalog.billTypes.map((type) => ({
      id: type.id,
      label: type.label,
      technicalKey: type.technical_key,
      count: type.count ?? 0,
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
  };
}

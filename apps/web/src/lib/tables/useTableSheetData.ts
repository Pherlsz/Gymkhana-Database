import { useQueries, useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import {
  APIRequestError,
  getProfile,
  listBillTypes,
  listBills,
  listCustomFields,
  listCustomOptions,
  listDistinctCities,
  listDocumentTypes,
  listDocuments,
  listProfiles,
  type BillRecord,
  type DocumentRecord,
  type ProfileListSearch,
} from "../api/client";
import { queryKeys } from "../api/queryKeys";
import { distinctValues } from "./tableFilters";
import { PEOPLE_DOC_KEY_SET } from "./peopleColumns";
import { buildSheetFilters } from "./sheetFilters";
import type { ToolbarFilterField } from "./FilterControl";
import { billSearch, documentSearch, profileListKey } from "./sheetQuery";
import {
  billRow,
  documentRow,
  personRow,
  uniqueCustomFields,
  type RecordSheetLabels,
  type TableRow,
} from "./tableRows";
import { groupedTypeFilterOptions } from "./typeFilterOptions";
import { clampSpreadsheetPageSize } from "./spreadsheetViewport";

export type UseTableSheetDataParams = {
  section: "profile" | "documents" | "bills";
  search: ProfileListSearch & { section: "profile" | "documents" | "bills" };
  copy: any;
  messages: any;
  localFilters: Record<string, string>;
  setLocal: (key: string, value: string) => void;
  updateSearch: (patch: Partial<ProfileListSearch>) => void;
};

export function useTableSheetData({
  section,
  search,
  copy,
  messages,
  localFilters,
  setLocal,
  updateSearch,
}: UseTableSheetDataParams) {
  const tableSearch = {
    ...search,
    limit: clampSpreadsheetPageSize(search.limit),
    document_limit: clampSpreadsheetPageSize(search.document_limit),
    bill_limit: clampSpreadsheetPageSize(search.bill_limit),
  };

  const peopleQuery = useQuery({
    queryKey: queryKeys.tables.profiles(profileListKey(tableSearch)),
    queryFn: ({ signal }) => listProfiles(tableSearch, signal),
    enabled: section === "profile",
  });

  const citiesQuery = useQuery({
    queryKey: queryKeys.profiles.cityOptions(
      search.full_name,
      search.cpf,
      search.email,
      search.state,
    ),
    queryFn: ({ signal }) =>
      listDistinctCities(
        {
          full_name: search.full_name,
          cpf: search.cpf,
          email: search.email,
          state: search.state,
        },
        signal,
      ),
    enabled: section === "profile",
    staleTime: 5 * 60 * 1000,
  });

  const documentQuery = useQuery({
    queryKey: queryKeys.tables.documents(documentSearch(tableSearch), tableSearch.records_owner),
    queryFn: ({ signal }) =>
      listDocuments(tableSearch.records_owner, documentSearch(tableSearch), signal),
    enabled: section === "documents",
  });

  const billQuery = useQuery({
    queryKey: queryKeys.tables.bills(billSearch(tableSearch), tableSearch.records_owner),
    queryFn: ({ signal }) => listBills(tableSearch.records_owner, billSearch(tableSearch), signal),
    enabled: section === "bills",
  });

  const documentTypes = useQuery({
    queryKey: queryKeys.types.documents,
    queryFn: ({ signal }) => listDocumentTypes(signal),
    enabled: section === "documents" || section === "profile",
  });

  const billTypes = useQuery({
    queryKey: queryKeys.types.bills,
    queryFn: ({ signal }) => listBillTypes(signal),
    enabled: section === "bills",
  });

  const selectedProfileQuery = useQuery({
    queryKey: queryKeys.profiles.detail(search.selected),
    queryFn: ({ signal }) => getProfile(search.selected!, signal),
    enabled: Boolean(search.mode && search.selected),
  });

  const profileFieldsQuery = useQuery({
    queryKey: queryKeys.customData.fields("PROFILE"),
    queryFn: () => listCustomFields("PROFILE"),
    enabled: section === "profile",
  });

  const documentTypeList = documentTypes.data?.types ?? [];
  const billTypeList = billTypes.data?.types ?? [];

  const documentFieldQueries = useQueries({
    queries: documentTypeList.map((type) => ({
      queryKey: queryKeys.customData.fields("DOCUMENT_TYPE", type.id),
      queryFn: () => listCustomFields("DOCUMENT_TYPE", type.id),
      enabled: section === "documents",
    })),
  });

  const billFieldQueries = useQueries({
    queries: billTypeList.map((type) => ({
      queryKey: queryKeys.customData.fields("BILL_TYPE", type.id),
      queryFn: () => listCustomFields("BILL_TYPE", type.id),
      enabled: section === "bills",
    })),
  });

  const documentFieldsStamp = section === "documents" ? queriesStamp(documentFieldQueries) : "";
  const billFieldsStamp = section === "bills" ? queriesStamp(billFieldQueries) : "";

  const columnMetadataLoading =
    section === "profile"
      ? profileFieldsQuery.isPending
      : section === "documents"
        ? documentTypes.isPending || documentFieldQueries.some((query) => query.isPending)
        : billTypes.isPending || billFieldQueries.some((query) => query.isPending);

  const recordLabels: RecordSheetLabels = {
    boolean: copy.boolean,
    status: copy.status,
    medium: copy.medium,
    idleCustody: copy.idleCustody,
  };

  const typeGroups = useMemo(() => {
    if (section === "documents") {
      return groupedTypeFilterOptions(
        documentTypeList.map((type) => ({
          id: type.id,
          label: type.label,
          technicalKey: type.technical_key,
        })),
        "document",
        messages.home.tables,
        messages.common.labels,
      );
    }
    if (section === "bills") {
      return groupedTypeFilterOptions(
        billTypeList.map((type) => ({
          id: type.id,
          label: type.label,
          technicalKey: type.technical_key,
        })),
        "bill",
        messages.home.tables,
        messages.common.labels,
      );
    }
    return [];
  }, [billTypeList, documentTypeList, messages.common.labels, messages.home.tables, section]);

  const extraFields = useMemo(() => {
    if (section === "profile") return uniqueCustomFields(profileFieldsQuery.data?.fields ?? []);
    if (section === "documents") {
      const selectedType = search.document_type;
      const fields = documentTypeList.flatMap((type, index) => {
        if (selectedType && type.id !== selectedType) return [];
        return documentFieldQueries[index]?.data?.fields ?? [];
      });
      return uniqueCustomFields(fields);
    }
    const selectedType = search.bill_type;
    const fields = billTypeList.flatMap((type, index) => {
      if (selectedType && type.id !== selectedType) return [];
      return billFieldQueries[index]?.data?.fields ?? [];
    });
    return uniqueCustomFields(fields);
  }, [
    billFieldsStamp,
    billTypeList,
    documentFieldsStamp,
    documentTypeList,
    profileFieldsQuery.data,
    search.bill_type,
    search.document_type,
    section,
  ]);

  const identifierTypes = useMemo(() => {
    if (section !== "profile") return [];
    const grouped = groupedTypeFilterOptions(
      documentTypeList.map((type) => ({
        id: type.id,
        label: type.label,
        technicalKey: type.technical_key,
      })),
      "document",
      messages.home.tables,
      messages.common.labels,
    );
    const ordered = grouped.flatMap((group) =>
      group.options.map((option) => documentTypeList.find((type) => type.id === option.value)),
    );
    return ordered.filter(
      (type): type is NonNullable<typeof type> =>
        type != null && PEOPLE_DOC_KEY_SET.has(type.technical_key),
    );
  }, [documentTypeList, messages.common.labels, messages.home.tables, section]);

  const baseRows: TableRow[] = useMemo(() => {
    if (section === "documents") {
      return (documentQuery.data?.documents ?? []).map((record: DocumentRecord) =>
        documentRow(record, recordLabels, extraFields),
      );
    }
    if (section === "bills") {
      return (billQuery.data?.bills ?? []).map((record: BillRecord) =>
        billRow(record, recordLabels, extraFields),
      );
    }
    return (peopleQuery.data?.profiles ?? []).map((profile) =>
      personRow(profile, extraFields, copy.boolean),
    );
  }, [
    billQuery.data,
    copy.boolean,
    documentQuery.data,
    extraFields,
    peopleQuery.data,
    recordLabels,
    section,
  ]);

  const selectCustomFields = useMemo(
    () =>
      extraFields.filter(
        (field) => field.field_kind === "SINGLE_SELECT" || field.field_kind === "MULTI_SELECT",
      ),
    [extraFields],
  );

  const optionQueries = useQueries({
    queries: selectCustomFields.map((field) => ({
      queryKey: queryKeys.customData.options(field.id),
      queryFn: ({ signal }) => listCustomOptions(field.id, signal),
    })),
  });

  const optionStamp = queriesStamp(optionQueries);

  const optionsByFieldId = useMemo(() => {
    const map = new Map<string, { value: string; label: string }[]>();
    selectCustomFields.forEach((field, index) => {
      const options = (optionQueries[index]?.data?.options ?? [])
        .filter((option) => option.active)
        .map((option) => ({ value: option.label, label: option.label }));
      if (options.length) map.set(field.id, options);
    });
    return map;
  }, [optionStamp, selectCustomFields]);

  const distinctByKey = useMemo(() => {
    const result: Record<string, string[]> = {};
    for (const field of selectCustomFields)
      result[field.technical_key] = distinctValues(baseRows, field.technical_key);
    return result;
  }, [baseRows, selectCustomFields]);

  const cityOptions = useMemo(
    () =>
      (citiesQuery.data?.values ?? []).map((city) => ({
        value: city,
        label: city,
      })),
    [citiesQuery.data],
  );

  const allFilters: ToolbarFilterField[] = useMemo(
    () =>
      buildSheetFilters({
        section,
        search,
        copy,
        localFilters,
        extraFields,
        optionsByFieldId,
        distinctByKey,
        typeGroups,
        identifierTypes,
        cityOptions,
        setLocal,
        updateSearch,
      }),
    [
      cityOptions,
      copy,
      distinctByKey,
      extraFields,
      identifierTypes,
      localFilters,
      optionsByFieldId,
      search,
      section,
      setLocal,
      typeGroups,
      updateSearch,
    ],
  );

  const loading =
    section === "documents"
      ? documentQuery.isLoading
      : section === "bills"
        ? billQuery.isLoading
        : peopleQuery.isLoading;

  const error =
    section === "documents"
      ? documentQuery.error
      : section === "bills"
        ? billQuery.error
        : peopleQuery.error;

  const errorDescription = tableErrorDescription(error);

  const total =
    section === "documents"
      ? (documentQuery.data?.page.total ?? 0)
      : section === "bills"
        ? (billQuery.data?.page.total ?? 0)
        : (peopleQuery.data?.page.total ?? 0);

  const page =
    section === "documents"
      ? search.document_page
      : section === "bills"
        ? search.bill_page
        : search.page;

  const pageSize =
    section === "documents"
      ? tableSearch.document_limit
      : section === "bills"
        ? tableSearch.bill_limit
        : tableSearch.limit;

  const sortField =
    section === "documents"
      ? search.document_sort
      : section === "bills"
        ? search.bill_sort
        : search.sort;

  const sortOrder =
    section === "documents"
      ? search.document_order
      : section === "bills"
        ? search.bill_order
        : search.order;

  const selectedProfile =
    peopleQuery.data?.profiles.find((value) => value.id === search.selected) ??
    selectedProfileQuery.data;

  const recordsOwnerName =
    (section === "documents"
      ? documentQuery.data?.documents.find(
          (value) => value.owner_profile_id === search.records_owner,
        )?.owner_full_name
      : billQuery.data?.bills.find((value) => value.owner_profile_id === search.records_owner)
          ?.owner_full_name) ??
    (search.records_owner && selectedProfile?.id === search.records_owner
      ? selectedProfile?.full_name
      : undefined) ??
    copy.inspector.ownerChip;

  return {
    extraFields,
    baseRows,
    allFilters,
    loading,
    errorDescription,
    total,
    page,
    pageSize,
    sortField,
    sortOrder,
    selectedProfile,
    selectedProfileLoading: selectedProfileQuery.isLoading,
    recordsOwnerName,
    columnMetadataLoading,
  };
}

function queriesStamp(queries: { dataUpdatedAt: number }[]): string {
  return queries.map((query) => String(query.dataUpdatedAt)).join(",");
}

export function tableErrorDescription(error: unknown) {
  if (!error || isAbortError(error)) return undefined;
  if (error instanceof APIRequestError) {
    const fields = error.fieldErrors
      .map((field) => (field.field ? `${field.field}: ${field.message}` : field.message))
      .filter(Boolean)
      .join(" · ");
    return fields ? `${error.message} (${fields})` : error.message;
  }
  return error instanceof Error ? error.message : undefined;
}

function isAbortError(error: unknown) {
  return error instanceof Error && error.name === "AbortError";
}

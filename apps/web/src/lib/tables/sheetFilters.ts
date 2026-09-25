import type { CustomField, ProfileListSearch } from "../api/client";
import type { CatalogV1 } from "../../i18n/v1/pt-BR";
import { defaultOp } from "./columnPredicate";
import { fieldFilterActive, fieldPredicate, resolveFunnelKind } from "./bindPredicates";
import {
  brazilStateOptions,
  customFieldFilter,
  dateFilter,
  selectFilter,
  textFilter,
} from "./tableFilters";
import { filterValuePreview, type ToolbarFilterField } from "./FilterControl";

type TablesCopy = CatalogV1["tables"];

export type TypeFilterGroup = {
  label: string;
  options: { value: string; label: string }[];
};

export function buildSheetFilters({
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
}: {
  section: ProfileListSearch["section"];
  search: ProfileListSearch;
  copy: TablesCopy;
  localFilters: Record<string, string>;
  extraFields: CustomField[];
  optionsByFieldId: Map<string, { value: string; label: string }[]>;
  distinctByKey: Record<string, string[]>;
  typeGroups: TypeFilterGroup[];
  identifierTypes: { technical_key: string; label: string }[];
  cityOptions: { value: string; label: string }[];
  setLocal: (key: string, value: string) => void;
  updateSearch: (patch: Partial<ProfileListSearch>) => void;
}): ToolbarFilterField[] {
  const customFilters = extraFields.map((field) =>
    customFieldFilter(
      field,
      localFilters[field.technical_key] ?? "",
      (value) => setLocal(field.technical_key, value),
      optionsByFieldId.get(field.id) ?? [],
      distinctByKey[field.technical_key] ?? [],
      { all: copy.filters.all, boolean: copy.boolean },
    ),
  );
  const statusOptions = [
    { value: "AVAILABLE", label: copy.status.AVAILABLE },
    { value: "IN_USE", label: copy.status.IN_USE },
  ];
  const mediumOptions = [
    { value: "PHYSICAL", label: copy.medium.PHYSICAL },
    { value: "DIGITAL", label: copy.medium.DIGITAL },
  ];
  const typeOptions = typeGroups.flatMap((group) => group.options);
  const typeGroupsForSelect = typeGroups.map((group) => ({
    label: group.label,
    options: group.options,
  }));

  if (section === "documents") {
    return [
      selectFilter(
        "type",
        copy.columns.type,
        search.document_type,
        typeOptions,
        copy.filters.allTypes,
        (value) => updateSearch({ document_type: value, document_page: 1 }),
        typeGroupsForSelect,
      ),
      textFilter("identifier", copy.columns.identifier, search.document_identifier, (value) =>
        updateSearch({ document_identifier: value, document_page: 1 }),
      ),
      textFilter(
        "owner",
        copy.columns.owner,
        localFilters.owner ?? "",
        (value) => setLocal("owner", value),
        true,
      ),
      dateFilter(
        "date",
        copy.columns.date,
        localFilters.date ?? "",
        (value) => setLocal("date", value),
        true,
      ),
      textFilter(
        "current_holder",
        copy.columns.currentHolder,
        localFilters.current_holder ?? "",
        (value) => setLocal("current_holder", value),
        true,
      ),
      textFilter(
        "notes",
        copy.columns.notes,
        localFilters.notes ?? "",
        (value) => setLocal("notes", value),
        true,
      ),
      selectFilter(
        "status",
        copy.columns.status,
        search.document_status,
        statusOptions,
        copy.filters.allStatuses,
        (value) =>
          updateSearch({
            document_status: value as ProfileListSearch["document_status"],
            document_page: 1,
          }),
      ),
      selectFilter(
        "medium",
        copy.columns.medium,
        search.document_medium,
        mediumOptions,
        copy.filters.allMedia,
        (value) =>
          updateSearch({
            document_medium: value as ProfileListSearch["document_medium"],
            document_page: 1,
          }),
      ),
      ...customFilters,
    ];
  }

  if (section === "bills") {
    return [
      selectFilter(
        "type",
        copy.columns.type,
        search.bill_type,
        typeOptions,
        copy.filters.allTypes,
        (value) => updateSearch({ bill_type: value, bill_page: 1 }),
        typeGroupsForSelect,
      ),
      textFilter("reference", copy.columns.reference, search.bill_reference, (value) =>
        updateSearch({ bill_reference: value, bill_page: 1 }),
      ),
      textFilter(
        "owner",
        copy.columns.owner,
        localFilters.owner ?? "",
        (value) => setLocal("owner", value),
        true,
      ),
      textFilter("competence", copy.columns.competence, search.bill_competence, (value) =>
        updateSearch({ bill_competence: value, bill_page: 1 }),
      ),
      textFilter(
        "printed_holder_name",
        copy.columns.printedHolder,
        localFilters.printed_holder_name ?? "",
        (value) => setLocal("printed_holder_name", value),
        true,
      ),
      textFilter(
        "printed_address",
        copy.columns.printedAddress,
        localFilters.printed_address ?? "",
        (value) => setLocal("printed_address", value),
        true,
      ),
      textFilter(
        "notes",
        copy.columns.notes,
        localFilters.notes ?? "",
        (value) => setLocal("notes", value),
        true,
      ),
      selectFilter(
        "status",
        copy.columns.status,
        search.bill_status,
        statusOptions,
        copy.filters.allStatuses,
        (value) =>
          updateSearch({ bill_status: value as ProfileListSearch["bill_status"], bill_page: 1 }),
      ),
      selectFilter(
        "medium",
        copy.columns.medium,
        search.bill_medium,
        mediumOptions,
        copy.filters.allMedia,
        (value) =>
          updateSearch({ bill_medium: value as ProfileListSearch["bill_medium"], bill_page: 1 }),
      ),
      ...customFilters,
    ];
  }

  return [
    textFilter("full_name", copy.columns.fullName, search.full_name, (value) =>
      updateSearch({ full_name: value, page: 1 }),
    ),
    textFilter(
      "street",
      copy.columns.street,
      localFilters.street ?? "",
      (value) => setLocal("street", value),
      true,
    ),
    cityOptions.length > 0
      ? selectFilter(
          "city",
          copy.columns.city,
          search.city,
          cityOptions,
          copy.filters.all,
          (value) => updateSearch({ city: value, page: 1 }),
        )
      : textFilter("city", copy.columns.city, search.city, (value) =>
          updateSearch({ city: value, page: 1 }),
        ),
    textFilter(
      "neighborhood",
      copy.columns.neighborhood,
      localFilters.neighborhood ?? "",
      (value) => setLocal("neighborhood", value),
      true,
    ),
    textFilter(
      "social_name",
      copy.columns.socialName,
      localFilters.social_name ?? "",
      (value) => setLocal("social_name", value),
      true,
    ),
    textFilter("cpf", copy.columns.cpf, search.cpf, (value) =>
      updateSearch({ cpf: value, page: 1 }),
    ),
    textFilter("email", copy.columns.email, search.email, (value) =>
      updateSearch({ email: value, page: 1 }),
    ),
    selectFilter(
      "state",
      copy.columns.state,
      search.state,
      brazilStateOptions(),
      copy.filters.all,
      (value) => updateSearch({ state: value, page: 1 }),
    ),
    textFilter(
      "number",
      copy.columns.number,
      localFilters.number ?? "",
      (value) => setLocal("number", value),
      true,
    ),
    textFilter(
      "complement",
      copy.columns.complement,
      localFilters.complement ?? "",
      (value) => setLocal("complement", value),
      true,
    ),
    textFilter(
      "postal_code",
      copy.columns.postalCode,
      localFilters.postal_code ?? "",
      (value) => setLocal("postal_code", value),
      true,
    ),
    textFilter(
      "mobile",
      copy.columns.mobile,
      localFilters.mobile ?? "",
      (value) => setLocal("mobile", value),
      true,
    ),
    textFilter(
      "landline",
      copy.columns.landline,
      localFilters.landline ?? "",
      (value) => setLocal("landline", value),
      true,
    ),
    dateFilter(
      "birth_date",
      copy.columns.birthDate,
      localFilters.birth_date ?? "",
      (value) => setLocal("birth_date", value),
      true,
    ),
    ...identifierTypes.map((type) =>
      textFilter(
        `doc:${type.technical_key}`,
        type.label,
        localFilters[type.technical_key] ?? "",
        (value) => setLocal(type.technical_key, value),
        true,
      ),
    ),
    ...customFilters,
  ];
}

export function activeFilterChips(fields: ToolbarFilterField[]): {
  key: string;
  field: string;
  value: string;
  local?: boolean;
  onClear: () => void;
}[] {
  return fields
    .filter((field) => fieldFilterActive(field))
    .map((field) => {
      const predicate = fieldPredicate(field);
      const preview =
        predicate.op === "between"
          ? `${predicate.values[0] ?? ""}–${predicate.values[1] ?? ""}`
          : predicate.op === "in"
            ? predicate.values.join(", ")
            : predicate.op === "is_null" || predicate.op === "not_null"
              ? predicate.op
              : predicate.values[0] || (field.kind === "select" ? filterValuePreview(field) : field.value) || "";
      const kind = resolveFunnelKind(field);
      const fallback = defaultOp(kind);
      return {
        key: field.key,
        field: field.label,
        value: preview,
        onClear: () => {
          field.onPredicate?.({ op: fallback, values: [] });
          field.onChange("");
        },
      };
    });
}

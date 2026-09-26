import type { ProfileListSearch } from "../api/client";

export function resetPage(section: ProfileListSearch["section"]): Partial<ProfileListSearch> {
  if (section === "documents") return { document_page: 1 };
  if (section === "bills") return { bill_page: 1 };
  return { page: 1 };
}

export function pagePatch(
  section: ProfileListSearch["section"],
  page: number,
  limit: number,
): Partial<ProfileListSearch> {
  if (section === "documents") return { document_page: page, document_limit: limit };
  if (section === "bills") return { bill_page: page, bill_limit: limit };
  return { page, limit };
}

export function clearFilters(section: ProfileListSearch["section"]): Partial<ProfileListSearch> {
  if (section === "documents") {
    return {
      q: "",
      document_identifier: "",
      document_type: "",
      document_status: "",
      document_medium: "",
      document_page: 1,
      records_owner: undefined,
      recorte: "",
      result: "",
    };
  }
  if (section === "bills") {
    return {
      q: "",
      bill_reference: "",
      bill_type: "",
      bill_status: "",
      bill_medium: "",
      bill_competence: "",
      bill_page: 1,
      records_owner: undefined,
      recorte: "",
      result: "",
    };
  }
  return {
    q: "",
    full_name: "",
    cpf: "",
    email: "",
    city: "",
    state: "",
    page: 1,
    recorte: "",
    result: "",
  };
}

export function profileListKey(search: ProfileListSearch) {
  return {
    page: search.page,
    limit: search.limit,
    sort: search.sort,
    order: search.order,
    q: search.q,
    full_name: search.full_name,
    cpf: search.cpf,
    email: search.email,
    city: search.city,
    state: search.state,
  };
}

export function documentSearch(search: ProfileListSearch) {
  return {
    q: search.q,
    document_page: search.document_page,
    document_limit: search.document_limit,
    document_sort: search.document_sort,
    document_order: search.document_order,
    document_identifier: search.document_identifier,
    document_status: search.document_status,
    document_medium: search.document_medium,
    document_type: search.document_type,
  };
}

export function billSearch(search: ProfileListSearch) {
  return {
    q: search.q,
    bill_page: search.bill_page,
    bill_limit: search.bill_limit,
    bill_sort: search.bill_sort,
    bill_order: search.bill_order,
    bill_reference: search.bill_reference,
    bill_competence: search.bill_competence,
    bill_status: search.bill_status,
    bill_medium: search.bill_medium,
    bill_type: search.bill_type,
  };
}

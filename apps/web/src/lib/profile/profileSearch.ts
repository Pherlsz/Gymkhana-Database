import type { ProfileListSearch, ProfileValuesRequest } from "../api/client";

export const emptyValues: ProfileValuesRequest = {
  full_name: "",
  social_name: "",
  cpf: "",
  email: "",
  mobile_phone: "",
  landline_phone: "",
  address: {
    street: "",
    number: "",
    complement: "",
    neighborhood: "",
    city: "",
    state: "",
    postal_code: "",
  },
  notes: "",
};

export function validateProfileForm(
  values: ProfileValuesRequest,
  validation?: { fullNameRequired: string; invalidEmail: string },
):
  | { success: true; output: ProfileValuesRequest }
  | { success: false; issues: { message: string }[] } {
  const issues: { message: string }[] = [];
  const fullName = values.full_name?.trim() ?? "";
  if (fullName.length < 1) {
    issues.push({ message: validation?.fullNameRequired ?? "" });
  }
  const email = values.email?.trim() ?? "";
  if (email.length > 0) {
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!emailRegex.test(email)) {
      issues.push({ message: validation?.invalidEmail ?? "" });
    }
  }
  if (issues.length > 0) {
    return { success: false, issues };
  }
  return {
    success: true,
    output: {
      ...values,
      full_name: fullName,
      social_name: values.social_name ?? "",
      cpf: values.cpf ?? "",
      email: email,
      mobile_phone: values.mobile_phone ?? "",
      landline_phone: values.landline_phone ?? "",
      address: {
        street: values.address?.street ?? "",
        number: values.address?.number ?? "",
        complement: values.address?.complement ?? "",
        neighborhood: values.address?.neighborhood ?? "",
        city: values.address?.city ?? "",
        state: values.address?.state ?? "",
        postal_code: values.address?.postal_code ?? "",
      },
      notes: values.notes ?? "",
    },
  };
}

export function positiveInteger(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.trunc(parsed) : fallback;
}

export function normalizeProfileSearch(search: Record<string, unknown>): ProfileListSearch {
  const sortValues = [
    "full_name",
    "cpf",
    "email",
    "address_city",
    "address_street",
    "address_neighborhood",
    "mobile_phone",
    "birth_date",
    "created_at",
    "updated_at",
  ] as const;
  const sort = sortValues.includes(search.sort as (typeof sortValues)[number])
    ? (search.sort as ProfileListSearch["sort"])
    : "full_name";
  return {
    page: positiveInteger(search.page, 1),
    limit: Math.min(1000, Math.max(50, positiveInteger(search.limit, 100))),
    sort,
    order: search.order === "desc" ? "desc" : "asc",
    q: typeof search.q === "string" ? search.q : "",
    full_name: typeof search.full_name === "string" ? search.full_name : "",
    cpf: typeof search.cpf === "string" ? search.cpf : "",
    email: typeof search.email === "string" ? search.email : "",
    city: typeof search.city === "string" ? search.city : "",
    state: typeof search.state === "string" ? search.state : "",
    selected: typeof search.selected === "string" ? search.selected : undefined,
    mode:
      search.mode === "create" || search.mode === "edit" || search.mode === "view"
        ? search.mode
        : undefined,
    section:
      search.section === "documents" || search.section === "bills" ? search.section : "profile",
    document_page: positiveInteger(search.document_page, 1),
    document_limit: Math.min(1000, Math.max(50, positiveInteger(search.document_limit, 100))),
    document_sort: [
      "identifier_value",
      "type_label",
      "document_date",
      "created_at",
      "updated_at",
    ].includes(String(search.document_sort))
      ? (search.document_sort as ProfileListSearch["document_sort"])
      : "identifier_value",
    document_order: search.document_order === "desc" ? "desc" : "asc",
    document_identifier:
      typeof search.document_identifier === "string" ? search.document_identifier : "",
    document_status:
      search.document_status === "AVAILABLE" || search.document_status === "IN_USE"
        ? search.document_status
        : "",
    document_medium:
      search.document_medium === "PHYSICAL" || search.document_medium === "DIGITAL"
        ? search.document_medium
        : "",
    document_type: typeof search.document_type === "string" ? search.document_type : "",
    document_selected:
      typeof search.document_selected === "string" ? search.document_selected : undefined,
    document_mode: ["create", "view", "edit", "types"].includes(String(search.document_mode))
      ? (search.document_mode as ProfileListSearch["document_mode"])
      : undefined,
    bill_page: positiveInteger(search.bill_page, 1),
    bill_limit: Math.min(1000, Math.max(50, positiveInteger(search.bill_limit, 100))),
    bill_sort: [
      "reference_value",
      "type_label",
      "competence",
      "amount",
      "created_at",
      "updated_at",
    ].includes(String(search.bill_sort))
      ? (search.bill_sort as ProfileListSearch["bill_sort"])
      : "reference_value",
    bill_order: search.bill_order === "desc" ? "desc" : "asc",
    bill_reference: typeof search.bill_reference === "string" ? search.bill_reference : "",
    bill_competence: typeof search.bill_competence === "string" ? search.bill_competence : "",
    bill_status:
      search.bill_status === "AVAILABLE" || search.bill_status === "IN_USE"
        ? search.bill_status
        : "",
    bill_medium:
      search.bill_medium === "PHYSICAL" || search.bill_medium === "DIGITAL"
        ? search.bill_medium
        : "",
    bill_type: typeof search.bill_type === "string" ? search.bill_type : "",
    bill_selected: typeof search.bill_selected === "string" ? search.bill_selected : undefined,
    bill_mode: ["create", "view", "edit", "types"].includes(String(search.bill_mode))
      ? (search.bill_mode as ProfileListSearch["bill_mode"])
      : undefined,
    records_owner:
      typeof search.records_owner === "string" && search.records_owner
        ? search.records_owner
        : undefined,
    cols: typeof search.cols === "string" ? search.cols : "",
    recorte: normalizeRecorte(search.recorte),
    result: normalizeResult(search.result),
  };
}

function normalizeResult(value: unknown): string {
  if (typeof value !== "string") return "";
  const token = value.trim();
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(token)
    ? token
    : "";
}

function normalizeRecorte(value: unknown): string {
  // The router parses a bare 1 as a number, so the marker is the word on.
  if (value === 1 || value === "1" || value === "on") return "on";
  if (typeof value !== "string") return "";
  const token = value.trim();
  if (token === "1" || token === "on") return "on";
  const keys = token
    .split(",")
    .map((key) => key.trim())
    .filter((key) => /^[a-z][a-z0-9_]*$/i.test(key));
  return keys.join(",");
}

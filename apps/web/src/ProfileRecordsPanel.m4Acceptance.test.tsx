import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ProfileRecordsPanel } from "./ProfileRecordsPanel";
import { normalizeProfileSearch } from "./ProfilesPage";
import type { ProfileListSearch, UserRole } from "./lib/api/client";

const profile = {
  id: "019bf789-4400-7f12-9abc-123456789abc",
  full_name: "Ana da Silva",
  social_name: "Ana",
  cpf: "52998224725",
  email: "ana@example.com",
  mobile_phone: "+5551999998888",
  landline_phone: "",
  address: {
    street: "Rua Atual",
    number: "10",
    complement: "",
    neighborhood: "Centro",
    city: "Porto Alegre",
    state: "RS",
    postal_code: "90000000",
  },
  notes: "",
  version: 1,
  created_at: "2026-07-15T12:00:00Z",
  updated_at: "2026-07-15T12:00:00Z",
};

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "content-type": "application/json" },
  });
}

function renderRecords(
  section: "documents" | "bills",
  patch: Record<string, unknown> = {},
  role: UserRole = "MEMBER",
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const onSearch = vi.fn<(patch: Partial<ProfileListSearch>) => void>();
  const onNotice = vi.fn<(message: string) => void>();
  const search = normalizeProfileSearch({ section, ...patch });
  render(
    <QueryClientProvider client={queryClient}>
      <ProfileRecordsPanel
        profile={profile}
        role={role}
        search={search}
        section={section}
        onSearch={onSearch}
        onNotice={onNotice}
      />
    </QueryClientProvider>,
  );
  return { onSearch, onNotice };
}

const billType = {
  id: "bill-type-1",
  technical_key: "energy",
  label: "Energia",
  active: true,
  supports_current_use: true,
  version: 1,
  created_at: "2026-07-15T12:00:00Z",
  updated_at: "2026-07-15T12:00:00Z",
};

function billRecord(supportsCurrentUse = true) {
  return {
    id: "bill-1",
    owner_profile_id: profile.id,
    bill_type_id: billType.id,
    printed_holder_name: "Titular impresso original",
    printed_address: "Endereço impresso original, 001",
    reference_value: "000A-99",
    competence: "2026-07",
    amount: "123.40",
    currency: "BRL",
    notes: "",
    record_state: "REPLACED" as const,
    status: supportsCurrentUse ? ("IN_USE" as const) : ("AVAILABLE" as const),
    type: { ...billType, supports_current_use: supportsCurrentUse },
    ...(supportsCurrentUse
      ? {
          current_use: {
            holder_profile_id: "019bf789-4400-7f12-9abc-123456789abd",
            assigned_at: "2026-07-16T12:00:00Z",
          },
        }
      : {}),
    version: 3,
    created_at: "2026-07-15T12:00:00Z",
    updated_at: "2026-07-16T12:00:00Z",
  };
}

function billFetchMock(supportsCurrentUse = true) {
  const record = billRecord(supportsCurrentUse);
  return vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    if (url.includes("/api/v1/bill-types"))
      return Promise.resolve(
        jsonResponse({
          types: [record.type],
          page: { total: 1, limit: 1000, offset: 0, sort_field: "label", sort_order: "asc" },
        }),
      );
    if (url.includes("/api/v1/bills?") && (!init?.method || init.method === "GET"))
      return Promise.resolve(
        jsonResponse({
          bills: [record],
          page: {
            total: 1,
            limit: 100,
            offset: 0,
            sort_field: "reference_value",
            sort_order: "asc",
          },
        }),
      );
    if (url.includes("/api/v1/profiles?limit=1000"))
      return Promise.resolve(
        jsonResponse({
          profiles: [
            profile,
            {
              ...profile,
              id: "019bf789-4400-7f12-9abc-123456789abd",
              full_name: "Bruno Atual",
            },
          ],
          page: { total: 2, limit: 1000, offset: 0, sort_field: "full_name", sort_order: "asc" },
        }),
      );
    if (url.includes("/api/v1/bills/bill-1/current-use") && init?.method === "DELETE")
      return Promise.resolve(new Response(null, { status: 204 }));
    return Promise.resolve(jsonResponse({}));
  });
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("M4 Profile records acceptance", () => {
  it("normalizes the complete URL-backed record state deterministically", () => {
    const value = normalizeProfileSearch({
      section: "bills",
      bill_page: "3",
      bill_limit: "5000",
      bill_sort: "amount",
      bill_order: "desc",
      bill_reference: "000A",
      bill_competence: "2026-07",
      bill_status: "IN_USE",
      bill_state: "EXPIRED",
      bill_type: "bill-type-1",
      bill_selected: "bill-1",
      bill_mode: "view",
      document_state: "INVALID",
      document_mode: "invalid",
    });

    expect(value).toMatchObject({
      section: "bills",
      bill_page: 3,
      bill_limit: 1000,
      bill_sort: "amount",
      bill_order: "desc",
      bill_reference: "000A",
      bill_competence: "2026-07",
      bill_status: "IN_USE",
      bill_state: "EXPIRED",
      bill_type: "bill-type-1",
      bill_selected: "bill-1",
      bill_mode: "view",
      document_state: "",
      document_mode: undefined,
    });
  });

  it("keeps printed bill data independent, exposes native controls, and returns current use", async () => {
    const fetchMock = billFetchMock(true);
    vi.stubGlobal("fetch", fetchMock);
    const { onNotice } = renderRecords("bills", { bill_selected: "bill-1", bill_mode: "view" });

    expect(await screen.findByDisplayValue("Titular impresso original")).toBeDisabled();
    expect(screen.getByDisplayValue("Endereço impresso original, 001")).toBeDisabled();
    expect(screen.getByDisplayValue("000A-99")).toBeDisabled();
    expect(screen.getByDisplayValue("2026-07")).toBeDisabled();
    expect(screen.getByDisplayValue("123.40")).toBeDisabled();

    const holder = screen.getByRole("combobox", { name: "Pessoa em uso" });
    expect(holder.tagName).toBe("SELECT");
    expect(holder).toHaveValue("019bf789-4400-7f12-9abc-123456789abd");
    const giveBack = screen.getByRole("button", { name: "Devolver" });
    expect(giveBack.tagName).toBe("BUTTON");
    expect(giveBack.tabIndex).toBe(0);
    fireEvent.click(giveBack);

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/bills/bill-1/current-use"),
        expect.objectContaining({ method: "DELETE" }),
      ),
    );
    await waitFor(() =>
      expect(onNotice).toHaveBeenCalledWith("Registro devolvido e disponibilizado."),
    );
  });

  it("does not fabricate current-use controls for unsupported bill types", async () => {
    vi.stubGlobal("fetch", billFetchMock(false));
    renderRecords("bills", { bill_selected: "bill-1", bill_mode: "view" });

    expect(await screen.findByDisplayValue("Titular impresso original")).toBeDisabled();
    expect(screen.queryByText("Uso atual")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Devolver" })).not.toBeInTheDocument();
  });

  it("surfaces optimistic conflicts and reloads instead of silently overwriting", async () => {
    const documentType = {
      id: "document-type-1",
      technical_key: "rg",
      label: "RG",
      active: true,
      uniqueness_policy: "PER_PROFILE",
      validation_regex: "",
      date_required: false,
      version: 1,
      created_at: "2026-07-15T12:00:00Z",
      updated_at: "2026-07-15T12:00:00Z",
    };
    const document = {
      id: "document-1",
      owner_profile_id: profile.id,
      document_type_id: documentType.id,
      identifier_value: "00123",
      document_date: "2026-07-01",
      notes: "",
      record_state: "CURRENT",
      status: "AVAILABLE",
      type: documentType,
      version: 1,
      created_at: "2026-07-15T12:00:00Z",
      updated_at: "2026-07-15T12:00:00Z",
    };
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/api/v1/document-types"))
        return Promise.resolve(
          jsonResponse({
            types: [documentType],
            page: { total: 1, limit: 1000, offset: 0, sort_field: "label", sort_order: "asc" },
          }),
        );
      if (url.includes("/api/v1/documents?") && (!init?.method || init.method === "GET"))
        return Promise.resolve(
          jsonResponse({
            documents: [document],
            page: {
              total: 1,
              limit: 100,
              offset: 0,
              sort_field: "identifier_value",
              sort_order: "asc",
            },
          }),
        );
      if (url.includes("/api/v1/documents/document-1") && init?.method === "PUT")
        return Promise.resolve(
          jsonResponse(
            {
              error: {
                code: "conflict",
                message: "The document changed concurrently.",
              },
            },
            409,
          ),
        );
      return Promise.resolve(jsonResponse({}));
    });
    vi.stubGlobal("fetch", fetchMock);
    const { onNotice } = renderRecords("documents");

    const identifier = await screen.findByLabelText("Identificador de RG");
    fireEvent.change(identifier, { target: { value: "00999" } });
    fireEvent.blur(identifier);

    await waitFor(() =>
      expect(onNotice).toHaveBeenCalledWith(
        "Outro usuário alterou o registro. Os dados foram recarregados.",
      ),
    );
    expect(screen.getByDisplayValue("00123")).toBeInTheDocument();
  });
});

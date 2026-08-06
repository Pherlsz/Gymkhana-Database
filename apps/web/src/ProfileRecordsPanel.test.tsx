import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ProfileRecordsPanel } from "./ProfileRecordsPanel";
import { normalizeProfileSearch } from "./ProfilesPage";

const profile = {
  id: "019bf789-4400-7f12-9abc-123456789abc",
  full_name: "Ana da Silva",
  social_name: "Ana",
  cpf: "52998224725",
  email: "ana@example.com",
  mobile_phone: "+5551999998888",
  landline_phone: "",
  address: {
    street: "Rua A",
    number: "1",
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

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { "content-type": "application/json" },
  });
}

function renderPanel(role: "EXTERNAL" | "ADMIN" = "EXTERNAL") {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const onSearch = vi.fn();
  const onNotice = vi.fn();
  render(
    <QueryClientProvider client={queryClient}>
      <ProfileRecordsPanel
        profile={profile}
        role={role}
        search={normalizeProfileSearch({ section: "documents" })}
        section="documents"
        onSearch={onSearch}
        onNotice={onNotice}
      />
    </QueryClientProvider>,
  );
  return { onSearch, onNotice };
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("ProfileRecordsPanel", () => {
  it("lists Profile documents, keeps filters in navigation state, and saves inline fields", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/api/v1/document-types"))
        return Promise.resolve(
          jsonResponse({
            types: [
              {
                id: "type-1",
                technical_key: "rg",
                label: "RG",
                active: true,
                uniqueness_policy: "PER_PROFILE",
                validation_regex: "",
                date_required: false,
                version: 1,
                created_at: "2026-07-15T12:00:00Z",
                updated_at: "2026-07-15T12:00:00Z",
              },
            ],
            page: { total: 1, limit: 1000, offset: 0, sort_field: "label", sort_order: "asc" },
          }),
        );
      if (url.includes("/api/v1/documents?") && (!init?.method || init.method === "GET"))
        return Promise.resolve(
          jsonResponse({
            documents: [
              {
                id: "doc-1",
                owner_profile_id: profile.id,
                document_type_id: "type-1",
                identifier_value: "00123",
                document_date: "2026-07-01",
                notes: "",
                record_state: "CURRENT",
                status: "AVAILABLE",
                type: {
                  id: "type-1",
                  technical_key: "rg",
                  label: "RG",
                  active: true,
                  uniqueness_policy: "PER_PROFILE",
                  validation_regex: "",
                  date_required: false,
                  version: 1,
                  created_at: "2026-07-15T12:00:00Z",
                  updated_at: "2026-07-15T12:00:00Z",
                },
                version: 1,
                created_at: "2026-07-15T12:00:00Z",
                updated_at: "2026-07-15T12:00:00Z",
              },
            ],
            page: {
              total: 1,
              limit: 100,
              offset: 0,
              sort_field: "identifier_value",
              sort_order: "asc",
            },
          }),
        );
      if (url.includes("/api/v1/documents/doc-1") && init?.method === "PUT")
        return Promise.resolve(jsonResponse({}));
      return Promise.resolve(
        jsonResponse({
          profiles: [profile],
          page: { total: 1, limit: 1000, offset: 0, sort_field: "full_name", sort_order: "asc" },
        }),
      );
    });
    vi.stubGlobal("fetch", fetchMock);
    const { onSearch } = renderPanel();
    expect(await screen.findAllByText("RG")).not.toHaveLength(0);
    expect(screen.getByDisplayValue("00123")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Identificador"), { target: { value: "001" } });
    expect(onSearch).toHaveBeenCalledWith({ document_identifier: "001", document_page: 1 });
    const inline = screen.getByLabelText("Identificador de RG");
    fireEvent.change(inline, { target: { value: "00099" } });
    fireEvent.blur(inline);
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/documents/doc-1"),
        expect.objectContaining({ method: "PUT" }),
      ),
    );
  });

  it("shows type administration only to administrators", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse({
          types: [],
          documents: [],
          page: {
            total: 0,
            limit: 100,
            offset: 0,
            sort_field: "identifier_value",
            sort_order: "asc",
          },
        }),
      ),
    );
    renderPanel("EXTERNAL");
    await screen.findByText("Nenhum documento cadastrado para esta pessoa.");
    expect(screen.queryByRole("button", { name: "Administrar tipos" })).not.toBeInTheDocument();
    cleanup();
    renderPanel("ADMIN");
    expect(await screen.findByRole("button", { name: "Administrar tipos" })).toBeInTheDocument();
  });
});

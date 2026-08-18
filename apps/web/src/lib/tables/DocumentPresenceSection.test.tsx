import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Profile } from "../api/client";
import { DocumentPresenceSection } from "./DocumentPresenceSection";

const profile: Profile = {
  id: "019bf789-4400-7f12-9abc-123456789abc",
  full_name: "Ana da Silva",
  social_name: "Ana",
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
  version: 1,
  created_at: "2026-07-15T12:00:00Z",
  updated_at: "2026-07-15T12:00:00Z",
  document_presences: [
    {
      document_type_id: "type-rg",
      technical_key: "rg",
      label: "RG",
      claim: "absence",
      has_physical: false,
      has_digital: false,
    },
  ],
  document_badges: [],
};

const copy = {
  title: "Presença de documentos",
  unspecified: "Não informado",
  absence: "Não tem",
  indication: "Tem, sem número",
  informedNumber: "Número informado",
  number: "Número",
  hasExemplar: "Há exemplar cadastrado",
  saveError: "Não foi possível atualizar a presença.",
  withOwner: "Com o dono",
};

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { "content-type": "application/json" },
  });
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("DocumentPresenceSection", () => {
  it("saves indication without creating an exemplar", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/api/v1/document-types")) {
        return Promise.resolve(
          jsonResponse({
            types: [
              {
                id: "type-rg",
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
            page: { total: 1, limit: 1000, offset: 0 },
          }),
        );
      }
      if (url.includes("/api/v1/document-presences") && init?.method === "PUT") {
        return Promise.resolve(
          jsonResponse({
            id: "presence-1",
            profile_id: profile.id,
            document_type_id: "type-rg",
            claim: "indication",
            version: 1,
          }),
        );
      }
      return Promise.resolve(jsonResponse({}));
    });
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <DocumentPresenceSection copy={copy} editable profile={profile} />
      </QueryClientProvider>,
    );
    // The claim control is an Ant Select now, so the value changes by opening
    // the listbox rather than by setting a native select's value.
    // The claim control is an Ant Select now, so the value changes by opening
    // the listbox and choosing, not by setting a native select's value.
    const select = await screen.findByRole("combobox", { name: "RG" });
    fireEvent.mouseDown(select);
    const option = await screen.findByTitle(copy.indication);
    fireEvent.click(option);
    await waitFor(() => {
      const call = fetchMock.mock.calls.find((entry) =>
        String(entry[0]).includes("/api/v1/document-presences"),
      );
      expect(call?.[1]).toMatchObject({ method: "PUT" });
      expect(JSON.parse(String(call?.[1]?.body))).toEqual({
        profile_id: profile.id,
        document_type_id: "type-rg",
        claim: "indication",
      });
    });
  });
});

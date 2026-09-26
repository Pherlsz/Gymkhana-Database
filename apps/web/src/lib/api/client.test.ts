import { afterEach, describe, expect, it, vi } from "vitest";
import {
  APIRequestError,
  executeSearch,
  getAuthSession,
  getLiveHealth,
  listBills,
  listDocuments,
  logout,
} from "./client";

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  const headers = new Headers(init.headers);
  headers.set("content-type", "application/json");
  return new Response(JSON.stringify(body), { ...init, headers });
}

describe("generated API client helpers", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("returns health and protected session responses", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ status: "ok" }))
      .mockResolvedValueOnce(
        jsonResponse({
          authenticated: true,
          user: { login: "member", display_name: "Member", role: "EXTERNAL" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    await expect(getLiveHealth()).resolves.toEqual({ status: "ok" });
    await expect(getAuthSession()).resolves.toMatchObject({ user: { login: "member" } });
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      expect.stringContaining("/health/live"),
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("preserves stable API error metadata", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(
          {
            error: { code: "unauthorized", message: "Authentication is required" },
            request_id: "request-123",
          },
          { status: 401 },
        ),
      ),
    );

    const error = await getAuthSession().catch((cause: unknown) => cause);
    expect(error).toBeInstanceOf(APIRequestError);
    expect(error).toMatchObject({
      status: 401,
      code: "unauthorized",
      requestId: "request-123",
      message: "Authentication is required",
    });
  });

  it("accepts the empty logout response", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(logout()).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/auth/logout"),
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
  });

  it("lists documents without requiring an owner", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        documents: [],
        page: {
          total: 0,
          limit: 100,
          offset: 0,
          sort_field: "identifier_value",
          sort_order: "asc",
        },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    await listDocuments(undefined, {
      q: "",
      document_page: 1,
      document_limit: 100,
      document_sort: "identifier_value",
      document_order: "asc",
      document_identifier: "",
      document_status: "",
      document_medium: "",
      document_type: "type-rg",
    });
    const url = String(fetchMock.mock.calls[0]?.[0]);
    expect(url).toContain("/api/v1/documents?");
    expect(url).toContain("document_type_id=type-rg");
    expect(url).not.toContain("owner_profile_id=");
  });

  it("sends the document status filter as status", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        documents: [],
        page: {
          total: 0,
          limit: 100,
          offset: 0,
          sort_field: "identifier_value",
          sort_order: "asc",
        },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    await listDocuments(undefined, {
      q: "",
      document_page: 1,
      document_limit: 100,
      document_sort: "identifier_value",
      document_order: "asc",
      document_identifier: "",
      document_status: "IN_USE",
      document_medium: "PHYSICAL",
      document_type: "",
    });
    const url = String(fetchMock.mock.calls[0]?.[0]);
    expect(url).toContain("status=IN_USE");
    expect(url).toContain("medium=PHYSICAL");
    expect(url).not.toContain("document_status=");
  });

  it("lists bills without requiring an owner", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        bills: [],
        page: { total: 0, limit: 100, offset: 0, sort_field: "reference_value", sort_order: "asc" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    await listBills(undefined, {
      q: "",
      bill_page: 1,
      bill_limit: 100,
      bill_sort: "reference_value",
      bill_order: "asc",
      bill_reference: "",
      bill_competence: "",
      bill_status: "",
      bill_medium: "",
      bill_type: "type-luz",
    });
    const url = String(fetchMock.mock.calls[0]?.[0]);
    expect(url).toContain("/api/v1/bills?");
    expect(url).toContain("bill_type_id=type-luz");
    expect(url).not.toContain("owner_profile_id=");
  });

  it("posts a Search body with numeric paging and no URL state", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        results: [],
        page: { total: 0, limit: 50, offset: 0, sort: "relevance", sort_order: "desc" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await executeSearch({
      q: "Ana",
      limit: "50" as unknown as number,
      offset: "0" as unknown as number,
      sort: "relevance",
      order: "desc",
      page: 1,
      modules: "",
    } as unknown as Parameters<typeof executeSearch>[0]);

    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(JSON.parse(String(init.body))).toEqual({
      q: "Ana",
      limit: 50,
      offset: 0,
      sort: "relevance",
      order: "desc",
    });
  });
});

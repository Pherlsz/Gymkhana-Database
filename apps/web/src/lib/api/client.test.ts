import { afterEach, describe, expect, it, vi } from "vitest";
import { APIRequestError, getAuthSession, getLiveHealth, logout } from "./client";

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
});

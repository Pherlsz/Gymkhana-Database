import { afterEach, describe, expect, it, vi } from "vitest";
import {
  APIRequestError,
  getAuthSession,
  getLiveHealth,
  listUsers,
  logout,
  updateUserAccess,
} from "./client";

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  const headers = new Headers(init.headers);
  headers.set("content-type", "application/json");
  return new Response(JSON.stringify(body), { ...init, headers });
}

describe("generated API client helpers", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("returns health, session, and managed users", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ status: "ok" }))
      .mockResolvedValueOnce(
        jsonResponse({
          authenticated: true,
          user: { login: "admin", display_name: "Admin", role: "ADMIN" },
          capabilities: { manage_users: true },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          users: [
            {
              id: "11111111-1111-4111-8111-111111111111",
              login: "member",
              display_name: "Member",
              role: "MEMBER",
              active: true,
              version: 1,
              protected: false,
            },
          ],
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    await expect(getLiveHealth()).resolves.toEqual({ status: "ok" });
    await expect(getAuthSession()).resolves.toMatchObject({ user: { login: "admin" } });
    await expect(listUsers()).resolves.toHaveLength(1);
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

  it("updates access and accepts the empty logout response", async () => {
    const updated = {
      id: "11111111-1111-4111-8111-111111111111",
      login: "member",
      display_name: "Member",
      role: "ADMIN",
      active: true,
      version: 2,
      protected: false,
    };
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(updated))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      updateUserAccess(updated.id, { role: "ADMIN", active: true, version: 1 }),
    ).resolves.toEqual(updated);
    await expect(logout()).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      `/api/admin/users/${updated.id}/access`,
      expect.objectContaining({ method: "PATCH", credentials: "include" }),
    );
  });
});

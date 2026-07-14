import { afterEach, describe, expect, it, vi } from "vitest";
import { APIRequestError, getLiveHealth } from "./client";

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  const headers = new Headers(init.headers);
  headers.set("content-type", "application/json");
  return new Response(JSON.stringify(body), { ...init, headers });
}

describe("generated API client helpers", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the generated health response type", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ status: "ok" })));

    await expect(getLiveHealth()).resolves.toEqual({ status: "ok" });
  });

  it("preserves stable API error metadata", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(
          {
            error: { code: "not_found", message: "Resource was not found" },
            request_id: "request-123",
          },
          { status: 404 },
        ),
      ),
    );

    const error = await getLiveHealth().catch((cause: unknown) => cause);
    expect(error).toBeInstanceOf(APIRequestError);
    expect(error).toMatchObject({
      status: 404,
      code: "not_found",
      requestId: "request-123",
      message: "Resource was not found",
    });
  });
});

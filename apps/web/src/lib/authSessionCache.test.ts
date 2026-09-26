import { afterEach, describe, expect, it } from "vitest";
import {
  clearCachedAuthSession,
  readCachedAuthSession,
  writeCachedAuthSession,
} from "./authSessionCache";
import type { AuthSessionResponse } from "./api/client";

const sample: AuthSessionResponse = {
  authenticated: true,
  user: {
    login: "member@example.com",
    display_name: "Member",
    role: "EXTERNAL",
  },
};

describe("authSessionCache", () => {
  afterEach(() => {
    clearCachedAuthSession();
  });

  it("round-trips a valid session", () => {
    writeCachedAuthSession(sample);
    expect(readCachedAuthSession()).toEqual(sample);
  });

  it("rejects malformed cache entries", () => {
    sessionStorage.setItem("gymkhana-auth-session", JSON.stringify({ authenticated: true }));
    expect(readCachedAuthSession()).toBeNull();
  });

  it("clears the cached session", () => {
    writeCachedAuthSession(sample);
    clearCachedAuthSession();
    expect(readCachedAuthSession()).toBeNull();
  });
});

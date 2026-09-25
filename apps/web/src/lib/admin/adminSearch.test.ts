import { describe, expect, it } from "vitest";
import { normalizeAdminSearch } from "./adminSearch";

describe("normalizeAdminSearch", () => {
  it("defaults to access and keeps the integrations tab", () => {
    expect(normalizeAdminSearch({})).toEqual({ tab: "access" });
    expect(normalizeAdminSearch({ tab: "integrations" })).toEqual({ tab: "integrations" });
    expect(normalizeAdminSearch({ tab: "users" })).toEqual({ tab: "access" });
  });

  it("keeps the Google Forms OAuth result on integrations", () => {
    expect(normalizeAdminSearch({ tab: "integrations", google_forms: "connected" })).toEqual({
      tab: "integrations",
      google_forms: "connected",
    });
    expect(normalizeAdminSearch({ tab: "access", google_forms: "denied" })).toEqual({
      tab: "access",
      google_forms: "denied",
    });
    expect(normalizeAdminSearch({ tab: "integrations", google_forms: "other" })).toEqual({
      tab: "integrations",
    });
  });
});

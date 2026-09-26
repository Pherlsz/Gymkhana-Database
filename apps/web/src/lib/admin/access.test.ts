import { describe, expect, it } from "vitest";
import {
  canEditUserAccess,
  DEFAULT_MEMBER_CAPABILITIES,
  GRANTABLE_CAPABILITIES,
  matchesAdminUserQuery,
  memberAccessIsComplete,
} from "./access";

describe("canEditUserAccess", () => {
  it("blocks the signed-in account and the superadmin", () => {
    expect(canEditUserAccess("admin@x.com", { login: "admin@x.com", role: "ADMIN" })).toBe(false);
    expect(canEditUserAccess("Admin@x.com", { login: "admin@x.com", role: "EXTERNAL" })).toBe(
      false,
    );
    expect(canEditUserAccess("admin@x.com", { login: "owner@x.com", role: "SUPERADMIN" })).toBe(
      false,
    );
    expect(canEditUserAccess("admin@x.com", { login: "member@x.com", role: "EXTERNAL" })).toBe(
      true,
    );
    expect(canEditUserAccess("admin@x.com", { login: "other@x.com", role: "ADMIN" })).toBe(true);
  });

  it("filters the access list by name, email, or extra terms", () => {
    const user = { display_name: "Ana Silva", login: "ana@x.com" };
    expect(matchesAdminUserQuery(user, "")).toBe(true);
    expect(matchesAdminUserQuery(user, "ANA")).toBe(true);
    expect(matchesAdminUserQuery(user, "x.com")).toBe(true);
    expect(matchesAdminUserQuery(user, "membro", ["Membro"])).toBe(true);
    expect(matchesAdminUserQuery(user, "admin")).toBe(false);
  });

  it("lists only capabilities that exist in the product", () => {
    expect(GRANTABLE_CAPABILITIES).toEqual([
      "SEARCH",
      "PROFILES",
      "DATA_TABLES",
      "CUSTOM_DATA",
      "ATTACHMENTS",
      "OCR",
      "OPERATIONS",
      "GOOGLE_FORMS",
      "CHAT",
    ]);
  });

  it("defaults a new member to people, tables, extra fields, and bulk import", () => {
    expect(DEFAULT_MEMBER_CAPABILITIES).toEqual([
      "SEARCH",
      "PROFILES",
      "DATA_TABLES",
      "CUSTOM_DATA",
      "ATTACHMENTS",
      "OPERATIONS",
    ]);
  });

  it("requires at least one permission when the role is member", () => {
    expect(memberAccessIsComplete("ADMIN", [])).toBe(true);
    expect(memberAccessIsComplete("EXTERNAL", [])).toBe(false);
    expect(memberAccessIsComplete("EXTERNAL", ["SEARCH"])).toBe(true);
  });
});

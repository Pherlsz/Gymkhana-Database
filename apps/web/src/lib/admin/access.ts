import type { UserRole } from "../api/client";

// Wire values match the server capability constants. Only surfaces that exist in the product.
export const GRANTABLE_CAPABILITIES = [
  "SEARCH",
  "PROFILES",
  "DATA_TABLES",
  "CUSTOM_DATA",
  "ATTACHMENTS",
  "OCR",
  "OPERATIONS",
  "GOOGLE_FORMS",
  "CHAT",
] as const;

export type GrantableCapability = (typeof GRANTABLE_CAPABILITIES)[number];

export const DEFAULT_MEMBER_CAPABILITIES: GrantableCapability[] = [
  "SEARCH",
  "PROFILES",
  "DATA_TABLES",
  "CUSTOM_DATA",
  "ATTACHMENTS",
  "OPERATIONS",
];

export type AccessSubject = {
  login: string;
  role: UserRole;
};

function sameLogin(left: string, right: string): boolean {
  return left.trim().toLowerCase() === right.trim().toLowerCase();
}

// Session exposes login (the email), not the user id. The server still rejects the id match.
export function accessLock(actorLogin: string, user: AccessSubject): "self" | "superadmin" | null {
  if (sameLogin(actorLogin, user.login)) return "self";
  if (user.role === "SUPERADMIN") return "superadmin";
  return null;
}

export function canEditUserAccess(actorLogin: string, user: AccessSubject): boolean {
  return accessLock(actorLogin, user) == null;
}

export function matchesAdminUserQuery(
  user: { display_name: string; login: string },
  query: string,
  terms: readonly string[] = [],
): boolean {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  return [user.display_name, user.login, ...terms].some((value) =>
    value.toLowerCase().includes(needle),
  );
}

export function assignableRole(role: UserRole): "EXTERNAL" | "ADMIN" | null {
  if (role === "ADMIN" || role === "EXTERNAL") return role;
  return null;
}

export function memberAccessIsComplete(
  role: "EXTERNAL" | "ADMIN",
  capabilities: readonly string[],
): boolean {
  if (role === "ADMIN") return true;
  return capabilities.length > 0;
}

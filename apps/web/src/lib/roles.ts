import type { UserRole } from "./api/client";

export function canManageUsers(role: UserRole): boolean {
  return role === "ADMIN" || role === "SUPERADMIN";
}

export function isSuperadmin(role: UserRole): boolean {
  return role === "SUPERADMIN";
}

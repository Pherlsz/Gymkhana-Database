import type { UserRole } from "./api/client";

export function canManageUsers(role: UserRole): boolean {
  return role === "ADMIN" || role === "SUPERADMIN";
}

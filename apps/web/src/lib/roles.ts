import type { UserRole } from "./lib/api/client";

export function canManageUsers(role: UserRole) {
  return role === "ADMIN" || role === "SUPERADMIN";
}

export function roleLabel(role: UserRole) {
  return role === "SUPERADMIN" ? "Superadmin" : role === "ADMIN" ? "Admin" : "Membro";
}

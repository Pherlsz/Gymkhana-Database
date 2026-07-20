import { requestJSON } from "./client";

export type UserRole = "MEMBER" | "ADMIN" | "SUPERADMIN";

export type AuthUser = {
  email: string;
  display_name: string;
  avatar_url?: string;
  role: UserRole;
};

export type AuthSessionResponse = {
  authenticated: boolean;
  user?: AuthUser;
};

export type AdminUser = {
  id: string;
  email: string;
  display_name: string;
  avatar_url?: string;
  role: UserRole;
  active: boolean;
  version: number;
};

export type AdminUsersResponse = { users: AdminUser[] };
export type UpdateUserAccessRequest = { role: UserRole; active: boolean; version: number };

export function getAuthSession(signal?: AbortSignal): Promise<AuthSessionResponse> {
  return requestJSON<AuthSessionResponse>("/api/auth/session", signal ? { signal } : {});
}

export async function logout(): Promise<void> {
  await requestJSON<void>("/api/auth/logout", { method: "POST" });
}

export function listApplicationUsers(signal?: AbortSignal): Promise<AdminUsersResponse> {
  return requestJSON<AdminUsersResponse>("/api/admin/users?limit=100&offset=0", signal ? { signal } : {});
}

export function updateApplicationUserAccess(
  id: string,
  request: UpdateUserAccessRequest,
): Promise<AdminUser> {
  return requestJSON<AdminUser>(`/api/admin/users/${encodeURIComponent(id)}/access`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });
}

import type { AuthSessionResponse } from "./api/client";

const SESSION_CACHE_KEY = "gymkhana-auth-session";

const ROLES = new Set(["EXTERNAL", "ADMIN", "SUPERADMIN"]);

export function readCachedAuthSession(): AuthSessionResponse | null {
  try {
    const raw = sessionStorage.getItem(SESSION_CACHE_KEY);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (!isAuthSession(parsed)) return null;
    return parsed;
  } catch {
    return null;
  }
}

export function writeCachedAuthSession(session: AuthSessionResponse): void {
  try {
    sessionStorage.setItem(SESSION_CACHE_KEY, JSON.stringify(session));
  } catch {
    // Quota / private mode — ignore; next load will revalidate from the cookie.
  }
}

export function clearCachedAuthSession(): void {
  try {
    sessionStorage.removeItem(SESSION_CACHE_KEY);
  } catch {
    // ignore
  }
}

function isAuthSession(value: unknown): value is AuthSessionResponse {
  if (!value || typeof value !== "object") return false;
  const record = value as Record<string, unknown>;
  if (record.authenticated !== true) return false;
  const user = record.user;
  if (!user || typeof user !== "object") return false;
  const u = user as Record<string, unknown>;
  return (
    typeof u.login === "string" &&
    typeof u.display_name === "string" &&
    typeof u.role === "string" &&
    ROLES.has(u.role)
  );
}

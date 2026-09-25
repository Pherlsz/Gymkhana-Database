export const ADMIN_TABS = ["access", "integrations"] as const;
export type AdminTab = (typeof ADMIN_TABS)[number];

export const ADMIN_OAUTH_FLAGS = ["connected", "denied"] as const;
export type AdminOAuthFlag = (typeof ADMIN_OAUTH_FLAGS)[number];

export type AdminPageSearch = {
  tab: AdminTab;
  google_forms?: AdminOAuthFlag;
};

export const ADMIN_SEARCH_DEFAULTS: AdminPageSearch = { tab: "access" };

export const ADMIN_INTEGRATIONS_RETURN_PATH = "/admin?tab=integrations";

export function isAdminTab(value: unknown): value is AdminTab {
  return typeof value === "string" && (ADMIN_TABS as readonly string[]).includes(value);
}

function isOAuthFlag(value: unknown): value is AdminOAuthFlag {
  return typeof value === "string" && (ADMIN_OAUTH_FLAGS as readonly string[]).includes(value);
}

export function normalizeAdminSearch(search: Record<string, unknown>): AdminPageSearch {
  const result: AdminPageSearch = {
    tab: isAdminTab(search.tab) ? search.tab : "access",
  };
  if (isOAuthFlag(search.google_forms)) result.google_forms = search.google_forms;
  return result;
}

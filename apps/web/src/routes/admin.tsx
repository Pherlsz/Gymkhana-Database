import { createFileRoute, stripSearchParams } from "@tanstack/react-router";
import { AdminPage } from "../AdminPage";
import { ADMIN_SEARCH_DEFAULTS, normalizeAdminSearch } from "../lib/admin/adminSearch";

export const Route = createFileRoute("/admin")({
  validateSearch: normalizeAdminSearch,
  search: {
    middlewares: [stripSearchParams(ADMIN_SEARCH_DEFAULTS)],
  },
  component: AdminPage,
});

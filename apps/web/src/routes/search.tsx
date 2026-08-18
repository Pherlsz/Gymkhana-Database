import { createFileRoute, stripSearchParams } from "@tanstack/react-router";
import { EmptyTab } from "../EmptyTab";
import { GLOBAL_SEARCH_DEFAULTS, normalizeGlobalSearch } from "../SearchPage";

export const Route = createFileRoute("/search")({
  validateSearch: normalizeGlobalSearch,
  search: {
    middlewares: [stripSearchParams(GLOBAL_SEARCH_DEFAULTS)],
  },
  component: EmptyTab,
});

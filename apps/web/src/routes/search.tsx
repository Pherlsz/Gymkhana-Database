import { createFileRoute, stripSearchParams } from "@tanstack/react-router";
import { GLOBAL_SEARCH_DEFAULTS, normalizeGlobalSearch } from "../lib/search/urlState";
import { SearchPage } from "../SearchPage";

export const Route = createFileRoute("/search")({
  validateSearch: normalizeGlobalSearch,
  search: {
    middlewares: [stripSearchParams(GLOBAL_SEARCH_DEFAULTS)],
  },
  component: SearchPage,
});

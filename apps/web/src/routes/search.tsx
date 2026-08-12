import { createFileRoute } from "@tanstack/react-router";
import { SearchPage, normalizeGlobalSearch } from "../SearchPage";

export const Route = createFileRoute("/search")({
  validateSearch: normalizeGlobalSearch,
  component: SearchPage,
});

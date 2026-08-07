import { createFileRoute } from "@tanstack/react-router";
import { QueryPage } from "../QueryPage";
import { normalizeQuerySearch } from "../lib/queryState";

export const Route = createFileRoute("/query")({
  validateSearch: normalizeQuerySearch,
  component: QueryPage,
});

import { createFileRoute } from "@tanstack/react-router";
import { EmptyTab } from "../EmptyTab";
import { normalizeOperationsSearch } from "../OperationsPage";

export const Route = createFileRoute("/admin")({
  validateSearch: normalizeOperationsSearch,
  component: EmptyTab,
});

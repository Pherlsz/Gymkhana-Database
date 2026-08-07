import { createFileRoute } from "@tanstack/react-router";
import { OperationsPage, normalizeOperationsSearch } from "../OperationsPage";

export const Route = createFileRoute("/operations")({
  validateSearch: normalizeOperationsSearch,
  component: OperationsPage,
});

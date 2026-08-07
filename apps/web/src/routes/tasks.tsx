import { createFileRoute } from "@tanstack/react-router";
import { TaskPage, normalizeTaskSearch } from "../TaskPage";

export const Route = createFileRoute("/tasks")({
  validateSearch: normalizeTaskSearch,
  component: TaskPage,
});

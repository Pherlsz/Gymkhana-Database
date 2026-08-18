import { createFileRoute } from "@tanstack/react-router";
import { EmptyTab } from "../EmptyTab";

export const Route = createFileRoute("/settings")({
  component: EmptyTab,
});

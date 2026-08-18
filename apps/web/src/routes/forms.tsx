import { createFileRoute } from "@tanstack/react-router";
import { EmptyTab } from "../EmptyTab";

export const Route = createFileRoute("/forms")({
  component: EmptyTab,
});

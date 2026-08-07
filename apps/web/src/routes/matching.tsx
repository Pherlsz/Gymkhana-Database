import { createFileRoute } from "@tanstack/react-router";
import { MatchingPage, normalizeMatchingSearch } from "../MatchingPage";

export const Route = createFileRoute("/matching")({
  validateSearch: normalizeMatchingSearch,
  component: MatchingPage,
});

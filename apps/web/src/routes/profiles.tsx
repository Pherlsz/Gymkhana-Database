import { createFileRoute } from "@tanstack/react-router";
import { ProfilesPage, normalizeProfileSearch } from "../ProfilesPage";

export const Route = createFileRoute("/profiles")({
  validateSearch: normalizeProfileSearch,
  component: ProfilesPage,
});

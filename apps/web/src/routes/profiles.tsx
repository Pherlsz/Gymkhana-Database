import { createFileRoute, redirect } from "@tanstack/react-router";
import { normalizeProfileSearch } from "../ProfilesPage";
import { tableFromSection } from "../lib/tables/tableRoutes";

export const Route = createFileRoute("/profiles")({
  validateSearch: normalizeProfileSearch,
  beforeLoad: ({ search }) => {
    const { section, ...rest } = search;
    throw redirect({
      params: { table: tableFromSection(section) },
      search: rest,
      to: "/tables/$table",
    });
  },
});

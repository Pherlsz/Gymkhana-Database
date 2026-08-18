import { createFileRoute, redirect } from "@tanstack/react-router";
import { TABLE_SEARCH_DEFAULTS } from "../../lib/tables/tableRoutes";

export const Route = createFileRoute("/tables/")({
  beforeLoad: () => {
    throw redirect({
      params: { table: "people" },
      search: TABLE_SEARCH_DEFAULTS,
      to: "/tables/$table",
    });
  },
});

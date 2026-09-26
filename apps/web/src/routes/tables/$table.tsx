import { createFileRoute, redirect, stripSearchParams } from "@tanstack/react-router";
import { TablesPage } from "../../TablesPage";
import {
  isTableKind,
  normalizeTableSearch,
  TABLE_SEARCH_DEFAULTS,
} from "../../lib/tables/tableRoutes";

export const Route = createFileRoute("/tables/$table")({
  beforeLoad: ({ params }) => {
    if (!isTableKind(params.table)) {
      throw redirect({
        params: { table: "people" },
        search: TABLE_SEARCH_DEFAULTS,
        to: "/tables/$table",
      });
    }
  },
  validateSearch: normalizeTableSearch,
  search: {
    middlewares: [stripSearchParams(TABLE_SEARCH_DEFAULTS)],
  },
  component: TablesPage,
});

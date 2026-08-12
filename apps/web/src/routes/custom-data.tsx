import { createFileRoute } from "@tanstack/react-router";
import { CustomDataPage } from "../CustomDataPage";

export const Route = createFileRoute("/custom-data")({
  component: CustomDataPage,
});

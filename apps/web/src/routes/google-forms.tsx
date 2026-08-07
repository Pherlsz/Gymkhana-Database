import { createFileRoute } from "@tanstack/react-router";
import { GoogleFormsPage } from "../GoogleFormsPage";

export const Route = createFileRoute("/google-forms")({
  component: GoogleFormsPage,
});

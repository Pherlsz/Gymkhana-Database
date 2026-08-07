import { createRootRoute } from "@tanstack/react-router";
import { AuthenticatedShell } from "../App";

export const Route = createRootRoute({
  component: AuthenticatedShell,
});

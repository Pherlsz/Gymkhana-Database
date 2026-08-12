import { createRootRoute } from "@tanstack/react-router";
import { AuthenticatedShell } from "../AuthenticatedShell";

export const Route = createRootRoute({
  component: AuthenticatedShell,
});

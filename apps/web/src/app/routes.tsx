import { createRootRouteWithContext, createRoute, createRouter } from "@tanstack/react-router";
import { AIChatPage } from "../AIChatPage";
import { AttachmentsPage } from "../AttachmentsPage";
import { CustomDataPage } from "../CustomDataPage";
import { GoogleFormsPage } from "../GoogleFormsPage";
import { MatchingPage } from "../MatchingPage";
import { OCRPage } from "../OCRPage";
import { OperationsPage } from "../OperationsPage";
import { ProfilesPage, normalizeProfileSearch } from "../ProfilesPage";
import { QueryPage } from "../QueryPage";
import { SearchPage, normalizeSearchRoute } from "../SearchPage";
import { TaskWorkflowPage } from "../TaskWorkflowPage";
import { Dashboard } from "./dashboard";
import { RootLayout } from "./layout";
import { queryClient, type RouterContext } from "./router_context";

const rootRoute = createRootRouteWithContext<RouterContext>()({ component: RootLayout });
const indexRoute = createRoute({ getParentRoute: () => rootRoute, path: "/", component: Dashboard });

export const profilesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/profiles",
  validateSearch: normalizeProfileSearch,
  component: ProfilesPage,
});

export const searchRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/search",
  validateSearch: normalizeSearchRoute,
  component: SearchPage,
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  profilesRoute,
  searchRoute,
  createRoute({ getParentRoute: () => rootRoute, path: "/custom-data", component: CustomDataPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/attachments", component: AttachmentsPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/operations", component: OperationsPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/google-forms", component: GoogleFormsPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/query", component: QueryPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/matching", component: MatchingPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/chat", component: AIChatPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/ocr", component: OCRPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/tasks", component: TaskWorkflowPage }),
]);

export const router = createRouter({ routeTree, context: { queryClient } });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

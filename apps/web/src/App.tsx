import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { lazy, Suspense, useCallback, useEffect, useState } from "react";
import {
  APIRequestError,
  apiURL,
  getAuthSession,
  logout,
  type AuthSessionResponse,
} from "./lib/api/client";
import { createAppRouter } from "./router";
import { SessionContext, useApplicationSession } from "./session";
import { ThemeProvider } from "./theme";

const LoginScreen = lazy(() => import("./LoginScreen").then((m) => ({ default: m.LoginScreen })));

export { useApplicationSession };

type AuthState =
  | { kind: "checking" }
  | { kind: "authenticated"; session: AuthSessionResponse }
  | { kind: "unauthenticated" }
  | { kind: "disabled" }
  | { kind: "unavailable" };

export function App() {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: { queries: { staleTime: 15_000, retry: 1 } },
      }),
  );
  const [router] = useState(() => createAppRouter());
  const [authentication, setAuthentication] = useState<AuthState>({ kind: "checking" });
  const [signingOut, setSigningOut] = useState(false);

  const refreshAuthentication = useCallback(async (signal?: AbortSignal) => {
    setAuthentication((current) =>
      current.kind === "authenticated" ? current : { kind: "checking" },
    );
    try {
      setAuthentication({ kind: "authenticated", session: await getAuthSession(signal) });
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      if (error instanceof APIRequestError && error.status === 401) {
        setAuthentication({ kind: "unauthenticated" });
        return;
      }
      if (error instanceof APIRequestError && error.status === 503) {
        setAuthentication({ kind: "disabled" });
        return;
      }
      setAuthentication({ kind: "unavailable" });
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refreshAuthentication(controller.signal);
    return () => controller.abort();
  }, [refreshAuthentication]);

  const signOut = useCallback(async () => {
    setSigningOut(true);
    try {
      await logout();
      queryClient.clear();
      window.location.replace("/");
    } catch {
      setSigningOut(false);
      setAuthentication({ kind: "unavailable" });
    }
  }, [queryClient]);

  let tree;
  if (authentication.kind === "checking") {
    // Stay blank until the session cookie is known — never flash LoginScreen on F5/OAuth.
    // Authenticated routes own their own QueryView / AppCard loading once the shell mounts.
    tree = null;
  } else if (authentication.kind === "authenticated") {
    tree = (
      <QueryClientProvider client={queryClient}>
        <SessionContext.Provider
          value={{ session: authentication.session, signingOut, signOut: () => void signOut() }}
        >
          <RouterProvider router={router} />
        </SessionContext.Provider>
      </QueryClientProvider>
    );
  } else {
    tree = (
      <Suspense fallback={null}>
        <LoginScreen onLogin={() => window.location.assign(apiURL("/auth/login"))} />
      </Suspense>
    );
  }

  return <ThemeProvider forceDark={authentication.kind !== "authenticated"}>{tree}</ThemeProvider>;
}

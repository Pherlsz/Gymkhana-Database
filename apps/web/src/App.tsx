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
import {
  clearCachedAuthSession,
  readCachedAuthSession,
  writeCachedAuthSession,
} from "./lib/authSessionCache";
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

function initialAuthState(): AuthState {
  const cached = readCachedAuthSession();
  return cached ? { kind: "authenticated", session: cached } : { kind: "checking" };
}

export function App() {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: { queries: { staleTime: 15_000, retry: 1 } },
      }),
  );
  const [router] = useState(() => createAppRouter());
  const [authentication, setAuthentication] = useState<AuthState>(initialAuthState);
  const [signingOut, setSigningOut] = useState(false);

  const refreshAuthentication = useCallback(async (signal?: AbortSignal) => {
    try {
      const session = await getAuthSession(signal);
      writeCachedAuthSession(session);
      setAuthentication({ kind: "authenticated", session });
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      if (error instanceof APIRequestError && error.status === 401) {
        clearCachedAuthSession();
        setAuthentication({ kind: "unauthenticated" });
        return;
      }
      // Background revalidation failed while we already have a session — keep the shell;
      // only an explicit 401 (above) disconnects the user.
      setAuthentication((current) => {
        if (current.kind === "authenticated") return current;
        if (error instanceof APIRequestError && error.status === 503) {
          return { kind: "disabled" };
        }
        return { kind: "unavailable" };
      });
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
      clearCachedAuthSession();
      queryClient.clear();
      window.location.replace("/");
    } catch {
      setSigningOut(false);
      setAuthentication({ kind: "unavailable" });
    }
  }, [queryClient]);

  let tree;
  if (authentication.kind === "checking") {
    // Cold load with no cached session — stay blank until the cookie check returns.
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

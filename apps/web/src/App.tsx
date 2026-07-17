import {
  Alert,
  AppShell,
  Button,
  Inline,
  Page,
  Stack,
  StatusBadge,
  Surface,
} from "@pherlsz/gymkhana-ui";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  Link,
  Outlet,
  RouterProvider,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { AdminUsersPanel } from "./AdminUsersPanel";
import { CustomDataPage } from "./CustomDataPage";
import { GoogleFormsPage } from "./GoogleFormsPage";
import { MatchingPage, normalizeMatchingSearch } from "./MatchingPage";
import { ProfilesPage, normalizeProfileSearch } from "./ProfilesPage";
import { OperationsPage, normalizeOperationsSearch } from "./OperationsPage";
import { QueryPage } from "./QueryPage";
import { SearchPage, normalizeGlobalSearch } from "./SearchPage";
import {
  APIRequestError,
  apiURL,
  getAuthSession,
  logout,
  type AuthSessionResponse,
} from "./lib/api/client";
import { checkLiveHealth } from "./lib/api/health";
import { normalizeQuerySearch } from "./lib/queryState";

type HealthState = "checking" | "available" | "unavailable";
type AuthState =
  | { kind: "checking" }
  | { kind: "authenticated"; session: AuthSessionResponse }
  | { kind: "unauthenticated" }
  | { kind: "disabled" }
  | { kind: "unavailable" };

type ApplicationContextValue = {
  session: AuthSessionResponse;
  signingOut: boolean;
  signOut: () => void;
};
const SessionContext = createContext<ApplicationContextValue | null>(null);
export function useApplicationSession(): AuthSessionResponse {
  const value = useContext(SessionContext);
  if (!value) throw new Error("Application session is unavailable");
  return value.session;
}

const rootRoute = createRootRoute({ component: AuthenticatedShell });
const homeRoute = createRoute({ getParentRoute: () => rootRoute, path: "/", component: HomePage });
export const profilesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/profiles",
  validateSearch: normalizeProfileSearch,
  component: ProfilesPage,
});
const customDataRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/custom-data",
  component: CustomDataPage,
});
export const searchRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/search",
  validateSearch: normalizeGlobalSearch,
  component: SearchPage,
});
export const operationsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/operations",
  validateSearch: normalizeOperationsSearch,
  component: OperationsPage,
});
export const googleFormsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/google-forms",
  component: GoogleFormsPage,
});
export const queryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/query",
  validateSearch: normalizeQuerySearch,
  component: QueryPage,
});
export const matchingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/matching",
  validateSearch: normalizeMatchingSearch,
  component: MatchingPage,
});
const routeTree = rootRoute.addChildren([
  homeRoute,
  profilesRoute,
  searchRoute,
  customDataRoute,
  operationsRoute,
  googleFormsRoute,
  queryRoute,
  matchingRoute,
]);
const router = createRouter({ routeTree });
declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 15_000, retry: 1 } },
});

export function App() {
  const [health, setHealth] = useState<HealthState>("checking");
  const [authentication, setAuthentication] = useState<AuthState>({ kind: "checking" });
  const [signingOut, setSigningOut] = useState(false);

  const refreshHealth = useCallback(async (signal?: AbortSignal) => {
    setHealth("checking");
    try {
      setHealth((await checkLiveHealth(signal)) ? "available" : "unavailable");
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      setHealth("unavailable");
    }
  }, []);

  const refreshAuthentication = useCallback(async (signal?: AbortSignal) => {
    setAuthentication({ kind: "checking" });
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
    void refreshHealth(controller.signal);
    void refreshAuthentication(controller.signal);
    return () => controller.abort();
  }, [refreshAuthentication, refreshHealth]);

  const signOut = useCallback(async () => {
    setSigningOut(true);
    try {
      await logout();
      queryClient.clear();
      setAuthentication({ kind: "unauthenticated" });
    } catch {
      setAuthentication({ kind: "unavailable" });
    } finally {
      setSigningOut(false);
    }
  }, []);

  if (authentication.kind !== "authenticated") {
    return (
      <PublicShell
        health={health}
        authentication={authentication}
        signingOut={signingOut}
        onLogin={() => window.location.assign(apiURL("/auth/login"))}
        onRetry={() => void refreshAuthentication()}
        onSignOut={() => void signOut()}
        onRefreshHealth={() => void refreshHealth()}
      />
    );
  }

  return (
    <QueryClientProvider client={queryClient}>
      <SessionContext.Provider
        value={{ session: authentication.session, signingOut, signOut: () => void signOut() }}
      >
        <RouterProvider router={router} />
      </SessionContext.Provider>
    </QueryClientProvider>
  );
}

function AuthenticatedShell() {
  const context = useContext(SessionContext);
  if (!context) throw new Error("Application session is unavailable");
  const { session, signingOut, signOut } = context;
  return (
    <AppShell.Root>
      <AppShell.Header className="app-header">
        <Inline align="center">
          <strong>Gymkhana Database</strong>
          <nav aria-label="Navegação principal" className="app-nav">
            <Link
              activeProps={{ className: "app-nav__link app-nav__link--active" }}
              className="app-nav__link"
              to="/"
            >
              Início
            </Link>
            <Link
              activeProps={{ className: "app-nav__link app-nav__link--active" }}
              className="app-nav__link"
              to="/profiles"
              search={normalizeProfileSearch({})}
            >
              Pessoas
            </Link>
            <Link
              activeProps={{ className: "app-nav__link app-nav__link--active" }}
              className="app-nav__link"
              search={normalizeGlobalSearch({})}
              to="/search"
            >
              Buscar
            </Link>
            <Link
              activeProps={{ className: "app-nav__link app-nav__link--active" }}
              className="app-nav__link"
              search={normalizeQuerySearch({})}
              to="/query"
            >
              Consultar
            </Link>
            <Link
              activeProps={{ className: "app-nav__link app-nav__link--active" }}
              className="app-nav__link"
              search={normalizeMatchingSearch({})}
              to="/matching"
            >
              Duplicidades
            </Link>
            <Link
              activeProps={{ className: "app-nav__link app-nav__link--active" }}
              className="app-nav__link"
              to="/custom-data"
            >
              Dados personalizados
            </Link>
            <Link
              activeProps={{ className: "app-nav__link app-nav__link--active" }}
              className="app-nav__link"
              search={normalizeOperationsSearch({})}
              to="/operations"
            >
              Operações
            </Link>
            {canManageUsers(session.user.role) ? (
              <Link
                activeProps={{ className: "app-nav__link app-nav__link--active" }}
                className="app-nav__link"
                to="/google-forms"
              >
                Google Forms
              </Link>
            ) : null}
          </nav>
        </Inline>
        <Inline align="center">
          <span className="current-user">@{session.user.login}</span>
          <StatusBadge tone="success">{roleLabel(session.user.role)}</StatusBadge>
          <Button disabled={signingOut} onClick={signOut}>
            {signingOut ? "Saindo" : "Sair"}
          </Button>
        </Inline>
      </AppShell.Header>
      <AppShell.Main>
        <Outlet />
      </AppShell.Main>
    </AppShell.Root>
  );
}

function HomePage() {
  const session = useApplicationSession();
  return (
    <Page.Root maxWidth="lg">
      <Page.Header>
        <Page.Eyebrow>Aplicação privada</Page.Eyebrow>
        <Page.Title>Gymkhana Database</Page.Title>
        <Page.Description>
          Gerencie pessoas e permissões com sessões privadas e dados normalizados.
        </Page.Description>
      </Page.Header>
      <Page.Content>
        <Stack gap="6">
          <Surface className="authentication-panel" tone="raised">
            <Stack gap="3">
              <strong>{session.user.display_name}</strong>
              <span className="authentication-panel__description">
                @{session.user.login} · {roleLabel(session.user.role)}
              </span>
              <StatusBadge tone="success">Sessão ativa</StatusBadge>
            </Stack>
          </Surface>
          {canManageUsers(session.user.role) ? (
            <Page.Section
              description="Funções, acesso ativo e revogação de sessões são controlados pela aplicação."
              title="Administração de usuários"
            >
              <AdminUsersPanel currentLogin={session.user.login} />
            </Page.Section>
          ) : null}
          <Page.Section
            description="A infraestrutura compartilhada continua consumida somente por versões exatas."
            title="Foundation status"
          >
            <Inline align="stretch">
              <FoundationCard label="Frontend" value="React + TypeScript" />
              <FoundationCard label="Core" value="v0.2.1" />
              <FoundationCard label="UI" value="v0.3.0" />
            </Inline>
          </Page.Section>
        </Stack>
      </Page.Content>
    </Page.Root>
  );
}

function PublicShell(props: {
  health: HealthState;
  authentication: AuthState;
  signingOut: boolean;
  onLogin: () => void;
  onRetry: () => void;
  onSignOut: () => void;
  onRefreshHealth: () => void;
}) {
  return (
    <AppShell.Root>
      <AppShell.Header className="app-header">
        <strong>Gymkhana Database</strong>
        <StatusBadge tone={authenticationTone(props.authentication)}>Acesso privado</StatusBadge>
      </AppShell.Header>
      <AppShell.Main>
        <Page.Root maxWidth="lg">
          <Page.Header>
            <Page.Eyebrow>Private application access</Page.Eyebrow>
            <Page.Title>Gymkhana Database</Page.Title>
            <Page.Description>
              Acesso privado com GitHub, sessões revogáveis de 24 horas e permissões da aplicação.
            </Page.Description>
            <Page.Actions>
              <Button disabled={props.health === "checking"} onClick={props.onRefreshHealth}>
                Verificar API
              </Button>
            </Page.Actions>
          </Page.Header>
          <Page.Content>
            <Stack gap="6">
              {props.health === "unavailable" ? (
                <Alert title="API indisponível" tone="danger">
                  Verifique se o serviço está em execução e tente novamente.
                </Alert>
              ) : null}
              <AuthenticationPanel
                authentication={props.authentication}
                signingOut={props.signingOut}
                onLogin={props.onLogin}
                onRetry={props.onRetry}
                onSignOut={props.onSignOut}
              />
            </Stack>
          </Page.Content>
        </Page.Root>
      </AppShell.Main>
    </AppShell.Root>
  );
}

function AuthenticationPanel({
  authentication,
  signingOut,
  onLogin,
  onRetry,
  onSignOut,
}: {
  authentication: AuthState;
  signingOut: boolean;
  onLogin: () => void;
  onRetry: () => void;
  onSignOut: () => void;
}) {
  switch (authentication.kind) {
    case "checking":
      return <Alert title="Verificando acesso">Validando a sessão da aplicação.</Alert>;
    case "unauthenticated":
      return (
        <Surface className="authentication-panel" tone="raised">
          <Stack gap="4">
            <div>
              <strong>Autenticação necessária</strong>
              <p className="authentication-panel__description">
                Entre com uma conta GitHub previamente autorizada.
              </p>
            </div>
            <Inline>
              <Button onClick={onLogin}>Entrar com GitHub</Button>
            </Inline>
          </Stack>
        </Surface>
      );
    case "disabled":
      return (
        <Alert title="Autenticação desativada neste ambiente" tone="info">
          Configure as variáveis OAuth para testar o acesso privado localmente.
        </Alert>
      );
    case "unavailable":
      return (
        <Alert title="Não foi possível verificar a sessão" tone="danger">
          <Stack gap="3">
            <span>Tente novamente sem recarregar a página.</span>
            <Inline>
              <Button onClick={onRetry}>Tentar novamente</Button>
            </Inline>
          </Stack>
        </Alert>
      );
    case "authenticated":
      return (
        <Surface className="authentication-panel" tone="raised">
          <Stack gap="4">
            <strong>{authentication.session.user.display_name}</strong>
            <Inline>
              <StatusBadge tone="success">Sessão ativa</StatusBadge>
              <Button disabled={signingOut} onClick={onSignOut}>
                {signingOut ? "Saindo" : "Sair"}
              </Button>
            </Inline>
          </Stack>
        </Surface>
      );
  }
}
function FoundationCard({ label, value }: { label: string; value: string }) {
  return (
    <Surface className="foundation-card" tone="raised">
      <Stack gap="2">
        <span className="foundation-card__label">{label}</span>
        <strong>{value}</strong>
      </Stack>
    </Surface>
  );
}
function canManageUsers(role: "MEMBER" | "ADMIN" | "SUPERADMIN") {
  return role === "ADMIN" || role === "SUPERADMIN";
}
function roleLabel(role: "MEMBER" | "ADMIN" | "SUPERADMIN") {
  return role === "SUPERADMIN" ? "Superadmin" : role === "ADMIN" ? "Admin" : "Membro";
}
function authenticationTone(authentication: AuthState): "neutral" | "success" | "danger" | "info" {
  return authentication.kind === "authenticated"
    ? "success"
    : authentication.kind === "unavailable"
      ? "danger"
      : authentication.kind === "unauthenticated"
        ? "info"
        : "neutral";
}

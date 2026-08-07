import { Alert, Button, Card, Flex, Layout, Tag, Typography } from "antd";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { useCallback, useEffect, useState } from "react";
import { LoginScreen } from "./LoginScreen";
import {
  APIRequestError,
  apiURL,
  getAuthSession,
  logout,
  type AuthSessionResponse,
} from "./lib/api/client";
import { checkLiveHealth } from "./lib/api/health";
import { router } from "./router";
import { SessionContext } from "./session";

type HealthState = "checking" | "available" | "unavailable";
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
  }, [queryClient]);

  if (authentication.kind === "unauthenticated") {
    return <LoginScreen onLogin={() => window.location.assign(apiURL("/auth/login"))} />;
  }

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
    <Layout>
      <Layout.Header className="app-header">
        <strong>Gymkhana Database</strong>
        <Tag color={authenticationTone(props.authentication)}>Acesso privado</Tag>
      </Layout.Header>
      <Layout.Content>
        <Layout style={{ maxWidth: "64rem", margin: "0 auto" }}>
          <header className="page-header">
            <div className="page-eyebrow">Private application access</div>
            <Typography.Title level={1} className="page-title">
              Gymkhana Database
            </Typography.Title>
            <Typography.Paragraph className="page-description">
              Acesso privado com Google, sessões revogáveis de 24 horas e permissões da aplicação.
            </Typography.Paragraph>
            <div className="page-actions">
              <Button disabled={props.health === "checking"} onClick={props.onRefreshHealth}>
                Verificar API
              </Button>
            </div>
          </header>
          <div className="page-content">
            <Flex vertical gap="1.5rem">
              {props.health === "unavailable" ? (
                <Alert
                  message="API indisponível"
                  type="error"
                  description="Verifique se o serviço está em execução e tente novamente."
                />
              ) : null}
              <AuthenticationPanel
                authentication={props.authentication}
                signingOut={props.signingOut}
                onLogin={props.onLogin}
                onRetry={props.onRetry}
                onSignOut={props.onSignOut}
              />
            </Flex>
          </div>
        </Layout>
      </Layout.Content>
    </Layout>
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
      return (
        <Alert
          message="Verificando acesso"
          description="Validando a sessão da aplicação."
          type="info"
        />
      );
    case "unauthenticated":
      return (
        <Card className="authentication-panel" style={{ padding: "1rem" }}>
          <Flex vertical gap="1rem">
            <div>
              <strong>Autenticação necessária</strong>
              <p className="authentication-panel__description">
                Entre com uma conta Google previamente autorizada.
              </p>
            </div>
            <Flex gap="0.5rem">
              <Button onClick={onLogin}>Entrar com Google</Button>
            </Flex>
          </Flex>
        </Card>
      );
    case "disabled":
      return (
        <Alert
          message="Autenticação desativada neste ambiente"
          description="Configure as variáveis OAuth para testar o acesso privado localmente."
          type="info"
        />
      );
    case "unavailable":
      return (
        <Alert
          message="Não foi possível verificar a sessão"
          type="error"
          description={
            <Flex vertical gap="0.75rem">
              <span>Tente novamente sem recarregar a página.</span>
              <Flex gap="0.5rem">
                <Button onClick={onRetry}>Tentar novamente</Button>
              </Flex>
            </Flex>
          }
        />
      );
    case "authenticated":
      return (
        <Card className="authentication-panel" style={{ padding: "1rem" }}>
          <Flex vertical gap="1rem">
            <strong>{authentication.session.user.display_name}</strong>
            <Flex gap="0.5rem">
              <Tag color="success">Sessão ativa</Tag>
              <Button disabled={signingOut} onClick={onSignOut}>
                {signingOut ? "Saindo" : "Sair"}
              </Button>
            </Flex>
          </Flex>
        </Card>
      );
  }
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

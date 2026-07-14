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
import { useCallback, useEffect, useState } from "react";
import {
  APIRequestError,
  apiURL,
  getAuthSession,
  logout,
  type AuthSessionResponse,
} from "./lib/api/client";
import { checkLiveHealth } from "./lib/api/health";

type HealthState = "checking" | "available" | "unavailable";
type AuthState =
  | { kind: "checking" }
  | { kind: "authenticated"; session: AuthSessionResponse }
  | { kind: "unauthenticated" }
  | { kind: "disabled" }
  | { kind: "unavailable" };

export function App() {
  const [health, setHealth] = useState<HealthState>("checking");
  const [authentication, setAuthentication] = useState<AuthState>({ kind: "checking" });
  const [signingOut, setSigningOut] = useState(false);

  const refreshHealth = useCallback(async (signal?: AbortSignal) => {
    setHealth("checking");

    try {
      const available = await checkLiveHealth(signal);
      setHealth(available ? "available" : "unavailable");
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name === "AbortError") {
        return;
      }
      setHealth("unavailable");
    }
  }, []);

  const refreshAuthentication = useCallback(async (signal?: AbortSignal) => {
    setAuthentication({ kind: "checking" });
    try {
      const session = await getAuthSession(signal);
      setAuthentication({ kind: "authenticated", session });
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name === "AbortError") {
        return;
      }
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
      setAuthentication({ kind: "unauthenticated" });
    } catch {
      setAuthentication({ kind: "unavailable" });
    } finally {
      setSigningOut(false);
    }
  }, []);

  return (
    <AppShell.Root>
      <AppShell.Header className="app-header">
        <strong>Gymkhana Database</strong>
        <Inline align="center">
          {authentication.kind === "authenticated" ? (
            <span className="current-user">@{authentication.session.user.login}</span>
          ) : null}
          <StatusBadge tone={authenticationTone(authentication)}>M2 authentication</StatusBadge>
        </Inline>
      </AppShell.Header>
      <AppShell.Main>
        <Page.Root maxWidth="lg">
          <Page.Header>
            <Page.Eyebrow>Private application access</Page.Eyebrow>
            <Page.Title>Gymkhana Database</Page.Title>
            <Page.Description>
              Acesso privado com GitHub, sessões revogáveis de 24 horas e permissões vinculadas ao
              usuário da aplicação.
            </Page.Description>
            <Page.Actions>
              <Button
                aria-describedby="api-status"
                disabled={health === "checking"}
                onClick={() => void refreshHealth()}
              >
                Verificar API
              </Button>
            </Page.Actions>
          </Page.Header>

          <Page.Content>
            <Stack gap="6">
              {health === "unavailable" ? (
                <Alert title="API indisponível" tone="danger">
                  Verifique se o serviço está em execução e tente novamente.
                </Alert>
              ) : null}

              <AuthenticationPanel
                authentication={authentication}
                signingOut={signingOut}
                onLogin={() => window.location.assign(apiURL("/auth/login"))}
                onRetry={() => void refreshAuthentication()}
                onSignOut={() => void signOut()}
              />

              <Page.Section
                description="A infraestrutura compartilhada continua consumida somente por versões exatas."
                title="Foundation status"
              >
                <Inline align="stretch">
                  <FoundationCard label="Frontend" value="React + TypeScript" />
                  <FoundationCard label="Core" value="v0.2.1" />
                  <FoundationCard label="UI" value="v0.3.0" />
                  <Surface className="foundation-card" tone="raised">
                    <Stack gap="2">
                      <span className="foundation-card__label">API</span>
                      <StatusBadge id="api-status" tone={healthTone(health)}>
                        {healthLabel(health)}
                      </StatusBadge>
                    </Stack>
                  </Surface>
                </Inline>
              </Page.Section>
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
                Entre com uma conta GitHub previamente autorizada para acessar os módulos privados.
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
            <div>
              <strong>{authentication.session.user.display_name}</strong>
              <p className="authentication-panel__description">
                @{authentication.session.user.login} · {roleLabel(authentication.session.user.role)}
              </p>
            </div>
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

function authenticationTone(authentication: AuthState): "neutral" | "success" | "danger" | "info" {
  switch (authentication.kind) {
    case "authenticated":
      return "success";
    case "unavailable":
      return "danger";
    case "unauthenticated":
      return "info";
    default:
      return "neutral";
  }
}

function roleLabel(role: "MEMBER" | "ADMIN" | "SUPERADMIN"): string {
  switch (role) {
    case "SUPERADMIN":
      return "Superadmin";
    case "ADMIN":
      return "Admin";
    default:
      return "Membro";
  }
}

function healthTone(health: HealthState): "neutral" | "success" | "danger" {
  switch (health) {
    case "available":
      return "success";
    case "unavailable":
      return "danger";
    default:
      return "neutral";
  }
}

function healthLabel(health: HealthState): string {
  switch (health) {
    case "available":
      return "Disponível";
    case "unavailable":
      return "Indisponível";
    default:
      return "Verificando";
  }
}

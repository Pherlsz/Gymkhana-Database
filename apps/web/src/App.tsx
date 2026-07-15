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
  listUsers,
  logout,
  updateUserAccess,
  type AuthSessionResponse,
  type ManagedUser,
  type UserAccessUpdate,
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
          <StatusBadge tone={authenticationTone(authentication)}>M2 authorization</StatusBadge>
        </Inline>
      </AppShell.Header>
      <AppShell.Main>
        <Page.Root maxWidth="lg">
          <Page.Header>
            <Page.Eyebrow>Private application access</Page.Eyebrow>
            <Page.Title>Gymkhana Database</Page.Title>
            <Page.Description>
              Acesso privado com GitHub, sessões revogáveis e permissões centralizadas por papel.
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

              {authentication.kind === "authenticated" &&
              authentication.session.capabilities.manage_users ? (
                <UserAdministration currentLogin={authentication.session.user.login} />
              ) : null}

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

function UserAdministration({ currentLogin }: { currentLogin: string }) {
  const [users, setUsers] = useState<ManagedUser[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");

  const refresh = useCallback(async (signal?: AbortSignal) => {
    setState("loading");
    try {
      setUsers(await listUsers(signal));
      setState("ready");
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name === "AbortError") {
        return;
      }
      setState("error");
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal);
    return () => controller.abort();
  }, [refresh]);

  const replaceUser = useCallback((updated: ManagedUser) => {
    setUsers((current) => current.map((user) => (user.id === updated.id ? updated : user)));
  }, []);

  return (
    <Page.Section
      description="Administradores podem alterar membros e administradores. O superadmin e a própria conta permanecem protegidos."
      title="Administração de usuários"
    >
      <Stack gap="4">
        {state === "loading" ? <Alert title="Carregando usuários">Aguarde.</Alert> : null}
        {state === "error" ? (
          <Alert title="Não foi possível carregar os usuários" tone="danger">
            <Inline>
              <Button onClick={() => void refresh()}>Tentar novamente</Button>
            </Inline>
          </Alert>
        ) : null}
        {state === "ready" && users.length === 0 ? (
          <Alert title="Nenhum usuário encontrado">A lista está vazia.</Alert>
        ) : null}
        {state === "ready"
          ? users.map((user) => (
              <ManagedUserEditor
                key={user.id}
                current={user.login === currentLogin}
                onUpdated={replaceUser}
                user={user}
              />
            ))
          : null}
      </Stack>
    </Page.Section>
  );
}

function ManagedUserEditor({
  current,
  onUpdated,
  user,
}: {
  current: boolean;
  onUpdated: (updated: ManagedUser) => void;
  user: ManagedUser;
}) {
  const [role, setRole] = useState<"MEMBER" | "ADMIN">(
    user.role === "ADMIN" ? "ADMIN" : "MEMBER",
  );
  const [active, setActive] = useState(user.active);
  const [saving, setSaving] = useState(false);
  const [failed, setFailed] = useState(false);
  const locked = user.protected || current;

  useEffect(() => {
    setRole(user.role === "ADMIN" ? "ADMIN" : "MEMBER");
    setActive(user.active);
  }, [user]);

  const save = useCallback(async () => {
    setSaving(true);
    setFailed(false);
    try {
      const update: UserAccessUpdate = { role, active, version: user.version };
      onUpdated(await updateUserAccess(user.id, update));
    } catch {
      setFailed(true);
    } finally {
      setSaving(false);
    }
  }, [active, onUpdated, role, user.id, user.version]);

  const changed = role !== user.role || active !== user.active;

  return (
    <Surface className="managed-user" tone="raised">
      <Stack gap="4">
        <Inline align="center" justify="space-between">
          <div>
            <strong>{user.display_name}</strong>
            <p className="managed-user__identity">@{user.login}</p>
          </div>
          <Inline align="center">
            {current ? <StatusBadge tone="info">Sua conta</StatusBadge> : null}
            {user.protected ? <StatusBadge tone="neutral">Protegido</StatusBadge> : null}
            <StatusBadge tone={user.active ? "success" : "neutral"}>
              {user.active ? "Ativo" : "Inativo"}
            </StatusBadge>
          </Inline>
        </Inline>

        <div className="managed-user__controls">
          <label>
            <span>Papel</span>
            <select
              disabled={locked || saving}
              onChange={(event) => setRole(event.target.value as "MEMBER" | "ADMIN")}
              value={role}
            >
              <option value="MEMBER">Membro</option>
              <option value="ADMIN">Admin</option>
            </select>
          </label>
          <label className="managed-user__checkbox">
            <input
              checked={active}
              disabled={locked || saving}
              onChange={(event) => setActive(event.target.checked)}
              type="checkbox"
            />
            <span>Acesso ativo</span>
          </label>
          <Button disabled={locked || saving || !changed} onClick={() => void save()}>
            {saving ? "Salvando" : "Salvar acesso"}
          </Button>
        </div>

        {failed ? (
          <Alert title="Alteração não salva" tone="danger">
            Recarregue a lista e tente novamente.
          </Alert>
        ) : null}
      </Stack>
    </Surface>
  );
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

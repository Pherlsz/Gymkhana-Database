import { Alert, Button, Inline, Stack, StatusBadge, Surface } from "@pherlsz/gymkhana-ui";
import { useCallback, useEffect, useState } from "react";
import {
  APIRequestError,
  listApplicationUsers,
  updateApplicationUserAccess,
  type AdminUser,
  type UserRole,
} from "./lib/api/client";

type LoadState =
  | { kind: "loading" }
  | { kind: "ready"; users: AdminUser[] }
  | { kind: "error"; message: string };

export function AdminUsersPanel({ currentLogin }: { currentLogin: string }) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });

  const refresh = useCallback(async (signal?: AbortSignal) => {
    setState({ kind: "loading" });
    try {
      const response = await listApplicationUsers(signal);
      setState({ kind: "ready", users: response.users });
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      setState({ kind: "error", message: errorMessage(error) });
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal);
    return () => controller.abort();
  }, [refresh]);

  if (state.kind === "loading") {
    return <Alert title="Carregando usuários">Consultando acessos da aplicação.</Alert>;
  }
  if (state.kind === "error") {
    return (
      <Alert title="Não foi possível carregar usuários" tone="danger">
        <Stack gap="3">
          <span>{state.message}</span>
          <Inline>
            <Button onClick={() => void refresh()}>Tentar novamente</Button>
          </Inline>
        </Stack>
      </Alert>
    );
  }

  return (
    <Stack gap="4">
      {state.users.map((user) => (
        <ManagedUserCard
          current={user.login === currentLogin}
          key={user.id}
          user={user}
          onUpdated={(updated) =>
            setState((current) =>
              current.kind === "ready"
                ? {
                    kind: "ready",
                    users: current.users.map((item) => (item.id === updated.id ? updated : item)),
                  }
                : current,
            )
          }
        />
      ))}
    </Stack>
  );
}

function ManagedUserCard({
  current,
  user,
  onUpdated,
}: {
  current: boolean;
  user: AdminUser;
  onUpdated: (user: AdminUser) => void;
}) {
  const [role, setRole] = useState<UserRole>(user.role);
  const [active, setActive] = useState(user.active);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const protectedAccount = user.role === "SUPERADMIN";
  const locked = current || protectedAccount;

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      const updated = await updateApplicationUserAccess(user.id, {
        role,
        active,
        version: user.version,
      });
      onUpdated(updated);
      setRole(updated.role);
      setActive(updated.active);
    } catch (cause: unknown) {
      setError(errorMessage(cause));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Surface className="managed-user" tone="raised">
      <Stack gap="4">
        <Inline align="center" justify="between">
          <div>
            <strong>{user.display_name}</strong>
            <p className="managed-user__secondary">@{user.login}</p>
          </div>
          <Inline align="center">
            {current ? <StatusBadge tone="info">Sua conta</StatusBadge> : null}
            {protectedAccount ? <StatusBadge tone="neutral">Protegido</StatusBadge> : null}
            <StatusBadge tone={user.active ? "success" : "neutral"}>
              {user.active ? "Ativo" : "Inativo"}
            </StatusBadge>
          </Inline>
        </Inline>

        <Inline align="end">
          <label className="managed-user__field">
            <span>Função</span>
            <select
              disabled={locked || saving}
              onChange={(event) => setRole(event.target.value as UserRole)}
              value={role}
            >
              <option value="EXTERNAL">Membro</option>
              <option value="ADMIN">Admin</option>
              <option disabled value="SUPERADMIN">
                Superadmin
              </option>
            </select>
          </label>
          <label className="managed-user__toggle">
            <input
              checked={active}
              disabled={locked || saving}
              onChange={(event) => setActive(event.target.checked)}
              type="checkbox"
            />
            Acesso ativo
          </label>
          <Button
            disabled={locked || saving || (role === user.role && active === user.active)}
            onClick={() => void save()}
          >
            {saving ? "Salvando" : "Salvar acesso"}
          </Button>
        </Inline>

        {current ? (
          <span className="managed-user__secondary">
            Sua própria função e acesso não podem ser alterados por esta tela.
          </span>
        ) : null}
        {protectedAccount ? (
          <span className="managed-user__secondary">
            O superadmin único permanece protegido contra alterações de acesso.
          </span>
        ) : null}
        {error ? (
          <Alert title="Alteração não aplicada" tone="danger">
            {error}
          </Alert>
        ) : null}
      </Stack>
    </Surface>
  );
}

function errorMessage(error: unknown): string {
  if (error instanceof APIRequestError) return error.message;
  return "O serviço de administração não respondeu.";
}

import { Alert, Button, Card, Flex, Tag } from "antd";
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
    return (
      <Alert
        message="Carregando usuários"
        type="info"
        description="Consultando acessos da aplicação."
      />
    );
  }
  if (state.kind === "error") {
    return (
      <Alert
        message="Não foi possível carregar usuários"
        type="error"
        description={
          <>
            <Flex vertical gap="0.75rem">
              <span>{state.message}</span>
              <Flex>
                <Button onClick={() => void refresh()}>Tentar novamente</Button>
              </Flex>
            </Flex>
          </>
        }
      />
    );
  }

  return (
    <Flex vertical gap="1rem">
      {state.users?.map((user) => (
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
    </Flex>
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
    <Card className="managed-user">
      <Flex vertical gap="1rem">
        <Flex align="center" justify="space-between">
          <div>
            <strong>{user.display_name}</strong>
            <p className="managed-user__secondary">@{user.login}</p>
          </div>
          <Flex align="center">
            {current ? <Tag color="info">Sua conta</Tag> : null}
            {protectedAccount ? <Tag color="neutral">Protegido</Tag> : null}
            <Tag color={user.active ? "success" : "neutral"}>
              {user.active ? "Ativo" : "Inativo"}
            </Tag>
          </Flex>
        </Flex>

        <Flex align="end">
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
        </Flex>

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
          <Alert message="Alteração não aplicada" type="error" description={<>{error}</>} />
        ) : null}
      </Flex>
    </Card>
  );
}

function errorMessage(error: unknown): string {
  if (error instanceof APIRequestError) return error.message;
  return "O serviço de administração não respondeu.";
}

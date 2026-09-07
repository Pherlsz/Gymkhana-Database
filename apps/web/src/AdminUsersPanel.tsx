import { Alert, Button, Card, Checkbox, Flex, Select, Tag } from "antd";
import { useCallback, useEffect, useState } from "react";
import { useI18n } from "./i18n";
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
  const { messages } = useI18n();
  const copy = messages.admin.users;

  const refresh = useCallback(async (signal?: AbortSignal) => {
    setState({ kind: "loading" });
    try {
      const response = await listApplicationUsers(signal);
      setState({ kind: "ready", users: response.users });
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      setState({ kind: "error", message: errorMessage(error, copy.loadErrorMessage) });
    }
  }, [copy.loadErrorMessage]);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal);
    return () => controller.abort();
  }, [refresh]);

  if (state.kind === "loading") {
    return (
      <Alert
        message={copy.loadingTitle}
        type="info"
        description={copy.loadingDescription}
      />
    );
  }
  if (state.kind === "error") {
    return (
      <Alert
        message={copy.errorTitle}
        type="error"
        description={
          <>
            <Flex vertical gap="0.75rem">
              <span>{state.message}</span>
              <Flex>
                <Button onClick={() => void refresh()}>{copy.retry}</Button>
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
  const { messages } = useI18n();
  const copy = messages.admin.users;
  const [role, setRole] = useState<UserRole>(user.role);
  const [active, setActive] = useState(user.active);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const protectedAccount = user.role === "SUPERADMIN";
  const locked = current || protectedAccount;
  const isDirty = role !== user.role || active !== user.active;

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
      setError(errorMessage(cause, copy.loadErrorMessage));
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
            {current ? <Tag color="info">{copy.currentAccount}</Tag> : null}
            {protectedAccount ? <Tag color="neutral">{copy.protected}</Tag> : null}
            <Tag color={user.active ? "success" : "neutral"}>
              {user.active ? copy.active : copy.inactive}
            </Tag>
          </Flex>
        </Flex>

        <Flex align="end">
          <label className="managed-user__field">
            <span>{copy.roleLabel}</span>
            <Select
              disabled={locked || saving}
              onChange={(value) => setRole(value as UserRole)}
              options={[
                { value: "EXTERNAL", label: copy.roleMember },
                { value: "ADMIN", label: copy.roleAdmin },
                { value: "SUPERADMIN", label: copy.roleSuperadmin, disabled: true },
              ]}
              style={{ minWidth: 140 }}
              value={role}
            />
          </label>
          <Checkbox
            checked={active}
            className="managed-user__toggle"
            disabled={locked || saving}
            onChange={(event) => setActive(event.target.checked)}
          >
            {copy.activeAccessToggle}
          </Checkbox>
          <Button
            disabled={locked || saving || !isDirty}
            onClick={() => void save()}
            type={isDirty ? "primary" : "default"}
          >
            {saving ? copy.saving : copy.saveAccess}
          </Button>
        </Flex>

        {current ? (
          <span className="managed-user__secondary">
            {copy.currentAccountNotice}
          </span>
        ) : null}
        {protectedAccount ? (
          <span className="managed-user__secondary">
            {copy.superadminNotice}
          </span>
        ) : null}
        {error ? (
          <Alert message={copy.updateErrorTitle} type="error" description={<>{error}</>} />
        ) : null}
      </Flex>
    </Card>
  );
}

function errorMessage(error: unknown, fallbackMessage: string): string {
  if (error instanceof APIRequestError) return error.message;
  return fallbackMessage;
}

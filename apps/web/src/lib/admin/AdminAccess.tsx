import { Button, Input, Popconfirm, Select, Switch, Table } from "antd";
import type { TableProps } from "antd";
import { useQueries } from "@tanstack/react-query";
import { useMemo, useRef, useState, type FormEvent, type MouseEvent } from "react";
import { QueryView } from "../../components/QueryView";
import { StatusBanner } from "../../components/StatusBanner";
import { useI18n } from "../../i18n";
import { listUserCapabilities, type AdminUser } from "../api/client";
import {
  DEFAULT_MEMBER_CAPABILITIES,
  GRANTABLE_CAPABILITIES,
  accessLock,
  assignableRole,
  canEditUserAccess,
  matchesAdminUserQuery,
  memberAccessIsComplete,
  type GrantableCapability,
} from "./access";
import { adminCapabilitiesKey, useAdminAccess } from "./useAdminAccess";

type DraftRole = "EXTERNAL" | "ADMIN";

export function AdminUsers({ actorLogin }: { actorLogin: string }) {
  const { messages, t, plural } = useI18n();
  const copy = messages.admin.users;
  const actions = messages.common.actions;
  const access = useAdminAccess();
  const users = access.users.data?.users ?? [];
  const [editingId, setEditingId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [newRole, setNewRole] = useState<DraftRole>("EXTERNAL");
  const [active, setActive] = useState(true);
  const [capabilities, setCapabilities] = useState<GrantableCapability[]>([
    ...DEFAULT_MEMBER_CAPABILITIES,
  ]);
  const [granted, setGranted] = useState<GrantableCapability[]>([]);
  const [formError, setFormError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [capsLoading, setCapsLoading] = useState(false);
  const [bulkBusy, setBulkBusy] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const saving = access.provision.isPending || access.updateAccess.isPending || bulkBusy;
  const canSubmit =
    name.trim() !== "" && email.trim() !== "" && memberAccessIsComplete(newRole, capabilities);
  const createDirty =
    !editingId &&
    (name.trim() !== "" ||
      email.trim() !== "" ||
      newRole !== "EXTERNAL" ||
      !sameCapabilities(capabilities, DEFAULT_MEMBER_CAPABILITIES));
  const members = users.filter((user) => user.role === "EXTERNAL");
  const capabilityQueries = useQueries({
    queries: members.map((user) => ({
      queryKey: adminCapabilitiesKey(user.id),
      queryFn: ({ signal }: { signal: AbortSignal }) => listUserCapabilities(user.id, signal),
    })),
  });
  const grantedByUser = new Map(
    members.map((user, index) => [
      user.id,
      (capabilityQueries[index]?.data ?? []).filter(isGrantable),
    ]),
  );
  const visibleUsers = useMemo(
    () => users.filter((user) => matchesAdminUserQuery(user, query, [roleName(user.role, copy)])),
    [copy, query, users],
  );
  const selectedUsers = users.filter(
    (user) => selectedIds.includes(user.id) && canEditUserAccess(actorLogin, user),
  );

  function resetForm() {
    setEditingId(null);
    setName("");
    setEmail("");
    setNewRole("EXTERNAL");
    setActive(true);
    setCapabilities([...DEFAULT_MEMBER_CAPABILITIES]);
    setGranted([]);
    setFormError(null);
  }

  async function beginEdit(user: AdminUser) {
    if (!canEditUserAccess(actorLogin, user)) return;
    const role = assignableRole(user.role) ?? "EXTERNAL";
    setEditingId(user.id);
    setName(user.display_name);
    setEmail(user.login);
    setNewRole(role);
    setActive(user.active);
    setFormError(null);
    setNotice(null);
    if (role !== "EXTERNAL") {
      setCapabilities([...DEFAULT_MEMBER_CAPABILITIES]);
      setGranted([]);
    } else {
      setCapsLoading(true);
      try {
        const current = (await listUserCapabilities(user.id)).filter(isGrantable);
        setCapabilities(current);
        setGranted(current);
      } catch {
        setCapabilities([]);
        setGranted([]);
        setFormError(copy.capabilitiesLoadError);
      } finally {
        setCapsLoading(false);
      }
    }
    formRef.current?.scrollIntoView({ block: "nearest" });
    document.getElementById("admin-user-name")?.focus();
  }

  async function submitUser(event: FormEvent) {
    event.preventDefault();
    const displayName = name.trim();
    const login = email.trim().toLowerCase();
    if (!displayName || !login) return;
    if (!memberAccessIsComplete(newRole, capabilities)) {
      setFormError(copy.capabilitiesRequired);
      return;
    }
    setFormError(null);
    const selected = newRole === "EXTERNAL" ? capabilities : [];
    if (!editingId) {
      access.provision.mutate(
        { display_name: displayName, email: login, role: newRole, capabilities: selected },
        {
          onSuccess: () => {
            resetForm();
            setNotice(copy.added);
          },
        },
      );
      return;
    }
    const user = users.find((item) => item.id === editingId);
    if (!user) return;
    try {
      await access.updateAccess.mutateAsync({
        userId: user.id,
        role: newRole,
        active,
        version: user.version,
        display_name: displayName,
        email: login,
      });
      if (newRole === "EXTERNAL") {
        const had = new Set(granted);
        for (const capability of GRANTABLE_CAPABILITIES) {
          const want = selected.includes(capability);
          if (want === had.has(capability)) continue;
          await access.toggleCapability.mutateAsync({
            userId: user.id,
            capability,
            granted: had.has(capability),
          });
        }
      }
      resetForm();
      setNotice(copy.savedUser);
    } catch {
      // The mutation error is shown below.
    }
  }

  async function setUserActive(user: AdminUser, next: boolean) {
    const role = assignableRole(user.role);
    if (!role || !canEditUserAccess(actorLogin, user) || user.active === next) return;
    if (editingId === user.id) setActive(next);
    await access.updateAccess.mutateAsync({
      userId: user.id,
      role,
      active: next,
      version: user.version,
      display_name: user.display_name,
      email: user.login,
    });
  }

  async function applySelectedActive(next: boolean) {
    setBulkBusy(true);
    try {
      for (const user of selectedUsers) {
        await setUserActive(user, next);
      }
      setSelectedIds([]);
    } catch {
      // Mutation error is shown below.
    } finally {
      setBulkBusy(false);
    }
  }

  async function deleteSelected() {
    setBulkBusy(true);
    try {
      for (const user of selectedUsers) {
        await access.removeUser.mutateAsync(user.id);
        if (editingId === user.id) resetForm();
      }
      setSelectedIds([]);
    } catch {
      // Mutation error is shown below.
    } finally {
      setBulkBusy(false);
    }
  }

  const columns: TableProps<AdminUser>["columns"] = [
    {
      title: copy.name,
      dataIndex: "display_name",
      render: (value: string) => (
        <span className="admin-user-cell" title={value}>
          {value}
        </span>
      ),
    },
    {
      title: copy.email,
      dataIndex: "login",
      render: (value: string) => (
        <span className="admin-user-cell" title={value} translate="no">
          {value}
        </span>
      ),
    },
    {
      title: copy.roleLabel,
      dataIndex: "role",
      width: 140,
      render: (role: AdminUser["role"]) => roleName(role, copy),
    },
    {
      title: copy.capabilitiesTitle,
      key: "capabilities",
      render: (_value, user) => {
        if (user.role !== "EXTERNAL") return "\u2014";
        const queryState = capabilityQueries[members.findIndex((item) => item.id === user.id)];
        if (queryState?.isPending) return messages.common.status.loading;
        const caps = grantedByUser.get(user.id) ?? [];
        if (caps.length === 0) return "\u2014";
        const labels = caps.map((capability) => copy.capabilities[capability]);
        return (
          <span className="admin-user-cell" title={labels.join(", ")}>
            {plural(
              caps.length,
              t(copy.capabilitiesCountOne, { count: caps.length }),
              t(copy.capabilitiesCountOther, { count: caps.length }),
            )}
          </span>
        );
      },
    },
    {
      title: copy.activeAccessToggle,
      dataIndex: "active",
      onHeaderCell: () => ({ style: { whiteSpace: "nowrap" } }),
      width: 132,
      render: (value: boolean, user) => {
        if (!canEditUserAccess(actorLogin, user)) return value ? copy.active : copy.inactive;
        const toggling =
          access.updateAccess.isPending && access.updateAccess.variables?.userId === user.id;
        return (
          <Switch
            aria-label={copy.activeAccessToggle}
            checked={value}
            loading={toggling}
            onChange={(checked) => void setUserActive(user, checked)}
            onClick={(_checked, event) => event.stopPropagation()}
          />
        );
      },
    },
    {
      title: copy.actionsColumn,
      key: "actions",
      width: 180,
      render: (_value, user) => {
        const lock = accessLock(actorLogin, user);
        if (lock) {
          return (
            <span className="admin-lock">
              {lock === "self" ? copy.currentAccount : copy.protected}
            </span>
          );
        }
        const deleting = access.removeUser.isPending && access.removeUser.variables === user.id;
        return (
          <span className="admin-row-actions">
            <Button
              type="link"
              onClick={(event) => {
                event.stopPropagation();
                void beginEdit(user);
              }}
            >
              {actions.edit}
            </Button>
            <Popconfirm
              cancelText={actions.cancel}
              okButtonProps={{ danger: true }}
              okText={actions.delete}
              title={copy.deleteConfirm}
              onConfirm={() =>
                access.removeUser.mutate(user.id, {
                  onSuccess: () => {
                    setSelectedIds((current) => current.filter((id) => id !== user.id));
                    if (editingId === user.id) resetForm();
                  },
                })
              }
            >
              <Button
                danger
                loading={deleting}
                type="link"
                onClick={(event) => event.stopPropagation()}
              >
                {actions.delete}
              </Button>
            </Popconfirm>
          </span>
        );
      },
    },
  ];

  return (
    <div className="admin-section">
      <form
        className="admin-form"
        id="admin-user-form"
        noValidate
        ref={formRef}
        onSubmit={(event) => void submitUser(event)}
      >
        <h2 className="admin-form__title">{editingId ? copy.formEdit : copy.formCreate}</h2>
        <div className="admin-form__grid">
          <label className="admin-field" htmlFor="admin-user-name">
            <span>{copy.name}</span>
            <Input
              autoComplete="off"
              id="admin-user-name"
              name="name"
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </label>
          <label className="admin-field" htmlFor="admin-user-email">
            <span>{copy.email}</span>
            <Input
              autoComplete="off"
              id="admin-user-email"
              inputMode="email"
              name="email"
              placeholder={copy.allowlistPlaceholder}
              spellCheck={false}
              translate="no"
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </label>
        </div>
        <div className="admin-form__line">
          <label className="admin-field admin-field--inline" htmlFor="admin-user-role">
            <span>{copy.roleLabel}</span>
            <Select<DraftRole>
              id="admin-user-role"
              options={[
                { value: "EXTERNAL", label: copy.roleMember },
                { value: "ADMIN", label: copy.roleAdmin },
              ]}
              style={{ width: "9.5rem" }}
              value={newRole}
              onChange={setNewRole}
            />
          </label>
          {newRole === "EXTERNAL" ? (
            <label className="admin-field admin-field--caps" htmlFor="admin-user-capabilities">
              <Select
                aria-label={copy.capabilitiesTitle}
                id="admin-user-capabilities"
                loading={capsLoading}
                maxTagCount={0}
                maxTagPlaceholder={(omitted) =>
                  plural(
                    omitted.length,
                    t(copy.capabilitiesCountOne, { count: omitted.length }),
                    t(copy.capabilitiesCountOther, { count: omitted.length }),
                  )
                }
                mode="multiple"
                options={GRANTABLE_CAPABILITIES.map((capability) => ({
                  value: capability,
                  label: copy.capabilities[capability],
                }))}
                placeholder={copy.capabilitiesTitle}
                style={{ width: "100%" }}
                value={capabilities}
                onChange={(value: GrantableCapability[]) => {
                  if (value.length === 0) {
                    setFormError(copy.capabilitiesRequired);
                    return;
                  }
                  setFormError(null);
                  setCapabilities(value);
                }}
              />
            </label>
          ) : (
            <p className="admin-form__status">{copy.adminFullAccess}</p>
          )}
          {editingId ? (
            <label className="admin-field admin-field--inline">
              <span>{copy.activeAccessToggle}</span>
              <Switch aria-label={copy.activeAccessToggle} checked={active} onChange={setActive} />
            </label>
          ) : null}
          <div className="admin-form__actions">
            <Button
              disabled={!canSubmit}
              htmlType="submit"
              loading={saving && !bulkBusy}
              type="primary"
            >
              {editingId ? actions.save : actions.add}
            </Button>
            {editingId || createDirty ? (
              <Button htmlType="button" onClick={resetForm}>
                {actions.cancel}
              </Button>
            ) : null}
          </div>
        </div>
      </form>
      {notice && !formError ? (
        <StatusBanner closable title={notice} tone="success" onClose={() => setNotice(null)} />
      ) : null}
      {formError ? <StatusBanner title={formError} tone="error" /> : null}
      {access.provision.error ? (
        <StatusBanner
          conflictTitle={copy.provisionConflict}
          error={access.provision.error}
          title={copy.provisionError}
        />
      ) : null}
      {access.updateAccess.error ? (
        <StatusBanner
          conflictTitle={copy.conflict}
          error={access.updateAccess.error}
          title={copy.updateErrorTitle}
        />
      ) : null}
      {access.removeUser.error ? (
        <StatusBanner
          conflictTitle={copy.deleteBlocked}
          error={access.removeUser.error}
          title={copy.deleteError}
        />
      ) : null}
      <QueryView
        compact
        error={access.users.error}
        errorTitle={copy.errorTitle}
        isError={access.users.isError}
        isPending={access.users.isPending}
        retryLabel={copy.retry}
        onRetry={() => void access.users.refetch()}
      >
        <>
          <div className="admin-toolbar">
            <Input
              allowClear
              aria-label={copy.searchUsers}
              className="admin-toolbar__search"
              placeholder={copy.searchUsers}
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
            {selectedUsers.length > 0 ? (
              <div className="admin-toolbar__selected">
                <p className="admin-toolbar__count">
                  {plural(
                    selectedUsers.length,
                    t(copy.selectedCountOne, { count: selectedUsers.length }),
                    t(copy.selectedCountOther, { count: selectedUsers.length }),
                  )}
                </p>
                <Button disabled={bulkBusy} onClick={() => void applySelectedActive(true)}>
                  {actions.activate}
                </Button>
                <Button disabled={bulkBusy} onClick={() => void applySelectedActive(false)}>
                  {copy.bulkDeactivate}
                </Button>
                <Popconfirm
                  cancelText={actions.cancel}
                  okButtonProps={{ danger: true }}
                  okText={actions.delete}
                  title={copy.deleteSelectedConfirm}
                  onConfirm={() => void deleteSelected()}
                >
                  <Button danger disabled={bulkBusy} loading={bulkBusy}>
                    {actions.delete}
                  </Button>
                </Popconfirm>
              </div>
            ) : null}
          </div>
          <Table<AdminUser>
            columns={columns}
            dataSource={visibleUsers}
            locale={{ emptyText: copy.empty }}
            pagination={false}
            rowClassName={(user) =>
              [
                canEditUserAccess(actorLogin, user) ? "admin-row--clickable" : "",
                user.id === editingId ? "admin-row--selected" : "",
                user.active ? "" : "admin-row--inactive",
              ]
                .filter(Boolean)
                .join(" ")
            }
            rowKey="id"
            rowSelection={{
              selectedRowKeys: selectedIds,
              getCheckboxProps: (user) => ({
                disabled: !canEditUserAccess(actorLogin, user),
              }),
              onChange: (keys) =>
                setSelectedIds(
                  keys
                    .map(String)
                    .filter((id) =>
                      users.some((user) => user.id === id && canEditUserAccess(actorLogin, user)),
                    ),
                ),
            }}
            scroll={{ x: true }}
            onRow={(user) => ({
              onClick: (event: MouseEvent) => {
                if (isRowChrome(event.target)) return;
                void beginEdit(user);
              },
            })}
          />
        </>
      </QueryView>
    </div>
  );
}

function isGrantable(value: string): value is GrantableCapability {
  return (GRANTABLE_CAPABILITIES as readonly string[]).includes(value);
}

function sameCapabilities(left: readonly string[], right: readonly string[]): boolean {
  if (left.length !== right.length) return false;
  const set = new Set(left);
  return right.every((value) => set.has(value));
}

function isRowChrome(target: EventTarget | null): boolean {
  return target instanceof Element
    ? Boolean(target.closest("button, a, input, .ant-switch, .ant-checkbox-wrapper, .ant-select"))
    : false;
}

function roleName(
  role: AdminUser["role"],
  copy: { roleSuperadmin: string; roleAdmin: string; roleMember: string },
) {
  if (role === "SUPERADMIN") return copy.roleSuperadmin;
  if (role === "ADMIN") return copy.roleAdmin;
  return copy.roleMember;
}

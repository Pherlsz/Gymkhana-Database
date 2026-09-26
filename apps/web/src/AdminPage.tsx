import { Tabs } from "antd";
import { getRouteApi, useNavigate } from "@tanstack/react-router";
import { PageShell } from "./components/PageShell";
import { StateCard } from "./components/StateCard";
import { useI18n } from "./i18n";
import { AdminUsers } from "./lib/admin/AdminAccess";
import { AdminFeatureFlags } from "./lib/admin/AdminFeatureFlags";
import { AdminIntegrations } from "./lib/admin/AdminIntegrations";
import { isAdminTab } from "./lib/admin/adminSearch";
import { canManageUsers, isSuperadmin } from "./lib/roles";
import { useApplicationSession } from "./session";
import "./admin.css";

const adminRoute = getRouteApi("/admin");

export function AdminPage() {
  const session = useApplicationSession();
  const { messages } = useI18n();
  const copy = messages.admin;
  const search = adminRoute.useSearch();
  const navigate = useNavigate();
  const superadmin = isSuperadmin(session.user.role);

  const items = [
    {
      key: "access",
      label: copy.tabs.access,
      children: <AdminUsers actorLogin={session.user.login} />,
    },
    {
      key: "integrations",
      label: copy.tabs.integrations,
      children: <AdminIntegrations />,
    },
    ...(superadmin
      ? [
          {
            key: "features",
            label: copy.tabs.features,
            children: <AdminFeatureFlags />,
          },
        ]
      : []),
  ];

  return (
    <PageShell className="admin-page" measure title={copy.users.title}>
      {canManageUsers(session.user.role) ? (
        <Tabs
          activeKey={superadmin || search.tab !== "features" ? search.tab : "access"}
          aria-label={copy.tabs.label}
          destroyOnHidden
          items={items}
          onChange={(tab) => {
            if (!isAdminTab(tab)) return;
            void navigate({ search: { tab }, to: "/admin" });
          }}
        />
      ) : (
        <StateCard
          compact
          description={copy.users.forbidden}
          kind="warning"
          title={messages.common.status.restricted}
        />
      )}
    </PageShell>
  );
}

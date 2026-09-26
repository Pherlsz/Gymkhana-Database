import { Tabs } from "antd";
import { getRouteApi, useNavigate } from "@tanstack/react-router";
import { PageShell } from "./components/PageShell";
import { StateCard } from "./components/StateCard";
import { useI18n } from "./i18n";
import { AdminUsers } from "./lib/admin/AdminAccess";
import { AdminIntegrations } from "./lib/admin/AdminIntegrations";
import { isAdminTab } from "./lib/admin/adminSearch";
import { canManageUsers } from "./lib/roles";
import { useApplicationSession } from "./session";
import "./admin.css";

const adminRoute = getRouteApi("/admin");

export function AdminPage() {
  const session = useApplicationSession();
  const { messages } = useI18n();
  const copy = messages.admin;
  const search = adminRoute.useSearch();
  const navigate = useNavigate();

  return (
    <PageShell className="admin-page" measure title={copy.users.title}>
      {canManageUsers(session.user.role) ? (
        <Tabs
          activeKey={search.tab}
          aria-label={copy.tabs.label}
          destroyOnHidden
          items={[
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
          ]}
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

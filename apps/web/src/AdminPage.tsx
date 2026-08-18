import { Link } from "@tanstack/react-router";
import { Layout, Typography } from "antd";
import { AdminUsersPanel } from "./AdminUsersPanel";
import { CustomDataPage } from "./CustomDataPage";
import { useI18n } from "./i18n";
import { canManageUsers } from "./lib/roles";
import { OperationsPage } from "./OperationsPage";
import { useApplicationSession } from "./session";

export function AdminPage() {
  const session = useApplicationSession();
  const { messages } = useI18n();
  const copy = messages.home;
  const nav = messages.shell.navigation;

  if (!canManageUsers(session.user.role)) {
    return (
      <Layout className="page-measure">
        <header className="page-header">
          <Typography.Title level={1} className="page-title">
            {copy.userAdministrationTitle}
          </Typography.Title>
          <Typography.Paragraph className="page-description">
            {copy.adminForbidden}
          </Typography.Paragraph>
        </header>
      </Layout>
    );
  }

  return (
    <Layout className="page-measure">
      <header className="page-header">
        <Typography.Title level={1} className="page-title">
          {nav.admin}
        </Typography.Title>
        <Typography.Paragraph className="page-description">
          {copy.userAdministrationDescription}
        </Typography.Paragraph>
      </header>
      <div className="page-content">
        <nav aria-label={nav.admin} className="admin-tools">
          <Link to="/forms">{nav.forms}</Link>
        </nav>
        <AdminUsersPanel currentLogin={session.user.login} />
        <OperationsPage />
        <CustomDataPage />
      </div>
    </Layout>
  );
}

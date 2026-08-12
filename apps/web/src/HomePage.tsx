import { Card, Flex, Layout, Tag, Typography } from "antd";
import { AdminUsersPanel } from "./AdminUsersPanel";
import { useI18n } from "./i18n";
import { canManageUsers } from "./lib/roles";
import { useApplicationSession } from "./session";

export function HomePage() {
  const session = useApplicationSession();
  const { messages } = useI18n();
  const copy = messages.home;
  const roleLabel =
    session.user.role === "SUPERADMIN"
      ? messages.shell.roles.superadmin
      : session.user.role === "ADMIN"
        ? messages.shell.roles.admin
        : messages.shell.roles.external;

  return (
    <Layout style={{ maxWidth: "64rem", margin: "0 auto" }}>
      <header className="page-header">
        <div className="page-eyebrow">{copy.eyebrow}</div>
        <Typography.Title level={1} className="page-title">
          {messages.shell.productName}
        </Typography.Title>
        <Typography.Paragraph className="page-description">{copy.description}</Typography.Paragraph>
      </header>
      <div className="page-content">
        <Flex vertical gap="1.5rem">
          <Card className="authentication-panel" style={{ padding: "1rem" }}>
            <Flex vertical gap="0.75rem">
              <strong>{session.user.display_name}</strong>
              <span className="authentication-panel__description">
                @{session.user.login} · {roleLabel}
              </span>
              <Tag color="success">{copy.sessionActive}</Tag>
            </Flex>
          </Card>
          {canManageUsers(session.user.role) ? (
            <section className="page-section">
              <Typography.Title level={2}>{copy.userAdministrationTitle}</Typography.Title>
              <Typography.Paragraph>{copy.userAdministrationDescription}</Typography.Paragraph>
              <AdminUsersPanel currentLogin={session.user.login} />
            </section>
          ) : null}
          <section className="page-section">
            <Typography.Title level={2}>{copy.foundationTitle}</Typography.Title>
            <Typography.Paragraph>{copy.foundationDescription}</Typography.Paragraph>
            <Flex gap="0.5rem" align="stretch">
              <FoundationCard label="Frontend" value="React + TypeScript" />
              <FoundationCard label="Core" value="v0.2.2" />
              <FoundationCard label="UI" value="Ant Design" />
            </Flex>
          </section>
        </Flex>
      </div>
    </Layout>
  );
}

function FoundationCard({ label, value }: { label: string; value: string }) {
  return (
    <Card className="foundation-card">
      <div className="foundation-card__label">{label}</div>
      <strong>{value}</strong>
    </Card>
  );
}

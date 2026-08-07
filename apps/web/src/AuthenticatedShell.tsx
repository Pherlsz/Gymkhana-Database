import { Button, Flex, Layout, Tag } from "antd";
import { Link, Outlet } from "@tanstack/react-router";
import { normalizeChatSearch } from "./ChatPage";
import { normalizeMatchingSearch } from "./MatchingPage";
import { normalizeOCRSearch } from "./OCRPage";
import { normalizeOperationsSearch } from "./OperationsPage";
import { normalizeProfileSearch } from "./ProfilesPage";
import { normalizeGlobalSearch } from "./SearchPage";
import { normalizeTaskSearch } from "./TaskPage";
import { useI18n } from "./i18n";
import { canManageUsers } from "./lib/roles";
import { normalizeQuerySearch } from "./lib/queryState";
import { useApplicationContext } from "./session";

export function AuthenticatedShell() {
  const { session, signingOut, signOut } = useApplicationContext();
  const { messages } = useI18n();
  const copy = messages.shell;
  const roleLabel =
    session.user.role === "SUPERADMIN"
      ? copy.roles.superadmin
      : session.user.role === "ADMIN"
        ? copy.roles.admin
        : copy.roles.external;

  return (
    <Layout>
      <Layout.Header className="app-header">
        <Flex align="center" gap="1rem">
          <strong>{copy.productName}</strong>
          <nav aria-label={copy.navigationLabel} className="app-nav">
            <Link activeProps={{ className: "app-nav__link app-nav__link--active" }} className="app-nav__link" to="/">
              {copy.navigation.home}
            </Link>
            <Link activeProps={{ className: "app-nav__link app-nav__link--active" }} className="app-nav__link" to="/profiles" search={normalizeProfileSearch({})}>
              {copy.navigation.profiles}
            </Link>
            <Link activeProps={{ className: "app-nav__link app-nav__link--active" }} className="app-nav__link" search={normalizeGlobalSearch({})} to="/search">
              {copy.navigation.search}
            </Link>
            <Link activeProps={{ className: "app-nav__link app-nav__link--active" }} className="app-nav__link" search={normalizeQuerySearch({})} to="/query">
              {copy.navigation.query}
            </Link>
            <Link activeProps={{ className: "app-nav__link app-nav__link--active" }} className="app-nav__link" search={normalizeTaskSearch({})} to="/tasks">
              {copy.navigation.tasks}
            </Link>
            <Link activeProps={{ className: "app-nav__link app-nav__link--active" }} className="app-nav__link" search={normalizeMatchingSearch({})} to="/matching">
              {copy.navigation.matching}
            </Link>
            <Link activeProps={{ className: "app-nav__link app-nav__link--active" }} className="app-nav__link" search={normalizeChatSearch({})} to="/chat">
              {copy.navigation.chat}
            </Link>
            <Link activeProps={{ className: "app-nav__link app-nav__link--active" }} className="app-nav__link" search={normalizeOCRSearch({})} to="/ocr">
              {copy.navigation.ocr}
            </Link>
            <Link activeProps={{ className: "app-nav__link app-nav__link--active" }} className="app-nav__link" to="/custom-data">
              {copy.navigation.customData}
            </Link>
            <Link activeProps={{ className: "app-nav__link app-nav__link--active" }} className="app-nav__link" search={normalizeOperationsSearch({})} to="/operations">
              {copy.navigation.operations}
            </Link>
            {canManageUsers(session.user.role) ? (
              <Link activeProps={{ className: "app-nav__link app-nav__link--active" }} className="app-nav__link" to="/google-forms">
                {copy.navigation.googleForms}
              </Link>
            ) : null}
          </nav>
        </Flex>
        <Flex align="center" gap="0.5rem">
          <span className="current-user">@{session.user.login}</span>
          <Tag color="success">{roleLabel}</Tag>
          <Button disabled={signingOut} onClick={signOut}>
            {signingOut ? copy.signingOut : copy.signOut}
          </Button>
        </Flex>
      </Layout.Header>
      <Layout.Content>
        <Outlet />
      </Layout.Content>
    </Layout>
  );
}

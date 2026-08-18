import { Avatar, Badge, Dropdown, Flex, Switch, Typography } from "antd";
import { Link, Outlet, useNavigate, useRouterState } from "@tanstack/react-router";
import {
  ChevronDown,
  ClipboardList,
  FileText,
  Folder,
  Home,
  LogOut,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  Receipt,
  Search,
  Settings,
  SlidersHorizontal,
  Sun,
  User,
} from "lucide-react";
import { useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { useI18n } from "./i18n";
import { canManageUsers } from "./lib/roles";
import { TABLE_SEARCH_DEFAULTS } from "./lib/tables/tableRoutes";
import { GLOBAL_SEARCH_DEFAULTS } from "./SearchPage";
import { useApplicationContext } from "./session";
import { useTheme } from "./theme";
import { ICON, ICON_STROKE } from "./components/icons";

const NAV_COLLAPSED_KEY = "gymkhana-nav-collapsed";
const SHELL_COMPACT_QUERY = "(width < 600px)";

export function AuthenticatedShell() {
  return <AuthenticatedShellLayout />;
}

function AuthenticatedShellLayout() {
  const { messages } = useI18n();
  const { session } = useApplicationContext();
  const copy = messages.shell;
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const [navOpen, setNavOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const [compact, setCompact] = useState(false);
  const [tablesOpen, setTablesOpen] = useState(true);
  const activeTable = pathname.startsWith("/tables/") ? pathname.slice("/tables/".length) : "";
  const admin = canManageUsers(session.user.role);

  useEffect(() => {
    setCollapsed(window.localStorage.getItem(NAV_COLLAPSED_KEY) === "1");
    if (typeof window.matchMedia !== "function") {
      return;
    }
    const mq = window.matchMedia(SHELL_COMPACT_QUERY);
    const sync = () => setCompact(mq.matches);
    sync();
    mq.addEventListener("change", sync);
    return () => mq.removeEventListener("change", sync);
  }, []);

  useEffect(() => {
    setNavOpen(false);
  }, [pathname]);

  const rail = collapsed && !compact;

  const toggleCollapsed = () => {
    setCollapsed((current) => {
      const next = !current;
      window.localStorage.setItem(NAV_COLLAPSED_KEY, next ? "1" : "0");
      return next;
    });
  };

  const toggleTables = () => {
    if (rail) {
      setCollapsed(false);
      setTablesOpen(true);
      window.localStorage.setItem(NAV_COLLAPSED_KEY, "0");
      return;
    }
    setTablesOpen((open) => !open);
  };

  const shellClass = [
    "app-shell",
    navOpen ? "app-shell--nav-open" : "",
    rail ? "app-shell--collapsed" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={shellClass}>
      <header className="app-shell__topbar">
        <button
          aria-expanded={navOpen}
          aria-label={navOpen ? copy.navigation.closeNavigation : copy.navigation.openNavigation}
          className="app-shell__menu"
          onClick={() => setNavOpen((open) => !open)}
          type="button"
        >
          {navOpen ? (
            <PanelLeftClose aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
          ) : (
            <PanelLeftOpen aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
          )}
        </button>
        {/* Theme lives in the account menu alone. The topbar carried a second
            control that was reachable at the same time on mobile. */}
        <ShellSearch compact />
      </header>
      <button
        aria-label={copy.navigation.closeNavigation}
        className="app-shell__scrim"
        onClick={() => setNavOpen(false)}
        type="button"
      />
      <aside aria-label={copy.navigationLabel} className="app-shell__sidebar">
        <div className="app-shell__brand">
          <Link className="app-shell__brand-link" title={copy.productName} to="/">
            <img alt="" className="app-shell__logo-img" height={32} src="/Gampa.png" width={32} />
            <span className="brand-text">{copy.productName}</span>
          </Link>
          {compact ? (
            <button
              aria-label={copy.navigation.closeNavigation}
              className="app-shell__collapse"
              onClick={() => setNavOpen(false)}
              type="button"
            >
              <PanelLeftClose aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
            </button>
          ) : (
            <button
              aria-label={rail ? copy.navigation.expandMenu : copy.navigation.collapseMenu}
              className="app-shell__collapse"
              onClick={toggleCollapsed}
              title={rail ? copy.navigation.expandMenu : copy.navigation.collapseMenu}
              type="button"
            >
              {rail ? (
                <PanelLeftOpen aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
              ) : (
                <PanelLeftClose aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
              )}
            </button>
          )}
        </div>
        {compact ? null : <ShellSearch />}
        <nav>
          <Link
            activeOptions={{ exact: true }}
            activeProps={{ className: "nav-item--active" }}
            className="nav-item"
            title={copy.navigation.home}
            to="/"
          >
            <Home aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
            <span className="nav-item__label">{copy.navigation.home}</span>
          </Link>
          <hr className="nav-divider" />
          <button
            aria-controls="shell-tables"
            aria-expanded={tablesOpen}
            className="nav-item nav-item--toggle"
            onClick={toggleTables}
            type="button"
          >
            <Folder aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
            <span className="nav-item__label">{copy.navigation.tables}</span>
            <ChevronDown
              aria-hidden
              className="nav-item__chev"
              size={ICON.md}
              strokeWidth={ICON_STROKE}
            />
          </button>
          <div
            aria-hidden={!tablesOpen || rail}
            className="nav-sub-wrap"
            inert={!tablesOpen || rail}
          >
            <div className="nav-sub" id="shell-tables">
              <TableLink
                active={activeTable === "people"}
                icon={<User aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
                label={copy.navigation.profiles}
                table="people"
              />
              <TableLink
                active={activeTable === "documents"}
                icon={<FileText aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
                label={copy.navigation.documents}
                table="documents"
              />
              <TableLink
                active={activeTable === "bills"}
                icon={<Receipt aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
                label={copy.navigation.bills}
                table="bills"
              />
            </div>
          </div>
          {admin ? (
            <>
              <hr className="nav-divider" />
              <Link
                activeOptions={{ exact: true }}
                activeProps={{ className: "nav-item--active" }}
                className="nav-item"
                title={copy.navigation.forms}
                to="/forms"
              >
                <ClipboardList aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
                <span className="nav-item__label">{copy.navigation.forms}</span>
              </Link>
              <Link
                activeOptions={{ exact: true }}
                activeProps={{ className: "nav-item--active" }}
                className="nav-item"
                title={copy.navigation.admin}
                to="/admin"
              >
                <SlidersHorizontal aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
                <span className="nav-item__label">{copy.navigation.admin}</span>
              </Link>
            </>
          ) : null}
        </nav>
        <UserAccountCard rail={rail} />
      </aside>
      <main className="app-shell__content">
        <Outlet />
      </main>
    </div>
  );
}

function ShellSearch({ compact = false }: { compact?: boolean }) {
  const navigate = useNavigate();
  const { messages } = useI18n();
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        inputRef.current?.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const value = String(new FormData(event.currentTarget).get("q") ?? "").trim();
    if (!value) {
      inputRef.current?.focus();
      return;
    }
    void navigate({ search: { ...GLOBAL_SEARCH_DEFAULTS, q: value }, to: "/search" });
    event.currentTarget.reset();
  };

  return (
    <form className="shell-search" onSubmit={onSubmit} role="search">
      <Search aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
      <input
        aria-label={messages.shell.navigation.searchPlaceholder}
        autoComplete="off"
        name="q"
        placeholder={messages.shell.navigation.searchPlaceholder}
        ref={inputRef}
        type="search"
      />
      {compact ? null : <kbd>Ctrl K</kbd>}
    </form>
  );
}

function TableLink({
  label,
  table,
  active,
  icon,
}: {
  label: string;
  table: "people" | "documents" | "bills";
  active: boolean;
  icon: ReactNode;
}) {
  return (
    <Link
      className={active ? "nav-sub__item nav-sub__item--active" : "nav-sub__item"}
      params={{ table }}
      search={TABLE_SEARCH_DEFAULTS}
      title={label}
      to="/tables/$table"
    >
      <span className="nav-sub__icon">{icon}</span>
      <span className="nav-sub__label">{label}</span>
    </Link>
  );
}

function UserAccountCard({ rail }: { rail: boolean }) {
  const navigate = useNavigate();
  const { messages } = useI18n();
  const { session, signingOut, signOut } = useApplicationContext();
  const { theme, toggleTheme } = useTheme();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const [open, setOpen] = useState(false);
  const keepMenuOpen = useRef(false);
  const user = session.user;
  const isDark = theme === "dark";
  const copy = messages.shell;

  useEffect(() => {
    setOpen(false);
  }, [pathname, rail]);

  return (
    <div className="sidebar-account">
      <div className="sidebar-account__anchor">
        <Dropdown
          destroyOnHidden
          getPopupContainer={() => document.body}
          menu={{
            className: "sidebar-account-menu",
            items: [
              {
                key: "settings",
                icon: <Settings aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />,
                label: copy.account.settings,
              },
              {
                key: "appearance",
                icon: isDark ? (
                  <Sun aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
                ) : (
                  <Moon aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
                ),
                label: (
                  <Flex align="center" gap={12} justify="space-between">
                    <span>{copy.account.appearance}</span>
                    <Switch
                      checked={isDark}
                      onChange={toggleTheme}
                      onClick={(_, event) => event.stopPropagation()}
                      size="small"
                    />
                  </Flex>
                ),
              },
              { type: "divider" },
              {
                key: "logout",
                disabled: signingOut,
                icon: <LogOut aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />,
                label: signingOut ? copy.signingOut : copy.signOut,
              },
            ],
            onClick: ({ key, domEvent }) => {
              if (key === "settings") {
                void navigate({ to: "/settings" });
                return;
              }
              if (key === "appearance") {
                domEvent.preventDefault();
                domEvent.stopPropagation();
                if ((domEvent.target as HTMLElement | null)?.closest?.(".ant-switch")) return;
                keepMenuOpen.current = true;
                toggleTheme();
                setOpen(true);
                window.setTimeout(() => {
                  keepMenuOpen.current = false;
                }, 0);
                return;
              }
              if (key === "logout") signOut();
            },
            style: { minWidth: 220 },
          }}
          onOpenChange={(next) => {
            if (!next && keepMenuOpen.current) {
              setOpen(true);
              return;
            }
            setOpen(next);
            if (next) return;
            const active = document.activeElement;
            if (active instanceof HTMLElement && active.closest(".sidebar-account__anchor")) {
              active.blur();
            }
          }}
          open={open}
          placement={rail ? "rightBottom" : "topLeft"}
          trigger={["click"]}
        >
          <button
            aria-label={copy.account.openMenu}
            className="sidebar-account__trigger"
            type="button"
          >
            <Badge className="sidebar-account__status" dot offset={rail ? [-1, 22] : [-2, 30]}>
              <Avatar
                size={rail ? 28 : 36}
                src={user.avatar_url}
                style={{
                  backgroundColor: "var(--md-primary-container)",
                  color: "var(--md-primary-text)",
                }}
              >
                {initials(user.display_name)}
              </Avatar>
            </Badge>
            <span className="sidebar-account__meta">
              <Typography.Text className="sidebar-account__name" ellipsis strong>
                {user.display_name}
              </Typography.Text>
              <Typography.Text className="sidebar-account__login" ellipsis>
                {user.login}
              </Typography.Text>
            </span>
            <ChevronDown
              aria-hidden
              className="sidebar-account__chev"
              size={ICON.md}
              strokeWidth={ICON_STROKE}
            />
          </button>
        </Dropdown>
      </div>
    </div>
  );
}

function initials(name: string) {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  const first = parts[0]?.[0] ?? "";
  const second = parts[1]?.[0] ?? "";
  return `${first}${second}`.toUpperCase() || "?";
}

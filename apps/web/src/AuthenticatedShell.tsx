import { Avatar, Badge, Button, Dropdown, Typography } from "antd";
import { Link, Outlet, useNavigate, useRouterState } from "@tanstack/react-router";
import {
  ChevronDown,
  FileText,
  Folder,
  Home,
  LogOut,
  Menu,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  Receipt,
  Shield,
  Sun,
  User,
  UserPlus,
  X,
} from "lucide-react";
import { useEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { useI18n } from "./i18n";
import { ADMIN_SEARCH_DEFAULTS } from "./lib/admin/adminSearch";
import { canManageUsers } from "./lib/roles";
import { TABLE_SEARCH_DEFAULTS } from "./lib/tables/tableRoutes";
import { CADASTRO_SEARCH_DEFAULTS } from "./lib/cadastro/cadastroSearch";
import { GLOBAL_SEARCH_DEFAULTS } from "./lib/search/urlState";
import { useApplicationContext } from "./session";
import { useTheme } from "./theme";
import { ICON, ICON_STROKE } from "./components/icons";
import { SearchField } from "./components/SearchField";

const NAV_COLLAPSED_KEY = "gymkhana-nav-collapsed";
const NAV_TABLES_OPEN_KEY = "gymkhana-nav-tables-open";
const SHELL_COMPACT_QUERY = "(width < 600px)";

export function AuthenticatedShell() {
  return <AuthenticatedShellLayout />;
}

function AuthenticatedShellLayout() {
  const { messages } = useI18n();
  const { session } = useApplicationContext();
  const navigate = useNavigate();
  const copy = messages.shell;
  const entities = messages.common.entities;
  const showAdmin = canManageUsers(session.user.role);
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const [navOpen, setNavOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const [compact, setCompact] = useState(false);
  const activeTable = pathname.startsWith("/tables/") ? pathname.slice("/tables/".length) : "";
  const [tablesOpen, setTablesOpen] = useState(() => {
    if (typeof window === "undefined") return true;
    return window.localStorage.getItem(NAV_TABLES_OPEN_KEY) !== "0";
  });

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

  useEffect(() => {
    if (!navOpen) {
      return;
    }
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setNavOpen(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [navOpen]);

  useEffect(() => {
    if (activeTable) {
      setTablesOpen(true);
    }
  }, [activeTable]);

  const rail = collapsed && !compact;

  const topbarSearchRef = useRef<HTMLInputElement>(null);
  const sidebarSearchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        if (!rail) {
          (compact ? topbarSearchRef : sidebarSearchRef).current?.focus();
          return;
        }
        void navigate({ to: "/search", search: { ...GLOBAL_SEARCH_DEFAULTS } });
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [compact, navigate, rail]);

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
    setTablesOpen((open) => {
      const next = !open;
      window.localStorage.setItem(NAV_TABLES_OPEN_KEY, next ? "1" : "0");
      return next;
    });
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
        <Button
          aria-expanded={navOpen}
          aria-label={navOpen ? copy.navigation.closeNavigation : copy.navigation.openNavigation}
          className="app-shell__menu"
          onClick={() => setNavOpen((open) => !open)}
          type="text"
        >
          {navOpen ? (
            <X aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
          ) : (
            <Menu aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
          )}
        </Button>
        <Link
          aria-label={copy.productName}
          className="app-shell__topbar-brand"
          title={copy.productName}
          to="/"
        >
          <img
            alt=""
            className="app-shell__logo-img app-shell__logo-img--topbar"
            height={28}
            src="/Gampa.png"
            width={28}
          />
        </Link>
        <ShellSearch compact inputRef={topbarSearchRef} />
      </header>
      <div
        aria-hidden
        className="app-shell__scrim"
        onClick={() => setNavOpen(false)}
        role="presentation"
      />
      <aside
        aria-label={copy.navigationLabel}
        className="app-shell__sidebar"
        inert={!navOpen && compact}
      >
        <div className="app-shell__brand">
          <Link className="app-shell__brand-link" title={copy.productName} to="/">
            <img alt="" className="app-shell__logo-img" height={32} src="/Gampa.png" width={32} />
            <span className="brand-text">{copy.productName}</span>
          </Link>
          {compact ? (
            <Button
              aria-label={copy.navigation.closeNavigation}
              className="app-shell__collapse"
              onClick={() => setNavOpen(false)}
              type="text"
            >
              <X aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
            </Button>
          ) : (
            <Button
              aria-label={rail ? copy.navigation.expandMenu : copy.navigation.collapseMenu}
              className="app-shell__collapse"
              onClick={toggleCollapsed}
              title={rail ? copy.navigation.expandMenu : copy.navigation.collapseMenu}
              type="text"
            >
              {rail ? (
                <PanelLeftOpen aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
              ) : (
                <PanelLeftClose aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
              )}
            </Button>
          )}
        </div>
        {compact ? null : <ShellSearch inputRef={sidebarSearchRef} />}
        <nav>
          <RailTip label={entities.home} rail={rail}>
            <Link
              activeOptions={{ exact: true }}
              activeProps={{ "aria-current": "page", className: "nav-item--active" }}
              className="nav-item"
              title={entities.home}
              to="/"
            >
              <Home aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
              <span className="nav-item__label">{entities.home}</span>
            </Link>
          </RailTip>
          <RailTip label={entities.tables} rail={rail}>
            <button
              aria-controls="shell-tables"
              aria-expanded={tablesOpen && !rail}
              className={`nav-item nav-item--toggle${activeTable ? " nav-item--parent-active" : ""}`}
              onClick={toggleTables}
              type="button"
            >
              <Folder aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
              <span className="nav-item__label">{entities.tables}</span>
              <ChevronDown
                aria-hidden
                className="nav-item__chev"
                size={ICON.sm}
                strokeWidth={ICON_STROKE}
              />
            </button>
          </RailTip>
          <div
            aria-hidden={!tablesOpen || rail}
            className="nav-sub-wrap"
            inert={!tablesOpen || rail}
          >
            <div className="nav-sub" id="shell-tables">
              <TableLink
                active={activeTable === "people"}
                icon={<User aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
                label={entities.people}
                rail={rail}
                table="people"
              />
              <TableLink
                active={activeTable === "documents"}
                icon={<FileText aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
                label={entities.documents}
                rail={rail}
                table="documents"
              />
              <TableLink
                active={activeTable === "bills"}
                icon={<Receipt aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
                label={entities.bills}
                rail={rail}
                table="bills"
              />
            </div>
          </div>
          <hr className="nav-divider" />
          <RailTip label={entities.cadastro} rail={rail}>
            <Link
              activeOptions={{ exact: true, includeSearch: false }}
              activeProps={{ "aria-current": "page", className: "nav-item--active" }}
              className="nav-item"
              search={CADASTRO_SEARCH_DEFAULTS}
              title={entities.cadastro}
              to="/cadastro"
            >
              <UserPlus aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
              <span className="nav-item__label">{entities.cadastro}</span>
            </Link>
          </RailTip>
          {showAdmin ? (
            <RailTip label={entities.admin} rail={rail}>
              <Link
                activeOptions={{ exact: true, includeSearch: false }}
                activeProps={{ "aria-current": "page", className: "nav-item--active" }}
                className="nav-item"
                search={ADMIN_SEARCH_DEFAULTS}
                title={entities.admin}
                to="/admin"
              >
                <Shield aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
                <span className="nav-item__label">{entities.admin}</span>
              </Link>
            </RailTip>
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

function ShellSearch({
  compact = false,
  inputRef,
}: {
  compact?: boolean;
  inputRef: RefObject<HTMLInputElement | null>;
}) {
  const navigate = useNavigate();
  const { messages } = useI18n();
  const [value, setValue] = useState("");

  return (
    <div className="shell-search">
      <SearchField
        inputRef={inputRef}
        label={messages.search.inputPlaceholder}
        mode="text"
        placeholder={messages.search.inputPlaceholder}
        shortcutHint={compact ? undefined : "Ctrl K"}
        value={value}
        onChange={setValue}
        onSubmit={(next) => {
          const trimmed = next.trim();
          if (!trimmed) return;
          void navigate({ search: { ...GLOBAL_SEARCH_DEFAULTS, q: trimmed }, to: "/search" });
          setValue("");
        }}
      />
    </div>
  );
}

function RailTip({ label, rail, children }: { label: string; rail: boolean; children: ReactNode }) {
  if (!rail) {
    return <>{children}</>;
  }
  return (
    <span className="rail-tip" data-tip={label}>
      {children}
    </span>
  );
}

function TableLink({
  label,
  table,
  active,
  icon,
  rail,
}: {
  label: string;
  table: "people" | "documents" | "bills";
  active: boolean;
  icon: ReactNode;
  rail: boolean;
}) {
  return (
    <RailTip label={label} rail={rail}>
      <Link
        aria-current={active ? "page" : undefined}
        className={active ? "nav-sub__item nav-sub__item--active" : "nav-sub__item"}
        params={{ table }}
        search={TABLE_SEARCH_DEFAULTS}
        title={label}
        to="/tables/$table"
      >
        <span className="nav-sub__icon">{icon}</span>
        <span className="nav-sub__label">{label}</span>
      </Link>
    </RailTip>
  );
}

function SidebarThemeToggle({ rail }: { rail: boolean }) {
  const { theme, toggleTheme } = useTheme();
  const { messages } = useI18n();
  const isDark = theme === "dark";
  const label = messages.shell.account.appearance;

  return (
    <RailTip label={label} rail={rail}>
      <Button
        aria-label={label}
        aria-pressed={isDark}
        className="sidebar-theme-toggle"
        onClick={toggleTheme}
        type="text"
      >
        {isDark ? (
          <Moon aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
        ) : (
          <Sun aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
        )}
        <span className="sidebar-theme-toggle__label">{label}</span>
        <span aria-hidden className="sidebar-theme-toggle__switch">
          <span className="sidebar-theme-toggle__thumb" />
        </span>
      </Button>
    </RailTip>
  );
}

function UserAccountCard({ rail }: { rail: boolean }) {
  const { messages } = useI18n();
  const { session, signingOut, signOut } = useApplicationContext();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const [open, setOpen] = useState(false);
  const [menuWidth, setMenuWidth] = useState<number>();
  const anchorRef = useRef<HTMLDivElement>(null);
  const user = session.user;
  const copy = messages.shell;

  useEffect(() => {
    setOpen(false);
  }, [pathname, rail]);

  return (
    <div className="sidebar-account">
      <SidebarThemeToggle rail={rail} />
      <div className="sidebar-account__anchor" ref={anchorRef}>
        <RailTip label={user.display_name} rail={rail}>
          <Dropdown
            getPopupContainer={() => document.body}
            onOpenChange={(next, info) => {
              if (!next && info.source === "menu") return;
              if (next) setMenuWidth(anchorRef.current?.offsetWidth);
              setOpen(next);
              if (next) return;
              const active = document.activeElement;
              if (active instanceof HTMLElement && active.closest(".sidebar-account__anchor")) {
                active.blur();
              }
            }}
            open={open}
            placement={rail ? "rightBottom" : "topLeft"}
            menu={{
              onClick: ({ key }) => {
                if (key === "logout") {
                  setOpen(false);
                  signOut();
                }
              },
              items: [
                {
                  key: "logout",
                  danger: true,
                  disabled: signingOut,
                  icon: <LogOut aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />,
                  label: signingOut ? copy.signingOut : copy.signOut,
                },
              ],
            }}
            styles={{
              root: rail
                ? { minWidth: 160 }
                : { width: menuWidth, minWidth: menuWidth, maxWidth: menuWidth },
            }}
            trigger={["click"]}
          >
            <Button
              aria-label={copy.account.openMenu}
              className="sidebar-account__trigger"
              type="text"
            >
              <Badge className="sidebar-account__status" dot offset={rail ? [-1, 22] : [-2, 30]}>
                <Avatar
                  size={rail ? 28 : 36}
                  src={user.avatar_url}
                  style={{
                    backgroundColor: "var(--md-state-selected)",
                    color: "var(--md-on-surface)",
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
            </Button>
          </Dropdown>
        </RailTip>
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

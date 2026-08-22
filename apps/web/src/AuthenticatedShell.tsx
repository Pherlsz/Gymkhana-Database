import { Avatar, Badge, Dropdown, Switch, Typography } from "antd";
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
import { useEffect, useRef, useState, type FormEvent, type ReactNode, type RefObject } from "react";
import { useI18n } from "./i18n";
import { canManageUsers } from "./lib/roles";
import { TABLE_SEARCH_DEFAULTS } from "./lib/tables/tableRoutes";
import { GLOBAL_SEARCH_DEFAULTS } from "./SearchPage";
import { useApplicationContext } from "./session";
import { useTheme } from "./theme";
import { ICON, ICON_STROKE } from "./components/icons";

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
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const [navOpen, setNavOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const [compact, setCompact] = useState(false);
  const activeTable = pathname.startsWith("/tables/") ? pathname.slice("/tables/".length) : "";
  const [tablesOpen, setTablesOpen] = useState(() => {
    if (typeof window === "undefined") return true;
    return window.localStorage.getItem(NAV_TABLES_OPEN_KEY) !== "0";
  });
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

  /* Keyboard escape route for the mobile drawer (mirrors the scrim click). */
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

  /*
   * The group never hides the current location: landing on a table with the
   * group collapsed (deep link, Home shortcut) reopens it so the "you are
   * here" rule stays visible.
   */
  useEffect(() => {
    if (activeTable) {
      setTablesOpen(true);
    }
  }, [activeTable]);

  const rail = collapsed && !compact;

  /*
   * One Ctrl+K owner at the shell level. The two search inputs (topbar on
   * mobile, sidebar on desktop) are mutually exclusive by breakpoint, so the
   * visible one wins; when the rail hides both, the shortcut falls back to
   * the global search page instead of focusing a display:none input.
   */
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
        <ShellSearch compact inputRef={topbarSearchRef} />
      </header>
      <button
        aria-hidden
        className="app-shell__scrim"
        onClick={() => setNavOpen(false)}
        tabIndex={-1}
        type="button"
      />
      {/* Inert only while the off-canvas drawer is closed on mobile — the
          desktop sidebar is a permanent fixture and must stay interactive. */}
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
        {compact ? null : <ShellSearch inputRef={sidebarSearchRef} />}
        <nav>
          <RailTip label={copy.navigation.home} rail={rail}>
            <Link
              activeOptions={{ exact: true }}
              activeProps={{ "aria-current": "page", className: "nav-item--active" }}
              className="nav-item"
              title={copy.navigation.home}
              to="/"
            >
              <Home aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
              <span className="nav-item__label">{copy.navigation.home}</span>
            </Link>
          </RailTip>
          <hr className="nav-divider" />
          <RailTip label={copy.navigation.tables} rail={rail}>
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
                label={copy.navigation.profiles}
                rail={rail}
                table="people"
              />
              <TableLink
                active={activeTable === "documents"}
                icon={<FileText aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
                label={copy.navigation.documents}
                rail={rail}
                table="documents"
              />
              <TableLink
                active={activeTable === "bills"}
                icon={<Receipt aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
                label={copy.navigation.bills}
                rail={rail}
                table="bills"
              />
            </div>
          </div>
          {admin ? (
            <>
              <hr className="nav-divider" />
              <RailTip label={copy.navigation.forms} rail={rail}>
                <Link
                  activeOptions={{ exact: true }}
                  activeProps={{ "aria-current": "page", className: "nav-item--active" }}
                  className="nav-item"
                  title={copy.navigation.forms}
                  to="/forms"
                >
                  <ClipboardList aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
                  <span className="nav-item__label">{copy.navigation.forms}</span>
                </Link>
              </RailTip>
              <RailTip label={copy.navigation.admin} rail={rail}>
                <Link
                  activeOptions={{ exact: true }}
                  activeProps={{ "aria-current": "page", className: "nav-item--active" }}
                  className="nav-item"
                  title={copy.navigation.admin}
                  to="/admin"
                >
                  <SlidersHorizontal aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
                  <span className="nav-item__label">{copy.navigation.admin}</span>
                </Link>
              </RailTip>
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

function ShellSearch({
  compact = false,
  inputRef,
}: {
  compact?: boolean;
  inputRef: RefObject<HTMLInputElement | null>;
}) {
  const navigate = useNavigate();
  const { messages } = useI18n();
  const [focused, setFocused] = useState(false);

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
    <form
      className={focused ? "shell-search shell-search--focused" : "shell-search"}
      onSubmit={onSubmit}
      role="search"
    >
      <Search aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
      <input
        aria-label={messages.shell.navigation.searchPlaceholder}
        autoComplete="off"
        name="q"
        placeholder={messages.shell.navigation.searchPlaceholder}
        ref={inputRef}
        spellCheck={false}
        type="text"
        onBlur={() => setFocused(false)}
        onFocus={() => setFocused(true)}
      />
      {compact || focused ? null : <kbd>Ctrl K</kbd>}
    </form>
  );
}

/* Icon-only rail items get a CSS tooltip (see .rail-tip in shell.css): the
   native `title` needs ~1s and never shows on touch, and antd Tooltip is
   deliberately unused in this app. Expanded items already show their label,
   so the wrapper only mounts in rail mode. */
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

function AccountMenuPanel({
  copy,
  signingOut,
  onLogout,
  onSettings,
}: {
  copy: ReturnType<typeof useI18n>["messages"]["shell"];
  signingOut: boolean;
  onLogout: () => void;
  onSettings: () => void;
}) {
  const { theme, toggleTheme } = useTheme();
  const isDark = theme === "dark";
  return (
    <div
      className="sidebar-account-menu"
      onMouseDown={(event) => {
        if ((event.target as HTMLElement | null)?.closest(".ant-switch, button")) return;
        event.preventDefault();
      }}
      role="menu"
    >
      <button
        className="sidebar-account-menu__item"
        onClick={onSettings}
        role="menuitem"
        type="button"
      >
        <Settings aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
        {copy.account.settings}
      </button>
      {/* Whole-row toggle (menuitemcheckbox): clicking anywhere on the row
          flips the theme. The Switch renders a <button role="switch"> (not a
          checkbox), so a <label> would not forward clicks; instead the row
          handles click/keyboard and the wrapper around the Switch stops the
          event so a direct hit does not toggle twice (row + Switch onChange). */}
      <div
        aria-checked={isDark}
        className="sidebar-account-menu__item sidebar-account-menu__appearance"
        onClick={toggleTheme}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            toggleTheme();
          }
        }}
        role="menuitemcheckbox"
        tabIndex={0}
      >
        {isDark ? (
          <Sun aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
        ) : (
          <Moon aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
        )}
        <span>{copy.account.appearance}</span>
        <span className="sidebar-account-menu__switch" onClick={(event) => event.stopPropagation()}>
          <Switch
            aria-label={copy.account.appearance}
            checked={isDark}
            onChange={toggleTheme}
            size="small"
          />
        </span>
      </div>
      <div className="sidebar-account-menu__divider" />
      <button
        className="sidebar-account-menu__item"
        disabled={signingOut}
        onClick={onLogout}
        role="menuitem"
        type="button"
      >
        <LogOut aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
        {signingOut ? copy.signingOut : copy.signOut}
      </button>
    </div>
  );
}

function UserAccountCard({ rail }: { rail: boolean }) {
  const navigate = useNavigate();
  const { messages } = useI18n();
  const { session, signingOut, signOut } = useApplicationContext();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const [open, setOpen] = useState(false);
  const user = session.user;
  const copy = messages.shell;

  useEffect(() => {
    setOpen(false);
  }, [pathname, rail]);

  return (
    <div className="sidebar-account">
      <div className="sidebar-account__anchor">
        <Dropdown
          getPopupContainer={() => document.body}
          onOpenChange={(next, info) => {
            if (!next && info.source === "menu") return;
            setOpen(next);
            if (next) return;
            const active = document.activeElement;
            if (active instanceof HTMLElement && active.closest(".sidebar-account__anchor")) {
              active.blur();
            }
          }}
          open={open}
          placement={rail ? "rightBottom" : "topLeft"}
          popupRender={() => (
            <AccountMenuPanel
              copy={copy}
              signingOut={signingOut}
              onLogout={() => {
                setOpen(false);
                signOut();
              }}
              onSettings={() => {
                setOpen(false);
                void navigate({ to: "/settings" });
              }}
            />
          )}
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

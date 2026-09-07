import { Button, ConfigProvider, theme as antdTheme } from "antd";
import { Moon, Sun } from "lucide-react";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type PropsWithChildren,
} from "react";

type Theme = "light" | "dark";

const STORAGE_KEY = "gymkhana-theme";
const THEME_SWITCH_MS = 240;

const PRIMARY = "#ffc53d";
const ON_PRIMARY = "#141414";
const SEEDS = {
  light: {
    primaryText: "#8b5500",
    error: "#b91c1e",
    warning: "#a14206",
    success: "#007742",
    info: "#065da0",
  },
  dark: {
    primaryText: "#ecd59f",
    error: "#f66d62",
    warning: "#f3ad6d",
    success: "#59d38c",
    info: "#73b6fa",
  },
} as const;

const ThemeContext = createContext<{
  theme: Theme;
  toggleTheme: () => void;
} | null>(null);

function gymkhanaAntdTheme(mode: Theme) {
  const dark = mode === "dark";
  const seed = dark ? SEEDS.dark : SEEDS.light;
  return {
    cssVar: { key: "gymkhana" },
    hashed: false,
    algorithm: dark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
    token: {
      colorPrimary: PRIMARY,
      colorTextLightSolid: ON_PRIMARY,
      colorLink: "var(--md-on-surface)",
      colorLinkHover: "var(--md-on-surface)",
      colorLinkActive: "var(--md-on-surface)",
      colorError: seed.error,
      colorWarning: seed.warning,
      colorSuccess: seed.success,
      colorInfo: seed.info,
      controlOutline: "var(--md-focus-ring)",
      colorBgTextHover: "var(--md-state-hover)",
      controlItemBgHover: "var(--md-state-hover)",
      colorPrimaryBg: "var(--md-state-hover)",
      colorPrimaryBgHover: "var(--md-state-pressed)",
      colorFillTertiary: "var(--md-state-hover)",
      borderRadius: 8,
      fontSize: 14,
    },
    components: {
      Button: {
        controlHeight: 36,
        controlHeightSM: 30,
        controlHeightLG: 42,
        borderRadius: 8,
        borderRadiusSM: 6,
        borderRadiusLG: 10,
        primaryColor: dark ? "#141414" : "#ffffff",
        defaultColor: "var(--md-on-surface)",
        defaultBorderColor: "var(--md-outline-variant)",
        defaultHoverColor: "var(--md-on-surface)",
        defaultHoverBorderColor: "var(--md-outline)",
        defaultActiveColor: "var(--md-on-surface)",
        defaultActiveBorderColor: "var(--md-on-surface)",
        defaultGhostColor: "var(--md-on-surface)",
        defaultGhostBorderColor: "var(--md-on-surface)",
        textTextColor: "var(--md-on-surface)",
        textTextHoverColor: "var(--md-on-surface)",
        textTextActiveColor: "var(--md-on-surface)",
        fontWeight: 600,
      },
      Input: {
        hoverBorderColor: "var(--md-outline)",
        activeBorderColor: "var(--brand-accent-solid)",
        activeShadow: "0 0 0 2px var(--md-focus-ring)",
      },
      Select: {
        hoverBorderColor: "var(--md-outline)",
        activeBorderColor: "var(--brand-accent-solid)",
        optionSelectedBg: "var(--md-state-selected)",
      },
      DatePicker: {
        hoverBorderColor: "var(--md-outline)",
        activeBorderColor: "var(--brand-accent-solid)",
      },
      Tag: {
        defaultColor: "var(--md-on-surface)",
      },
      Menu: {
        itemSelectedColor: "var(--brand-accent-text)",
        itemHoverColor: "var(--brand-accent-text)",
        itemHoverBg: "var(--md-state-hover)",
        itemSelectedBg: "var(--md-state-selected)",
        subMenuItemSelectedColor: "var(--brand-accent-text)",
        horizontalItemSelectedColor: "var(--brand-accent-text)",
        horizontalItemHoverColor: "var(--brand-accent-text)",
      },
      Tabs: {
        inkBarColor: "var(--brand-accent-solid)",
        itemSelectedColor: "var(--brand-accent-text)",
        itemHoverColor: "var(--brand-accent-text)",
        itemActiveColor: "var(--brand-accent-text)",
      },
      Pagination: {
        colorPrimary: "var(--md-on-surface)",
        itemActiveColor: "var(--md-on-surface)",
        itemActiveColorHover: "var(--md-on-surface)",
      },
      Radio: {
        buttonSolidCheckedColor: "var(--md-surface)",
        colorPrimary: "var(--md-on-surface)",
      },
      Table: {
        headerBg: "var(--md-surface-container)",
        headerSortActiveBg: "var(--md-surface-container)",
        headerSortHoverBg: "var(--md-surface-container)",
        headerColor: "var(--md-on-surface-variant)",
        borderColor: "var(--md-outline-variant)",
        rowHoverBg: "var(--md-state-hover)",
        rowSelectedBg: "var(--md-state-selected)",
        rowSelectedHoverBg: "var(--md-state-selected)",
      },
      Tooltip: {
        colorBgSpotlight: "var(--md-on-surface)",
        colorTextLightSolid: "var(--md-surface)",
      },
      Checkbox: {
        colorPrimary: dark ? PRIMARY : "#d97706",
        colorPrimaryHover: dark ? "#ffd566" : "#b45309",
        colorPrimaryBorder: dark ? PRIMARY : "#d97706",
        colorTextLightSolid: dark ? ON_PRIMARY : "#ffffff",
        borderRadiusSM: 4,
      },
      Switch: {
        colorPrimary: PRIMARY,
      },
      Card: {
        bodyPadding: 16,
      },
      Form: {
        itemMarginBottom: 0,
        verticalLabelPadding: "0 0 8px",
      },
    },
  };
}

function applyThemeClass(theme: Theme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
}

function prefersDark(): boolean {
  if (typeof window.matchMedia !== "function") return false;
  try {
    return window.matchMedia("(prefers-color-scheme: dark)").matches;
  } catch {
    return false;
  }
}

function readStoredTheme(): Theme {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "dark" || stored === "light") return stored;
  } catch {
    return "light";
  }
  return prefersDark() ? "dark" : "light";
}

function persistTheme(theme: Theme) {
  try {
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    /* ignore quota / private mode */
  }
}

export function ThemeProvider({ children }: PropsWithChildren) {
  const [theme, setThemeState] = useState<Theme>(() =>
    typeof window === "undefined" ? "light" : readStoredTheme(),
  );
  const switchTimer = useRef(0);

  useEffect(() => {
    applyThemeClass(theme);
  }, [theme]);

  useEffect(() => {
    const root = document.documentElement;
    const frame = window.requestAnimationFrame(() => {
      root.removeAttribute("data-theme-boot");
    });
    return () => {
      window.cancelAnimationFrame(frame);
      window.clearTimeout(switchTimer.current);
      root.classList.remove("theme-switching");
    };
  }, []);

  const toggleTheme = useCallback(() => {
    setThemeState((prev) => {
      const next = prev === "dark" ? "light" : "dark";
      persistTheme(next);
      const root = document.documentElement;
      root.classList.add("theme-switching");
      applyThemeClass(next);
      window.clearTimeout(switchTimer.current);
      switchTimer.current = window.setTimeout(() => {
        root.classList.remove("theme-switching");
      }, THEME_SWITCH_MS);
      return next;
    });
  }, []);

  const antdThemeConfig = useMemo(() => gymkhanaAntdTheme(theme), [theme]);

  return (
    <ThemeContext.Provider value={{ theme, toggleTheme }}>
      <ConfigProvider theme={antdThemeConfig}>{children}</ConfigProvider>
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used within ThemeProvider");
  return ctx;
}

export function ThemeToggle({
  activateLight,
  activateDark,
  lightLabel,
  darkLabel,
}: {
  activateLight: string;
  activateDark: string;
  lightLabel: string;
  darkLabel: string;
}) {
  const { theme, toggleTheme } = useTheme();
  const isDark = theme === "dark";

  return (
    <Button
      type="text"
      className="login-theme-toggle"
      data-theme={theme}
      onClick={toggleTheme}
      aria-label={isDark ? activateLight : activateDark}
      title={isDark ? lightLabel : darkLabel}
    >
      <span
        className="login-theme-toggle__icon"
        style={{
          opacity: isDark ? 1 : 0,
          transform: isDark ? "rotate(0deg) scale(1)" : "rotate(-90deg) scale(0.5)",
        }}
        aria-hidden
      >
        <Sun aria-hidden size={16} strokeWidth={1.75} />
      </span>
      <span
        className="login-theme-toggle__icon"
        style={{
          opacity: isDark ? 0 : 1,
          transform: isDark ? "rotate(90deg) scale(0.5)" : "rotate(0deg) scale(1)",
        }}
        aria-hidden
      >
        <Moon aria-hidden size={16} strokeWidth={1.75} />
      </span>
    </Button>
  );
}

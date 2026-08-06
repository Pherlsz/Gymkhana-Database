import React, { createContext, useContext, useMemo, ReactNode } from "react";

export type Theme = "light" | "dark" | "auto" | "system";
export type Density = "comfortable" | "compact";

interface ThemeContextValue {
  theme: Theme;
  density: Density;
}

const ThemeContext = createContext<ThemeContextValue>({
  theme: "light",
  density: "comfortable",
});

export function useTheme() {
  return useContext(ThemeContext);
}

interface ThemeProviderProps {
  theme?: Theme;
  density?: Density;
  children: ReactNode;
}

export function ThemeProvider({ theme = "light", density = "comfortable", children }: ThemeProviderProps) {
  const value = useMemo(() => ({ theme, density }), [theme, density]);
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

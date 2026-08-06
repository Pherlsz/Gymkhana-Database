import type { ReactNode } from "react";

export type Theme = "light" | "dark" | "system";
export type Density = "compact" | "comfortable" | "spacious";

type ThemeProviderProps = {
  theme: Theme;
  density?: Density;
  children: ReactNode;
};

export function ThemeProvider({ children }: ThemeProviderProps) {
  return <>{children}</>;
}

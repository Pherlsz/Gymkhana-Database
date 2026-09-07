export const BREAKPOINT = {
  sm: 600,
  md: 840,
  lg: 1100,
  xl: 1500,
} as const;

export type Breakpoint = keyof typeof BREAKPOINT;

export function atLeast(breakpoint: Breakpoint): string {
  return `(min-width: ${BREAKPOINT[breakpoint]}px)`;
}

export function below(breakpoint: Breakpoint): string {
  return `(max-width: ${BREAKPOINT[breakpoint] - 1}px)`;
}

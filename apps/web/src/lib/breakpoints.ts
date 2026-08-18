/**
 * The only breakpoint ladder. CSS uses these as `min-width` queries and JS reads
 * them through `useMediaQuery`, so a layout decision cannot drift between the
 * two. `xl` belongs to the home catalog alone.
 */
export const BREAKPOINT = {
  /** Shell gains a sidebar; toolbar surfaces become anchored popovers. */
  sm: 600,
  /** Content and the row inspector can sit side by side. */
  md: 840,
  /** Widest content column. */
  lg: 1100,
  /** Home catalog only. */
  xl: 1500,
} as const;

export type Breakpoint = keyof typeof BREAKPOINT;

/** Matches at and above the breakpoint. Mirrors `@media (min-width: …)`. */
export function atLeast(breakpoint: Breakpoint): string {
  return `(min-width: ${BREAKPOINT[breakpoint]}px)`;
}

/** Matches strictly below the breakpoint, so it never overlaps `atLeast`. */
export function below(breakpoint: Breakpoint): string {
  return `(max-width: ${BREAKPOINT[breakpoint] - 1}px)`;
}

/**
 * Three icon sizes, not seven. The app had shipped 8, 13, 14, 15, 16, 17, 18 and
 * 24 px side by side, which made otherwise identical rows sit at different
 * heights. `strokeWidth` is fixed here too so every glyph carries one weight.
 */
export const ICON = {
  /** Presence marks inside a 1.15rem document chip. Heavier stroke, so the
   *  folded corner / scan line / person silhouette still read at this size. */
  badge: 11,
  /** Inline with body text: chips, list affordances. */
  sm: 14,
  /** Default: buttons, menu items, toolbar triggers. */
  md: 16,
  /** Standalone marks: empty states, page-level affordances. */
  lg: 24,
} as const;

export const ICON_STROKE = 1.75;
export const ICON_BADGE_STROKE = 2.25;

/** Leftover viewport width is added to the last column.
 * Earlier columns keep their preferred widths, so resizing them still sticks.
 * When the preferred sum already fills the sheet, widths are unchanged and the sheet scrolls.
 * The last column is not a drag handle — it only absorbs leftover width.
 * A filled row never sums past the visible sheet.
 */
export function fillSheetColumnWidths(preferred: number[], viewport: number): number[] {
  if (preferred.length === 0) return [];
  const fit = Math.floor(viewport);
  if (!(fit > 0)) return preferred.slice();
  const sum = preferred.reduce((total, width) => total + width, 0);
  if (sum >= fit) return preferred.slice();
  const next = preferred.slice();
  const last = next.length - 1;
  const others = sum - (next[last] ?? 0);
  next[last] = fit - others;
  return next;
}

/** Visible data columns stay wide enough that a divider drag cannot hide one. */
export const SHEET_DATA_COLUMN_MIN_WIDTH = 72;

/** Move an internal divider by giving pixels to the next column, or taking them back.
 * The row sum stays put, and neither data column drops below the minimum.
 */
export function dragSheetDivider(
  widths: number[],
  index: number,
  requested: number,
  minWidth = SHEET_DATA_COLUMN_MIN_WIDTH,
): number[] {
  if (index < 0 || index >= widths.length - 1) return widths.slice();
  const next = widths.slice();
  const current = next[index] ?? 0;
  const neighbor = next[index + 1] ?? 0;
  const room = Math.max(0, neighbor - minWidth);
  const width = Math.max(minWidth, Math.min(current + room, Math.round(requested)));
  next[index] = width;
  next[index + 1] = neighbor - (width - current);
  return next;
}

/** A handle belongs on a divider between two columns, never the table's outer edge. */
export function columnHasResizeHandle(index: number, columnCount: number): boolean {
  return columnCount > 1 && index >= 0 && index < columnCount - 1;
}

/** Leftover viewport width is added to the last column.
 * Earlier columns keep their preferred widths, so resizing them still sticks.
 * When the preferred sum already fills the sheet, widths are unchanged and the sheet scrolls.
 * The last column is not a drag handle — it only absorbs leftover width.
 */
export function fillSheetColumnWidths(preferred: number[], viewport: number): number[] {
  if (preferred.length === 0) return [];
  const sum = preferred.reduce((total, width) => total + width, 0);
  if (!(viewport > sum)) return preferred.slice();
  const next = preferred.slice();
  const last = next.length - 1;
  next[last] = (next[last] ?? 0) + (viewport - sum);
  return next;
}

/** A handle belongs on a divider between two columns, never the table's outer edge. */
export function columnHasResizeHandle(index: number, columnCount: number): boolean {
  return columnCount > 1 && index >= 0 && index < columnCount - 1;
}

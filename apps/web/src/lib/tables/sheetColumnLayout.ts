/** Leftover viewport width is added to the last column.
 * Earlier columns keep their preferred widths, so resizing them still sticks.
 * When the preferred sum already fills the sheet, widths are unchanged and the sheet scrolls.
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

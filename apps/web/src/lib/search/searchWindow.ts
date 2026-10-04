/** Matches the page window already enforced by the search API (offset cap). */
export const SEARCH_RESULT_CAP = 10_000;

export function searchResultWindow(total: number, limit: number): {
  shown: number;
  truncated: boolean;
} {
  const safeLimit = Math.max(1, limit);
  const safeTotal = Math.max(0, total);
  const pageCap = Math.floor(SEARCH_RESULT_CAP / safeLimit) + 1;
  const pages = Math.max(1, Math.min(pageCap, Math.ceil(safeTotal / safeLimit) || 1));
  const shown = Math.min(safeTotal, pages * safeLimit);
  return { shown, truncated: safeTotal > SEARCH_RESULT_CAP };
}

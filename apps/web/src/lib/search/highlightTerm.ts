/** Words from a search box that should be marked in a result.
 * Field operators keep the value after the colon. Short tokens and boolean words are skipped.
 */
export function highlightNeedles(query: string): string[] {
  const seen = new Set<string>();
  const needles: string[] = [];
  for (const raw of query.split(/\s+/)) {
    const token = raw.trim();
    if (!token) continue;
    const value = token.includes(":") ? token.slice(token.indexOf(":") + 1) : token;
    const cleaned = value.replace(/^[("']+|[)"']+$/g, "");
    if (cleaned.length < 2) continue;
    if (/^(ou|e|n[aã]o|and|or|not)$/i.test(cleaned)) continue;
    const key = cleaned.toLocaleLowerCase("pt-BR");
    if (seen.has(key)) continue;
    seen.add(key);
    needles.push(cleaned);
  }
  return needles;
}

export function highlightParts(
  text: string,
  query: string,
): Array<{ text: string; match: boolean }> {
  const needles = highlightNeedles(query);
  if (!text || needles.length === 0) return [{ text, match: false }];
  const lower = text.toLocaleLowerCase("pt-BR");
  const ranges: Array<[number, number]> = [];
  for (const needle of needles) {
    const target = needle.toLocaleLowerCase("pt-BR");
    let from = 0;
    while (from < lower.length) {
      const at = lower.indexOf(target, from);
      if (at < 0) break;
      ranges.push([at, at + target.length]);
      from = at + target.length;
    }
  }
  ranges.sort((left, right) => left[0] - right[0] || right[1] - left[1]);
  const merged: Array<[number, number]> = [];
  for (const range of ranges) {
    const last = merged[merged.length - 1];
    if (!last || range[0] > last[1]) merged.push([range[0], range[1]]);
    else last[1] = Math.max(last[1], range[1]);
  }
  const parts: Array<{ text: string; match: boolean }> = [];
  let cursor = 0;
  for (const [start, end] of merged) {
    if (start > cursor) parts.push({ text: text.slice(cursor, start), match: false });
    parts.push({ text: text.slice(start, end), match: true });
    cursor = end;
  }
  if (cursor < text.length) parts.push({ text: text.slice(cursor), match: false });
  return parts.length > 0 ? parts : [{ text, match: false }];
}

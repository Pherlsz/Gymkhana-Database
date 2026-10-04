/** Keyboard for the result list. Typing in a field is left alone, except the slash shortcut. */
export function searchResultKey(
  key: string,
  index: number,
  count: number,
  typing: boolean,
): "focus" | "toggle" | number | null {
  if (key === "/" && !typing) return "focus";
  if (typing || count < 1) return null;
  if (key === "ArrowDown") return Math.min(count - 1, index + 1);
  if (key === "ArrowUp") return Math.max(0, index - 1);
  if (key === "Enter") return "toggle";
  return null;
}

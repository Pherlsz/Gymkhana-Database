export const DEFAULT_THREAD_TITLE = "Nova conversa";
export const THREAD_TITLE_RUNES = 50;

export function threadTitle(content: string): string {
  const firstLine = content.split("\n").find((line) => line.trim()) ?? content;
  const runes = Array.from(firstLine.trim());
  if (runes.length === 0) return DEFAULT_THREAD_TITLE;
  return runes.length > THREAD_TITLE_RUNES
    ? `${runes.slice(0, THREAD_TITLE_RUNES - 1).join("")}…`
    : runes.join("");
}

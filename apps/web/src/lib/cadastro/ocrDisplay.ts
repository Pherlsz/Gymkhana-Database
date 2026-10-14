export function formatOCRConfidence(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value) || value < 0) return "—";
  const percent = value <= 1 ? value * 100 : value <= 100 ? value : value / 100;
  return `${Math.round(percent)}%`;
}

export function ocrJobStateLabel(
  state: string,
  labels: Record<string, string> | undefined,
): string {
  return labels?.[state] ?? state;
}

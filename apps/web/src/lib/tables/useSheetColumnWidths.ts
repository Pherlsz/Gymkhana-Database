import { useCallback, useEffect, useState } from "react";
import { SHEET_DATA_COLUMN_MIN_WIDTH } from "./sheetColumnLayout";

const PREFIX = "gymkhana.sheet.colw.v1.";
const MIN = SHEET_DATA_COLUMN_MIN_WIDTH;
const MAX = 8192;

function read(scope: string): Record<string, number> {
  try {
    const raw = localStorage.getItem(PREFIX + scope);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    if (!parsed || typeof parsed !== "object") return {};
    const out: Record<string, number> = {};
    for (const [key, value] of Object.entries(parsed)) {
      if (typeof value === "number" && Number.isFinite(value)) {
        out[key] = Math.min(MAX, Math.max(MIN, Math.round(value)));
      }
    }
    return out;
  } catch {
    return {};
  }
}

function write(scope: string, value: Record<string, number>) {
  localStorage.setItem(PREFIX + scope, JSON.stringify(value));
}

export function clampColumnWidth(width: number): number {
  return Math.min(MAX, Math.max(MIN, Math.round(width)));
}

export function useSheetColumnWidths(scope: string) {
  const [widths, setWidths] = useState<Record<string, number>>(() =>
    typeof localStorage === "undefined" ? {} : read(scope),
  );

  useEffect(() => {
    setWidths(typeof localStorage === "undefined" ? {} : read(scope));
  }, [scope]);

  const setWidth = useCallback(
    (key: string, width: number) => {
      setWidths((current) => {
        const next = { ...current, [key]: clampColumnWidth(width) };
        write(scope, next);
        return next;
      });
    },
    [scope],
  );

  return { widths, setWidth };
}

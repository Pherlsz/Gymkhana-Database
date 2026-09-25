import { useCallback, useEffect, useState } from "react";

const PREFIX = "gymkhana.sheet.fx.v1.";

function read(scope: string): Record<string, string> {
  try {
    const raw = localStorage.getItem(PREFIX + scope);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as Record<string, string>;
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function write(scope: string, value: Record<string, string>) {
  localStorage.setItem(PREFIX + scope, JSON.stringify(value));
}

export function useSheetFormulas(scope: string) {
  const [formulas, setFormulas] = useState<Record<string, string>>(() =>
    typeof localStorage === "undefined" ? {} : read(scope),
  );

  useEffect(() => {
    setFormulas(typeof localStorage === "undefined" ? {} : read(scope));
  }, [scope]);

  const setFormula = useCallback((sourceKey: string, expression: string) => {
    setFormulas((current) => {
      const next = { ...current };
      if (!expression.trim()) delete next[sourceKey];
      else next[sourceKey] = expression.trim();
      write(scope, next);
      return next;
    });
  }, [scope]);

  return { formulas, setFormula };
}

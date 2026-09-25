import type { ChangeEvent } from "react";

export function phoneInputDigits(value: string): string {
  const digits = value.replace(/\D/g, "");
  if (digits.startsWith("55") && digits.length >= 12) {
    return digits.slice(2, 13);
  }
  return digits.slice(0, 11);
}

export function cpfDigits(value: string): string {
  return value.replace(/\D/g, "").slice(0, 11);
}

export function isCompleteCpf(value: string): boolean {
  return cpfDigits(value).length === 11;
}

export function matchesValidationRegex(value: string, regex: string): boolean {
  const pattern = regex.trim();
  if (!pattern) return true;
  try {
    const compiled = new RegExp(pattern);
    const trimmed = value.trim();
    if (compiled.test(trimmed)) return true;
    const digits = trimmed.replace(/\D/g, "");
    return digits.length > 0 && compiled.test(digits);
  } catch {
    return true;
  }
}

export function canonicalBillAmount(raw: string): string {
  const trimmed = raw.trim().replace(/[R$\s]/g, "");
  if (!trimmed) return "";
  if (trimmed.includes(",") && trimmed.includes(".")) {
    return trimmed.replace(/\./g, "").replace(",", ".");
  }
  if (trimmed.includes(",")) return trimmed.replace(",", ".");
  return trimmed;
}

export function isCompetenceMonth(value: string): boolean {
  return /^[0-9]{4}-(0[1-9]|1[0-2])$/.test(value.trim());
}

export function fileFromInput(event: ChangeEvent<HTMLInputElement>): File | undefined {
  return event.target.files?.[0] ?? undefined;
}

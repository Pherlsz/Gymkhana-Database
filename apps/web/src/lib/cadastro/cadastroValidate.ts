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

export function isValidCpf(value: string): boolean {
  const digits = cpfDigits(value);
  if (digits.length !== 11) return false;
  if (/^(\d)\1{10}$/.test(digits)) return false;
  const check = (len: number) => {
    let sum = 0;
    for (let i = 0; i < len; i += 1) {
      sum += Number(digits[i]) * (len + 1 - i);
    }
    const rem = (sum * 10) % 11;
    return rem === 10 ? 0 : rem;
  };
  return check(9) === Number(digits[9]) && check(10) === Number(digits[10]);
}

export type DocumentFieldCopy = {
  errorDocTypeRequired: string;
  errorDocNumberRequired: string;
  errorIdentifierRegex: string;
  errorDocDateRequired: string;
};

export function documentFieldError(
  fields: { typeId: string; identifier: string; date?: string | undefined },
  type: { validation_regex?: string | undefined; date_required?: boolean | undefined } | undefined,
  copy: DocumentFieldCopy,
): string | null {
  if (!fields.typeId && !type) return copy.errorDocTypeRequired;
  if (!fields.identifier.trim()) return copy.errorDocNumberRequired;
  const regex = type?.validation_regex ?? "";
  if (!matchesValidationRegex(fields.identifier, regex)) return copy.errorIdentifierRegex;
  if (type?.date_required && !fields.date) return copy.errorDocDateRequired;
  return null;
}

export type BillFieldCopy = {
  errorBillTypeRequired: string;
  errorBillInstallationRequired: string;
  errorBillCompetenceRequired: string;
  errorBillAmountRequired: string;
};

const billAmountPattern = /^[0-9]+(?:\.[0-9]{1,2})?$/;

export function billFieldError(
  fields: { typeId: string; installation: string; competence: string; amount: string },
  typeIdFallback: string | undefined,
  copy: BillFieldCopy,
): string | null {
  if (!fields.typeId && !typeIdFallback) return copy.errorBillTypeRequired;
  if (!fields.installation.trim()) return copy.errorBillInstallationRequired;
  if (!isCompetenceMonth(fields.competence)) return copy.errorBillCompetenceRequired;
  const amount = canonicalBillAmount(fields.amount);
  if (!amount || !billAmountPattern.test(amount)) return copy.errorBillAmountRequired;
  return null;
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

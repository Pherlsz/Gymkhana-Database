import { APIRequestError } from "./api/client";

export function formatDate(value: string): string {
  if (!value) return "";
  const isoStr = /^\d{4}-\d{2}-\d{2}$/.test(value) ? `${value}T00:00:00` : value;
  const date = new Date(isoStr);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString("pt-BR");
}

export function formatDateTime(value: string): string {
  if (!value) return "";
  try {
    return new Intl.DateTimeFormat("pt-BR", {
      dateStyle: "short",
      timeStyle: "short",
    }).format(new Date(value));
  } catch {
    return value;
  }
}

export function formatBytes(value: number): string {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

export function formatCPF(value: string): string {
  const digits = value.replace(/\D/g, "");
  if (digits.length !== 11) return value;
  return `${digits.slice(0, 3)}.${digits.slice(3, 6)}.${digits.slice(6, 9)}-${digits.slice(9, 11)}`;
}

export function formatAmount(amount: unknown, currency: unknown = "BRL"): string {
  const text = String(amount ?? "").trim();
  if (!text) return "";
  let number: number;
  if (typeof amount === "number") {
    number = amount;
  } else if (/^\d+[.,]\d{1,2}$/.test(text) || /^\d+$/.test(text)) {
    number = Number(text.replace(",", "."));
  } else {
    number = Number(text.replace(/\./g, "").replace(",", "."));
  }
  if (!Number.isFinite(number)) return text;
  try {
    return new Intl.NumberFormat("pt-BR", {
      style: "currency",
      currency: typeof currency === "string" && currency ? currency : "BRL",
    }).format(number);
  } catch {
    return text;
  }
}

export function errorMessage(error: unknown): string {
  if (error instanceof APIRequestError) {
    if (error.fieldErrors && error.fieldErrors.length > 0) {
      return error.fieldErrors.map((field) => `${field.field}: ${field.message}`).join(" · ");
    }
    return error.requestId ? `${error.message} (requisição ${error.requestId})` : error.message;
  }
  if (error instanceof Error) return error.message;
  return "Ocorreu um erro inesperado.";
}

export function errorAlertProps(
  title: string,
  error: unknown,
): { message: string; description?: string } {
  const detail = errorMessage(error).trim();
  const fold = (value: string) => value.replace(/[.]+$/u, "").trim();
  if (!detail || fold(detail) === fold(title)) return { message: title };
  return { message: title, description: detail };
}

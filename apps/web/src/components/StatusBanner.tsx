import { Alert } from "antd";
import type { ReactNode } from "react";
import { APIRequestError } from "../lib/api/client";
import { errorAlertProps } from "../lib/formatters";

export type StatusBannerTone = "success" | "error" | "warning" | "info";

export function StatusBanner({
  tone,
  title,
  description,
  error,
  conflictTitle,
  closable,
  onClose,
  action,
}: {
  tone?: StatusBannerTone | undefined;
  title: ReactNode;
  description?: ReactNode | undefined;
  error?: unknown;
  conflictTitle?: string | undefined;
  closable?: boolean | undefined;
  onClose?: (() => void) | undefined;
  action?: ReactNode | undefined;
}) {
  const conflict = error instanceof APIRequestError && error.status === 409;
  const stringTitle = typeof title === "string" ? title : undefined;
  const fromError =
    error !== undefined && stringTitle !== undefined && !conflict
      ? errorAlertProps(stringTitle, error)
      : undefined;
  const resolvedTone: StatusBannerTone = conflict
    ? "warning"
    : (tone ?? (error !== undefined ? "error" : "info"));
  const resolvedTitle = conflict ? (conflictTitle ?? title) : (fromError?.message ?? title);
  const resolvedDescription = conflict ? description : (description ?? fromError?.description);

  return (
    <Alert
      showIcon
      type={resolvedTone}
      title={resolvedTitle}
      {...(resolvedDescription !== undefined ? { description: resolvedDescription } : {})}
      {...(action !== undefined ? { action } : {})}
      {...(closable ? { closable: true, ...(onClose ? { onClose } : {}) } : {})}
    />
  );
}

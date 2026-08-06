import type { ReactNode } from "react";

type AlertTone = "info" | "success" | "warning" | "error" | "danger";

type AlertProps = {
  tone?: AlertTone;
  title?: string;
  children?: ReactNode;
};

export function Alert({ tone = "info", title, children }: AlertProps) {
  return (
    <div className={`alert alert-${tone}`}>
      {title && <strong>{title}</strong>}
      {children}
    </div>
  );
}

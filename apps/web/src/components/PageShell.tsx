import type { ReactNode } from "react";
import { PageHeader } from "./PageHeader";

export function PageShell({
  className,
  measure = false,
  title,
  description,
  actions,
  children,
}: {
  className: string;
  measure?: boolean | undefined;
  title?: string | undefined;
  description?: string | undefined;
  actions?: ReactNode | undefined;
  children?: ReactNode | undefined;
}) {
  const classes = [className, measure ? "page-measure" : null].filter(Boolean).join(" ");
  return (
    <div className={classes}>
      {title ? <PageHeader actions={actions} description={description} title={title} /> : null}
      {children}
    </div>
  );
}

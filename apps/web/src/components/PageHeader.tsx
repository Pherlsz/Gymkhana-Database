import { Typography } from "antd";
import type { ReactNode } from "react";

/**
 * The one page title treatment. Six variants had grown across the app, three of
 * them leaning on `.page-header` / `.page-title` classes that carried no CSS at
 * all. Actions sit beside the title and wrap under it when the line runs out.
 */
export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: string | undefined;
  actions?: ReactNode;
}) {
  return (
    <header className="page-header">
      <div className="page-header__copy">
        <Typography.Title className="page-header__title" level={1}>
          {title}
        </Typography.Title>
        {description ? (
          <Typography.Paragraph className="page-header__description">
            {description}
          </Typography.Paragraph>
        ) : null}
      </div>
      {actions ? <div className="page-header__actions">{actions}</div> : null}
    </header>
  );
}

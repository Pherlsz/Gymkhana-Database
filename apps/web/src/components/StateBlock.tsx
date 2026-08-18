import { Alert, Empty, Skeleton } from "antd";
import type { ReactNode } from "react";

export type StateBlockKind = "loading" | "empty" | "error";

/**
 * Empty, loading and error in one place. The app had eight empty states, twelve
 * loading states and four Alert treatments that disagreed on tone, spacing and
 * whether they announced themselves; this fixes all three to one shape.
 *
 * `error` is assertive because it reports a failed action the operator is
 * waiting on; `loading` is polite so it does not interrupt.
 */
export function StateBlock({
  kind,
  title,
  description,
  action,
}: {
  kind: StateBlockKind;
  title: string;
  description?: string | undefined;
  action?: ReactNode;
}) {
  if (kind === "loading") {
    return (
      <div aria-busy="true" aria-live="polite" className="state-block" role="status">
        <span className="visually-hidden">{title}</span>
        <Skeleton active paragraph={{ rows: 3 }} title={false} />
      </div>
    );
  }

  if (kind === "error") {
    return (
      <Alert
        action={action}
        className="state-block state-block--error"
        description={description}
        message={title}
        role="alert"
        showIcon
        type="error"
      />
    );
  }

  return (
    <div className="state-block state-block--empty">
      <Empty description={description ?? title} image={Empty.PRESENTED_IMAGE_SIMPLE}>
        {action}
      </Empty>
    </div>
  );
}

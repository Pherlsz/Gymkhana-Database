import { Spin } from "antd";
import { AlertCircle, AlertTriangle, Inbox } from "lucide-react";
import type { CSSProperties, ReactNode } from "react";

export type StateCardKind = "loading" | "empty" | "warning" | "error";

export interface StateCardProps {
  kind?: StateCardKind;
  title: ReactNode;
  description?: ReactNode;
  icon?: ReactNode;
  action?: ReactNode;
  compact?: boolean;
  className?: string;
  style?: CSSProperties;
}

function defaultIconForKind(kind: StateCardKind): ReactNode {
  switch (kind) {
    case "loading":
      return <Spin className="state-card__spinner" size="large" />;
    case "empty":
      return <Inbox aria-hidden="true" size={26} strokeWidth={1.75} />;
    case "warning":
      return <AlertTriangle aria-hidden="true" size={26} strokeWidth={1.75} />;
    case "error":
      return <AlertCircle aria-hidden="true" size={26} strokeWidth={1.75} />;
  }
}

/**
 * Standardized StateCard for screens and panels showing loading, empty,
 * warning/disabled, or error states.
 *
 * Implements a centered glassmorphic surface with glowing badge halo,
 * responsive typography, and action buttons slot.
 */
export function StateCard({
  kind = "empty",
  title,
  description,
  icon,
  action,
  compact = false,
  className = "",
  style,
}: StateCardProps) {
  const renderedIcon = icon ?? defaultIconForKind(kind);
  const isPolite = kind === "loading";
  const isAlert = kind === "error";

  return (
    <div
      aria-busy={kind === "loading" ? "true" : undefined}
      aria-live={isAlert ? "assertive" : isPolite ? "polite" : undefined}
      className={`state-card state-card--${kind}${compact ? " state-card--compact" : ""}${className ? ` ${className}` : ""}`}
      role={isAlert ? "alert" : isPolite ? "status" : undefined}
      style={style}
    >
      <div className={`state-card__badge state-card__badge--${kind}`}>
        {renderedIcon}
      </div>

      <h3 className="state-card__title">{title}</h3>

      {description ? (
        <div className="state-card__desc">{description}</div>
      ) : null}

      {action ? <div className="state-card__actions">{action}</div> : null}
    </div>
  );
}

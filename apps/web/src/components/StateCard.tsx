import { Button, Spin } from "antd";
import { AlertCircle, AlertTriangle, ArrowLeft, Inbox, RotateCcw } from "lucide-react";
import type { CSSProperties, ReactNode } from "react";
import { useI18n } from "../i18n";

export type StateCardKind = "loading" | "empty" | "warning" | "error";

export interface StateCardProps {
  kind?: StateCardKind;
  title: ReactNode;
  description?: ReactNode | undefined;
  icon?: ReactNode | undefined;
  action?: ReactNode | undefined;
  compact?: boolean | undefined;
  className?: string | undefined;
  style?: CSSProperties | undefined;
  onBack?: (() => void) | undefined;
  backLabel?: ReactNode | undefined;
  onRetry?: (() => void) | undefined;
  retryLabel?: ReactNode | undefined;
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

export function StateCard({
  kind = "empty",
  title,
  description,
  icon,
  action,
  compact = false,
  className = "",
  style,
  onBack,
  backLabel,
  onRetry,
  retryLabel,
}: StateCardProps) {
  const { messages } = useI18n();
  const renderedIcon = icon ?? defaultIconForKind(kind);
  const isPolite = kind === "loading";
  const isAlert = kind === "error";
  const hasStandardActions = Boolean(onBack || onRetry);

  return (
    <div
      aria-busy={kind === "loading" ? "true" : undefined}
      aria-live={isAlert ? "assertive" : isPolite ? "polite" : undefined}
      className={`state-card state-card--${kind}${compact ? " state-card--compact" : ""}${className ? ` ${className}` : ""}`}
      role={isAlert ? "alert" : isPolite ? "status" : undefined}
      style={style}
    >
      <div className={`state-card__badge state-card__badge--${kind}`}>{renderedIcon}</div>

      <h3 className="state-card__title">{title}</h3>

      {description ? <div className="state-card__desc">{description}</div> : null}

      {hasStandardActions || action ? (
        <div className="state-card__actions">
          {onRetry ? (
            <Button icon={<RotateCcw size={14} />} onClick={onRetry}>
              {retryLabel ?? messages.common.actions.retry}
            </Button>
          ) : null}

          {onBack ? (
            <Button icon={<ArrowLeft size={14} />} onClick={onBack}>
              {backLabel ?? messages.common.actions.back}
            </Button>
          ) : null}

          {action}
        </div>
      ) : null}
    </div>
  );
}

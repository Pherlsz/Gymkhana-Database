import { Link } from "@tanstack/react-router";
import { ChevronRight, Inbox } from "lucide-react";
import type { CSSProperties, ReactNode } from "react";
import { ICON, ICON_STROKE } from "./icons";
import { useI18n } from "../i18n";

export type AppCardVariant =
  | "default"
  | "people"
  | "documents"
  | "identity"
  | "bills"
  | "work"
  | "health"
  | "education"
  | "certificates"
  | "forms"
  | "bulk"
  | "other";

export interface AppCardProps {
  id?: string;
  variant?: AppCardVariant;
  className?: string;
  style?: CSSProperties;
  icon?: ReactNode;
  title?: ReactNode;
  subtitle?: ReactNode;
  badge?: ReactNode;
  chevron?: boolean;

  open?: boolean;
  onToggle?: () => void;

  to?: string;
  search?: Record<string, unknown>;

  onClick?: () => void;
  onActivate?: () => void;

  loading?: boolean;
  empty?: boolean;
  emptyTitle?: ReactNode;
  emptyDescription?: ReactNode;
  emptyIcon?: ReactNode;
  emptyAction?: ReactNode;
  loadingRows?: number;

  children?: ReactNode;
  "aria-label"?: string;
}

export function AppCard({
  id,
  variant = "default",
  className = "",
  style,
  icon,
  title,
  subtitle,
  badge,
  chevron,
  open,
  onToggle,
  to,
  search,
  onClick,
  onActivate,
  loading,
  empty,
  emptyTitle,
  emptyDescription,
  emptyIcon,
  emptyAction,
  loadingRows = 3,
  children,
  "aria-label": ariaLabel,
}: AppCardProps) {
  const handleClick = onActivate ?? onClick;
  const isAccordion = typeof onToggle === "function";
  const showChevron = chevron ?? isAccordion;
  const isInteractive = Boolean(handleClick || isAccordion || to);
  const { messages } = useI18n();

  const header = (
    <div className="app-card__header">
      {icon ? (
        <span aria-hidden="true" className="app-card__icon">
          {icon}
        </span>
      ) : null}
      {title || subtitle ? (
        <div className="app-card__head-text">
          {title ? <span className="app-card__title">{title}</span> : null}
          {subtitle ? <span className="app-card__subtitle">{subtitle}</span> : null}
        </div>
      ) : null}
      {loading && !badge ? (
        <span aria-hidden="true" className="app-card__badge app-card__badge--loading" />
      ) : badge ? (
        <span className="app-card__badge">{badge}</span>
      ) : null}
      {showChevron ? (
        <ChevronRight
          aria-hidden
          className={`app-card__chevron ${open ? "app-card__chevron--open" : ""}`}
          size={ICON.md}
          strokeWidth={ICON_STROKE}
        />
      ) : null}
    </div>
  );

  const renderBody = () => {
    if (loading) {
      return (
        <div aria-busy="true" aria-live="polite" className="app-card__loading" role="status">
          {Array.from({ length: loadingRows }).map((_, idx) => (
            <div className="app-card__loading-row" key={idx}>
              <span className="app-card__loading-icon" />
              <span className="app-card__loading-text" />
              <span className="app-card__loading-pill" />
            </div>
          ))}
        </div>
      );
    }

    if (empty) {
      return (
        <div className="app-card__empty" role="status">
          <span aria-hidden="true" className="app-card__empty-icon">
            {emptyIcon ?? <Inbox size={22} strokeWidth={ICON_STROKE} />}
          </span>
          <span className="app-card__empty-title">
            {emptyTitle ?? messages.common.status.empty}
          </span>
          {emptyDescription ? (
            <span className="app-card__empty-desc">{emptyDescription}</span>
          ) : null}
          {emptyAction ? (
            <div className="app-card__empty-action">{emptyAction}</div>
          ) : null}
        </div>
      );
    }

    return children;
  };

  const hasBody = Boolean(loading || empty || children);

  const content = (
    <>
      {isAccordion ? (
        <button
          aria-expanded={open}
          className="app-card__trigger"
          onClick={onToggle}
          type="button"
        >
          {header}
        </button>
      ) : (
        header
      )}
      {(!isAccordion || open) && hasBody ? (
        <div className="app-card__body">{renderBody()}</div>
      ) : null}
    </>
  );

  const classes =
    `app-card app-card--${variant} ${isAccordion ? "app-card--accordion" : ""} ${isInteractive ? "app-card--interactive" : ""} ${open ? "app-card--open" : ""} ${className}`.trim();

  if (to) {
    return (
      <Link
        aria-label={ariaLabel}
        className={classes}
        id={id}
        onClick={handleClick}
        style={style}
        to={to}
        {...(search ? { search } : {})}
      >
        {content}
      </Link>
    );
  }

  return (
    <div
      aria-label={ariaLabel}
      className={classes}
      id={id}
      onClick={isAccordion ? undefined : handleClick}
      onKeyDown={(event) => {
        if (!isAccordion && handleClick && (event.key === "Enter" || event.key === " ")) {
          event.preventDefault();
          handleClick();
        }
      }}
      role={isInteractive && !isAccordion ? "button" : undefined}
      style={style}
      tabIndex={isInteractive && !isAccordion ? 0 : undefined}
    >
      {content}
    </div>
  );
}

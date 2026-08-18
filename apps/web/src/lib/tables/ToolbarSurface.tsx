import { Button, Drawer, Popover } from "antd";
import { ChevronDown } from "lucide-react";
import { useCallback, useEffect, useId, useRef, type KeyboardEvent, type ReactNode } from "react";
import { atLeast } from "../breakpoints";
import { useMediaQuery } from "../useMediaQuery";
import { SHEET_INSPECTOR_SHEET_SIZE } from "./sheetDefaults";

/**
 * One surface per toolbar subject, so opening Colunas or Filtros never pushes
 * the grid down. Anchored to its trigger from `sm` up and a bottom sheet below
 * it, with the same body in both — which is what lets the column funnel in
 * Orchestration 12.1.1 mount this body in a header cell later without a rewrite.
 */
export function ToolbarSurface({
  label,
  title,
  icon,
  count,
  active = false,
  open,
  onOpenChange,
  children,
}: {
  label: string;
  title: string;
  icon: ReactNode;
  count?: number | undefined;
  active?: boolean | undefined;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: ReactNode;
}) {
  const surfaceId = useId();
  const anchored = useMediaQuery(atLeast("sm"));
  const bodyRef = useRef<HTMLDivElement>(null);

  // Ant moves focus into a Drawer on its own; a Popover leaves it on the
  // trigger, so keyboard users would tab through the page to reach the panel.
  useEffect(() => {
    if (!open || !anchored) return;
    const frame = requestAnimationFrame(() => {
      const body = bodyRef.current;
      if (!body) return;
      const focusable = body.querySelector<HTMLElement>(
        "input:not([disabled]), button:not([disabled]), [tabindex]:not([tabindex='-1'])",
      );
      (focusable ?? body).focus();
    });
    return () => cancelAnimationFrame(frame);
  }, [open, anchored]);

  const closeOnEscape = useCallback(
    (event: KeyboardEvent<HTMLDivElement>) => {
      if (event.key !== "Escape") return;
      event.stopPropagation();
      onOpenChange(false);
    },
    [onOpenChange],
  );

  const body = (
    <div
      aria-label={title}
      className="toolbar-surface__body"
      id={surfaceId}
      ref={bodyRef}
      role="dialog"
      tabIndex={-1}
      onKeyDown={closeOnEscape}
    >
      {children}
    </div>
  );

  const trigger = (
    <Button
      aria-controls={open ? surfaceId : undefined}
      aria-expanded={open}
      aria-haspopup="dialog"
      className={active || open ? "toolbar-surface__trigger is-active" : "toolbar-surface__trigger"}
      icon={icon}
      onClick={() => onOpenChange(!open)}
      onKeyDown={(event) => {
        if (event.key === "Escape" && open) onOpenChange(false);
      }}
    >
      <span className="toolbar-surface__label">{label}</span>
      {count ? <span className="toolbar-surface__count">{count}</span> : null}
      <ChevronDown
        aria-hidden
        className={open ? "toolbar-surface__chevron is-open" : "toolbar-surface__chevron"}
        size={14}
        strokeWidth={1.75}
      />
    </Button>
  );

  if (!anchored) {
    return (
      <>
        {trigger}
        <Drawer
          className="toolbar-surface__drawer"
          height={SHEET_INSPECTOR_SHEET_SIZE}
          open={open}
          placement="bottom"
          title={title}
          onClose={() => onOpenChange(false)}
        >
          {body}
        </Drawer>
      </>
    );
  }

  return (
    <Popover
      arrow={false}
      classNames={{ root: "toolbar-surface__popover" }}
      content={body}
      open={open}
      placement="bottomLeft"
      trigger="click"
      onOpenChange={onOpenChange}
    >
      {trigger}
    </Popover>
  );
}

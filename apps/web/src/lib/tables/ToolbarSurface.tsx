import { Button, Drawer, Popover } from "antd";
import { ChevronDown } from "lucide-react";
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { atLeast } from "../breakpoints";
import { useMediaQuery } from "../useMediaQuery";
import { SHEET_INSPECTOR_SHEET_SIZE } from "./sheetDefaults";
import { ICON, ICON_STROKE } from "../../components/icons";

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

  const body = useMemo(
    () => (
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
    ),
    [title, surfaceId, closeOnEscape, children],
  );

  const trigger = (
    <Button
      aria-controls={open ? surfaceId : undefined}
      aria-expanded={open}
      aria-haspopup="dialog"
      aria-label={label}
      className={active || open ? "toolbar-surface__trigger is-active" : "toolbar-surface__trigger"}
      icon={icon}
      onClick={() => onOpenChange(!open)}
      onKeyDown={(event) => {
        if (event.key === "Escape" && open) onOpenChange(false);
      }}
    >
      <span className="toolbar-surface__label">{label}</span>
      {count ? (
        <span aria-hidden className="toolbar-surface__count">
          {count}
        </span>
      ) : null}
      <ChevronDown
        aria-hidden
        className={open ? "toolbar-surface__chevron is-open" : "toolbar-surface__chevron"}
        size={ICON.sm}
        strokeWidth={ICON_STROKE}
      />
    </Button>
  );

  if (!anchored) {
    return (
      <>
        {trigger}
        <Drawer
          className="toolbar-surface__drawer"
          destroyOnClose
          height={SHEET_INSPECTOR_SHEET_SIZE}
          open={open}
          placement="bottom"
          title={undefined}
          onClose={() => onOpenChange(false)}
        >
          {open ? body : null}
        </Drawer>
      </>
    );
  }

  return (
    <Popover
      arrow={false}
      classNames={{ root: "toolbar-surface__popover" }}
      content={open ? body : null}
      destroyTooltipOnHide
      open={open}
      placement="bottomLeft"
      trigger="click"
      onOpenChange={onOpenChange}
    >
      {trigger}
    </Popover>
  );
}

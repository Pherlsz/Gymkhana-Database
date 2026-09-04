import { Spin } from "antd";
import { Inbox } from "lucide-react";

export type InlineStatusKind = "loading" | "empty";

/**
 * Lightweight in-panel status message for loading and empty states.
 * Use this inside cards, panels and list sections where the heavier
 * StateCard surface (card + glow badge) would be visually disproportionate.
 *
 * – `loading` renders a small spinner with a polite live region.
 * – `empty`   renders a muted paragraph with an optional inbox icon.
 */
export function InlineStatus({
  kind,
  label,
  showIcon = true,
}: {
  kind: InlineStatusKind;
  label: string;
  showIcon?: boolean;
}) {
  if (kind === "loading") {
    return (
      <p aria-live="polite" className="inline-status inline-status--loading" role="status">
        <Spin aria-hidden size="small" />
        <span>{label}</span>
      </p>
    );
  }

  return (
    <p className="inline-status inline-status--empty">
      {showIcon ? <Inbox aria-hidden size={14} strokeWidth={1.75} /> : null}
      <span>{label}</span>
    </p>
  );
}

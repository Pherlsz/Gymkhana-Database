import { Spin } from "antd";
import { Inbox } from "lucide-react";

export type InlineStatusKind = "loading" | "empty";

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

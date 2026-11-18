import { Spin } from "antd";
import { Inbox } from "lucide-react";

export type InlineStatusKind = "loading" | "empty";

export function InlineStatus({
  kind,
  label,
  showIcon = true,
  className = "",
}: {
  kind: InlineStatusKind;
  label: string;
  showIcon?: boolean;
  className?: string;
}) {
  const extra = className ? ` ${className}` : "";
  if (kind === "loading") {
    return (
      <div
        aria-live="polite"
        className={`inline-status inline-status--loading${extra}`}
        role="status"
      >
        <Spin aria-hidden size="small" />
        <span>{label}</span>
      </div>
    );
  }

  return (
    <div className={`inline-status inline-status--empty${extra}`}>
      {showIcon ? <Inbox aria-hidden size={14} strokeWidth={1.75} /> : null}
      <span>{label}</span>
    </div>
  );
}

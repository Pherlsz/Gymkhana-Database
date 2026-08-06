type StatusBadgeTone = "neutral" | "success" | "warning" | "error" | "info" | "danger";

type StatusBadgeProps = {
  tone?: StatusBadgeTone;
  children: React.ReactNode;
};

export function StatusBadge({ tone = "neutral", children }: StatusBadgeProps) {
  return <span className={`status-badge status-badge-${tone}`}>{children}</span>;
}

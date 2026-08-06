import React from "react";

interface StatusBadgeProps {
  variant?: "default" | "success" | "warning" | "error" | "info";
  tone?: "default" | "success" | "warning" | "error" | "info" | "danger" | "neutral";
  children?: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
  [key: string]: any;
}

export function StatusBadge({ variant, tone, children, className, style, ...props }: StatusBadgeProps) {
  const v = variant || tone || "default";
  const colors = {
    default: { bg: "#f3f4f6", text: "#374151" },
    success: { bg: "#d1fae5", text: "#065f46" },
    warning: { bg: "#fef3c7", text: "#92400e" },
    error: { bg: "#fee2e2", text: "#991b1b" },
    danger: { bg: "#fee2e2", text: "#991b1b" },
    info: { bg: "#dbeafe", text: "#1e40af" },
    neutral: { bg: "#f3f4f6", text: "#374151" },
  };

  const c = colors[v] || colors.default;

  return (
    <span
      className={className}
      style={{
        display: "inline-flex",
        alignItems: "center",
        padding: "2px 8px",
        borderRadius: "9999px",
        fontSize: "12px",
        fontWeight: 500,
        backgroundColor: c.bg,
        color: c.text,
        ...style,
      }}
      {...props}
    >
      {children}
    </span>
  );
}

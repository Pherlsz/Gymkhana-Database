import React from "react";

interface AlertProps {
  variant?: "info" | "success" | "warning" | "error" | "danger";
  tone?: "info" | "success" | "warning" | "error" | "danger" | "neutral";
  title?: string;
  children?: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
  [key: string]: any;
}

export function Alert({ variant, tone, title, children, className, style, ...props }: AlertProps) {
  const v = variant || tone || "info";
  const colors = {
    info: { bg: "#dbeafe", border: "#3b82f6", text: "#1e40af" },
    success: { bg: "#d1fae5", border: "#10b981", text: "#065f46" },
    warning: { bg: "#fef3c7", border: "#f59e0b", text: "#92400e" },
    error: { bg: "#fee2e2", border: "#ef4444", text: "#991b1b" },
    danger: { bg: "#fee2e2", border: "#ef4444", text: "#991b1b" },
    neutral: { bg: "#f3f4f6", border: "#9ca3af", text: "#374151" },
  };

  const c = colors[v] || colors.info;

  return (
    <div
      className={className}
      style={{
        padding: "12px 16px",
        backgroundColor: c.bg,
        borderLeft: `4px solid ${c.border}`,
        borderRadius: "6px",
        color: c.text,
        fontSize: "14px",
        ...style,
      }}
      {...props}
    >
      {title && <div style={{ fontWeight: 600, marginBottom: "4px" }}>{title}</div>}
      {children}
    </div>
  );
}

import React, { ButtonHTMLAttributes } from "react";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "danger";
  size?: "sm" | "md" | "lg";
}

export function Button({
  variant = "primary",
  size = "md",
  children,
  style,
  className,
  ...props
}: ButtonProps) {
  const baseStyle: React.CSSProperties = {
    display: "inline-flex",
    alignItems: "center",
    justifyContent: "center",
    fontWeight: 500,
    borderRadius: "6px",
    border: "none",
    cursor: "pointer",
    transition: "all 0.2s",
    padding: size === "sm" ? "6px 12px" : size === "lg" ? "12px 24px" : "8px 16px",
    fontSize: size === "sm" ? "13px" : size === "lg" ? "16px" : "14px",
  };

  const variantStyle: React.CSSProperties =
    variant === "primary"
      ? { backgroundColor: "#2563eb", color: "white" }
      : variant === "danger"
      ? { backgroundColor: "#dc2626", color: "white" }
      : { backgroundColor: "#f3f4f6", color: "#111827" };

  return (
    <button className={className} style={{ ...baseStyle, ...variantStyle, ...style }} {...props}>
      {children}
    </button>
  );
}

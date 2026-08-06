import React from "react";

interface SurfaceProps {
  children?: React.ReactNode;
  padding?: string | number;
  tone?: string;
  className?: string;
  style?: React.CSSProperties;
  [key: string]: any;
}

export function Surface({ children, padding = "16px", tone, className, style, ...props }: SurfaceProps) {
  return (
    <div
      className={className}
      style={{
        padding,
        backgroundColor: "white",
        borderRadius: "8px",
        border: "1px solid #e5e7eb",
        boxShadow: "0 1px 2px 0 rgba(0, 0, 0, 0.05)",
        ...style,
      }}
      {...props}
    >
      {children}
    </div>
  );
}

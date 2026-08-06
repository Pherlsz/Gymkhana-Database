import React from "react";

interface StackProps {
  children?: React.ReactNode;
  gap?: string | number;
  className?: string;
  style?: React.CSSProperties;
  [key: string]: any;
}

export function Stack({ children, gap = "16px", className, style, ...props }: StackProps) {
  return (
    <div
      className={className}
      style={{ display: "flex", flexDirection: "column", gap, ...style }}
      {...props}
    >
      {children}
    </div>
  );
}

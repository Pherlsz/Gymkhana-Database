import React from "react";

interface InlineProps {
  children?: React.ReactNode;
  gap?: string | number;
  align?: "flex-start" | "center" | "flex-end" | "stretch" | "end";
  justify?: string;
  wrap?: boolean;
  className?: string;
  style?: React.CSSProperties;
  [key: string]: any;
}

export function Inline({ children, gap = "8px", align = "center", justify, wrap, className, style, ...props }: InlineProps) {
  const alignItems = align === "end" ? "flex-end" : align;
  return (
    <div
      className={className}
      style={{
        display: "flex",
        gap,
        alignItems,
        justifyContent: justify,
        flexWrap: wrap ? "wrap" : undefined,
        ...style,
      }}
      {...props}
    >
      {children}
    </div>
  );
}

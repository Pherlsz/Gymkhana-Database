import type { ReactNode, HTMLAttributes } from "react";

type InlineProps = HTMLAttributes<HTMLDivElement> & {
  gap?: string | number;
  align?: "start" | "center" | "end" | "stretch";
  justify?: "start" | "center" | "end" | "between";
  wrap?: boolean;
  className?: string;
  children: ReactNode;
};

export function Inline({ gap = "0.5rem", align = "center", justify = "start", wrap = false, className, children, style, ...rest }: InlineProps) {
  const gapValue = typeof gap === "number" ? `${gap * 0.25}rem` : gap;
  return (
    <div
      className={`inline ${className || ""}`}
      {...rest}
      style={{
        display: "flex",
        gap: gapValue,
        alignItems: align,
        justifyContent: justify === "between" ? "space-between" : `flex-${justify}`,
        flexWrap: wrap ? "wrap" : "nowrap",
        ...style,
      }}
    >
      {children}
    </div>
  );
}

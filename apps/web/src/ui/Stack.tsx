import type { ReactNode } from "react";

type StackProps = {
  gap?: string | number;
  children: ReactNode;
};

export function Stack({ gap = "1rem", children }: StackProps) {
  const gapValue = typeof gap === "number" ? `${gap * 0.25}rem` : gap;
  return (
    <div className="stack" style={{ display: "flex", flexDirection: "column", gap: gapValue }}>
      {children}
    </div>
  );
}

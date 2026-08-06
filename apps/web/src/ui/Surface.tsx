import type { ReactNode } from "react";

type SurfaceTone = "raised" | "sunken" | "neutral";

type SurfaceProps = {
  padding?: string;
  className?: string;
  tone?: SurfaceTone;
  children: ReactNode;
};

export function Surface({ padding = "1rem", className, tone = "neutral", children }: SurfaceProps) {
  return (
    <div className={`surface surface-${tone} ${className || ""}`} style={{ padding }}>
      {children}
    </div>
  );
}

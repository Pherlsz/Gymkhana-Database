import { useSyncExternalStore } from "react";

const QUERIES = {
  four: "(min-width: 1400px)",
  three: "(min-width: 1000px)",
  two: "(min-width: 640px)",
} as const;

function getColumnsSnapshot(): number {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return 4;
  if (window.matchMedia(QUERIES.four).matches) return 4;
  if (window.matchMedia(QUERIES.three).matches) return 3;
  if (window.matchMedia(QUERIES.two).matches) return 2;
  return 1;
}

function subscribe(callback: () => void) {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return () => {};
  }
  const mqls = [
    window.matchMedia(QUERIES.four),
    window.matchMedia(QUERIES.three),
    window.matchMedia(QUERIES.two),
  ];
  mqls.forEach((mql) => mql.addEventListener("change", callback));
  return () => {
    mqls.forEach((mql) => mql.removeEventListener("change", callback));
  };
}

export function useContainerColumns(_ref?: unknown): number {
  return useSyncExternalStore(subscribe, getColumnsSnapshot, () => 4);
}

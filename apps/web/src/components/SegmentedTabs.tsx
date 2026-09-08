import { Segmented } from "antd";
import type { ReactNode } from "react";

export type SegmentedTabItem = {
  key: string;
  label: ReactNode;
  disabled?: boolean;
  title?: string;
};

export function SegmentedTabs({
  label,
  items,
  value,
  onChange,
}: {
  label: string;
  items: SegmentedTabItem[];
  value: string;
  onChange: (key: string) => void;
}) {
  return (
    <Segmented
      aria-label={label}
      className="segmented-tabs"
      onChange={(next) => onChange(String(next))}
      options={items.map((item) => ({
        label: item.title ? <span title={item.title}>{item.label}</span> : item.label,
        value: item.key,
        ...(item.disabled ? { disabled: true as const } : {}),
      }))}
      value={value}
    />
  );
}

import { Segmented } from "antd";
import type { ReactNode } from "react";

export type SegmentedTabItem = {
  key: string;
  label: ReactNode;
};

/**
 * One tab control. The app ran three: `.custom-data-tabs`, `.profile-sections`
 * and bare `Button role="tab"` sets, the last of which announced tabs without
 * ever exposing a tablist or panel relationship. Ant's Segmented owns the
 * roving focus and selected state.
 */
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
      options={items.map((item) => ({ label: item.label, value: item.key }))}
      value={value}
    />
  );
}

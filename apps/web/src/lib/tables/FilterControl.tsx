import type { ColumnPredicate, FunnelKind } from "./columnPredicate";

type FilterBase = {
  funnelKind?: FunnelKind | undefined;
  predicate?: ColumnPredicate | undefined;
  onPredicate?: ((next: ColumnPredicate) => void) | undefined;
};

export type ToolbarFilterField = FilterBase &
  (
    | {
        key: string;
        kind: "text";
        label: string;
        value: string;
        local?: boolean | undefined;
        onChange: (value: string) => void;
      }
    | {
        key: string;
        kind: "date";
        label: string;
        value: string;
        local?: boolean | undefined;
        onChange: (value: string) => void;
      }
    | {
        key: string;
        kind: "select";
        label: string;
        value: string;
        local?: boolean | undefined;
        options: { value: string; label: string }[];
        groups?: { label: string; options: { value: string; label: string }[] }[] | undefined;
        allLabel: string;
        onChange: (value: string) => void;
      }
  );

export function filterValuePreview(field: ToolbarFilterField): string {
  if (field.kind !== "select") return field.value;
  const option = field.options.find((item) => item.value === field.value);
  if (option) return option.label;
  const grouped = field.groups
    ?.flatMap((group) => group.options)
    .find((item) => item.value === field.value);
  return grouped?.label ?? field.value;
}

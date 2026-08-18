import { Input, Select } from "antd";
import type { DefaultOptionType } from "antd/es/select";
import { useEffect, useState } from "react";

export type ToolbarFilterField =
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
    };

/**
 * The value control for one column filter. Text and date commit on blur or Enter
 * so typing does not refetch per keystroke; select commits immediately.
 */
export function FilterControl({
  field,
  autoFocus = false,
}: {
  field: ToolbarFilterField;
  autoFocus?: boolean;
}) {
  const [draft, setDraft] = useState(field.value);
  useEffect(() => setDraft(field.value), [field.key, field.value]);

  const commitDraft = () => {
    if (draft !== field.value) field.onChange(draft);
  };

  if (field.kind === "select") {
    const selectOptions: DefaultOptionType[] = field.groups?.length
      ? field.groups.map((group) => ({ label: group.label, options: group.options }))
      : field.options;
    return (
      <Select
        allowClear
        aria-label={field.label}
        autoFocus={autoFocus}
        optionFilterProp="label"
        options={selectOptions}
        placeholder={field.allLabel || field.label}
        popupMatchSelectWidth
        showSearch
        style={{ width: "100%" }}
        value={field.value || undefined}
        onChange={(value) => field.onChange(value ?? "")}
      />
    );
  }

  return (
    <Input
      allowClear
      aria-label={field.label}
      autoFocus={autoFocus}
      placeholder={field.label}
      style={{ width: "100%" }}
      value={draft}
      {...(field.kind === "date" ? { type: "date" } : {})}
      onBlur={commitDraft}
      onChange={(event) => setDraft(event.target.value)}
      onPressEnter={(event) => event.currentTarget.blur()}
    />
  );
}

/** The human-readable value of an applied filter, for chips and list rows. */
export function filterValuePreview(field: ToolbarFilterField): string {
  if (field.kind !== "select") return field.value;
  const option = field.options.find((item) => item.value === field.value);
  if (option) return option.label;
  const grouped = field.groups
    ?.flatMap((group) => group.options)
    .find((item) => item.value === field.value);
  return grouped?.label ?? field.value;
}

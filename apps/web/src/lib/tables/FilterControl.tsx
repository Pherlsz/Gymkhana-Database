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

export function FilterControl({
  field,
  autoFocus = false,
}: {
  field: ToolbarFilterField;
  autoFocus?: boolean;
}) {
  const [draft, setDraft] = useState(field.value);
  const [prevValue, setPrevValue] = useState(field.value);
  if (field.value !== prevValue) {
    setPrevValue(field.value);
    setDraft(field.value);
  }

  const commitDraft = () => {
    if (draft !== field.value) field.onChange(draft);
  };

  useEffect(() => {
    if (draft === field.value) return;
    const timer = setTimeout(() => {
      field.onChange(draft);
    }, 450);
    return () => clearTimeout(timer);
  }, [draft, field]);

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
        placeholder={field.allLabel || `Selecionar ${field.label.toLowerCase()}...`}
        popupMatchSelectWidth
        showSearch
        style={{ width: "100%" }}
        value={field.value || undefined}
        onChange={(value) => field.onChange(value ?? "")}
      />
    );
  }

  const placeholder =
    field.kind === "date" ? "Selecionar data" : `Filtrar por ${field.label.toLowerCase()}...`;

  return (
    <Input
      allowClear
      aria-label={field.label}
      autoFocus={autoFocus}
      placeholder={placeholder}
      style={{ width: "100%" }}
      value={draft}
      {...(field.kind === "date" ? { type: "date" } : {})}
      onBlur={commitDraft}
      onChange={(event) => setDraft(event.target.value)}
      onPressEnter={(event) => {
        commitDraft();
        event.currentTarget.blur();
      }}
    />
  );
}

export function filterValuePreview(field: ToolbarFilterField): string {
  if (field.kind !== "select") return field.value;
  const option = field.options.find((item) => item.value === field.value);
  if (option) return option.label;
  const grouped = field.groups
    ?.flatMap((group) => group.options)
    .find((item) => item.value === field.value);
  return grouped?.label ?? field.value;
}

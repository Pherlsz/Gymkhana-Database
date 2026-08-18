import { Input } from "antd";
import { Search } from "lucide-react";
import { ICON, ICON_STROKE } from "../../components/icons";

/**
 * One search field for the table toolbar and the column picker. Both used to
 * ship a different chrome (Ant Search vs Input + Lucide), which made the same
 * action look like two products.
 */
export function SheetSearch({
  label,
  placeholder,
  value,
  onChange,
  onSubmit,
}: {
  label: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
  onSubmit?: (value: string) => void;
}) {
  return (
    <Input
      allowClear
      aria-label={label}
      className="sheet-search"
      placeholder={placeholder}
      prefix={<Search aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />}
      value={value}
      onBlur={() => onSubmit?.(value)}
      onChange={(event) => {
        const next = event.target.value;
        onChange(next);
        if (next === "") onSubmit?.("");
      }}
      onPressEnter={(event) => {
        event.currentTarget.blur();
        onSubmit?.(value);
      }}
    />
  );
}

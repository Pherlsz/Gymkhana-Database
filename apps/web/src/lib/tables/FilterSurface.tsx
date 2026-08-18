import { Button, Empty, Input } from "antd";
import { Search } from "lucide-react";
import { useMemo, useState } from "react";
import { FilterControl, type ToolbarFilterField } from "./FilterControl";
import { ICON, ICON_STROKE } from "../../components/icons";

/**
 * One labeled control per column, in a two-column grid that fills the
 * surface. Expanding a name to reveal the value was an extra click and left
 * the panel's width unused. Search still narrows the list; applied columns
 * stay first so a long sheet never buries what is in force.
 */
export function FilterSurface({
  searchFieldsLabel,
  noFieldsLabel,
  appliedLabel,
  clearLabel,
  localHint,
  fields,
}: {
  searchFieldsLabel: string;
  noFieldsLabel: string;
  appliedLabel: string;
  clearLabel: string;
  localHint: string;
  fields: ToolbarFilterField[];
}) {
  const [query, setQuery] = useState("");

  const matches = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return fields;
    return fields.filter((field) => field.label.toLowerCase().includes(needle));
  }, [fields, query]);

  const ordered = useMemo(() => {
    const applied = matches.filter((field) => field.value.trim());
    const rest = matches.filter((field) => !field.value.trim());
    return [...applied, ...rest];
  }, [matches]);

  const appliedCount = fields.filter((field) => field.value.trim()).length;
  const showsLocalHint = ordered.some((field) => field.local);

  return (
    <div className="filter-surface">
      <Input
        allowClear
        aria-label={searchFieldsLabel}
        className="filter-surface__search"
        placeholder={searchFieldsLabel}
        prefix={<Search aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />}
        value={query}
        onChange={(event) => setQuery(event.target.value)}
      />

      {ordered.length === 0 ? (
        <Empty description={noFieldsLabel} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <div aria-label={appliedLabel} className="filter-surface__list" role="group">
          {ordered.map((field) => (
            <div
              className={
                field.value.trim() ? "filter-surface__field is-applied" : "filter-surface__field"
              }
              key={field.key}
            >
              <span className="filter-surface__name">{field.label}</span>
              <div className="filter-surface__control">
                <FilterControl field={field} />
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="filter-surface__footer">
        {showsLocalHint ? <p className="filter-surface__hint">{localHint}</p> : null}
        {appliedCount > 0 ? (
          <Button
            size="small"
            type="text"
            onClick={() => {
              for (const field of fields) {
                if (field.value.trim()) field.onChange("");
              }
            }}
          >
            {clearLabel}
          </Button>
        ) : null}
      </div>
    </div>
  );
}

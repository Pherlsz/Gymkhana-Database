import { Button, Empty, Input } from "antd";
import { ChevronDown, Search } from "lucide-react";
import { useMemo, useState } from "react";
import { FilterControl, filterValuePreview, type ToolbarFilterField } from "./FilterControl";

/**
 * Column-first filtering, the shape a spreadsheet user expects: pick the column,
 * then narrow it. One searchable list, no quick/advanced split, applied columns
 * lifted to the top. This is the same body Orchestration 12.1.1 wants inside the
 * header funnel, reached from the bar until the funnel exists.
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
  const [expandedKey, setExpandedKey] = useState<string | null>(null);

  const matches = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return fields;
    return fields.filter((field) => field.label.toLowerCase().includes(needle));
  }, [fields, query]);

  // Applied columns first so a long list never buries the filters in force.
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
        prefix={<Search aria-hidden size={15} strokeWidth={1.75} />}
        value={query}
        onChange={(event) => setQuery(event.target.value)}
      />

      {ordered.length === 0 ? (
        <Empty description={noFieldsLabel} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <ul aria-label={appliedLabel} className="filter-surface__list">
          {ordered.map((field) => {
            const applied = Boolean(field.value.trim());
            const expanded = expandedKey === field.key;
            return (
              <li
                className={applied ? "filter-surface__row is-applied" : "filter-surface__row"}
                key={field.key}
              >
                <button
                  aria-expanded={expanded}
                  aria-pressed={applied}
                  className="filter-surface__toggle"
                  type="button"
                  onClick={() => setExpandedKey(expanded ? null : field.key)}
                >
                  <span className="filter-surface__name">{field.label}</span>
                  {applied ? (
                    <span className="filter-surface__value">{filterValuePreview(field)}</span>
                  ) : null}
                  <ChevronDown
                    aria-hidden
                    className={
                      expanded ? "filter-surface__chevron is-open" : "filter-surface__chevron"
                    }
                    size={14}
                    strokeWidth={1.75}
                  />
                </button>
                {expanded ? (
                  <div className="filter-surface__control">
                    <FilterControl autoFocus field={field} />
                  </div>
                ) : null}
              </li>
            );
          })}
        </ul>
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
              setExpandedKey(null);
            }}
          >
            {clearLabel}
          </Button>
        ) : null}
      </div>
    </div>
  );
}

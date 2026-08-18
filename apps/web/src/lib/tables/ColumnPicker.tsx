import { Button, Checkbox, Empty, Input } from "antd";
import { LockKeyhole, Search } from "lucide-react";
import { useMemo, useState } from "react";

export type ColumnPickerItem = {
  key: string;
  label: string;
  group: string;
  visible: boolean;
  locked: boolean;
};

/**
 * Columns as a grouped, searchable list instead of a flat checkbox grid. People
 * carries ~57 columns; a grid of that size is a wall, and the group headings plus
 * the per-group count are what make it scannable at a glance.
 */
export function ColumnPicker({
  searchLabel,
  showAllLabel,
  resetLabel,
  lockedLabel,
  visibleCountLabel,
  emptyLabel,
  items,
  onToggle,
  onShowAll,
  onReset,
}: {
  searchLabel: string;
  showAllLabel: string;
  resetLabel: string;
  lockedLabel: string;
  visibleCountLabel: (visible: number, total: number) => string;
  emptyLabel: string;
  items: ColumnPickerItem[];
  onToggle: (key: string, visible: boolean) => void;
  onShowAll: () => void;
  onReset: () => void;
}) {
  const [query, setQuery] = useState("");

  const groups = useMemo(() => {
    const needle = query.trim().toLowerCase();
    const matching = needle
      ? items.filter((item) => item.label.toLowerCase().includes(needle))
      : items;
    const byGroup = new Map<string, ColumnPickerItem[]>();
    for (const item of matching) {
      const bucket = byGroup.get(item.group);
      if (bucket) bucket.push(item);
      else byGroup.set(item.group, [item]);
    }
    return [...byGroup.entries()];
  }, [items, query]);

  const visibleCount = items.filter((item) => item.visible).length;

  return (
    <div className="column-picker">
      <Input
        allowClear
        aria-label={searchLabel}
        className="column-picker__search"
        placeholder={searchLabel}
        prefix={<Search aria-hidden size={15} strokeWidth={1.75} />}
        value={query}
        onChange={(event) => setQuery(event.target.value)}
      />

      <p className="column-picker__count">{visibleCountLabel(visibleCount, items.length)}</p>

      {groups.length === 0 ? (
        <Empty description={emptyLabel} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <div className="column-picker__groups">
          {groups.map(([group, groupItems]) => (
            <section className="column-picker__group" key={group}>
              <h3 className="column-picker__group-title">
                {group}
                <span>{groupItems.filter((item) => item.visible).length}</span>
              </h3>
              <ul className="column-picker__list">
                {groupItems.map((item) => (
                  <li className="column-picker__item" key={item.key}>
                    <Checkbox
                      aria-label={item.locked ? `${item.label} (${lockedLabel})` : item.label}
                      checked={item.visible}
                      disabled={item.locked}
                      onChange={(event) => onToggle(item.key, event.target.checked)}
                    >
                      <span className="column-picker__label">{item.label}</span>
                      {item.locked ? (
                        <LockKeyhole
                          aria-hidden
                          className="column-picker__locked"
                          size={13}
                          strokeWidth={1.75}
                        />
                      ) : null}
                    </Checkbox>
                  </li>
                ))}
              </ul>
            </section>
          ))}
        </div>
      )}

      <div className="column-picker__footer">
        <Button size="small" type="text" onClick={onShowAll}>
          {showAllLabel}
        </Button>
        <Button size="small" type="text" onClick={onReset}>
          {resetLabel}
        </Button>
      </div>
    </div>
  );
}

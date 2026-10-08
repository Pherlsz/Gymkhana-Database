import { Button, Checkbox, Empty } from "antd";
import type { CheckboxChangeEvent } from "antd/es/checkbox";
import { LockKeyhole } from "lucide-react";
import { memo, useCallback, useDeferredValue, useEffect, useMemo, useState } from "react";
import { ICON, ICON_STROKE } from "../../components/icons";
import { SearchField } from "../../components/SearchField";

export type ColumnPickerItem = {
  key: string;
  label: string;
  group: string;
  visible: boolean;
  locked: boolean;
};

type ColumnPickerRowProps = {
  itemKey: string;
  label: string;
  visible: boolean;
  locked: boolean;
  lockedLabel: string;
  onToggle: (key: string, visible: boolean) => void;
};

const ColumnPickerRow = memo(function ColumnPickerRow({
  itemKey,
  label,
  visible,
  locked,
  lockedLabel,
  onToggle,
}: ColumnPickerRowProps) {
  const handleChange = useCallback(
    (event: CheckboxChangeEvent) => {
      onToggle(itemKey, event.target.checked);
    },
    [itemKey, onToggle],
  );

  return (
    <li
      className={`column-picker__item ${visible ? "is-selected" : ""} ${locked ? "is-locked" : ""}`}
      key={itemKey}
    >
      <Checkbox
        aria-label={locked ? `${label} (${lockedLabel})` : label}
        checked={visible}
        disabled={locked}
        onChange={handleChange}
      >
        <span className="column-picker__label">{label}</span>
        {locked ? (
          <LockKeyhole
            aria-hidden
            className="column-picker__locked"
            size={ICON.sm}
            strokeWidth={ICON_STROKE}
          />
        ) : null}
      </Checkbox>
    </li>
  );
});

export function ColumnPicker({
  searchLabel,
  showAllLabel,
  hideAllLabel,
  resetLabel,
  lockedLabel,
  visibleCountLabel,
  emptyLabel,
  items,
  onToggle,
  onShowAll,
  onHideAll,
  onReset,
}: {
  searchLabel: string;
  showAllLabel: string;
  hideAllLabel: string;
  resetLabel: string;
  lockedLabel: string;
  visibleCountLabel: (visible: number, total: number) => string;
  emptyLabel: string;
  items: ColumnPickerItem[];
  onToggle: (key: string, visible: boolean) => void;
  onShowAll: (keys: string[]) => void;
  onHideAll: (keys: string[]) => void;
  onReset: () => void;
}) {
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);

  const [localVisibility, setLocalVisibility] = useState<Record<string, boolean>>({});

  useEffect(() => {
    setLocalVisibility({});
  }, [items]);

  const handleToggle = useCallback(
    (key: string, visible: boolean) => {
      setLocalVisibility((prev) => ({ ...prev, [key]: visible }));
      onToggle(key, visible);
    },
    [onToggle],
  );

  const matchingKeys = useCallback(() => {
    const needle = query.trim().toLowerCase();
    const matching = needle
      ? items.filter((item) => item.label.toLowerCase().includes(needle))
      : items;
    return matching.map((item) => item.key);
  }, [items, query]);

  const handleShowAll = useCallback(() => {
    setLocalVisibility({});
    onShowAll(matchingKeys());
  }, [matchingKeys, onShowAll]);

  const handleHideAll = useCallback(() => {
    setLocalVisibility({});
    onHideAll(matchingKeys());
  }, [matchingKeys, onHideAll]);

  const handleReset = useCallback(() => {
    setLocalVisibility({});
    onReset();
  }, [onReset]);

  const groups = useMemo(() => {
    const needle = deferredQuery.trim().toLowerCase();
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
  }, [items, deferredQuery]);

  const visibleCount = useMemo(() => {
    return items.reduce((acc, item) => {
      const isVisible = localVisibility[item.key] ?? item.visible;
      return isVisible ? acc + 1 : acc;
    }, 0);
  }, [items, localVisibility]);

  return (
    <div className="column-picker">
      <SearchField
        label={searchLabel}
        mode="local"
        placeholder={searchLabel}
        value={query}
        onChange={setQuery}
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
                <span>
                  {groupItems.filter((item) => localVisibility[item.key] ?? item.visible).length}
                </span>
              </h3>
              <ul className="column-picker__list">
                {groupItems.map((item) => (
                  <ColumnPickerRow
                    itemKey={item.key}
                    key={item.key}
                    label={item.label}
                    locked={item.locked}
                    lockedLabel={lockedLabel}
                    visible={localVisibility[item.key] ?? item.visible}
                    onToggle={handleToggle}
                  />
                ))}
              </ul>
            </section>
          ))}
        </div>
      )}

      <div className="column-picker__footer">
        <Button size="small" type="text" onClick={handleShowAll}>
          {showAllLabel}
        </Button>
        <Button size="small" type="text" onClick={handleHideAll}>
          {hideAllLabel}
        </Button>
        <Button size="small" type="text" onClick={handleReset}>
          {resetLabel}
        </Button>
      </div>
    </div>
  );
}

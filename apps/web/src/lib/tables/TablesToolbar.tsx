import { Button, Input, Tag } from "antd";
import { Columns3, SlidersHorizontal } from "lucide-react";
import { useState } from "react";
import { ColumnPicker, type ColumnPickerItem } from "./ColumnPicker";
import { FilterSurface } from "./FilterSurface";
import { ToolbarSurface } from "./ToolbarSurface";
import type { ToolbarFilterField } from "./FilterControl";

export type { ToolbarFilterField } from "./FilterControl";

export type ToolbarChip = {
  key: string;
  /** The column name. Chips read "Campo: valor" so a screen reader hears both. */
  field: string;
  value: string;
  onClear: () => void;
};

/** Chips beyond this collapse behind a `+N` button, so the bar stays one row. */
const CHIP_LIMIT = 4;

export function TablesToolbar({
  searchLabel,
  searchPlaceholder,
  searchValue,
  onSearchChange,
  onSearchSubmit,
  fieldFiltersLabel,
  searchFieldsLabel,
  appliedFiltersLabel,
  noFieldsLabel,
  moreChipsLabel,
  chips,
  clearLabel,
  onClearAll,
  localHint,
  filters,
  columnPicker,
}: {
  searchLabel: string;
  searchPlaceholder: string;
  searchValue: string;
  onSearchChange: (value: string) => void;
  onSearchSubmit: (value: string) => void;
  fieldFiltersLabel: string;
  searchFieldsLabel: string;
  appliedFiltersLabel: string;
  noFieldsLabel: string;
  moreChipsLabel: (count: number) => string;
  chips: ToolbarChip[];
  clearLabel: string;
  onClearAll: () => void;
  localHint: string;
  filters: ToolbarFilterField[];
  columnPicker?:
    | {
        label: string;
        title: string;
        searchLabel: string;
        showAllLabel: string;
        resetLabel: string;
        lockedLabel: string;
        emptyLabel: string;
        visibleCountLabel: (visible: number, total: number) => string;
        hiddenCount: number;
        items: ColumnPickerItem[];
        onToggle: (key: string, visible: boolean) => void;
        onShowAll: () => void;
        onReset: () => void;
      }
    | undefined;
}) {
  const [openSurface, setOpenSurface] = useState<"columns" | "filters" | null>(null);
  const [allChips, setAllChips] = useState(false);
  const filterCount = filters.filter((field) => field.value.trim()).length;
  const shownChips = allChips ? chips : chips.slice(0, CHIP_LIMIT);
  const hiddenChips = chips.length - shownChips.length;

  return (
    <div className="tables-toolbar">
      <div className="tables-toolbar__row">
        <Input.Search
          allowClear
          aria-label={searchLabel}
          placeholder={searchPlaceholder}
          value={searchValue}
          onBlur={(event) => {
            const search = event.currentTarget.closest(".ant-input-search");
            if (event.relatedTarget instanceof Node && search?.contains(event.relatedTarget)) {
              return;
            }
            onSearchSubmit(searchValue);
          }}
          onChange={(event) => onSearchChange(event.target.value)}
          onSearch={onSearchSubmit}
        />
        {columnPicker ? (
          <ToolbarSurface
            active={columnPicker.hiddenCount > 0}
            count={columnPicker.hiddenCount}
            icon={<Columns3 aria-hidden size={16} strokeWidth={1.75} />}
            label={columnPicker.label}
            open={openSurface === "columns"}
            title={columnPicker.title}
            onOpenChange={(open) => setOpenSurface(open ? "columns" : null)}
          >
            <ColumnPicker
              emptyLabel={columnPicker.emptyLabel}
              items={columnPicker.items}
              lockedLabel={columnPicker.lockedLabel}
              resetLabel={columnPicker.resetLabel}
              searchLabel={columnPicker.searchLabel}
              showAllLabel={columnPicker.showAllLabel}
              visibleCountLabel={columnPicker.visibleCountLabel}
              onReset={columnPicker.onReset}
              onShowAll={columnPicker.onShowAll}
              onToggle={columnPicker.onToggle}
            />
          </ToolbarSurface>
        ) : null}
        {filters.length > 0 ? (
          <ToolbarSurface
            active={filterCount > 0}
            count={filterCount}
            icon={<SlidersHorizontal aria-hidden size={16} strokeWidth={1.75} />}
            label={fieldFiltersLabel}
            open={openSurface === "filters"}
            title={fieldFiltersLabel}
            onOpenChange={(open) => setOpenSurface(open ? "filters" : null)}
          >
            <FilterSurface
              appliedLabel={appliedFiltersLabel}
              clearLabel={clearLabel}
              fields={filters}
              localHint={localHint}
              noFieldsLabel={noFieldsLabel}
              searchFieldsLabel={searchFieldsLabel}
            />
          </ToolbarSurface>
        ) : null}
      </div>

      {chips.length > 0 ? (
        <div aria-label={appliedFiltersLabel} className="tables-toolbar__chips" role="group">
          {shownChips.map((chip) => (
            <Tag
              className="tables-toolbar__chip"
              closable
              key={chip.key}
              onClose={(event) => {
                event.preventDefault();
                chip.onClear();
              }}
            >
              <span className="tables-toolbar__chip-field">{chip.field}</span>
              <span className="tables-toolbar__chip-value">{chip.value}</span>
            </Tag>
          ))}
          {hiddenChips > 0 ? (
            <Button size="small" type="text" onClick={() => setAllChips(true)}>
              {moreChipsLabel(hiddenChips)}
            </Button>
          ) : null}
          <Button size="small" type="link" onClick={onClearAll}>
            {clearLabel}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

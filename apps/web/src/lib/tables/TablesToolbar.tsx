import { Button, Tag } from "antd";
import { Columns3, SlidersHorizontal } from "lucide-react";
import { useState } from "react";
import { SearchField } from "../../components/SearchField";
import { ColumnPicker, type ColumnPickerItem } from "./ColumnPicker";
import { FilterSurface } from "./FilterSurface";
import { ToolbarSurface } from "./ToolbarSurface";
import type { ToolbarFilterField } from "./FilterControl";
import { ICON, ICON_STROKE } from "../../components/icons";

export type { ToolbarFilterField } from "./FilterControl";

export type ToolbarChip = {
  key: string;
  /** The column name. Chips read "Campo: valor" so a screen reader hears both. */
  field: string;
  value: string;
  /** True when the filter only sees the loaded page, so its scope is flagged. */
  local?: boolean | undefined;
  onClear: () => void;
};

/** Chips beyond this collapse behind a `+N` button, so the bar stays one row. */
const CHIP_LIMIT = 4;

export function TablesToolbar({
  searchLabel,
  searchPlaceholder,
  searchValue,
  searchGrain,
  onSearchChange,
  onSearchSubmit,
  fieldFiltersLabel,
  addFilterLabel,
  chooseFieldLabel,
  chooseValueLabel,
  removeFilterLabel,
  appliedFiltersLabel,
  noFieldsLabel,
  moreChipsLabel,
  moreChipsCollapseLabel,
  chips,
  clearLabel,
  onClearAll,
  localHint,
  localScopeBadge,
  filters,
  columnPicker,
}: {
  searchLabel: string;
  searchPlaceholder: string;
  searchValue: string;
  searchGrain: "profiles" | "documents" | "bills";
  onSearchChange: (value: string) => void;
  onSearchSubmit: (value: string) => void;
  fieldFiltersLabel: string;
  addFilterLabel: string;
  chooseFieldLabel: string;
  chooseValueLabel: string;
  removeFilterLabel: string;
  appliedFiltersLabel: string;
  noFieldsLabel: string;
  moreChipsLabel: (count: number) => string;
  moreChipsCollapseLabel: string;
  chips: ToolbarChip[];
  clearLabel: string;
  onClearAll: () => void;
  localHint: string;
  localScopeBadge: string;
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
        <SearchField
          grain={searchGrain}
          label={searchLabel}
          mode="suggest"
          placeholder={searchPlaceholder}
          value={searchValue}
          onChange={onSearchChange}
          onSubmit={onSearchSubmit}
        />
        {columnPicker ? (
          <ToolbarSurface
            active={columnPicker.hiddenCount > 0}
            count={columnPicker.hiddenCount}
            icon={<Columns3 aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
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
            icon={<SlidersHorizontal aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
            label={fieldFiltersLabel}
            open={openSurface === "filters"}
            title={fieldFiltersLabel}
            onOpenChange={(open) => setOpenSurface(open ? "filters" : null)}
          >
            <FilterSurface
              addFilterLabel={addFilterLabel}
              appliedLabel={appliedFiltersLabel}
              chooseFieldLabel={chooseFieldLabel}
              chooseValueLabel={chooseValueLabel}
              clearLabel={clearLabel}
              fields={filters}
              localHint={localHint}
              noFieldsLabel={noFieldsLabel}
              removeFilterLabel={removeFilterLabel}
            />
          </ToolbarSurface>
        ) : null}
      </div>

      {chips.length > 0 ? (
        <div aria-label={appliedFiltersLabel} className="tables-toolbar__chips" role="group">
          {shownChips.map((chip) => (
            <Tag
              className={chip.local ? "tables-toolbar__chip is-local" : "tables-toolbar__chip"}
              closable
              key={chip.key}
              onClose={(event) => {
                event.preventDefault();
                chip.onClear();
              }}
            >
              {/* The separator is a real text node: a CSS ::after colon is not
                  reliably exposed, which left the chip reading "PessoaAna". */}
              <span className="tables-toolbar__chip-field">{chip.field}:</span>{" "}
              <span className="tables-toolbar__chip-value">{chip.value}</span>
              {chip.local ? (
                <span className="tables-toolbar__chip-local" title={localHint}>
                  {localScopeBadge}
                </span>
              ) : null}
            </Tag>
          ))}
          {hiddenChips > 0 ? (
            <Button size="small" type="text" onClick={() => setAllChips(true)}>
              {moreChipsLabel(hiddenChips)}
            </Button>
          ) : allChips && chips.length > CHIP_LIMIT ? (
            <Button size="small" type="text" onClick={() => setAllChips(false)}>
              {moreChipsCollapseLabel}
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

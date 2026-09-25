import { Button, Tag } from "antd";
import { Columns3, Sparkles } from "lucide-react";
import { useState } from "react";
import { SearchField, type SearchFieldMode } from "../../components/SearchField";
import { ColumnPicker, type ColumnPickerItem } from "./ColumnPicker";
import { ToolbarSurface } from "./ToolbarSurface";
import { ICON, ICON_STROKE } from "../../components/icons";

export type { ToolbarFilterField } from "./FilterControl";

export type ToolbarChip = {
  key: string;
  field: string;
  value: string;
  local?: boolean | undefined;
  onClear: () => void;
};

const CHIP_LIMIT = 4;

export function TablesToolbar({
  searchLabel,
  searchPlaceholder,
  searchValue,
  searchGrain,
  searchMode = "suggest",
  onSearchChange,
  onSearchSubmit,
  appliedFiltersLabel,
  moreChipsLabel,
  moreChipsCollapseLabel,
  chips,
  clearLabel,
  onClearAll,
  localHint,
  localScopeBadge,
  columnPicker,
  assistant,
}: {
  searchLabel: string;
  searchPlaceholder: string;
  searchValue: string;
  searchGrain: "profiles" | "documents" | "bills";
  searchMode?: SearchFieldMode | undefined;
  onSearchChange: (value: string) => void;
  onSearchSubmit: (value: string) => void;
  appliedFiltersLabel: string;
  moreChipsLabel: (count: number) => string;
  moreChipsCollapseLabel: string;
  chips: ToolbarChip[];
  clearLabel: string;
  onClearAll: () => void;
  localHint: string;
  localScopeBadge: string;
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
  assistant?:
    | {
        label: string;
        active: boolean;
        onClick: () => void;
      }
    | undefined;
}) {
  const [openColumns, setOpenColumns] = useState(false);
  const [allChips, setAllChips] = useState(false);
  const shownChips = allChips ? chips : chips.slice(0, CHIP_LIMIT);
  const hiddenChips = chips.length - shownChips.length;

  return (
    <div className="tables-toolbar">
      <div className="tables-toolbar__row">
        <SearchField
          grain={searchGrain}
          label={searchLabel}
          mode={searchMode}
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
            open={openColumns}
            title={columnPicker.title}
            onOpenChange={setOpenColumns}
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
        {assistant ? (
          <Button
            aria-label={assistant.label}
            className={
              assistant.active
                ? "toolbar-surface__trigger is-ai is-active"
                : "toolbar-surface__trigger is-ai"
            }
            icon={<Sparkles aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
            title={assistant.label}
            onClick={assistant.onClick}
          />
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
              <span className="tables-toolbar__chip-field">{chip.field}:</span>{" "}
              <span className="tables-toolbar__chip-value">{chip.value}</span>
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
          <Button className="tables-toolbar__clear" size="small" type="text" onClick={onClearAll}>
            {clearLabel}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

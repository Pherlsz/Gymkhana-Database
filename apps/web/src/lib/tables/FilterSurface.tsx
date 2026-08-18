import { Button, Empty, Input, Select } from "antd";
import { Plus, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { FilterControl, type ToolbarFilterField } from "./FilterControl";
import { ICON, ICON_STROKE } from "../../components/icons";

type PendingRow = { id: string; fieldKey: string | null };

function hasValue(field: ToolbarFilterField): boolean {
  return field.value.trim().length > 0;
}

/**
 * Conditional filter builder: pick a column, then a value, then add another
 * row. Applied rows stay in catalog order so turning a filter on does not
 * jump it to the top of the sheet.
 */
export function FilterSurface({
  addFilterLabel,
  chooseFieldLabel,
  chooseValueLabel,
  removeFilterLabel,
  noFieldsLabel,
  appliedLabel,
  clearLabel,
  localHint,
  fields,
}: {
  addFilterLabel: string;
  chooseFieldLabel: string;
  chooseValueLabel: string;
  removeFilterLabel: string;
  noFieldsLabel: string;
  appliedLabel: string;
  clearLabel: string;
  localHint: string;
  fields: ToolbarFilterField[];
}) {
  const draftSeq = useRef(0);
  const nextDraftId = () => {
    draftSeq.current += 1;
    return `draft-${draftSeq.current}`;
  };

  const [pending, setPending] = useState<PendingRow[]>(() =>
    fields.some(hasValue) ? [] : [{ id: "draft-0", fieldKey: null }],
  );

  const applied = useMemo(() => fields.filter(hasValue), [fields]);

  useEffect(() => {
    const appliedKeys = new Set(applied.map((field) => field.key));
    setPending((current) => {
      const pruned = current.filter(
        (row) => row.fieldKey === null || !appliedKeys.has(row.fieldKey),
      );
      if (applied.length === 0 && pruned.length === 0) {
        return [{ id: nextDraftId(), fieldKey: null }];
      }
      return pruned.length === current.length ? current : pruned;
    });
  }, [applied]);

  const pendingShown = pending.filter(
    (row) => row.fieldKey === null || !applied.some((field) => field.key === row.fieldKey),
  );
  const usedKeys = new Set([
    ...applied.map((field) => field.key),
    ...pendingShown.flatMap((row) => (row.fieldKey ? [row.fieldKey] : [])),
  ]);
  const unusedCount = fields.filter((field) => !usedKeys.has(field.key)).length;
  const hasEmptyPending = pendingShown.some((row) => row.fieldKey === null);
  const canAdd = unusedCount > 0 && !hasEmptyPending;
  const showsLocalHint = fields.some(
    (field) =>
      field.local && (hasValue(field) || pendingShown.some((row) => row.fieldKey === field.key)),
  );

  const fieldOptionsFor = (currentKey: string | null) =>
    fields
      .filter((field) => field.key === currentKey || !usedKeys.has(field.key))
      .map((field) => ({ value: field.key, label: field.label }));

  const clearField = (key: string | null) => {
    if (!key) return;
    const field = fields.find((item) => item.key === key);
    if (field && hasValue(field)) field.onChange("");
  };

  const pickAppliedField = (field: ToolbarFilterField, nextKey: string | null) => {
    field.onChange("");
    setPending((current) => [...current, { id: nextDraftId(), fieldKey: nextKey }]);
  };

  const pickPendingField = (row: PendingRow, nextKey: string | null) => {
    if (row.fieldKey && row.fieldKey !== nextKey) clearField(row.fieldKey);
    setPending((current) =>
      current.map((item) => (item.id === row.id ? { ...item, fieldKey: nextKey } : item)),
    );
  };

  const removePending = (row: PendingRow) => {
    clearField(row.fieldKey);
    setPending((current) => {
      const next = current.filter((item) => item.id !== row.id);
      if (applied.length === 0 && next.length === 0) {
        return [{ id: nextDraftId(), fieldKey: null }];
      }
      return next;
    });
  };

  const rows = [
    ...applied.map((field) => ({ kind: "applied" as const, id: field.key, field })),
    ...pendingShown.map((row) => ({ kind: "pending" as const, ...row })),
  ];
  const canRemoveRow = (fieldKey: string | null) =>
    applied.length + pendingShown.length > 1 || Boolean(fieldKey);

  return (
    <div className="filter-surface">
      {rows.length === 0 ? (
        <Empty description={noFieldsLabel} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <div aria-label={appliedLabel} className="filter-surface__list" role="group">
          {rows.map((row) => {
            const fieldKey = row.kind === "applied" ? row.field.key : row.fieldKey;
            const field =
              row.kind === "applied" ? row.field : fields.find((item) => item.key === row.fieldKey);
            const appliedRow = Boolean(field && hasValue(field));
            return (
              <div
                className={appliedRow ? "filter-surface__row is-applied" : "filter-surface__row"}
                key={row.id}
              >
                <Select
                  allowClear
                  aria-label={chooseFieldLabel}
                  className="filter-surface__field-select"
                  optionFilterProp="label"
                  options={fieldOptionsFor(fieldKey)}
                  placeholder={chooseFieldLabel}
                  popupMatchSelectWidth
                  showSearch
                  value={fieldKey ?? undefined}
                  onChange={(value) => {
                    const nextKey = value ?? null;
                    if (row.kind === "applied") pickAppliedField(row.field, nextKey);
                    else pickPendingField(row, nextKey);
                  }}
                />
                <div className="filter-surface__control">
                  {field ? (
                    <FilterControl autoFocus={!hasValue(field)} field={field} />
                  ) : (
                    <Input disabled placeholder={chooseValueLabel} />
                  )}
                </div>
                <Button
                  aria-label={removeFilterLabel}
                  className="filter-surface__remove"
                  disabled={!canRemoveRow(fieldKey)}
                  icon={<X aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />}
                  type="text"
                  onClick={() => {
                    if (row.kind === "applied") {
                      row.field.onChange("");
                      return;
                    }
                    removePending(row);
                  }}
                />
              </div>
            );
          })}
        </div>
      )}

      <div className="filter-surface__footer">
        <Button
          disabled={!canAdd}
          icon={<Plus aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />}
          type="dashed"
          onClick={() =>
            setPending((current) => [...current, { id: nextDraftId(), fieldKey: null }])
          }
        >
          {addFilterLabel}
        </Button>
        {showsLocalHint ? <p className="filter-surface__hint">{localHint}</p> : null}
        {applied.length > 0 ? (
          <Button
            size="small"
            type="text"
            onClick={() => {
              for (const field of fields) {
                if (hasValue(field)) field.onChange("");
              }
              setPending([{ id: nextDraftId(), fieldKey: null }]);
            }}
          >
            {clearLabel}
          </Button>
        ) : null}
      </div>
    </div>
  );
}
